package service

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const folderSplitterExecutorType = "folder-splitter"

type folderSplitterNodeExecutor struct{}

func newFolderSplitterExecutor() *folderSplitterNodeExecutor {
	return &folderSplitterNodeExecutor{}
}

func NewFolderSplitterExecutor() WorkflowNodeExecutor {
	return newFolderSplitterExecutor()
}

func (e *folderSplitterNodeExecutor) Type() string {
	return folderSplitterExecutorType
}

func (e *folderSplitterNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "文件夹拆分器",
		Description: "将分类条目转为处理项列表；支持递归拆分到叶子目录，确保后续处理流聚焦单个目录",
		Inputs: []PortDef{
			{Name: "entry", Type: PortTypeJSON, Description: "已分类条目", Required: true},
		},
		Outputs: []PortDef{
			{Name: "items", Type: PortTypeProcessingItemList, RequiredOutput: true, Description: "拆分后的处理项列表"},
		},
	}
}

func (e *folderSplitterNodeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	rawInputs := typedInputsToAny(input.Inputs)
	entry, ok := classificationReaderResolveInputEntry(rawInputs)
	if !ok {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: entry is required", e.Type())
	}

	splitMixed := folderSplitterBoolConfig(input.Node.Config, "split_mixed", true)
	splitWithSubdirs := folderSplitterBoolConfig(input.Node.Config, "split_with_subdirs", true)
	splitDepth := intConfig(input.Node.Config, "split_depth", 1)
	if splitWithSubdirs {
		splitDepth = math.MaxInt32
	}

	rootPath := normalizeWorkflowPath(entry.Path)
	items := []ProcessingItem{folderSplitterBuildSelfItem(entry, rootPath)}
	shouldSplit := (splitWithSubdirs && len(entry.Subtree) > 0) ||
		(strings.EqualFold(entry.Category, "mixed") && splitMixed && splitDepth > 0)
	if shouldSplit {
		splitItems := folderSplitterBuildRecursiveItems(entry, rootPath, splitDepth, splitWithSubdirs)
		if len(splitItems) > 0 {
			items = splitItems
		}
	}

	return NodeExecutionOutput{Outputs: map[string]TypedValue{"items": {Type: PortTypeProcessingItemList, Value: items}}, Status: ExecutionSuccess}, nil
}

func (e *folderSplitterNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *folderSplitterNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func folderSplitterBoolConfig(config map[string]any, key string, fallback bool) bool {
	if config == nil {
		return fallback
	}

	raw, ok := config[key]
	if !ok {
		return fallback
	}

	switch value := raw.(type) {
	case bool:
		return value
	case string:
		trimmed := strings.TrimSpace(strings.ToLower(value))
		if trimmed == "true" {
			return true
		}
		if trimmed == "false" {
			return false
		}
	}

	return fallback
}

func folderSplitterBuildSelfItem(entry ClassifiedEntry, rootPath string) ProcessingItem {
	normalizedPath := normalizeWorkflowPath(entry.Path)
	normalizedRoot := normalizeWorkflowPath(rootPath)
	relativePath := folderSplitterRelativePath(normalizedRoot, normalizedPath)
	return ProcessingItem{
		SourcePath:         normalizedPath,
		CurrentPath:        normalizedPath,
		FolderID:           entry.FolderID,
		FolderName:         entry.Name,
		TargetName:         entry.Name,
		Category:           entry.Category,
		Files:              append([]FileEntry(nil), entry.Files...),
		ParentPath:         normalizeWorkflowPath(filepath.Dir(normalizedPath)),
		RootPath:           normalizedRoot,
		RelativePath:       relativePath,
		SourceKind:         ProcessingItemSourceKindDirectory,
		OriginalSourcePath: normalizedPath,
	}
}

func folderSplitterBuildRecursiveItems(entry ClassifiedEntry, rootPath string, splitDepth int, splitWithSubdirs bool) []ProcessingItem {
	if len(entry.Subtree) == 0 || splitDepth <= 0 {
		return nil
	}

	collected := make([]ProcessingItem, 0)
	folderSplitterCollectItems(entry, rootPath, splitDepth, splitWithSubdirs, &collected)
	sort.Slice(collected, func(i, j int) bool {
		if collected[i].SourcePath == collected[j].SourcePath {
			return collected[i].FolderName < collected[j].FolderName
		}
		return collected[i].SourcePath < collected[j].SourcePath
	})

	return collected
}

func folderSplitterCollectItems(entry ClassifiedEntry, rootPath string, depth int, splitWithSubdirs bool, out *[]ProcessingItem) {
	if depth <= 0 || len(entry.Subtree) == 0 {
		*out = append(*out, folderSplitterBuildSelfItem(entry, rootPath))
		return
	}

	for _, child := range entry.Subtree {
		if !folderSplitterIsDirectChild(entry, child) {
			continue
		}
		if splitWithSubdirs && len(child.Subtree) > 0 {
			folderSplitterCollectItems(child, rootPath, depth-1, splitWithSubdirs, out)
			continue
		}
		if !splitWithSubdirs && len(child.Subtree) > 0 && depth-1 > 0 {
			folderSplitterCollectItems(child, rootPath, depth-1, splitWithSubdirs, out)
			continue
		}
		*out = append(*out, folderSplitterBuildSelfItem(child, rootPath))
	}
}

func folderSplitterRelativePath(rootPath, sourcePath string) string {
	normalizedRoot := normalizeWorkflowPath(rootPath)
	normalizedSource := normalizeWorkflowPath(sourcePath)
	if normalizedRoot == "" || normalizedSource == "" {
		return ""
	}
	rel, err := filepath.Rel(normalizedRoot, normalizedSource)
	if err != nil || rel == "." || rel == "" || strings.HasPrefix(rel, "..") {
		return ""
	}
	return normalizeWorkflowPath(rel)
}

func folderSplitterIsDirectChild(entry ClassifiedEntry, child ClassifiedEntry) bool {
	if entry.Path != "" && child.Path != "" {
		rel, err := filepath.Rel(entry.Path, child.Path)
		if err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return !strings.ContainsRune(rel, os.PathSeparator)
		}
	}

	return false
}
