package service

import (
	"context"
	"fmt"
	"strings"
)

const folderSelectorExecutorType = "folder-selector"

type folderSelectorNodeExecutor struct{}

func newFolderSelectorNodeExecutor() *folderSelectorNodeExecutor {
	return &folderSelectorNodeExecutor{}
}

func (e *folderSelectorNodeExecutor) Type() string {
	return folderSelectorExecutorType
}

func (e *folderSelectorNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "文件夹筛选器",
		Description: "展示扫描到的文件夹列表，暂停工作流等待用户勾选哪些参与后续分类和处理",
		Inputs: []PortDef{
			{Name: "trees", Type: PortTypeFolderTreeList, Description: "候选目录树列表", Required: true},
		},
		Outputs: []PortDef{
			{Name: "trees", Type: PortTypeFolderTreeList, RequiredOutput: true, Description: "用户选中的目录树列表"},
		},
		ConfigSchema: map[string]any{
			"auto_select_all": map[string]any{
				"type":        "boolean",
				"default":     false,
				"description": "开启后不暂停，自动透传全部候选目录树",
			},
		},
	}
}

func (e *folderSelectorNodeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	autoSelectAll := folderSplitterBoolConfig(input.Node.Config, "auto_select_all", false)

	rawInputs := typedInputsToAny(input.Inputs)
	rawTrees, ok := firstPresent(rawInputs, "trees")
	if !ok {
		return NodeExecutionOutput{
			Outputs: map[string]TypedValue{"trees": {Type: PortTypeFolderTreeList, Value: []FolderTree{}}},
			Status:  ExecutionSuccess,
		}, nil
	}

	trees, found, err := parseFolderTreesInput(rawTrees)
	if err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute parse trees: %w", e.Type(), err)
	}
	if !found || len(trees) == 0 {
		return NodeExecutionOutput{
			Outputs: map[string]TypedValue{"trees": {Type: PortTypeFolderTreeList, Value: []FolderTree{}}},
			Status:  ExecutionSuccess,
		}, nil
	}

	if autoSelectAll {
		return NodeExecutionOutput{
			Outputs: map[string]TypedValue{"trees": {Type: PortTypeFolderTreeList, Value: trees}},
			Status:  ExecutionSuccess,
		}, nil
	}

	candidatePaths := make([]string, 0, len(trees))
	for _, tree := range trees {
		if strings.TrimSpace(tree.Path) != "" {
			candidatePaths = append(candidatePaths, tree.Path)
		}
	}

	pendingState := map[string]any{
		"candidate_paths": candidatePaths,
		"trees_snapshot":  trees,
	}

	return NodeExecutionOutput{
		PendingState:  pendingState,
		Status:        ExecutionPending,
		PendingReason: "awaiting folder selection",
	}, nil
}

func (e *folderSelectorNodeExecutor) Resume(_ context.Context, input NodeExecutionInput, data map[string]any) (NodeExecutionOutput, error) {
	if len(data) == 0 {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Resume: resume data is required", e.Type())
	}

	rawSnapshot, hasSnapshot := data["trees_snapshot"]
	trees := []FolderTree{}
	if hasSnapshot {
		parsed, _, err := parseFolderTreesInput(rawSnapshot)
		if err == nil {
			trees = parsed
		}
	}

	rawSelected, ok := data["selected_paths"]
	if !ok {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Resume: selected_paths is required", e.Type())
	}

	selectedPaths, err := parsePendingPaths(rawSelected)
	if err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Resume: %w", e.Type(), err)
	}

	selectedSet := make(map[string]struct{}, len(selectedPaths))
	for _, p := range selectedPaths {
		selectedSet[strings.TrimSpace(p)] = struct{}{}
	}

	filtered := make([]FolderTree, 0, len(selectedPaths))
	for _, tree := range trees {
		if _, ok := selectedSet[strings.TrimSpace(tree.Path)]; ok {
			filtered = append(filtered, tree)
		}
	}

	return NodeExecutionOutput{
		Outputs: map[string]TypedValue{"trees": {Type: PortTypeFolderTreeList, Value: filtered}},
		Status:  ExecutionSuccess,
	}, nil
}

func (e *folderSelectorNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func parsePendingPaths(raw any) ([]string, error) {
	if raw == nil {
		return nil, nil
	}

	items, ok := raw.([]any)
	if ok {
		out := make([]string, 0, len(items))
		for idx, item := range items {
			path, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("pending_paths[%d] must be string", idx)
			}
			out = append(out, path)
		}
		return compactPaths(out), nil
	}

	paths, ok := raw.([]string)
	if ok {
		return compactPaths(paths), nil
	}

	return nil, fmt.Errorf("pending_paths must be string array")
}
