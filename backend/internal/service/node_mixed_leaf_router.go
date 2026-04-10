package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/liqiye/classifier/internal/fs"
)

const mixedLeafRouterExecutorType = "mixed-leaf-router"

const (
	mixedLeafRouterVideoPort       = "video"
	mixedLeafRouterPhotoPort       = "photo"
	mixedLeafRouterUnsupportedPort = "unsupported"
)

var mixedLeafRouterPortOrder = []string{
	mixedLeafRouterVideoPort,
	mixedLeafRouterPhotoPort,
	mixedLeafRouterUnsupportedPort,
}

var mixedLeafRouterStagingDirs = map[string]string{
	mixedLeafRouterVideoPort:       "__video",
	mixedLeafRouterPhotoPort:       "__photo",
	mixedLeafRouterUnsupportedPort: "__unsupported",
}

type mixedLeafRouterNodeExecutor struct {
	fs fs.FSAdapter
}

func newMixedLeafRouterExecutor(fsAdapter fs.FSAdapter) *mixedLeafRouterNodeExecutor {
	return &mixedLeafRouterNodeExecutor{fs: fsAdapter}
}

func NewMixedLeafRouterExecutor(fsAdapter fs.FSAdapter) WorkflowNodeExecutor {
	return newMixedLeafRouterExecutor(fsAdapter)
}

func (e *mixedLeafRouterNodeExecutor) Type() string {
	return mixedLeafRouterExecutorType
}

func (e *mixedLeafRouterNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "混合叶子分流器",
		Description: "消费 mixed_leaf 目录并原地拆分为 video/photo/unsupported 三路目录处理项",
		Inputs: []PortDef{
			{Name: "items", Type: PortTypeProcessingItemList, Description: "mixed 叶子目录处理项列表", Required: true},
		},
		Outputs: []PortDef{
			{Name: mixedLeafRouterVideoPort, Type: PortTypeProcessingItemList, AllowEmpty: true, Description: "视频目录处理项"},
			{Name: mixedLeafRouterPhotoPort, Type: PortTypeProcessingItemList, AllowEmpty: true, Description: "图片目录处理项"},
			{Name: mixedLeafRouterUnsupportedPort, Type: PortTypeProcessingItemList, AllowEmpty: true, Description: "不支持文件目录处理项"},
		},
	}
}

func (e *mixedLeafRouterNodeExecutor) Execute(ctx context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	items, ok := categoryRouterExtractItems(input.Inputs)
	if !ok {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: items input is required", e.Type())
	}

	outputs := mixedLeafRouterEmptyOutputs()
	if len(items) == 0 {
		return NodeExecutionOutput{Outputs: outputs, Status: ExecutionSuccess}, nil
	}

	for _, rawItem := range items {
		item := processingItemNormalize(rawItem)
		if !strings.EqualFold(strings.TrimSpace(item.Category), "mixed") {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: item category must be mixed, got %q", e.Type(), item.Category)
		}

		mixedRoot := processingItemCurrentPath(item)
		if strings.TrimSpace(mixedRoot) == "" {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: current_path is required for mixed item", e.Type())
		}

		info, err := e.fs.Stat(ctx, mixedRoot)
		if err != nil {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: stat mixed root %q: %w", e.Type(), mixedRoot, err)
		}
		if !info.IsDir {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: mixed current_path %q must be a directory", e.Type(), mixedRoot)
		}

		entries, err := e.fs.ReadDir(ctx, mixedRoot)
		if err != nil {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: read mixed root %q: %w", e.Type(), mixedRoot, err)
		}

		for _, entry := range entries {
			if !entry.IsDir {
				continue
			}
			if mixedLeafRouterIsInternalStagingDir(entry.Name) {
				continue
			}
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: mixed root %q contains business subdirectory %q", e.Type(), mixedRoot, entry.Name)
		}

		for _, entry := range entries {
			if entry.IsDir {
				continue
			}

			portName := mixedLeafRouterClassifyPort(entry.Name)
			stagePath := mixedLeafRouterStagePath(mixedRoot, portName)
			if err := e.fs.MkdirAll(ctx, stagePath, 0o755); err != nil {
				return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: create staging dir %q: %w", e.Type(), stagePath, err)
			}

			sourcePath := joinWorkflowPath(mixedRoot, entry.Name)
			targetPath := joinWorkflowPath(stagePath, entry.Name)
			if err := e.fs.MoveFile(ctx, sourcePath, targetPath); err != nil {
				return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: move file %q to %q: %w", e.Type(), sourcePath, targetPath, err)
			}
		}

		for _, portName := range mixedLeafRouterPortOrder {
			stagePath := mixedLeafRouterStagePath(mixedRoot, portName)
			exists, err := e.fs.Exists(ctx, stagePath)
			if err != nil {
				return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: check staging dir %q: %w", e.Type(), stagePath, err)
			}
			if !exists {
				continue
			}

			stageEntries, err := e.fs.ReadDir(ctx, stagePath)
			if err != nil {
				return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: read staging dir %q: %w", e.Type(), stagePath, err)
			}
			hasFiles := false
			for _, stageEntry := range stageEntries {
				if !stageEntry.IsDir {
					hasFiles = true
					break
				}
			}
			if !hasFiles {
				continue
			}

			outputs[portName] = TypedValue{
				Type:  PortTypeProcessingItemList,
				Value: append(outputs[portName].Value.([]ProcessingItem), mixedLeafRouterBuildOutputItem(item, mixedRoot, stagePath, portName)),
			}
		}
	}

	return NodeExecutionOutput{Outputs: outputs, Status: ExecutionSuccess}, nil
}

func (e *mixedLeafRouterNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *mixedLeafRouterNodeExecutor) Rollback(ctx context.Context, input NodeRollbackInput) error {
	roots, err := mixedLeafRouterCollectRollbackRoots(input)
	if err != nil {
		return fmt.Errorf("%s.Rollback: %w", e.Type(), err)
	}

	for _, root := range roots {
		for _, portName := range mixedLeafRouterPortOrder {
			stagePath := mixedLeafRouterStagePath(root, portName)
			exists, err := e.fs.Exists(ctx, stagePath)
			if err != nil {
				return fmt.Errorf("check staging dir %q: %w", stagePath, err)
			}
			if !exists {
				continue
			}

			entries, err := e.fs.ReadDir(ctx, stagePath)
			if err != nil {
				return fmt.Errorf("read staging dir %q: %w", stagePath, err)
			}
			for _, entry := range entries {
				if entry.IsDir {
					continue
				}
				src := joinWorkflowPath(stagePath, entry.Name)
				dst := joinWorkflowPath(root, entry.Name)
				if err := e.fs.MoveFile(ctx, src, dst); err != nil {
					return fmt.Errorf("move file back %q to %q: %w", src, dst, err)
				}
			}

			if err := phase4MoveRemoveDirIfEmpty(ctx, e.fs, stagePath); err != nil {
				return err
			}
		}
	}

	return nil
}

func mixedLeafRouterEmptyOutputs() map[string]TypedValue {
	return map[string]TypedValue{
		mixedLeafRouterVideoPort:       {Type: PortTypeProcessingItemList, Value: []ProcessingItem{}},
		mixedLeafRouterPhotoPort:       {Type: PortTypeProcessingItemList, Value: []ProcessingItem{}},
		mixedLeafRouterUnsupportedPort: {Type: PortTypeProcessingItemList, Value: []ProcessingItem{}},
	}
}

func mixedLeafRouterIsInternalStagingDir(name string) bool {
	for _, dirName := range mixedLeafRouterStagingDirs {
		if name == dirName {
			return true
		}
	}
	return false
}

func mixedLeafRouterClassifyPort(fileName string) string {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(fileName)))
	if videoExtsSet[ext] {
		return mixedLeafRouterVideoPort
	}
	if imageExtsSet[ext] {
		return mixedLeafRouterPhotoPort
	}
	return mixedLeafRouterUnsupportedPort
}

func mixedLeafRouterStagePath(mixedRoot, portName string) string {
	return joinWorkflowPath(mixedRoot, mixedLeafRouterStagingDirs[portName])
}

func mixedLeafRouterBuildOutputItem(source ProcessingItem, mixedRoot, stagePath, portName string) ProcessingItem {
	folderName := strings.TrimSpace(source.FolderName)
	if folderName == "" {
		folderName = strings.TrimSpace(source.TargetName)
	}
	if folderName == "" {
		folderName = strings.TrimSpace(filepath.Base(mixedRoot))
	}

	return ProcessingItem{
		SourcePath:         stagePath,
		CurrentPath:        stagePath,
		FolderID:           "",
		FolderName:         folderName,
		TargetName:         folderName,
		Category:           portName,
		ParentPath:         mixedRoot,
		RootPath:           mixedRoot,
		RelativePath:       portName,
		SourceKind:         ProcessingItemSourceKindDirectory,
		OriginalSourcePath: mixedRoot,
	}
}

func mixedLeafRouterCollectRollbackRoots(input NodeRollbackInput) ([]string, error) {
	roots := map[string]struct{}{}
	collect := func(raw string, source string) error {
		if strings.TrimSpace(raw) == "" {
			return nil
		}

		typedOutputs, typed, err := parseTypedNodeOutputs(raw)
		if err != nil {
			return fmt.Errorf("parse %s output json: %w", source, err)
		}
		if !typed {
			return nil
		}

		for _, portName := range mixedLeafRouterPortOrder {
			typedValue, ok := typedOutputs[portName]
			if !ok {
				continue
			}
			items, ok := categoryRouterToItems(typedValue.Value)
			if !ok {
				return fmt.Errorf("parse %s port %q items", source, portName)
			}
			for _, item := range items {
				normalized := processingItemNormalize(item)
				root := strings.TrimSpace(normalized.RootPath)
				if root == "" {
					root = normalizeWorkflowPath(filepath.Dir(normalized.CurrentPath))
				}
				if root == "" {
					continue
				}
				roots[root] = struct{}{}
			}
		}
		return nil
	}

	if input.NodeRun != nil {
		if err := collect(input.NodeRun.OutputJSON, "node run"); err != nil {
			return nil, err
		}
	}
	for _, snapshot := range input.Snapshots {
		if snapshot == nil || snapshot.Kind != "post" {
			continue
		}
		if err := collect(snapshot.OutputJSON, fmt.Sprintf("snapshot %q", snapshot.ID)); err != nil {
			return nil, err
		}
	}

	ordered := make([]string, 0, len(roots))
	for root := range roots {
		ordered = append(ordered, root)
	}
	return ordered, nil
}
