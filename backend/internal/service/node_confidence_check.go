package service

import (
	"context"
	"fmt"
)

type confidenceCheckConfig struct {
	Threshold float64 `json:"threshold"`
}

type confidenceCheckNodeExecutor struct{}

func newConfidenceCheckExecutor() *confidenceCheckNodeExecutor {
	return &confidenceCheckNodeExecutor{}
}

func NewConfidenceCheckExecutor() WorkflowNodeExecutor {
	return newConfidenceCheckExecutor()
}

func (e *confidenceCheckNodeExecutor) Type() string {
	return "confidence-check"
}

func (e *confidenceCheckNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "Confidence Check",
		Description: "Route classification signal by confidence threshold",
		InputPorts: []NodeSchemaPort{
			{Name: "signal", Description: "CLASSIFICATION_SIGNAL", Required: true},
		},
		OutputPorts: []NodeSchemaPort{
			{Name: "high", Description: "CLASSIFICATION_SIGNAL", Required: false},
			{Name: "low", Description: "CLASSIFICATION_SIGNAL", Required: false},
		},
	}
}

func (e *confidenceCheckNodeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	rawSignal := input.Inputs["signal"]
	signal, ok := rawSignal.(ClassificationSignal)
	if !ok {
		return NodeExecutionOutput{Outputs: []any{nil, nil}, Status: ExecutionSuccess}, nil
	}

	threshold := 0.75
	if v, ok := input.Node.Config["threshold"].(float64); ok {
		threshold = v
	}

	if signal.Confidence >= threshold {
		return NodeExecutionOutput{Outputs: []any{signal, nil}, Status: ExecutionSuccess}, nil
	}

	return NodeExecutionOutput{Outputs: []any{nil, signal}, Status: ExecutionSuccess}, nil
}

func (e *confidenceCheckNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *confidenceCheckNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}
