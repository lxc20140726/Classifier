package service

import (
	"context"
	"testing"
)

func TestManualClassifierExecutor(t *testing.T) {
	t.Parallel()

	executor := newManualClassifierExecutor()
	ctx := context.Background()

	tests := []struct {
		name            string
		resumeData      map[string]any
		inputSignal     any
		useResume       bool
		wantErr         bool
		wantStatus      ExecutionStatus
		wantCategory    string
		wantConfidence  float64
		wantPendingText bool
	}{
		{
			name:            "pending_when_no_signal",
			inputSignal:     nil,
			wantStatus:      ExecutionPending,
			wantPendingText: true,
		},
		{
			name:           "pass_through_when_signal_present",
			inputSignal:    ClassificationSignal{Category: "photo", Confidence: 0.9},
			wantStatus:     ExecutionSuccess,
			wantCategory:   "photo",
			wantConfidence: 0.9,
		},
		{
			name:           "resume_valid_category",
			useResume:      true,
			resumeData:     map[string]any{"category": "manga"},
			wantStatus:     ExecutionSuccess,
			wantCategory:   "manga",
			wantConfidence: 1.0,
		},
		{
			name:       "resume_invalid_category",
			useResume:  true,
			resumeData: map[string]any{"category": "bad"},
			wantErr:    true,
		},
		{
			name:       "resume_missing_category",
			useResume:  true,
			resumeData: map[string]any{},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := NodeExecutionInput{Inputs: map[string]any{"signal": tt.inputSignal}}

			var (
				output NodeExecutionOutput
				err    error
			)
			if tt.useResume {
				output, err = executor.Resume(ctx, input, tt.resumeData)
			} else {
				output, err = executor.Execute(ctx, input)
			}

			if tt.wantErr {
				if err == nil {
					t.Fatalf("error = nil, want non-nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("error = %v, want nil", err)
			}

			if output.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", output.Status, tt.wantStatus)
			}

			if tt.wantPendingText {
				if output.PendingReason == "" {
					t.Fatalf("pending reason is empty, want non-empty")
				}
				return
			}

			if len(output.Outputs) != 1 {
				t.Fatalf("len(output.Outputs) = %d, want 1", len(output.Outputs))
			}

			signal, ok := output.Outputs[0].(ClassificationSignal)
			if !ok {
				t.Fatalf("output.Outputs[0] type = %T, want ClassificationSignal", output.Outputs[0])
			}

			if signal.Category != tt.wantCategory {
				t.Fatalf("signal.Category = %q, want %q", signal.Category, tt.wantCategory)
			}

			if signal.Confidence != tt.wantConfidence {
				t.Fatalf("signal.Confidence = %v, want %v", signal.Confidence, tt.wantConfidence)
			}
		})
	}
}
