package service

import (
	"context"
	"testing"

	"github.com/liqiye/classifier/internal/repository"
)

func TestNameKeywordClassifier(t *testing.T) {
	t.Parallel()

	executor := newNameKeywordClassifierExecutor()

	tests := []struct {
		name                 string
		inputs               map[string]any
		expectedCategory     string
		expectedConfidence   float64
		expectSignal         bool
		expectPassFolderPort bool
	}{
		{
			name: "hit_manga_japanese",
			inputs: map[string]any{
				"folder": FolderTree{Name: "進撃の巨人漫画"},
			},
			expectedCategory:   "manga",
			expectedConfidence: 1.0,
			expectSignal:       true,
		},
		{
			name: "hit_manga_english",
			inputs: map[string]any{
				"folder": FolderTree{Name: "My Comic Collection"},
			},
			expectedCategory:   "manga",
			expectedConfidence: 1.0,
			expectSignal:       true,
		},
		{
			name: "hit_photo",
			inputs: map[string]any{
				"folder": FolderTree{Name: "夏日写真集"},
			},
			expectedCategory:   "photo",
			expectedConfidence: 0.9,
			expectSignal:       true,
		},
		{
			name: "miss_random",
			inputs: map[string]any{
				"folder": FolderTree{Name: "random-folder-123"},
			},
			expectSignal:         false,
			expectPassFolderPort: true,
		},
		{
			name: "priority_order",
			inputs: map[string]any{
				"folder": FolderTree{Name: "漫画写真"},
			},
			expectedCategory:   "manga",
			expectedConfidence: 1.0,
			expectSignal:       true,
		},
		{
			name: "nil_input",
			inputs: map[string]any{
				"folder": nil,
			},
			expectSignal:         false,
			expectPassFolderPort: false,
		},
		{
			name:                 "empty_inputs",
			inputs:               map[string]any{},
			expectSignal:         false,
			expectPassFolderPort: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			output, err := executor.Execute(context.Background(), NodeExecutionInput{
				Node:   repository.WorkflowGraphNode{Type: executor.Type()},
				Inputs: tt.inputs,
			})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if output.Status != ExecutionSuccess {
				t.Fatalf("output.Status = %q, want %q", output.Status, ExecutionSuccess)
			}

			if len(output.Outputs) != 2 {
				t.Fatalf("len(output.Outputs) = %d, want 2", len(output.Outputs))
			}

			if tt.expectSignal {
				signal, ok := output.Outputs[0].(ClassificationSignal)
				if !ok {
					t.Fatalf("output.Outputs[0] type = %T, want ClassificationSignal", output.Outputs[0])
				}
				if signal.Category != tt.expectedCategory {
					t.Fatalf("signal.Category = %q, want %q", signal.Category, tt.expectedCategory)
				}
				if signal.Confidence != tt.expectedConfidence {
					t.Fatalf("signal.Confidence = %v, want %v", signal.Confidence, tt.expectedConfidence)
				}
				if output.Outputs[1] != nil {
					t.Fatalf("output.Outputs[1] = %v, want nil", output.Outputs[1])
				}
			} else {
				if output.Outputs[0] != nil {
					t.Fatalf("output.Outputs[0] = %v, want nil", output.Outputs[0])
				}
			}

			if tt.expectPassFolderPort {
				if _, ok := output.Outputs[1].(FolderTree); !ok {
					t.Fatalf("output.Outputs[1] type = %T, want FolderTree", output.Outputs[1])
				}
			} else if !tt.expectSignal {
				if output.Outputs[1] != nil {
					t.Fatalf("output.Outputs[1] = %v, want nil", output.Outputs[1])
				}
			}
		})
	}
}
