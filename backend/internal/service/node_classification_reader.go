package service

import (
	"context"
	"fmt"
)

const classificationReaderExecutorType = "classification-reader"

type classificationReaderNodeExecutor struct{}

func newClassificationReaderExecutor() *classificationReaderNodeExecutor {
	return &classificationReaderNodeExecutor{}
}

func NewClassificationReaderExecutor() WorkflowNodeExecutor {
	return newClassificationReaderExecutor()
}

func (e *classificationReaderNodeExecutor) Type() string {
	return classificationReaderExecutorType
}

func (e *classificationReaderNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "Classification Reader",
		Description: "Read classified entry from input or current workflow folder",
		InputPorts: []NodeSchemaPort{
			{Name: "entry", Description: "CLASSIFIED_ENTRY (optional)", Required: false},
			{Name: "job_id", Description: "JOB_ID (optional compatibility)", Required: false},
		},
		OutputPorts: []NodeSchemaPort{
			{Name: "entry", Description: "CLASSIFIED_ENTRY", Required: true},
		},
	}
}

func (e *classificationReaderNodeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	entry, ok := classificationReaderResolveInputEntry(input.Inputs)
	if !ok {
		if input.Folder == nil || input.Folder.ID == "" {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: folder is required when entry input is missing", e.Type())
		}
		entry = ClassifiedEntry{
			FolderID: input.Folder.ID,
			Path:     input.Folder.Path,
			Name:     input.Folder.Name,
			Category: input.Folder.Category,
		}
	}

	if input.Folder != nil {
		if entry.FolderID == "" {
			entry.FolderID = input.Folder.ID
		}
		if entry.Path == "" {
			entry.Path = input.Folder.Path
		}
		if entry.Name == "" {
			entry.Name = input.Folder.Name
		}
		if entry.Category == "" {
			entry.Category = input.Folder.Category
		}
	}

	if entry.Category == "" {
		entry.Category = "other"
	}
	if entry.Classifier == "" {
		entry.Classifier = e.Type()
	}

	return NodeExecutionOutput{Outputs: []any{entry}, Status: ExecutionSuccess}, nil
}

func (e *classificationReaderNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *classificationReaderNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func classificationReaderResolveInputEntry(inputs map[string]any) (ClassifiedEntry, bool) {
	for _, key := range []string{"entry", "classified_entry"} {
		raw, ok := inputs[key]
		if !ok {
			continue
		}

		entry, ok := classificationReaderToEntry(raw)
		if !ok {
			continue
		}

		return entry, true
	}

	return ClassifiedEntry{}, false
}

func classificationReaderToEntry(raw any) (ClassifiedEntry, bool) {
	switch value := raw.(type) {
	case ClassifiedEntry:
		return value, true
	case *ClassifiedEntry:
		if value == nil {
			return ClassifiedEntry{}, false
		}
		return *value, true
	case map[string]any:
		entry := ClassifiedEntry{
			FolderID:   anyString(value["folder_id"]),
			Path:       anyString(value["path"]),
			Name:       anyString(value["name"]),
			Category:   anyString(value["category"]),
			Confidence: asFloat64(value["confidence"]),
			Reason:     anyString(value["reason"]),
			Classifier: anyString(value["classifier"]),
		}
		return entry, true
	default:
		return ClassifiedEntry{}, false
	}
}
