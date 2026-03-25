package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
	"github.com/liqiye/classifier/internal/sse"
)

type StartWorkflowJobInput struct {
	WorkflowDefID string
	FolderIDs     []string
}

type WorkflowRunDetail struct {
	Run      *repository.WorkflowRun
	NodeRuns []*repository.NodeRun
}

type NodeExecutionInput struct {
	WorkflowRun *repository.WorkflowRun
	NodeRun     *repository.NodeRun
	Node        repository.WorkflowGraphNode
	Folder      *repository.Folder
	Inputs      map[string]any
}

type ExecutionStatus string

const (
	ExecutionSuccess ExecutionStatus = "success"
	ExecutionFailure ExecutionStatus = "failure"
	ExecutionPending ExecutionStatus = "pending"
)

type NodeExecutionOutput struct {
	Outputs       []any
	Status        ExecutionStatus
	PendingReason string
}

type NodeSchemaPort struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type NodeSchema struct {
	Type         string           `json:"type"`
	Label        string           `json:"label"`
	Description  string           `json:"description"`
	InputPorts   []NodeSchemaPort `json:"input_ports"`
	OutputPorts  []NodeSchemaPort `json:"output_ports"`
	ConfigSchema map[string]any   `json:"config_schema,omitempty"`
}

type NodeRollbackInput struct {
	WorkflowRun *repository.WorkflowRun
	NodeRun     *repository.NodeRun
	Snapshots   []*repository.NodeSnapshot
	Folder      *repository.Folder
}

type WorkflowNodeExecutor interface {
	Type() string
	Schema() NodeSchema
	Execute(ctx context.Context, input NodeExecutionInput) (NodeExecutionOutput, error)
	Resume(ctx context.Context, input NodeExecutionInput, resumeData map[string]any) (NodeExecutionOutput, error)
	Rollback(ctx context.Context, input NodeRollbackInput) error
}

type WorkflowRunnerService struct {
	jobs              repository.JobRepository
	folders           repository.FolderRepository
	workflowDefs      repository.WorkflowDefinitionRepository
	workflowRuns      repository.WorkflowRunRepository
	nodeRuns          repository.NodeRunRepository
	nodeSnapshots     repository.NodeSnapshotRepository
	executors         map[string]WorkflowNodeExecutor
	broker            *sse.Broker
	pendingResumeData map[string]map[string]any
	pendingResumeMu   sync.Mutex
}

func NewWorkflowRunnerService(
	jobRepo repository.JobRepository,
	folderRepo repository.FolderRepository,
	workflowDefRepo repository.WorkflowDefinitionRepository,
	workflowRunRepo repository.WorkflowRunRepository,
	nodeRunRepo repository.NodeRunRepository,
	nodeSnapshotRepo repository.NodeSnapshotRepository,
	fsAdapter fs.FSAdapter,
	broker *sse.Broker,
	auditSvc *AuditService,
) *WorkflowRunnerService {
	svc := &WorkflowRunnerService{
		jobs:              jobRepo,
		folders:           folderRepo,
		workflowDefs:      workflowDefRepo,
		workflowRuns:      workflowRunRepo,
		nodeRuns:          nodeRunRepo,
		nodeSnapshots:     nodeSnapshotRepo,
		executors:         make(map[string]WorkflowNodeExecutor),
		broker:            broker,
		pendingResumeData: make(map[string]map[string]any),
	}

	svc.RegisterExecutor(&triggerNodeExecutor{})
	svc.RegisterExecutor(&extRatioClassifierNodeExecutor{fs: fsAdapter})
	svc.RegisterExecutor(newManualClassifierExecutor())
	svc.RegisterExecutor(newClassificationReaderExecutor())
	svc.RegisterExecutor(newFolderSplitterExecutor())
	svc.RegisterExecutor(newCategoryRouterExecutor())
	svc.RegisterExecutor(newRenameNodeExecutor())
	svc.RegisterExecutor(&moveNodeExecutor{fs: fsAdapter, folders: folderRepo})
	svc.RegisterExecutor(newPhase4MoveNodeExecutor(fsAdapter, folderRepo))
	svc.RegisterExecutor(newThumbnailNodeExecutor(fsAdapter, folderRepo))
	svc.RegisterExecutor(newCompressNodeExecutor(fsAdapter))
	svc.RegisterExecutor(newAuditLogNodeExecutor(auditSvc))

	return svc
}

func (s *WorkflowRunnerService) RegisterExecutor(executor WorkflowNodeExecutor) {
	if executor == nil {
		return
	}

	s.executors[executor.Type()] = executor
}

func (s *WorkflowRunnerService) ListNodeSchemas() []NodeSchema {
	schemas := make([]NodeSchema, 0, len(s.executors))
	for _, executor := range s.executors {
		schemas = append(schemas, executor.Schema())
	}

	sort.Slice(schemas, func(i, j int) bool {
		return schemas[i].Type < schemas[j].Type
	})

	return schemas
}

func (s *WorkflowRunnerService) StartJob(ctx context.Context, input StartWorkflowJobInput) (string, error) {
	if input.WorkflowDefID == "" {
		return "", fmt.Errorf("workflowRunner.StartJob: workflow_def_id is required")
	}
	if len(input.FolderIDs) == 0 {
		return "", fmt.Errorf("workflowRunner.StartJob: folder_ids is required")
	}

	if _, err := s.workflowDefs.GetByID(ctx, input.WorkflowDefID); err != nil {
		return "", fmt.Errorf("workflowRunner.StartJob get workflow def %q: %w", input.WorkflowDefID, err)
	}

	folderIDsJSON, err := json.Marshal(input.FolderIDs)
	if err != nil {
		return "", fmt.Errorf("workflowRunner.StartJob marshal folder_ids: %w", err)
	}

	jobID := uuid.NewString()
	if err := s.jobs.Create(ctx, &repository.Job{
		ID:            jobID,
		Type:          "workflow",
		WorkflowDefID: input.WorkflowDefID,
		Status:        "pending",
		FolderIDs:     string(folderIDsJSON),
		Total:         len(input.FolderIDs),
	}); err != nil {
		return "", fmt.Errorf("workflowRunner.StartJob create job: %w", err)
	}

	go s.runJob(context.Background(), jobID, input.WorkflowDefID, append([]string(nil), input.FolderIDs...))
	return jobID, nil
}

func (s *WorkflowRunnerService) runJob(ctx context.Context, jobID, workflowDefID string, folderIDs []string) {
	_ = s.jobs.UpdateStatus(ctx, jobID, "running", "")

	failed := 0
	for _, folderID := range folderIDs {
		run := &repository.WorkflowRun{
			ID:            uuid.NewString(),
			JobID:         jobID,
			FolderID:      folderID,
			WorkflowDefID: workflowDefID,
			Status:        "pending",
		}
		if err := s.workflowRuns.Create(ctx, run); err != nil {
			failed++
			_ = s.jobs.IncrementProgress(ctx, jobID, 0, 1)
			continue
		}

		if err := s.executeWorkflowRun(ctx, run.ID, false); err != nil {
			failed++
			_ = s.jobs.IncrementProgress(ctx, jobID, 0, 1)
			continue
		}

		_ = s.jobs.IncrementProgress(ctx, jobID, 1, 0)
	}

	status := "succeeded"
	if failed == len(folderIDs) {
		status = "failed"
	} else if failed > 0 {
		status = "partial"
	}
	_ = s.jobs.UpdateStatus(ctx, jobID, status, "")
}

func (s *WorkflowRunnerService) ListWorkflowRuns(ctx context.Context, jobID string, page, limit int) ([]*repository.WorkflowRun, int, error) {
	items, total, err := s.workflowRuns.List(ctx, repository.WorkflowRunListFilter{
		JobID: jobID,
		Page:  page,
		Limit: limit,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("workflowRunner.ListWorkflowRuns: %w", err)
	}

	return items, total, nil
}

func (s *WorkflowRunnerService) GetWorkflowRunDetail(ctx context.Context, workflowRunID string) (*WorkflowRunDetail, error) {
	run, err := s.workflowRuns.GetByID(ctx, workflowRunID)
	if err != nil {
		return nil, fmt.Errorf("workflowRunner.GetWorkflowRunDetail get run %q: %w", workflowRunID, err)
	}
	nodeRuns, _, err := s.nodeRuns.List(ctx, repository.NodeRunListFilter{WorkflowRunID: workflowRunID, Page: 1, Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("workflowRunner.GetWorkflowRunDetail list node runs %q: %w", workflowRunID, err)
	}

	return &WorkflowRunDetail{Run: run, NodeRuns: nodeRuns}, nil
}

func (s *WorkflowRunnerService) ResumeWorkflowRun(ctx context.Context, workflowRunID string) error {
	if err := s.ResumeWorkflowRunWithData(ctx, workflowRunID, nil); err != nil {
		return fmt.Errorf("workflowRunner.ResumeWorkflowRun: %w", err)
	}

	return nil
}

func (s *WorkflowRunnerService) ResumeWorkflowRunWithData(ctx context.Context, workflowRunID string, resumeData map[string]any) error {
	s.pendingResumeMu.Lock()
	if resumeData == nil {
		delete(s.pendingResumeData, workflowRunID)
	} else {
		copied := make(map[string]any, len(resumeData))
		for key, value := range resumeData {
			copied[key] = value
		}
		s.pendingResumeData[workflowRunID] = copied
	}
	s.pendingResumeMu.Unlock()

	if err := s.executeWorkflowRun(ctx, workflowRunID, true); err != nil {
		return fmt.Errorf("workflowRunner.ResumeWorkflowRunWithData: %w", err)
	}

	return nil
}

func (s *WorkflowRunnerService) RollbackWorkflowRun(ctx context.Context, workflowRunID string) error {
	run, err := s.workflowRuns.GetByID(ctx, workflowRunID)
	if err != nil {
		return fmt.Errorf("workflowRunner.RollbackWorkflowRun get workflow run %q: %w", workflowRunID, err)
	}

	nodeRuns, _, err := s.nodeRuns.List(ctx, repository.NodeRunListFilter{WorkflowRunID: workflowRunID, Page: 1, Limit: 2000})
	if err != nil {
		return fmt.Errorf("workflowRunner.RollbackWorkflowRun list node runs %q: %w", workflowRunID, err)
	}
	sort.Slice(nodeRuns, func(i, j int) bool {
		return nodeRuns[i].Sequence > nodeRuns[j].Sequence
	})

	folder, err := s.folders.GetByID(ctx, run.FolderID)
	if err != nil {
		return fmt.Errorf("workflowRunner.RollbackWorkflowRun get folder %q: %w", run.FolderID, err)
	}

	for _, nodeRun := range nodeRuns {
		if nodeRun.Status != "succeeded" {
			continue
		}
		executor, ok := s.executors[nodeRun.NodeType]
		if !ok {
			continue
		}
		snaps, snapErr := s.nodeSnapshots.ListByNodeRunID(ctx, nodeRun.ID)
		if snapErr != nil {
			return fmt.Errorf("workflowRunner.RollbackWorkflowRun list snapshots for node run %q: %w", nodeRun.ID, snapErr)
		}
		if rbErr := executor.Rollback(ctx, NodeRollbackInput{
			WorkflowRun: run,
			NodeRun:     nodeRun,
			Snapshots:   snaps,
			Folder:      folder,
		}); rbErr != nil {
			return fmt.Errorf("workflowRunner.RollbackWorkflowRun rollback node %q: %w", nodeRun.NodeID, rbErr)
		}

		folder, err = s.folders.GetByID(ctx, run.FolderID)
		if err != nil {
			return fmt.Errorf("workflowRunner.RollbackWorkflowRun reload folder %q: %w", run.FolderID, err)
		}
	}

	if err := s.workflowRuns.UpdateStatus(ctx, workflowRunID, "rolled_back", ""); err != nil {
		return fmt.Errorf("workflowRunner.RollbackWorkflowRun update workflow run status %q: %w", workflowRunID, err)
	}

	return nil
}

func (s *WorkflowRunnerService) executeWorkflowRun(ctx context.Context, workflowRunID string, resume bool) error {
	run, err := s.workflowRuns.GetByID(ctx, workflowRunID)
	if err != nil {
		return fmt.Errorf("workflowRunner.executeWorkflowRun get workflow run %q: %w", workflowRunID, err)
	}

	if err := s.workflowRuns.UpdateStatus(ctx, run.ID, "running", run.ResumeNodeID); err != nil {
		return fmt.Errorf("workflowRunner.executeWorkflowRun set running for %q: %w", run.ID, err)
	}

	def, err := s.workflowDefs.GetByID(ctx, run.WorkflowDefID)
	if err != nil {
		return fmt.Errorf("workflowRunner.executeWorkflowRun get workflow def %q: %w", run.WorkflowDefID, err)
	}

	graph, err := parseWorkflowGraph(def.GraphJSON)
	if err != nil {
		return fmt.Errorf("workflowRunner.executeWorkflowRun parse graph for workflow def %q: %w", def.ID, err)
	}

	nodes, err := topologicalNodes(graph)
	if err != nil {
		return fmt.Errorf("workflowRunner.executeWorkflowRun topo sort for workflow def %q: %w", def.ID, err)
	}

	folder, err := s.folders.GetByID(ctx, run.FolderID)
	if err != nil {
		return fmt.Errorf("workflowRunner.executeWorkflowRun get folder %q: %w", run.FolderID, err)
	}

	existingRuns, _, err := s.nodeRuns.List(ctx, repository.NodeRunListFilter{WorkflowRunID: run.ID, Page: 1, Limit: 2000})
	if err != nil {
		return fmt.Errorf("workflowRunner.executeWorkflowRun list node runs for workflow run %q: %w", run.ID, err)
	}
	outputCache := make(map[string][]any)
	for _, existingRun := range existingRuns {
		if existingRun.NodeID == "" || strings.TrimSpace(existingRun.OutputJSON) == "" {
			continue
		}
		outputs, parseErr := parseNodeOutputs(existingRun.OutputJSON)
		if parseErr != nil {
			continue
		}
		outputCache[existingRun.NodeID] = outputs
	}
	seq := len(existingRuns)

	resumeNodeID := ""
	if resume {
		resumeNodeID = run.ResumeNodeID
	}
	startNow := resumeNodeID == ""

	var resumeData map[string]any
	if resume {
		s.pendingResumeMu.Lock()
		resumeData = s.pendingResumeData[workflowRunID]
		delete(s.pendingResumeData, workflowRunID)
		s.pendingResumeMu.Unlock()
	}

	for _, node := range nodes {
		if !startNow {
			if node.ID == resumeNodeID {
				startNow = true
			} else {
				continue
			}
		}

		inputs := make(map[string]any)
		for portName, spec := range node.Inputs {
			if spec.ConstValue != nil {
				inputs[portName] = *spec.ConstValue
				continue
			}
			if spec.LinkSource == nil {
				continue
			}

			sourceOutputs := outputCache[spec.LinkSource.SourceNodeID]
			if spec.LinkSource.OutputPortIndex < 0 || spec.LinkSource.OutputPortIndex >= len(sourceOutputs) {
				inputs[portName] = nil
				continue
			}

			inputs[portName] = sourceOutputs[spec.LinkSource.OutputPortIndex]
		}

		// Skip nodes whose required inputs resolved to nil (upstream chose the other branch)
		if len(inputs) > 0 && hasNilRequiredInput(inputs) {
			seq++
			skippedRun := &repository.NodeRun{
				ID:            uuid.NewString(),
				WorkflowRunID: run.ID,
				NodeID:        node.ID,
				NodeType:      node.Type,
				Sequence:      seq,
				Status:        "pending",
			}
			if err := s.nodeRuns.Create(ctx, skippedRun); err != nil {
				_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
				return fmt.Errorf("workflowRunner.executeWorkflowRun create skipped node run %q: %w", node.ID, err)
			}
			if err := s.nodeRuns.UpdateFinish(ctx, skippedRun.ID, "skipped", "{}", ""); err != nil {
				_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
				return fmt.Errorf("workflowRunner.executeWorkflowRun finish skipped node run %q: %w", node.ID, err)
			}
			outputCache[node.ID] = nil
			continue
		}

		seq++
		nodeRun := &repository.NodeRun{
			ID:            uuid.NewString(),
			WorkflowRunID: run.ID,
			NodeID:        node.ID,
			NodeType:      node.Type,
			Sequence:      seq,
			Status:        "pending",
		}
		if err := s.nodeRuns.Create(ctx, nodeRun); err != nil {
			_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
			return fmt.Errorf("workflowRunner.executeWorkflowRun create node run for node %q: %w", node.ID, err)
		}

		inputJSON, err := json.Marshal(map[string]any{
			"workflow_run_id": run.ID,
			"folder_id":       folder.ID,
			"folder_path":     folder.Path,
			"node":            node,
			"inputs":          inputs,
		})
		if err != nil {
			_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
			return fmt.Errorf("workflowRunner.executeWorkflowRun marshal node input for node %q: %w", node.ID, err)
		}

		if err := s.nodeRuns.UpdateStart(ctx, nodeRun.ID, string(inputJSON)); err != nil {
			_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
			return fmt.Errorf("workflowRunner.executeWorkflowRun update start for node run %q: %w", nodeRun.ID, err)
		}

		if err := s.createNodeSnapshot(ctx, run, nodeRun, "pre", folder, nil); err != nil {
			_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
			return fmt.Errorf("workflowRunner.executeWorkflowRun create pre snapshot for node %q: %w", node.ID, err)
		}

		s.publish("workflow_run.node_started", map[string]any{
			"job_id":          run.JobID,
			"workflow_run_id": run.ID,
			"folder_id":       run.FolderID,
			"node_run_id":     nodeRun.ID,
			"node_id":         node.ID,
			"node_type":       node.Type,
			"sequence":        nodeRun.Sequence,
		})

		executor, ok := s.executors[node.Type]
		if !ok {
			err := fmt.Errorf("workflowRunner.executeWorkflowRun: executor not found for node type %q", node.Type)
			_ = s.nodeRuns.UpdateFinish(ctx, nodeRun.ID, "failed", "", err.Error())
			_ = s.createNodeSnapshot(ctx, run, nodeRun, "post", folder, []any{map[string]any{"error": err.Error()}})
			_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
			s.publish("workflow_run.node_failed", map[string]any{
				"job_id":          run.JobID,
				"workflow_run_id": run.ID,
				"folder_id":       run.FolderID,
				"node_run_id":     nodeRun.ID,
				"node_id":         node.ID,
				"node_type":       node.Type,
				"error":           err.Error(),
			})
			return err
		}

		execInput := NodeExecutionInput{
			WorkflowRun: run,
			NodeRun:     nodeRun,
			Node:        node,
			Folder:      folder,
			Inputs:      inputs,
		}

		var execOutput NodeExecutionOutput
		var execErr error
		if resume && node.ID == resumeNodeID {
			execOutput, execErr = executor.Resume(ctx, execInput, resumeData)
		} else {
			execOutput, execErr = executor.Execute(ctx, execInput)
		}
		if execErr != nil {
			_ = s.nodeRuns.UpdateFinish(ctx, nodeRun.ID, "failed", "", execErr.Error())
			_ = s.createNodeSnapshot(ctx, run, nodeRun, "post", folder, []any{map[string]any{"error": execErr.Error()}})
			_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
			s.publish("workflow_run.node_failed", map[string]any{
				"job_id":          run.JobID,
				"workflow_run_id": run.ID,
				"folder_id":       run.FolderID,
				"node_run_id":     nodeRun.ID,
				"node_id":         node.ID,
				"node_type":       node.Type,
				"error":           execErr.Error(),
			})
			return fmt.Errorf("workflowRunner.executeWorkflowRun execute node %q: %w", node.ID, execErr)
		}

		if execOutput.Status == "" {
			execOutput.Status = ExecutionSuccess
		}

		outputJSON, marshalErr := json.Marshal(execOutput.Outputs)
		if marshalErr != nil {
			_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
			return fmt.Errorf("workflowRunner.executeWorkflowRun marshal node output for node %q: %w", node.ID, marshalErr)
		}

		if execOutput.Status == ExecutionPending {
			errMsg := execOutput.PendingReason
			if err := s.nodeRuns.UpdateFinish(ctx, nodeRun.ID, "waiting_input", string(outputJSON), errMsg); err != nil {
				_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
				return fmt.Errorf("workflowRunner.executeWorkflowRun update waiting_input for node run %q: %w", nodeRun.ID, err)
			}

			if err := s.workflowRuns.UpdateStatus(ctx, run.ID, "waiting_input", node.ID); err != nil {
				return fmt.Errorf("workflowRunner.executeWorkflowRun set waiting_input for %q: %w", run.ID, err)
			}

			s.publish("workflow_run.node_pending", map[string]any{
				"job_id":          run.JobID,
				"workflow_run_id": run.ID,
				"folder_id":       run.FolderID,
				"node_run_id":     nodeRun.ID,
				"node_id":         node.ID,
				"node_type":       node.Type,
				"error":           execOutput.PendingReason,
			})

			return nil
		}
		if execOutput.Status == ExecutionFailure {
			errMsg := execOutput.PendingReason
			if strings.TrimSpace(errMsg) == "" {
				errMsg = "node returned failure status"
			}
			_ = s.nodeRuns.UpdateFinish(ctx, nodeRun.ID, "failed", string(outputJSON), errMsg)
			_ = s.createNodeSnapshot(ctx, run, nodeRun, "post", folder, []any{map[string]any{"error": errMsg}})
			_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
			s.publish("workflow_run.node_failed", map[string]any{
				"job_id":          run.JobID,
				"workflow_run_id": run.ID,
				"folder_id":       run.FolderID,
				"node_run_id":     nodeRun.ID,
				"node_id":         node.ID,
				"node_type":       node.Type,
				"error":           errMsg,
			})
			return fmt.Errorf("workflowRunner.executeWorkflowRun execute node %q: %s", node.ID, errMsg)
		}

		if err := s.nodeRuns.UpdateFinish(ctx, nodeRun.ID, "succeeded", string(outputJSON), ""); err != nil {
			_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
			return fmt.Errorf("workflowRunner.executeWorkflowRun update finish for node run %q: %w", nodeRun.ID, err)
		}

		outputCache[node.ID] = append([]any(nil), execOutput.Outputs...)

		folder, err = s.folders.GetByID(ctx, run.FolderID)
		if err != nil {
			_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
			return fmt.Errorf("workflowRunner.executeWorkflowRun reload folder %q after node %q: %w", run.FolderID, node.ID, err)
		}

		if err := s.createNodeSnapshot(ctx, run, nodeRun, "post", folder, execOutput.Outputs); err != nil {
			_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
			return fmt.Errorf("workflowRunner.executeWorkflowRun create post snapshot for node %q: %w", node.ID, err)
		}

		s.publish("workflow_run.node_done", map[string]any{
			"job_id":          run.JobID,
			"workflow_run_id": run.ID,
			"folder_id":       run.FolderID,
			"node_run_id":     nodeRun.ID,
			"node_id":         node.ID,
			"node_type":       node.Type,
			"sequence":        nodeRun.Sequence,
		})
	}

	if err := s.workflowRuns.UpdateStatus(ctx, run.ID, "succeeded", ""); err != nil {
		return fmt.Errorf("workflowRunner.executeWorkflowRun set succeeded for %q: %w", run.ID, err)
	}

	return nil
}

func (s *WorkflowRunnerService) createNodeSnapshot(
	ctx context.Context,
	run *repository.WorkflowRun,
	nodeRun *repository.NodeRun,
	kind string,
	folder *repository.Folder,
	outputs []any,
) error {
	manifestJSON, err := json.Marshal(map[string]any{
		"folder_id":   folder.ID,
		"folder_path": folder.Path,
		"name":        folder.Name,
		"category":    folder.Category,
		"status":      folder.Status,
	})
	if err != nil {
		return fmt.Errorf("workflowRunner.createNodeSnapshot marshal fs manifest for node run %q: %w", nodeRun.ID, err)
	}

	outputJSON := ""
	if outputs != nil {
		data, marshalErr := json.Marshal(map[string]any{"outputs": outputs})
		if marshalErr != nil {
			return fmt.Errorf("workflowRunner.createNodeSnapshot marshal output for node run %q: %w", nodeRun.ID, marshalErr)
		}
		outputJSON = string(data)
	}

	if err := s.nodeSnapshots.Create(ctx, &repository.NodeSnapshot{
		ID:            uuid.NewString(),
		NodeRunID:     nodeRun.ID,
		WorkflowRunID: run.ID,
		Kind:          kind,
		FSManifest:    string(manifestJSON),
		OutputJSON:    outputJSON,
	}); err != nil {
		return fmt.Errorf("workflowRunner.createNodeSnapshot create snapshot for node run %q: %w", nodeRun.ID, err)
	}

	return nil
}

func (s *WorkflowRunnerService) publish(eventType string, payload any) {
	if s.broker == nil {
		return
	}

	_ = s.broker.Publish(eventType, payload)
}

func parseWorkflowGraph(graphJSON string) (*repository.WorkflowGraph, error) {
	if strings.TrimSpace(graphJSON) == "" {
		return nil, fmt.Errorf("parseWorkflowGraph: graph_json is empty")
	}

	var raw struct {
		Nodes []struct {
			ID         string                              `json:"id"`
			Type       string                              `json:"type"`
			Label      string                              `json:"label"`
			Config     map[string]any                      `json:"config"`
			Inputs     map[string]repository.NodeInputSpec `json:"inputs"`
			UIPosition *repository.NodeUIPosition          `json:"ui_position"`
			Enabled    *bool                               `json:"enabled"`
		} `json:"nodes"`
		Edges []repository.WorkflowGraphEdge `json:"edges"`
	}
	if err := json.Unmarshal([]byte(graphJSON), &raw); err != nil {
		return nil, fmt.Errorf("parseWorkflowGraph: %w", err)
	}
	if len(raw.Nodes) == 0 {
		return nil, fmt.Errorf("parseWorkflowGraph: nodes is empty")
	}

	filtered := make([]repository.WorkflowGraphNode, 0, len(raw.Nodes))
	for _, node := range raw.Nodes {
		enabled := true
		if node.Enabled != nil {
			enabled = *node.Enabled
		}
		if !enabled {
			continue
		}
		filtered = append(filtered, repository.WorkflowGraphNode{
			ID:         node.ID,
			Type:       node.Type,
			Label:      node.Label,
			Config:     node.Config,
			Inputs:     node.Inputs,
			UIPosition: node.UIPosition,
			Enabled:    enabled,
		})
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("parseWorkflowGraph: all nodes are disabled")
	}

	graph := &repository.WorkflowGraph{
		Nodes: filtered,
		Edges: raw.Edges,
	}

	return graph, nil
}

func parseNodeOutputs(rawOutput string) ([]any, error) {
	var outputs []any
	if err := json.Unmarshal([]byte(rawOutput), &outputs); err == nil {
		return outputs, nil
	}

	var wrapped struct {
		Outputs []any `json:"outputs"`
	}
	if err := json.Unmarshal([]byte(rawOutput), &wrapped); err != nil {
		return nil, err
	}

	return wrapped.Outputs, nil
}

func topologicalNodes(graph *repository.WorkflowGraph) ([]repository.WorkflowGraphNode, error) {
	nodeMap := make(map[string]repository.WorkflowGraphNode, len(graph.Nodes))
	indegree := make(map[string]int, len(graph.Nodes))
	order := make([]string, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if node.ID == "" {
			return nil, fmt.Errorf("topologicalNodes: node id is empty")
		}
		if _, ok := nodeMap[node.ID]; ok {
			return nil, fmt.Errorf("topologicalNodes: duplicate node id %q", node.ID)
		}
		nodeMap[node.ID] = node
		indegree[node.ID] = 0
		order = append(order, node.ID)
	}

	adj := make(map[string][]string, len(graph.Nodes))
	for _, edge := range graph.Edges {
		if _, ok := nodeMap[edge.Source]; !ok {
			continue
		}
		if _, ok := nodeMap[edge.Target]; !ok {
			continue
		}
		adj[edge.Source] = append(adj[edge.Source], edge.Target)
		indegree[edge.Target]++
	}

	queue := make([]string, 0)
	for _, id := range order {
		if indegree[id] == 0 {
			queue = append(queue, id)
		}
	}

	out := make([]repository.WorkflowGraphNode, 0, len(graph.Nodes))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		out = append(out, nodeMap[id])

		for _, target := range adj[id] {
			indegree[target]--
			if indegree[target] == 0 {
				queue = append(queue, target)
			}
		}
	}

	if len(out) != len(graph.Nodes) {
		return nil, fmt.Errorf("topologicalNodes: cycle detected")
	}

	return out, nil
}

type triggerNodeExecutor struct{}

func (e *triggerNodeExecutor) Type() string {
	return "trigger"
}

func (e *triggerNodeExecutor) Schema() NodeSchema {
	return NodeSchema{Type: e.Type(), Label: "Trigger", Description: "Trigger node"}
}

func (e *triggerNodeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	if input.Folder == nil {
		return NodeExecutionOutput{Outputs: []any{nil}, Status: ExecutionSuccess}, nil
	}

	return NodeExecutionOutput{Outputs: []any{input.Folder}, Status: ExecutionSuccess}, nil
}

func (e *triggerNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *triggerNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

type extRatioClassifierNodeExecutor struct {
	fs fs.FSAdapter
}

func (e *extRatioClassifierNodeExecutor) Type() string {
	return "ext-ratio-classifier"
}

func (e *extRatioClassifierNodeExecutor) Schema() NodeSchema {
	return NodeSchema{Type: e.Type(), Label: "Ext Ratio Classifier", Description: "Classify folder by extension ratio"}
}

func (e *extRatioClassifierNodeExecutor) Execute(ctx context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	if input.Folder == nil {
		return NodeExecutionOutput{Outputs: []any{nil}, Status: ExecutionSuccess}, nil
	}

	entries, err := e.fs.ReadDir(ctx, input.Folder.Path)
	if err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("extRatioClassifier.Execute read dir %q: %w", input.Folder.Path, err)
	}

	fileNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}
		fileNames = append(fileNames, entry.Name)
	}

	category := Classify(input.Folder.Name, fileNames)
	confidence := 0.85
	if category == "other" {
		confidence = 0.5
	}
	reason := fmt.Sprintf("ext-ratio: %s", category)

	return NodeExecutionOutput{Outputs: []any{ClassificationSignal{
		Category:   category,
		Confidence: confidence,
		Reason:     reason,
	}}, Status: ExecutionSuccess}, nil
}

func (e *extRatioClassifierNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *extRatioClassifierNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

type moveNodeExecutor struct {
	fs      fs.FSAdapter
	folders repository.FolderRepository
}

func (e *moveNodeExecutor) Type() string {
	return "move"
}

func (e *moveNodeExecutor) Schema() NodeSchema {
	return NodeSchema{Type: e.Type(), Label: "Move", Description: "Move folder to target directory"}
}

func (e *moveNodeExecutor) Execute(ctx context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	targetDir := stringConfig(input.Node.Config, "target_dir")
	if targetDir == "" {
		targetDir = stringConfig(input.Node.Config, "targetDir")
	}
	if targetDir == "" {
		return NodeExecutionOutput{}, fmt.Errorf("moveNode.Execute: target_dir is required")
	}

	if err := e.fs.MkdirAll(ctx, targetDir, 0o755); err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("moveNode.Execute mkdir %q: %w", targetDir, err)
	}

	dst := filepath.Join(targetDir, input.Folder.Name)
	if err := e.fs.MoveDir(ctx, input.Folder.Path, dst); err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("moveNode.Execute move %q to %q: %w", input.Folder.Path, dst, err)
	}

	if err := e.folders.UpdatePath(ctx, input.Folder.ID, dst); err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("moveNode.Execute update path for folder %q: %w", input.Folder.ID, err)
	}
	input.Folder.Path = dst

	if input.Folder == nil {
		return NodeExecutionOutput{Outputs: []any{nil}, Status: ExecutionSuccess}, nil
	}

	return NodeExecutionOutput{Outputs: []any{input.Folder}, Status: ExecutionSuccess}, nil
}

func (e *moveNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *moveNodeExecutor) Rollback(ctx context.Context, input NodeRollbackInput) error {
	var pre *repository.NodeSnapshot
	var post *repository.NodeSnapshot
	for _, item := range input.Snapshots {
		if item.Kind == "pre" {
			pre = item
		}
		if item.Kind == "post" {
			post = item
		}
	}
	if pre == nil || post == nil {
		return nil
	}

	var preState struct {
		FolderPath string `json:"folder_path"`
	}
	if err := json.Unmarshal([]byte(pre.FSManifest), &preState); err != nil {
		return fmt.Errorf("moveNode.Rollback parse pre manifest for node run %q: %w", input.NodeRun.ID, err)
	}

	var postState struct {
		FolderPath string `json:"folder_path"`
	}
	if err := json.Unmarshal([]byte(post.FSManifest), &postState); err != nil {
		return fmt.Errorf("moveNode.Rollback parse post manifest for node run %q: %w", input.NodeRun.ID, err)
	}

	if preState.FolderPath == "" || postState.FolderPath == "" || preState.FolderPath == postState.FolderPath {
		return nil
	}

	if err := e.fs.MoveDir(ctx, postState.FolderPath, preState.FolderPath); err != nil {
		return fmt.Errorf("moveNode.Rollback move back %q to %q: %w", postState.FolderPath, preState.FolderPath, err)
	}

	if err := e.folders.UpdatePath(ctx, input.Folder.ID, preState.FolderPath); err != nil {
		return fmt.Errorf("moveNode.Rollback update folder path for %q: %w", input.Folder.ID, err)
	}

	return nil
}

func stringConfig(config map[string]any, key string) string {
	if config == nil {
		return ""
	}

	raw, ok := config[key]
	if !ok {
		return ""
	}
	text, ok := raw.(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(text)
}

// hasNilRequiredInput returns true if any input port value is explicitly nil,
// indicating the upstream node chose the other branch (nil-port skip semantics).
func hasNilRequiredInput(inputs map[string]any) bool {
	for _, v := range inputs {
		if v == nil {
			return true
		}
	}

	return false
}
