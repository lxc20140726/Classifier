package service

import (
	"context"
	"fmt"
)

const classificationPreviewExecutorType = "classification-preview"

type classificationPreviewNodeExecutor struct{}

func newClassificationPreviewNodeExecutor() *classificationPreviewNodeExecutor {
	return &classificationPreviewNodeExecutor{}
}

func (e *classificationPreviewNodeExecutor) Type() string {
	return classificationPreviewExecutorType
}

func (e *classificationPreviewNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "分类预览",
		Description: "透传分类条目列表，不暂停工作流；运行完成后可在节点运行记录中查看每个文件夹的分类结果",
		Inputs: []PortDef{
			{Name: "entries", Type: PortTypeClassifiedEntryList, Description: "已分类条目列表", Required: true},
		},
		Outputs: []PortDef{
			{Name: "entries", Type: PortTypeClassifiedEntryList, Description: "已分类条目列表（原样透传）"},
		},
	}
}

func (e *classificationPreviewNodeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	rawInputs := typedInputsToAny(input.Inputs)
	rawEntries, ok := rawInputs["entries"]
	if !ok || rawEntries == nil {
		return NodeExecutionOutput{
			Outputs: map[string]TypedValue{"entries": {Type: PortTypeClassifiedEntryList, Value: []ClassifiedEntry{}}},
			Status:  ExecutionSuccess,
		}, nil
	}

	entries, err := parseClassifiedEntryList(rawEntries)
	if err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: %w", e.Type(), err)
	}

	return NodeExecutionOutput{
		Outputs: map[string]TypedValue{"entries": {Type: PortTypeClassifiedEntryList, Value: entries}},
		Status:  ExecutionSuccess,
	}, nil
}

func (e *classificationPreviewNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *classificationPreviewNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func parseClassifiedEntryList(raw any) ([]ClassifiedEntry, error) {
	if raw == nil {
		return []ClassifiedEntry{}, nil
	}

	if entries, ok := raw.([]ClassifiedEntry); ok {
		return entries, nil
	}

	rawSlice, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("entries must be an array")
	}

	result := make([]ClassifiedEntry, 0, len(rawSlice))
	for i, item := range rawSlice {
		switch v := item.(type) {
		case ClassifiedEntry:
			result = append(result, v)
		case map[string]any:
			entry, err := classifiedEntryFromMap(v)
			if err != nil {
				return nil, fmt.Errorf("entries[%d]: %w", i, err)
			}
			result = append(result, entry)
		default:
			return nil, fmt.Errorf("entries[%d]: unsupported type %T", i, item)
		}
	}
	return result, nil
}

func classifiedEntryFromMap(m map[string]any) (ClassifiedEntry, error) {
	entry := ClassifiedEntry{}
	if v, ok := m["folder_id"].(string); ok {
		entry.FolderID = v
	}
	if v, ok := m["path"].(string); ok {
		entry.Path = v
	}
	if v, ok := m["name"].(string); ok {
		entry.Name = v
	}
	if v, ok := m["category"].(string); ok {
		entry.Category = v
	}
	if v, ok := m["confidence"].(float64); ok {
		entry.Confidence = v
	}
	if v, ok := m["reason"].(string); ok {
		entry.Reason = v
	}
	if v, ok := m["classifier"].(string); ok {
		entry.Classifier = v
	}
	return entry, nil
}
