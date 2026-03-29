package service

import (
	"context"
	"testing"
)

func TestClassificationDBResultPreviewExecutorMissingInput(t *testing.T) {
	t.Parallel()

	executor := newClassificationDBResultPreviewExecutor()
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

func TestClassificationDBResultPreviewExecutorEmptyInput(t *testing.T) {
	t.Parallel()

	executor := newClassificationDBResultPreviewExecutor()
	out, err := executor.Execute(context.Background(), NodeExecutionInput{
		Inputs: testInputs(map[string]any{"entries": []ClassifiedEntry{}}),
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

func TestClassificationDBResultPreviewExecutorTypeError(t *testing.T) {
	t.Parallel()

	executor := newClassificationDBResultPreviewExecutor()
	out, err := executor.Execute(context.Background(), NodeExecutionInput{
		Inputs: testInputs(map[string]any{"entries": "invalid"}),
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

func TestClassificationDBResultPreviewExecutorSuccess(t *testing.T) {
	t.Parallel()

	executor := newClassificationDBResultPreviewExecutor()
	entries := []ClassifiedEntry{
		{Path: "/a", Category: "video", Confidence: 0.9, Classifier: "kw"},
		{Path: "/b", Category: "photo", Confidence: 0.7, Classifier: "ft"},
		{Path: "/c", Category: "video", Confidence: 0.8, Classifier: "kw"},
	}
	out, err := executor.Execute(context.Background(), NodeExecutionInput{
		Inputs: testInputs(map[string]any{"entries": entries}),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out.Status != ExecutionSuccess {
		t.Fatalf("status = %q, want success", out.Status)
	}
	if _, exists := out.Outputs["entries"]; exists {
		t.Fatalf("entries output should not exist for preview node")
	}
	if _, exists := out.Outputs["summary"]; !exists {
		t.Fatalf("summary output should exist")
	}
}

func TestClassificationDBResultPreviewExecutorSummarySemantics(t *testing.T) {
	t.Parallel()

	executor := newClassificationDBResultPreviewExecutor()
	entries := []ClassifiedEntry{
		{Path: "/a", Category: "video", Confidence: 0.9, Classifier: "kw"},
		{Path: "/b", Category: "", Confidence: 0.6, Classifier: "ft"},
		{Path: "/c", Category: "video", Confidence: 0.3, Classifier: "kw"},
	}
	out, err := executor.Execute(context.Background(), NodeExecutionInput{
		Inputs: testInputs(map[string]any{"entries": entries}),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	summary, ok := out.Outputs["summary"].Value.(classificationDBResultPreviewSummary)
	if !ok {
		t.Fatalf("summary output type = %T, want classificationDBResultPreviewSummary", out.Outputs["summary"].Value)
	}
	if summary.Total != 3 {
		t.Fatalf("summary.Total = %d, want 3", summary.Total)
	}
	if summary.ByCategory["video"] != 2 || summary.ByCategory["other"] != 1 {
		t.Fatalf("summary.ByCategory = %#v, want video=2 and other=1", summary.ByCategory)
	}
	if summary.AvgConfidence != 0.6 {
		t.Fatalf("summary.AvgConfidence = %v, want 0.6", summary.AvgConfidence)
	}
	if len(summary.ClassifierSources) != 2 || summary.ClassifierSources[0] != "ft" || summary.ClassifierSources[1] != "kw" {
		t.Fatalf("summary.ClassifierSources = %#v, want [ft kw]", summary.ClassifierSources)
	}
}
