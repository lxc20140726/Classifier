package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/liqiye/classifier/internal/repository"
)

type WorkflowGraphNode = repository.WorkflowGraphNode

func makeConfidenceCheckInput(signal any, threshold *float64) NodeExecutionInput {
	cfg := map[string]any{}
	if threshold != nil {
		cfg["threshold"] = *threshold
	}

	return NodeExecutionInput{
		Node:   WorkflowGraphNode{Config: cfg},
		Inputs: map[string]any{"signal": signal},
	}
}

func TestConfidenceCheckExecutor(t *testing.T) {
	t.Parallel()

	executor := newConfidenceCheckExecutor()
	threshold := 0.75

	tests := []struct {
		name            string
		input           NodeExecutionInput
		expectedStatus  ExecutionStatus
		expectedOutput0 any
		expectedOutput1 any
	}{
		{
			name: "high_confidence",
			input: makeConfidenceCheckInput(ClassificationSignal{
				Category:   "video",
				Confidence: 0.9,
				Reason:     "high",
			}, &threshold),
			expectedStatus: ExecutionSuccess,
			expectedOutput0: ClassificationSignal{
				Category:   "video",
				Confidence: 0.9,
				Reason:     "high",
			},
			expectedOutput1: nil,
		},
		{
			name: "low_confidence",
			input: makeConfidenceCheckInput(ClassificationSignal{
				Category:   "photo",
				Confidence: 0.5,
				Reason:     "low",
			}, &threshold),
			expectedStatus:  ExecutionSuccess,
			expectedOutput0: nil,
			expectedOutput1: ClassificationSignal{
				Category:   "photo",
				Confidence: 0.5,
				Reason:     "low",
			},
		},
		{
			name: "exactly_threshold",
			input: makeConfidenceCheckInput(ClassificationSignal{
				Category:   "manga",
				Confidence: 0.75,
				Reason:     "exact",
			}, &threshold),
			expectedStatus: ExecutionSuccess,
			expectedOutput0: ClassificationSignal{
				Category:   "manga",
				Confidence: 0.75,
				Reason:     "exact",
			},
			expectedOutput1: nil,
		},
		{
			name:            "nil_signal",
			input:           makeConfidenceCheckInput(nil, &threshold),
			expectedStatus:  ExecutionSuccess,
			expectedOutput0: nil,
			expectedOutput1: nil,
		},
		{
			name: "default_threshold",
			input: makeConfidenceCheckInput(ClassificationSignal{
				Category:   "video",
				Confidence: 0.8,
				Reason:     "default-threshold",
			}, nil),
			expectedStatus: ExecutionSuccess,
			expectedOutput0: ClassificationSignal{
				Category:   "video",
				Confidence: 0.8,
				Reason:     "default-threshold",
			},
			expectedOutput1: nil,
		},
		{
			name:            "wrong_type",
			input:           makeConfidenceCheckInput("not-a-signal", &threshold),
			expectedStatus:  ExecutionSuccess,
			expectedOutput0: nil,
			expectedOutput1: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, err := executor.Execute(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if out.Status != tt.expectedStatus {
				t.Fatalf("status = %q, want %q", out.Status, tt.expectedStatus)
			}

			if len(out.Outputs) != 2 {
				t.Fatalf("len(outputs) = %d, want 2", len(out.Outputs))
			}

			if !reflect.DeepEqual(out.Outputs[0], tt.expectedOutput0) {
				t.Fatalf("port0 = %#v, want %#v", out.Outputs[0], tt.expectedOutput0)
			}

			if !reflect.DeepEqual(out.Outputs[1], tt.expectedOutput1) {
				t.Fatalf("port1 = %#v, want %#v", out.Outputs[1], tt.expectedOutput1)
			}
		})
	}
}
