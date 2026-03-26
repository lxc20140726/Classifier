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
			{Name: "signals", Description: "CLASSIFICATION_SIGNAL_LIST", Required: false},
			{Name: "signal", Description: "CLASSIFICATION_SIGNAL legacy input", Required: false},
		},
		OutputPorts: []NodeSchemaPort{
			{Name: "high", Description: "CLASSIFICATION_SIGNAL_LIST", Required: false},
			{Name: "low", Description: "CLASSIFICATION_SIGNAL_LIST", Required: false},
		},
	}
}

func (e *confidenceCheckNodeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	rawInputs := typedInputsToAny(input.Inputs)
	rawSignals, ok := firstPresent(rawInputs, "signals", "signal")
	if !ok {
		return NodeExecutionOutput{Outputs: map[string]TypedValue{"high": {Type: PortTypeClassificationSignalList, Value: nil}, "low": {Type: PortTypeClassificationSignalList, Value: nil}}, Status: ExecutionSuccess}, nil
	}
	signals, found, err := parseSignalListInput(rawSignals)
	if err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute parse signals: %w", e.Type(), err)
	}
	if !found {
		return NodeExecutionOutput{Outputs: map[string]TypedValue{"high": {Type: PortTypeClassificationSignalList, Value: nil}, "low": {Type: PortTypeClassificationSignalList, Value: nil}}, Status: ExecutionSuccess}, nil
	}

	threshold := 0.75
	if v, ok := input.Node.Config["threshold"].(float64); ok {
		threshold = v
	}

	high := make([]ClassificationSignal, 0, len(signals))
	low := make([]ClassificationSignal, 0, len(signals))
	for _, signal := range signals {
		empty := ClassificationSignal{SourcePath: signal.SourcePath, IsEmpty: true}
		if signal.IsEmpty {
			high = append(high, empty)
			low = append(low, empty)
			continue
		}
		if signal.Confidence >= threshold {
			high = append(high, signal)
			low = append(low, empty)
			continue
		}
		high = append(high, empty)
		low = append(low, signal)
	}

	return NodeExecutionOutput{Outputs: map[string]TypedValue{"high": {Type: PortTypeClassificationSignalList, Value: high}, "low": {Type: PortTypeClassificationSignalList, Value: low}}, Status: ExecutionSuccess}, nil
}

func (e *confidenceCheckNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *confidenceCheckNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}
