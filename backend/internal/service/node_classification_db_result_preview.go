package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const classificationDBResultPreviewExecutorType = "classification-db-result-preview"

type classificationDBResultPreviewSummary struct {
	Total             int            `json:"total"`
	ByCategory        map[string]int `json:"by_category"`
	AvgConfidence     float64        `json:"avg_confidence"`
	ClassifierSources []string       `json:"classifier_sources"`
}

type classificationDBResultPreviewNodeExecutor struct{}

func newClassificationDBResultPreviewExecutor() *classificationDBResultPreviewNodeExecutor {
	return &classificationDBResultPreviewNodeExecutor{}
}

func (e *classificationDBResultPreviewNodeExecutor) Type() string {
	return classificationDBResultPreviewExecutorType
}

func (e *classificationDBResultPreviewNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "分类落库结果预览",
		Description: "预览分类落库结果并输出分类统计摘要",
		Inputs: []PortDef{
			{Name: "entries", Type: PortTypeClassifiedEntryList, Required: true, Description: "已写入的分类条目列表"},
		},
	}
}

func (e *classificationDBResultPreviewNodeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	rawEntries, ok := firstPresentTyped(input.Inputs, "entries")
	if !ok {
		return NodeExecutionOutput{
			Status:        ExecutionFailure,
			ErrorCode:     "NODE_INPUT_MISSING",
			PendingReason: "entries input is required",
		}, nil
	}

	entries, err := parseClassifiedEntryList(rawEntries)
	if err != nil {
		return NodeExecutionOutput{
			Status:        ExecutionFailure,
			ErrorCode:     "NODE_INPUT_TYPE",
			PendingReason: fmt.Sprintf("entries parse failed: %v", err),
		}, nil
	}
	if len(entries) == 0 {
		return NodeExecutionOutput{
			Status:        ExecutionFailure,
			ErrorCode:     "NODE_INPUT_EMPTY",
			PendingReason: "entries input is empty",
		}, nil
	}

	byCategory := make(map[string]int)
	classifierSet := make(map[string]struct{})
	confidenceSum := 0.0
	for _, entry := range entries {
		category := strings.TrimSpace(entry.Category)
		if category == "" {
			category = "other"
		}
		byCategory[category]++
		confidenceSum += entry.Confidence

		classifier := strings.TrimSpace(entry.Classifier)
		if classifier != "" {
			classifierSet[classifier] = struct{}{}
		}
	}

	classifierSources := make([]string, 0, len(classifierSet))
	for source := range classifierSet {
		classifierSources = append(classifierSources, source)
	}
	sort.Strings(classifierSources)

	summary := classificationDBResultPreviewSummary{
		Total:             len(entries),
		ByCategory:        byCategory,
		AvgConfidence:     confidenceSum / float64(len(entries)),
		ClassifierSources: classifierSources,
	}

	return NodeExecutionOutput{
		Outputs: map[string]TypedValue{
			"summary": {Type: PortTypeJSON, Value: summary},
		},
		Status: ExecutionSuccess,
	}, nil
}

func (e *classificationDBResultPreviewNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *classificationDBResultPreviewNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}
