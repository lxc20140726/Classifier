package service

import (
	"context"
	"testing"
)

func TestManualClassifierExecutorBatchPendingAndResume(t *testing.T) {
	t.Parallel()

	executor := newManualClassifierExecutor()
	ctx := context.Background()

	output, err := executor.Execute(ctx, NodeExecutionInput{
		Inputs: testInputs(map[string]any{
			"trees": []FolderTree{{Path: "/src/a", Name: "a"}, {Path: "/src/b", Name: "b"}},
			"hint":  []ClassificationSignal{{SourcePath: "/src/a", Category: "other", Confidence: 0.5, Reason: "low"}, {SourcePath: "/src/b", IsEmpty: true}},
		}),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if output.Status != ExecutionPending {
		t.Fatalf("status = %q, want pending", output.Status)
	}
	if len(output.Outputs) != 1 {
		t.Fatalf("len(outputs) = %d, want 1", len(output.Outputs))
	}
	pendingState, ok := output.Outputs["state"].Value.(map[string]any)
	if !ok {
		t.Fatalf("pending state type = %T, want map[string]any", output.Outputs["state"].Value)
	}
	paths, ok := pendingState["pending_paths"].([]string)
	if !ok {
		t.Fatalf("pending_paths type = %T, want []string", pendingState["pending_paths"])
	}
	if len(paths) != 1 || paths[0] != "/src/a" {
		t.Fatalf("pending_paths = %#v, want [/src/a]", paths)
	}

	resumed, err := executor.Resume(ctx, NodeExecutionInput{}, map[string]any{
		"pending_paths": []string{"/src/a", "/src/b"},
		"category":      "video",
	})
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if resumed.Status != ExecutionSuccess {
		t.Fatalf("resume status = %q, want success", resumed.Status)
	}
	signals, ok := resumed.Outputs["signal"].Value.([]ClassificationSignal)
	if !ok {
		t.Fatalf("resume signals type = %T, want []ClassificationSignal", resumed.Outputs["signal"].Value)
	}
	if len(signals) != 2 || signals[0].SourcePath != "/src/a" || signals[1].SourcePath != "/src/b" {
		t.Fatalf("resume signals = %+v, want two signals with source path", signals)
	}
}

func TestManualClassifierExecutorResumeClassifications(t *testing.T) {
	t.Parallel()

	executor := newManualClassifierExecutor()
	resumed, err := executor.Resume(context.Background(), NodeExecutionInput{}, map[string]any{
		"classifications": []any{
			map[string]any{"source_path": "/src/a", "category": "manga"},
			map[string]any{"source_path": "/src/b", "category": "photo"},
		},
	})
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	signals, ok := resumed.Outputs["signal"].Value.([]ClassificationSignal)
	if !ok {
		t.Fatalf("signals type = %T, want []ClassificationSignal", resumed.Outputs["signal"].Value)
	}
	if len(signals) != 2 || signals[0].Category != "manga" || signals[1].Category != "photo" {
		t.Fatalf("signals = %+v, want two resumed classifications", signals)
	}
}
