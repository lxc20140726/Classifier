package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
)

const phase4MoveNodeExecutorType = "move-node"

type phase4MoveNodeExecutor struct {
	fs      fs.FSAdapter
	folders repository.FolderRepository
}

func newPhase4MoveNodeExecutor(fsAdapter fs.FSAdapter, folderRepo repository.FolderRepository) *phase4MoveNodeExecutor {
	return &phase4MoveNodeExecutor{fs: fsAdapter, folders: folderRepo}
}

func (e *phase4MoveNodeExecutor) Type() string {
	return phase4MoveNodeExecutorType
}

func (e *phase4MoveNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "移动节点",
		Description: "将处理项移动到目标目录，支持冲突策略和操作回滚",
		Inputs: []PortDef{
			{Name: "items", Type: PortTypeProcessingItemList, Description: "待移动的处理项列表", Required: true},
		},
		Outputs: []PortDef{
			{Name: "items", Type: PortTypeProcessingItemList, RequiredOutput: true, Description: "已移动的处理项列表"},
			{Name: "results", Type: PortTypeMoveResultList, RequiredOutput: true, Description: "移动操作结果列表"},
		},
	}
}

func (e *phase4MoveNodeExecutor) Execute(ctx context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	items, ok := categoryRouterExtractItems(input.Inputs)
	if !ok || len(items) == 0 {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: items input is required", e.Type())
	}

	targetDir := stringConfig(input.Node.Config, "target_dir")
	targetDir = normalizeWorkflowPath(targetDir)
	if targetDir == "" {
		targetDir = stringConfig(input.Node.Config, "targetDir")
	}
	if targetDir == "" {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: target_dir is required", e.Type())
	}

	moveUnit := strings.ToLower(strings.TrimSpace(stringConfig(input.Node.Config, "move_unit")))
	if moveUnit == "" {
		moveUnit = "folder"
	}
	if moveUnit != "folder" {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: unsupported move_unit %q, only folder is supported", e.Type(), moveUnit)
	}

	if !folderSplitterBoolConfig(input.Node.Config, "preserve_substructure", true) {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: preserve_substructure=false is not supported", e.Type())
	}

	createTarget := folderSplitterBoolConfig(input.Node.Config, "create_target_if_missing", true)
	targetExists, err := e.fs.Exists(ctx, targetDir)
	if err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: check target dir %q: %w", e.Type(), targetDir, err)
	}
	if !targetExists {
		if !createTarget {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: target dir %q does not exist and create_target_if_missing is false", e.Type(), targetDir)
		}
		if err := e.fs.MkdirAll(ctx, targetDir, 0o755); err != nil {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: create target dir %q: %w", e.Type(), targetDir, err)
		}
	}

	conflictPolicy := strings.ToLower(strings.TrimSpace(stringConfig(input.Node.Config, "conflict_policy")))
	if conflictPolicy == "" {
		conflictPolicy = "auto_rename"
	}
	if conflictPolicy != "auto_rename" && conflictPolicy != "skip" && conflictPolicy != "overwrite" {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: unsupported conflict_policy %q", e.Type(), conflictPolicy)
	}

	movedItems := make([]ProcessingItem, 0, len(items))
	results := make([]MoveResult, 0, len(items))
	for _, item := range items {
		itemName := phase4MoveItemName(item)
		if itemName == "" {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: target item name is empty", e.Type())
		}
		sourcePath := normalizeWorkflowPath(item.SourcePath)
		if sourcePath == "" {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: item source_path is required", e.Type())
		}

		destinationPath := joinWorkflowPath(targetDir, itemName)
		finalPath, skipped, err := e.resolveDestinationPath(ctx, destinationPath, conflictPolicy)
		if err != nil {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: resolve destination for %q: %w", e.Type(), sourcePath, err)
		}

		if skipped {
			results = append(results, MoveResult{SourcePath: sourcePath, TargetPath: normalizeWorkflowPath(destinationPath), Status: "skipped"})
			movedItems = append(movedItems, item)
			continue
		}

		if err := e.fs.MoveDir(ctx, sourcePath, finalPath); err != nil {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: move %q to %q: %w", e.Type(), sourcePath, finalPath, err)
		}

		if e.folders != nil && strings.TrimSpace(item.FolderID) != "" {
			if err := e.folders.UpdatePath(ctx, item.FolderID, finalPath); err != nil {
				return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: update folder path for %q: %w", e.Type(), item.FolderID, err)
			}
		}

		item.SourcePath = normalizeWorkflowPath(finalPath)
		item.ParentPath = normalizeWorkflowPath(filepath.Dir(finalPath))
		item.TargetName = filepath.Base(finalPath)
		movedItems = append(movedItems, item)
		results = append(results, MoveResult{SourcePath: sourcePath, TargetPath: normalizeWorkflowPath(finalPath), Status: "moved"})

		if input.ProgressFn != nil {
			percent := len(results) * 100 / len(items)
			input.ProgressFn(percent, fmt.Sprintf("已完成 %d/%d 项移动", len(results), len(items)))
		}
	}

	return NodeExecutionOutput{Outputs: map[string]TypedValue{"items": {Type: PortTypeProcessingItemList, Value: movedItems}, "results": {Type: PortTypeMoveResultList, Value: results}}, Status: ExecutionSuccess}, nil
}

func (e *phase4MoveNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *phase4MoveNodeExecutor) Rollback(ctx context.Context, input NodeRollbackInput) error {
	entries, err := phase4MoveCollectRollbackEntries(input)
	if err != nil {
		return fmt.Errorf("%s.Rollback: %w", e.Type(), err)
	}

	for _, entry := range entries {
		entry.TargetPath = normalizeWorkflowPath(entry.TargetPath)
		entry.SourcePath = normalizeWorkflowPath(entry.SourcePath)
		if strings.TrimSpace(entry.TargetPath) == "" || strings.TrimSpace(entry.SourcePath) == "" || entry.TargetPath == entry.SourcePath {
			continue
		}

		exists, err := e.fs.Exists(ctx, entry.TargetPath)
		if err != nil {
			return fmt.Errorf("check moved path %q: %w", entry.TargetPath, err)
		}
		if !exists {
			continue
		}

		if err := e.fs.MoveDir(ctx, entry.TargetPath, entry.SourcePath); err != nil {
			return fmt.Errorf("move back %q to %q: %w", entry.TargetPath, entry.SourcePath, err)
		}

		folderID := strings.TrimSpace(entry.FolderID)
		if e.folders != nil && folderID != "" {
			if err := e.folders.UpdatePath(ctx, folderID, entry.SourcePath); err != nil {
				return fmt.Errorf("update folder path for %q: %w", folderID, err)
			}
		}
	}

	return nil
}

type phase4MoveRollbackEntry struct {
	FolderID   string
	SourcePath string
	TargetPath string
}

func phase4MoveCollectRollbackEntries(input NodeRollbackInput) ([]phase4MoveRollbackEntry, error) {
	entryByTarget := make(map[string]phase4MoveRollbackEntry)
	collect := func(raw string, source string) error {
		entries, err := phase4MoveRollbackEntriesFromOutput(raw)
		if err != nil {
			return fmt.Errorf("parse %s output: %w", source, err)
		}
		for _, entry := range entries {
			key := strings.TrimSpace(entry.TargetPath)
			if key == "" {
				continue
			}
			entryByTarget[key] = entry
		}
		return nil
	}

	if input.NodeRun != nil && strings.TrimSpace(input.NodeRun.OutputJSON) != "" {
		if err := collect(input.NodeRun.OutputJSON, fmt.Sprintf("node run %q", input.NodeRun.ID)); err != nil {
			return nil, err
		}
	}
	for _, snapshot := range input.Snapshots {
		if snapshot == nil || snapshot.Kind != "post" || strings.TrimSpace(snapshot.OutputJSON) == "" {
			continue
		}
		if err := collect(snapshot.OutputJSON, fmt.Sprintf("snapshot %q", snapshot.ID)); err != nil {
			return nil, err
		}
	}

	entries := make([]phase4MoveRollbackEntry, 0, len(entryByTarget))
	for _, entry := range entryByTarget {
		entries = append(entries, entry)
	}
	return entries, nil
}

func phase4MoveRollbackEntriesFromOutput(raw string) ([]phase4MoveRollbackEntry, error) {
	var wrapped struct {
		Outputs map[string]TypedValueJSON `json:"outputs"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapped); err == nil && len(wrapped.Outputs) > 0 {
		decoded, err := typedValueMapFromJSON(wrapped.Outputs, NewTypeRegistry())
		if err != nil {
			return nil, err
		}
		return phase4MoveRollbackEntriesFromValues(decoded["items"].Value, decoded["results"].Value), nil
	}

	if typedOutputs, typed, err := parseTypedNodeOutputs(raw); err != nil {
		return nil, err
	} else if typed {
		return phase4MoveRollbackEntriesFromValues(typedOutputs["items"].Value, typedOutputs["results"].Value), nil
	}
	return nil, fmt.Errorf("invalid typed output json")
}

func phase4MoveRollbackEntriesFromValues(itemsValue any, resultsValue any) []phase4MoveRollbackEntry {
	items, _ := categoryRouterToItems(itemsValue)
	results := phase4MoveResultsFromAny(resultsValue)
	entries := make([]phase4MoveRollbackEntry, 0, len(results))
	for index, result := range results {
		if !strings.EqualFold(strings.TrimSpace(result.Status), "moved") {
			continue
		}
		entry := phase4MoveRollbackEntry{
			SourcePath: strings.TrimSpace(result.SourcePath),
			TargetPath: strings.TrimSpace(result.TargetPath),
		}
		if index < len(items) {
			entry.FolderID = strings.TrimSpace(items[index].FolderID)
		}
		entries = append(entries, entry)
	}
	return entries
}

func phase4MoveResultsFromAny(raw any) []MoveResult {
	switch typed := raw.(type) {
	case nil:
		return nil
	case MoveResult:
		return []MoveResult{typed}
	case *MoveResult:
		if typed == nil {
			return nil
		}
		return []MoveResult{*typed}
	case []MoveResult:
		return append([]MoveResult(nil), typed...)
	case []*MoveResult:
		out := make([]MoveResult, 0, len(typed))
		for _, item := range typed {
			if item != nil {
				out = append(out, *item)
			}
		}
		return out
	case []map[string]any:
		out := make([]MoveResult, 0, len(typed))
		for _, item := range typed {
			out = append(out, phase4MoveResultFromMap(item))
		}
		return out
	case []any:
		out := make([]MoveResult, 0, len(typed))
		for _, item := range typed {
			switch converted := item.(type) {
			case MoveResult:
				out = append(out, converted)
			case *MoveResult:
				if converted != nil {
					out = append(out, *converted)
				}
			case map[string]any:
				out = append(out, phase4MoveResultFromMap(converted))
			}
		}
		return out
	case map[string]any:
		return []MoveResult{phase4MoveResultFromMap(typed)}
	default:
		return nil
	}
}

func phase4MoveResultFromMap(raw map[string]any) MoveResult {
	return MoveResult{
		SourcePath: stringConfig(raw, "source_path"),
		TargetPath: stringConfig(raw, "target_path"),
		Status:     stringConfig(raw, "status"),
		Error:      stringConfig(raw, "error"),
	}
}

func (e *phase4MoveNodeExecutor) resolveDestinationPath(ctx context.Context, destinationPath, conflictPolicy string) (string, bool, error) {
	exists, err := e.fs.Exists(ctx, destinationPath)
	if err != nil {
		return "", false, err
	}
	if !exists {
		return destinationPath, false, nil
	}

	switch conflictPolicy {
	case "skip":
		return destinationPath, true, nil
	case "overwrite":
		if err := e.fs.Remove(ctx, destinationPath); err != nil {
			return "", false, fmt.Errorf("overwrite remove existing destination %q: %w", destinationPath, err)
		}
		return destinationPath, false, nil
	case "auto_rename":
		baseName := filepath.Base(destinationPath)
		parentDir := normalizeWorkflowPath(filepath.Dir(destinationPath))
		for index := 1; index <= 9999; index++ {
			candidate := joinWorkflowPath(parentDir, fmt.Sprintf("%s (%d)", baseName, index))
			taken, err := e.fs.Exists(ctx, candidate)
			if err != nil {
				return "", false, err
			}
			if !taken {
				return candidate, false, nil
			}
		}
		return "", false, fmt.Errorf("auto_rename exhausted candidates for %q", destinationPath)
	default:
		return "", false, fmt.Errorf("unsupported conflict_policy %q", conflictPolicy)
	}
}

func phase4MoveItemName(item ProcessingItem) string {
	if strings.TrimSpace(item.TargetName) != "" {
		return strings.TrimSpace(item.TargetName)
	}
	if strings.TrimSpace(item.FolderName) != "" {
		return strings.TrimSpace(item.FolderName)
	}

	return strings.TrimSpace(filepath.Base(strings.TrimSpace(item.SourcePath)))
}
