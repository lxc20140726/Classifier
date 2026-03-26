package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/liqiye/classifier/internal/repository"
)

const auditLogNodeExecutorType = "audit-log"

type auditLogNodeExecutor struct {
	audit *AuditService
}

func newAuditLogNodeExecutor(auditSvc *AuditService) *auditLogNodeExecutor {
	return &auditLogNodeExecutor{audit: auditSvc}
}

func (e *auditLogNodeExecutor) Type() string {
	return auditLogNodeExecutorType
}

func (e *auditLogNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "Audit Log",
		Description: "Write structured audit logs for processing results",
		InputPorts: []NodeSchemaPort{
			{Name: "item", Description: "PROCESSING_ITEM or PROCESSING_ITEM[]", Required: false},
			{Name: "result", Description: "Node result payload", Required: false},
		},
		OutputPorts: []NodeSchemaPort{
			{Name: "item", Description: "Pass-through item input", Required: false},
			{Name: "result", Description: "Pass-through result input", Required: false},
		},
	}
}

func (e *auditLogNodeExecutor) Execute(ctx context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	if e.audit == nil {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: audit service is not configured", e.Type())
	}

	rawInputs := typedInputsToAny(input.Inputs)
	items, _, _ := categoryRouterExtractItems(input.Inputs)
	resultPayload := auditLogNodeResolveResultInput(rawInputs)

	detailJSON, err := json.Marshal(map[string]any{
		"node_id":      input.Node.ID,
		"node_type":    input.Node.Type,
		"workflow_run": workflowRunIDFromInput(input.WorkflowRun),
		"item_count":   len(items),
		"result":       resultPayload,
	})
	if err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: marshal detail: %w", e.Type(), err)
	}

	logItem := &repository.AuditLog{
		JobID:      workflowJobIDFromInput(input.WorkflowRun),
		Action:     auditLogNodeAction(input.Node.Config),
		Level:      auditLogNodeLevel(input.Node.Config),
		Result:     auditLogNodeResult(input.Node.Config, resultPayload),
		FolderID:   firstItemFolderID(items),
		FolderPath: firstItemFolderPath(items),
		Detail:     detailJSON,
	}
	if err := e.audit.Write(ctx, logItem); err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: write audit log: %w", e.Type(), err)
	}

	return NodeExecutionOutput{Outputs: map[string]TypedValue{"item": {Type: PortTypeJSON, Value: auditLogNodeResolveItemInput(rawInputs)}, "result": {Type: PortTypeJSON, Value: resultPayload}}, Status: ExecutionSuccess}, nil
}

func (e *auditLogNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *auditLogNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func auditLogNodeResolveItemInput(inputs map[string]any) any {
	if raw, ok := inputs["items"]; ok {
		return raw
	}
	if raw, ok := inputs["item"]; ok {
		return raw
	}

	return nil
}

func auditLogNodeResolveResultInput(inputs map[string]any) any {
	for _, key := range []string{"results", "result", "move_result", "move_results"} {
		if value, ok := inputs[key]; ok {
			return value
		}
	}

	return nil
}

func workflowRunIDFromInput(run *repository.WorkflowRun) string {
	if run == nil {
		return ""
	}

	return run.ID
}

func workflowJobIDFromInput(run *repository.WorkflowRun) string {
	if run == nil {
		return ""
	}

	return run.JobID
}

func firstItemFolderID(items []ProcessingItem) string {
	if len(items) == 0 {
		return ""
	}

	return strings.TrimSpace(items[0].FolderID)
}

func firstItemFolderPath(items []ProcessingItem) string {
	if len(items) == 0 {
		return ""
	}

	return strings.TrimSpace(items[0].SourcePath)
}

func auditLogNodeAction(config map[string]any) string {
	action := strings.TrimSpace(stringConfig(config, "action"))
	if action == "" {
		return "workflow_node"
	}

	return action
}

func auditLogNodeLevel(config map[string]any) string {
	level := strings.TrimSpace(stringConfig(config, "level"))
	if level == "" {
		return "info"
	}

	return level
}

func auditLogNodeResult(config map[string]any, resultPayload any) string {
	configured := strings.TrimSpace(stringConfig(config, "result"))
	if configured != "" {
		return configured
	}

	if typed, ok := resultPayload.(MoveResult); ok {
		if strings.TrimSpace(typed.Status) != "" {
			return strings.TrimSpace(typed.Status)
		}
	}

	if list, ok := resultPayload.([]MoveResult); ok && len(list) > 0 {
		if strings.TrimSpace(list[0].Status) != "" {
			return strings.TrimSpace(list[0].Status)
		}
	}

	return "success"
}
