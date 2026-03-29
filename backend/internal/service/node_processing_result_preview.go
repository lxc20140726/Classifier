package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const processingResultPreviewExecutorType = "processing-result-preview"

type processingResultPreviewSummary struct {
	Total       int      `json:"total"`
	Succeeded   int      `json:"succeeded"`
	Failed      int      `json:"failed"`
	FailedPaths []string `json:"failed_paths"`
}

type processingResultPreviewNodeExecutor struct{}

func newProcessingResultPreviewExecutor() *processingResultPreviewNodeExecutor {
	return &processingResultPreviewNodeExecutor{}
}

func (e *processingResultPreviewNodeExecutor) Type() string {
	return processingResultPreviewExecutorType
}

func (e *processingResultPreviewNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "处理结果预览",
		Description: "预览处理节点结果并输出成功失败汇总",
		Inputs: []PortDef{
			{Name: "results", Type: PortTypeMoveResultList, Required: true, Description: "处理结果列表"},
		},
		Outputs: []PortDef{
			{Name: "summary", Type: PortTypeJSON, RequiredOutput: true, Description: "处理结果汇总信息"},
		},
	}
}

func (e *processingResultPreviewNodeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	rawResults, ok := firstPresentTyped(input.Inputs, "results")
	if !ok {
		return NodeExecutionOutput{
			Status:        ExecutionFailure,
			ErrorCode:     "NODE_INPUT_MISSING",
			PendingReason: "results input is required",
		}, nil
	}
	if !isSupportedMoveResultInput(rawResults) {
		return NodeExecutionOutput{
			Status:        ExecutionFailure,
			ErrorCode:     "NODE_INPUT_TYPE",
			PendingReason: fmt.Sprintf("results input has unsupported type %T", rawResults),
		}, nil
	}

	results := phase4MoveResultsFromAny(rawResults)
	if len(results) == 0 {
		return NodeExecutionOutput{
			Status:        ExecutionFailure,
			ErrorCode:     "NODE_INPUT_EMPTY",
			PendingReason: "results input is empty",
		}, nil
	}

	summary := processingResultPreviewSummary{
		Total:       len(results),
		FailedPaths: make([]string, 0),
	}
	for _, result := range results {
		if strings.EqualFold(strings.TrimSpace(result.Status), "moved") || strings.EqualFold(strings.TrimSpace(result.Status), "succeeded") {
			summary.Succeeded++
			continue
		}
		summary.Failed++
		failedPath := strings.TrimSpace(result.SourcePath)
		if failedPath == "" {
			failedPath = strings.TrimSpace(result.TargetPath)
		}
		if failedPath != "" {
			summary.FailedPaths = append(summary.FailedPaths, failedPath)
		}
	}
	sort.Strings(summary.FailedPaths)

	return NodeExecutionOutput{
		Outputs: map[string]TypedValue{
			"summary": {Type: PortTypeJSON, Value: summary},
		},
		Status: ExecutionSuccess,
	}, nil
}

func (e *processingResultPreviewNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *processingResultPreviewNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func isSupportedMoveResultInput(raw any) bool {
	switch raw.(type) {
	case MoveResult, *MoveResult, []MoveResult, []*MoveResult, []map[string]any, []any, map[string]any:
		return true
	default:
		return false
	}
}
