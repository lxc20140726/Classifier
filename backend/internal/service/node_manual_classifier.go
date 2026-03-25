package service

import (
	"context"
	"fmt"
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
		Label:       "Manual Classifier",
		Description: "Wait for manual category input and output classification signal",
		InputPorts: []NodeSchemaPort{{
			Name:        "signal",
			Description: "CLASSIFICATION_SIGNAL (optional lazy)",
			Required:    false,
		}},
		OutputPorts: []NodeSchemaPort{{
			Name:        "signal",
			Description: "CLASSIFICATION_SIGNAL",
			Required:    false,
		}},
	}
}

func (e *manualClassifierNodeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	rawSignal, ok := input.Inputs["signal"]
	if !ok || rawSignal == nil {
		return NodeExecutionOutput{Status: ExecutionPending, PendingReason: "awaiting manual classification"}, nil
	}

	signal, ok := rawSignal.(ClassificationSignal)
	if !ok {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: signal type %T is invalid", e.Type(), rawSignal)
	}

	return NodeExecutionOutput{Outputs: []any{signal}, Status: ExecutionSuccess}, nil
}

func (e *manualClassifierNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, data map[string]any) (NodeExecutionOutput, error) {
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

	signal := ClassificationSignal{
		Category:   category,
		Confidence: 1.0,
		Reason:     "manual",
		IsEmpty:    false,
	}

	return NodeExecutionOutput{Outputs: []any{signal}, Status: ExecutionSuccess}, nil
}

func (e *manualClassifierNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}
