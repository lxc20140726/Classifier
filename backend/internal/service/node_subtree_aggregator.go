package service

import (
	"context"
	"fmt"

	"github.com/liqiye/classifier/internal/repository"
)

const subtreeAggregatorExecutorType = "subtree-aggregator"

type subtreeAggregatorNodeExecutor struct {
	folders repository.FolderRepository
	audit   *AuditService
}

func newSubtreeAggregatorExecutor(folders repository.FolderRepository, audit *AuditService) *subtreeAggregatorNodeExecutor {
	return &subtreeAggregatorNodeExecutor{folders: folders, audit: audit}
}

func NewSubtreeAggregatorExecutor(folders repository.FolderRepository, audit *AuditService) WorkflowNodeExecutor {
	return newSubtreeAggregatorExecutor(folders, audit)
}

func (e *subtreeAggregatorNodeExecutor) Type() string {
	return subtreeAggregatorExecutorType
}

func (e *subtreeAggregatorNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "Subtree Aggregator",
		Description: "Aggregate classification signals and persist folder category",
		InputPorts: []NodeSchemaPort{
			{Name: "node", Description: "FOLDER_TREE_NODE", Required: false},
			{Name: "signal_kw", Description: "CLASSIFICATION_SIGNAL from name-keyword-classifier", Required: false},
			{Name: "signal_ft", Description: "CLASSIFICATION_SIGNAL from file-tree-classifier", Required: false},
			{Name: "signal_ext", Description: "CLASSIFICATION_SIGNAL from ext-ratio-classifier", Required: false},
			{Name: "signal_high", Description: "CLASSIFICATION_SIGNAL from confidence-check high port", Required: false},
			{Name: "signal_manual", Description: "CLASSIFICATION_SIGNAL from manual-classifier", Required: false},
		},
		OutputPorts: []NodeSchemaPort{
			{Name: "entry", Description: "CLASSIFIED_ENTRY", Required: false},
		},
	}
}

func (e *subtreeAggregatorNodeExecutor) Execute(ctx context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	if input.Folder == nil || input.Folder.ID == "" {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: folder is required", e.Type())
	}

	if e.folders == nil {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: folder repository is required", e.Type())
	}

	selectedSignal := firstAvailableSignal(input.Inputs)

	category := selectedSignal.Category
	if category == "" {
		category = input.Folder.Category
	}
	if category == "" {
		category = "other"
	}

	if err := e.folders.UpdateCategory(ctx, input.Folder.ID, category, "workflow"); err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute update category for folder %q: %w", e.Type(), input.Folder.ID, err)
	}

	entry := ClassifiedEntry{
		FolderID:   input.Folder.ID,
		Path:       input.Folder.Path,
		Name:       input.Folder.Name,
		Category:   category,
		Confidence: selectedSignal.Confidence,
		Reason:     selectedSignal.Reason,
		Classifier: e.Type(),
	}

	if rawNode, ok := input.Inputs["node"]; ok {
		switch v := rawNode.(type) {
		case FolderTree:
			if v.Path != "" {
				entry.Path = v.Path
			}
			if v.Name != "" {
				entry.Name = v.Name
			}
			entry.Files = append([]FileEntry(nil), v.Files...)
		case *FolderTree:
			if v != nil {
				if v.Path != "" {
					entry.Path = v.Path
				}
				if v.Name != "" {
					entry.Name = v.Name
				}
				entry.Files = append([]FileEntry(nil), v.Files...)
			}
		}
	}

	if e.audit != nil {
		log := &repository.AuditLog{
			JobID:      workflowRunID(input.WorkflowRun),
			FolderID:   input.Folder.ID,
			FolderPath: entry.Path,
			Action:     e.Type(),
			Result:     "success",
		}
		if err := e.audit.Write(ctx, log); err != nil {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute write audit for folder %q: %w", e.Type(), input.Folder.ID, err)
		}
	}

	return NodeExecutionOutput{Outputs: []any{entry}, Status: ExecutionSuccess}, nil
}

func (e *subtreeAggregatorNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *subtreeAggregatorNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func firstAvailableSignal(inputs map[string]any) ClassificationSignal {
	for _, key := range []string{"signal_kw", "signal_ft", "signal_ext", "signal_manual", "signal_high"} {
		raw, ok := inputs[key]
		if !ok {
			continue
		}

		signal, ok := toSignal(raw)
		if !ok {
			continue
		}

		if signal.IsEmpty || signal.Category == "" {
			continue
		}

		return signal
	}

	return ClassificationSignal{}
}

func toSignal(raw any) (ClassificationSignal, bool) {
	switch v := raw.(type) {
	case ClassificationSignal:
		return v, true
	case *ClassificationSignal:
		if v == nil {
			return ClassificationSignal{}, false
		}
		return *v, true
	case map[string]any:
		category, _ := v["category"].(string)
		reason, _ := v["reason"].(string)
		isEmpty, _ := v["is_empty"].(bool)
		return ClassificationSignal{
			Category:   category,
			Confidence: asFloat64(v["confidence"]),
			Reason:     reason,
			IsEmpty:    isEmpty,
		}, true
	default:
		return ClassificationSignal{}, false
	}
}

func asFloat64(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	default:
		return 0
	}
}

func workflowRunID(run *repository.WorkflowRun) string {
	if run == nil {
		return ""
	}

	return run.ID
}
