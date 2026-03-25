package service

import (
	"context"
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
		Label:       "Move Node",
		Description: "Move processing items to target directory",
		InputPorts: []NodeSchemaPort{
			{Name: "items", Description: "PROCESSING_ITEM or PROCESSING_ITEM[]", Required: true},
		},
		OutputPorts: []NodeSchemaPort{
			{Name: "items", Description: "Moved PROCESSING_ITEM or PROCESSING_ITEM[]", Required: true},
			{Name: "results", Description: "MOVE_RESULT[]", Required: true},
		},
	}
}

func (e *phase4MoveNodeExecutor) Execute(ctx context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	items, isList, ok := categoryRouterExtractItems(input.Inputs)
	if !ok || len(items) == 0 {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: item/items input is required", e.Type())
	}

	targetDir := stringConfig(input.Node.Config, "target_dir")
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
		sourcePath := strings.TrimSpace(item.SourcePath)
		if sourcePath == "" {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: item source_path is required", e.Type())
		}

		destinationPath := filepath.Join(targetDir, itemName)
		finalPath, skipped, err := e.resolveDestinationPath(ctx, destinationPath, conflictPolicy)
		if err != nil {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: resolve destination for %q: %w", e.Type(), sourcePath, err)
		}

		if skipped {
			results = append(results, MoveResult{SourcePath: sourcePath, TargetPath: destinationPath, Status: "skipped"})
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

		item.SourcePath = finalPath
		item.ParentPath = filepath.Dir(finalPath)
		item.TargetName = filepath.Base(finalPath)
		movedItems = append(movedItems, item)
		results = append(results, MoveResult{SourcePath: sourcePath, TargetPath: finalPath, Status: "moved"})
	}

	if isList {
		return NodeExecutionOutput{Outputs: []any{movedItems, results}, Status: ExecutionSuccess}, nil
	}
	if len(movedItems) == 0 {
		return NodeExecutionOutput{Outputs: []any{nil, results}, Status: ExecutionSuccess}, nil
	}

	return NodeExecutionOutput{Outputs: []any{movedItems[0], results}, Status: ExecutionSuccess}, nil
}

func (e *phase4MoveNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *phase4MoveNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
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
		parentDir := filepath.Dir(destinationPath)
		for index := 1; index <= 9999; index++ {
			candidate := filepath.Join(parentDir, fmt.Sprintf("%s (%d)", baseName, index))
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
