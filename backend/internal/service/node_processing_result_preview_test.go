package service

import (
	"context"
	"testing"
)

func TestProcessingResultPreviewExecutorMissingInput(t *testing.T) {
	t.Parallel()

	executor := newProcessingResultPreviewExecutor()
	out, err := executor.Execute(context.Background(), NodeExecutionInput{Inputs: map[string]*TypedValue{}})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out.Status != ExecutionFailure {
		t.Fatalf("status = %q, want failure", out.Status)
	}
	if out.ErrorCode != "NODE_INPUT_MISSING" {
		t.Fatalf("error code = %q, want NODE_INPUT_MISSING", out.ErrorCode)
	}
}

func TestProcessingResultPreviewExecutorEmptyInput(t *testing.T) {
	t.Parallel()

	executor := newProcessingResultPreviewExecutor()
	out, err := executor.Execute(context.Background(), NodeExecutionInput{
		Inputs: testInputs(map[string]any{"results": []MoveResult{}}),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out.Status != ExecutionFailure {
		t.Fatalf("status = %q, want failure", out.Status)
	}
	if out.ErrorCode != "NODE_INPUT_EMPTY" {
		t.Fatalf("error code = %q, want NODE_INPUT_EMPTY", out.ErrorCode)
	}
}

func TestProcessingResultPreviewExecutorTypeError(t *testing.T) {
	t.Parallel()

	executor := newProcessingResultPreviewExecutor()
	out, err := executor.Execute(context.Background(), NodeExecutionInput{
		Inputs: testInputs(map[string]any{"results": "invalid"}),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out.Status != ExecutionFailure {
		t.Fatalf("status = %q, want failure", out.Status)
	}
	if out.ErrorCode != "NODE_INPUT_TYPE" {
		t.Fatalf("error code = %q, want NODE_INPUT_TYPE", out.ErrorCode)
	}
}

func TestProcessingResultPreviewExecutorSuccess(t *testing.T) {
	t.Parallel()

	executor := newProcessingResultPreviewExecutor()
	results := []MoveResult{
		{SourcePath: "/src/a", TargetPath: "/dst/a", Status: "moved"},
		{SourcePath: "/src/b", TargetPath: "/dst/b", Status: "skipped"},
		{SourcePath: "/src/c", TargetPath: "/dst/c", Status: "succeeded"},
	}
	out, err := executor.Execute(context.Background(), NodeExecutionInput{
		Inputs: testInputs(map[string]any{"results": results}),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out.Status != ExecutionSuccess {
		t.Fatalf("status = %q, want success", out.Status)
	}
	if _, exists := out.Outputs["results"]; exists {
		t.Fatalf("results output should not exist for preview node")
	}
	if _, exists := out.Outputs["summary"]; !exists {
		t.Fatalf("summary output should exist")
	}
}

func TestProcessingResultPreviewExecutorSummarySemantics(t *testing.T) {
	t.Parallel()

	executor := newProcessingResultPreviewExecutor()
	results := []MoveResult{
		{SourcePath: "/src/z", TargetPath: "/dst/z", Status: "failed"},
		{SourcePath: "/src/a", TargetPath: "/dst/a", Status: "skipped"},
		{SourcePath: "", TargetPath: "/dst/b", Status: "error"},
		{SourcePath: "/src/c", TargetPath: "/dst/c", Status: "moved"},
	}
	out, err := executor.Execute(context.Background(), NodeExecutionInput{
		Inputs: testInputs(map[string]any{"results": results}),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	summary, ok := out.Outputs["summary"].Value.(processingResultPreviewSummary)
	if !ok {
		t.Fatalf("summary output type = %T, want processingResultPreviewSummary", out.Outputs["summary"].Value)
	}
	if summary.Total != 4 {
		t.Fatalf("summary.Total = %d, want 4", summary.Total)
	}
	if summary.Succeeded != 1 || summary.Failed != 3 {
		t.Fatalf("summary succeeded/failed = %d/%d, want 1/3", summary.Succeeded, summary.Failed)
	}
	if len(summary.FailedPaths) != 3 || summary.FailedPaths[0] != "/dst/b" || summary.FailedPaths[1] != "/src/a" || summary.FailedPaths[2] != "/src/z" {
		t.Fatalf("summary.FailedPaths = %#v, want sorted [/dst/b /src/a /src/z]", summary.FailedPaths)
	}
}
