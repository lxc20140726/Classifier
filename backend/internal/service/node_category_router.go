package service

import (
	"context"
	"fmt"
	"strings"
)

const categoryRouterExecutorType = "category-router"

type categoryRouterNodeExecutor struct{}

func newCategoryRouterExecutor() *categoryRouterNodeExecutor {
	return &categoryRouterNodeExecutor{}
}

func NewCategoryRouterExecutor() WorkflowNodeExecutor {
	return newCategoryRouterExecutor()
}

func (e *categoryRouterNodeExecutor) Type() string {
	return categoryRouterExecutorType
}

func (e *categoryRouterNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "Category Router",
		Description: "Route processing items by category",
		InputPorts: []NodeSchemaPort{
			{Name: "items", Description: "PROCESSING_ITEM or PROCESSING_ITEM[]", Required: true},
		},
		OutputPorts: []NodeSchemaPort{
			{Name: "video", Description: "PROCESSING_ITEM or PROCESSING_ITEM[]", Required: false},
			{Name: "manga", Description: "PROCESSING_ITEM or PROCESSING_ITEM[]", Required: false},
			{Name: "photo", Description: "PROCESSING_ITEM or PROCESSING_ITEM[]", Required: false},
			{Name: "other", Description: "PROCESSING_ITEM or PROCESSING_ITEM[]", Required: false},
			{Name: "mixed_leaf", Description: "PROCESSING_ITEM or PROCESSING_ITEM[]", Required: false},
		},
	}
}

func (e *categoryRouterNodeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	items, isList, ok := categoryRouterExtractItems(input.Inputs)
	if !ok {
		return NodeExecutionOutput{Outputs: []any{nil, nil, nil, nil, nil}, Status: ExecutionSuccess}, nil
	}

	if isList {
		ports := make([][]ProcessingItem, 5)
		for _, item := range items {
			idx := categoryRouterPortIndex(item.Category)
			ports[idx] = append(ports[idx], item)
		}

		outputs := make([]any, 5)
		for i := range ports {
			if len(ports[i]) == 0 {
				outputs[i] = nil
				continue
			}
			outputs[i] = ports[i]
		}

		return NodeExecutionOutput{Outputs: outputs, Status: ExecutionSuccess}, nil
	}

	outputs := []any{nil, nil, nil, nil, nil}
	outputs[categoryRouterPortIndex(items[0].Category)] = items[0]
	return NodeExecutionOutput{Outputs: outputs, Status: ExecutionSuccess}, nil
}

func (e *categoryRouterNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *categoryRouterNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func categoryRouterExtractItems(inputs map[string]any) ([]ProcessingItem, bool, bool) {
	for _, key := range []string{"items", "item"} {
		raw, ok := inputs[key]
		if !ok {
			continue
		}

		items, isList, ok := categoryRouterToItems(raw)
		if !ok {
			continue
		}

		return items, isList, true
	}

	return nil, false, false
}

func categoryRouterToItems(raw any) ([]ProcessingItem, bool, bool) {
	switch value := raw.(type) {
	case ProcessingItem:
		return []ProcessingItem{value}, false, true
	case *ProcessingItem:
		if value == nil {
			return nil, false, false
		}
		return []ProcessingItem{*value}, false, true
	case []ProcessingItem:
		return append([]ProcessingItem(nil), value...), true, true
	case []*ProcessingItem:
		out := make([]ProcessingItem, 0, len(value))
		for _, item := range value {
			if item == nil {
				continue
			}
			out = append(out, *item)
		}
		return out, true, true
	case []any:
		out := make([]ProcessingItem, 0, len(value))
		for _, item := range value {
			parsed, ok := categoryRouterToItem(item)
			if !ok {
				continue
			}
			out = append(out, parsed)
		}
		return out, true, true
	default:
		parsed, ok := categoryRouterToItem(value)
		if !ok {
			return nil, false, false
		}
		return []ProcessingItem{parsed}, false, true
	}
}

func categoryRouterToItem(raw any) (ProcessingItem, bool) {
	switch value := raw.(type) {
	case ProcessingItem:
		return value, true
	case *ProcessingItem:
		if value == nil {
			return ProcessingItem{}, false
		}
		return *value, true
	case map[string]any:
		item := ProcessingItem{
			SourcePath: anyString(value["source_path"]),
			FolderID:   anyString(value["folder_id"]),
			FolderName: anyString(value["folder_name"]),
			TargetName: anyString(value["target_name"]),
			Category:   anyString(value["category"]),
			ParentPath: anyString(value["parent_path"]),
		}
		return item, true
	default:
		return ProcessingItem{}, false
	}
}

func categoryRouterPortIndex(category string) int {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "video":
		return 0
	case "manga":
		return 1
	case "photo":
		return 2
	case "mixed":
		return 4
	default:
		return 3
	}
}
