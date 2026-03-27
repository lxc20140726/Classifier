package service

import (
	"context"
	"fmt"
	"strings"
)

const manualClassifierExecutorType = "manual-classifier"

var validManualCategories = map[string]struct{}{
	"photo": {},
	"video": {},
	"manga": {},
	"mixed": {},
	"other": {},
}

type manualClassifierNodeExecutor struct{}

func newManualClassifierExecutor() *manualClassifierNodeExecutor {
	return &manualClassifierNodeExecutor{}
}

func NewManualClassifierExecutor() WorkflowNodeExecutor {
	return newManualClassifierExecutor()
}

func (e *manualClassifierNodeExecutor) Type() string {
	return manualClassifierExecutorType
}

func (e *manualClassifierNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "人工分类器",
		Description: "暂停工作流等待人工输入分类结果，通过 provide-input 接口提交后继续执行",
		Inputs: []PortDef{
			{Name: "trees", Type: PortTypeFolderTreeList, Description: "待人工审核的目录树", Required: false, Lazy: true},
			{Name: "hint", Type: PortTypeClassificationSignalList, Description: "低置信度建议信号", Required: false, Lazy: true},
		},
		Outputs: []PortDef{{
			Name:        "signal",
			Type:        PortTypeClassificationSignalList,
			Description: "人工分类信号",
		}},
	}
}

func (e *manualClassifierNodeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	rawInputs := typedInputsToAny(input.Inputs)
	rawLegacySignal, hasLegacySignal := rawInputs["signal"]
	if hasLegacySignal && rawLegacySignal != nil {
		legacySignals, found, err := parseSignalListInput(rawLegacySignal)
		if err != nil {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute parse signal: %w", e.Type(), err)
		}
		if found {
			return NodeExecutionOutput{Outputs: map[string]TypedValue{"signal": {Type: PortTypeClassificationSignalList, Value: legacySignals}}, Status: ExecutionSuccess}, nil
		}
	}

	rawTrees, hasTrees := firstPresent(rawInputs, "trees")
	trees := []FolderTree{}
	if hasTrees {
		parsedTrees, _, err := parseFolderTreesInput(rawTrees)
		if err != nil {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute parse trees: %w", e.Type(), err)
		}
		trees = parsedTrees
	}

	rawHints, hasHints := firstPresent(rawInputs, "hint")
	hints := []ClassificationSignal{}
	if hasHints {
		parsedHints, _, err := parseSignalListInput(rawHints)
		if err != nil {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute parse hint signals: %w", e.Type(), err)
		}
		hints = parsedHints
	}

	pendingPaths := make([]string, 0)
	for _, hint := range hints {
		if hint.IsEmpty {
			continue
		}
		if strings.TrimSpace(hint.SourcePath) == "" {
			continue
		}
		pendingPaths = append(pendingPaths, hint.SourcePath)
	}

	if !hasHints && len(pendingPaths) == 0 {
		for _, tree := range trees {
			if strings.TrimSpace(tree.Path) == "" {
				continue
			}
			pendingPaths = append(pendingPaths, tree.Path)
		}
	}
	pendingPaths = compactPaths(pendingPaths)

	if len(pendingPaths) == 0 {
		return NodeExecutionOutput{Outputs: map[string]TypedValue{"signal": {Type: PortTypeClassificationSignalList, Value: []ClassificationSignal{}}}, Status: ExecutionSuccess}, nil
	}

	pendingState := map[string]any{
		"pending_paths":  pendingPaths,
		"hint_signals":   hints,
		"trees_snapshot": trees,
	}

	return NodeExecutionOutput{Outputs: map[string]TypedValue{"state": {Type: PortTypeJSON, Value: pendingState}}, Status: ExecutionPending, PendingReason: "awaiting manual classification"}, nil

}

type manualClassificationInput struct {
	SourcePath string `json:"source_path"`
	Category   string `json:"category"`
}

func (e *manualClassifierNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, data map[string]any) (NodeExecutionOutput, error) {
	if len(data) == 0 {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Resume: resume data is required", e.Type())
	}

	if rawClassifications, ok := data["classifications"]; ok {
		classifications, err := parseManualClassifications(rawClassifications)
		if err != nil {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Resume: %w", e.Type(), err)
		}
		signals := make([]ClassificationSignal, 0, len(classifications))
		for _, classification := range classifications {
			if _, ok := validManualCategories[classification.Category]; !ok {
				return NodeExecutionOutput{}, fmt.Errorf("%s.Resume: invalid category %q", e.Type(), classification.Category)
			}
			signals = append(signals, ClassificationSignal{
				SourcePath: classification.SourcePath,
				Category:   classification.Category,
				Confidence: 1.0,
				Reason:     "manual",
			})
		}

		return NodeExecutionOutput{Outputs: map[string]TypedValue{"signal": {Type: PortTypeClassificationSignalList, Value: signals}}, Status: ExecutionSuccess}, nil
	}

	rawCategory, ok := data["category"]
	if !ok {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Resume: category is required", e.Type())
	}

	category, ok := rawCategory.(string)
	if !ok {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Resume: category must be string", e.Type())
	}

	if _, ok := validManualCategories[category]; !ok {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Resume: invalid category %q", e.Type(), category)
	}

	pendingPaths, err := parsePendingPaths(data["pending_paths"])
	if err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Resume: %w", e.Type(), err)
	}

	if len(pendingPaths) == 0 {
		signal := ClassificationSignal{Category: category, Confidence: 1.0, Reason: "manual"}
		return NodeExecutionOutput{Outputs: map[string]TypedValue{"signal": {Type: PortTypeClassificationSignalList, Value: []ClassificationSignal{signal}}}, Status: ExecutionSuccess}, nil
	}

	signals := make([]ClassificationSignal, 0, len(pendingPaths))
	for _, sourcePath := range pendingPaths {
		signals = append(signals, ClassificationSignal{
			SourcePath: sourcePath,
			Category:   category,
			Confidence: 1.0,
			Reason:     "manual",
		})
	}

	return NodeExecutionOutput{Outputs: map[string]TypedValue{"signal": {Type: PortTypeClassificationSignalList, Value: signals}}, Status: ExecutionSuccess}, nil
}

func parseManualClassifications(raw any) ([]manualClassificationInput, error) {
	rawItems, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("classifications must be an array")
	}

	out := make([]manualClassificationInput, 0, len(rawItems))
	for idx, item := range rawItems {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("classifications[%d] must be an object", idx)
		}

		sourcePath, _ := entry["source_path"].(string)
		sourcePath = strings.TrimSpace(sourcePath)
		if sourcePath == "" {
			return nil, fmt.Errorf("classifications[%d].source_path is required", idx)
		}

		category, _ := entry["category"].(string)
		category = strings.TrimSpace(category)
		if category == "" {
			return nil, fmt.Errorf("classifications[%d].category is required", idx)
		}

		out = append(out, manualClassificationInput{SourcePath: sourcePath, Category: category})
	}

	return out, nil
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

func (e *manualClassifierNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}
