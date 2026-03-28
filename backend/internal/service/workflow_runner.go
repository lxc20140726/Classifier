package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
	"github.com/liqiye/classifier/internal/sse"
)

type StartWorkflowJobInput struct {
	WorkflowDefID string
	SourceDir     string
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
	SourceDir   string
	Inputs      map[string]*TypedValue
}

type ExecutionStatus string

const (
	ExecutionSuccess ExecutionStatus = "success"
	ExecutionFailure ExecutionStatus = "failure"
	ExecutionPending ExecutionStatus = "pending"
)

type NodeExecutionOutput struct {
	Outputs       map[string]TypedValue
	Status        ExecutionStatus
	PendingReason string
}

type PortDef struct {
	Name        string   `json:"name"`
	Type        PortType `json:"type"`
	Required    bool     `json:"required"`
	Lazy        bool     `json:"lazy"`
	Description string   `json:"description"`
}

type NodeSchema struct {
	Type         string         `json:"type,omitempty"`
	TypeID       string         `json:"type_id,omitempty"`
	Label        string         `json:"label,omitempty"`
	DisplayName  string         `json:"display_name,omitempty"`
	Description  string         `json:"description"`
	Category     string         `json:"category,omitempty"`
	Inputs       []PortDef      `json:"input_ports,omitempty"`
	Outputs      []PortDef      `json:"output_ports,omitempty"`
	ConfigSchema map[string]any `json:"config_schema,omitempty"`
}

func (s NodeSchema) TypeName() string {
	if s.TypeID != "" {
		return s.TypeID
	}
	return s.Type
}

func (s NodeSchema) DisplayLabel() string {
	if s.DisplayName != "" {
		return s.DisplayName
	}
	return s.Label
}

func (s NodeSchema) InputDefs() []PortDef {
	return s.Inputs
}

func (s NodeSchema) OutputDefs() []PortDef {
	return s.Outputs
}

func (s NodeSchema) InputPort(name string) *PortDef {
	for _, port := range s.InputDefs() {
		if port.Name == name {
			candidate := port
			return &candidate
		}
	}
	return nil
}

func (s NodeSchema) OutputPort(name string) *PortDef {
	for _, port := range s.OutputDefs() {
		if port.Name == name {
			candidate := port
			return &candidate
		}
	}
	return nil
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
	jobs          repository.JobRepository
	folders       repository.FolderRepository
	snapshots     repository.SnapshotRepository
	workflowDefs  repository.WorkflowDefinitionRepository
	workflowRuns  repository.WorkflowRunRepository
	nodeRuns      repository.NodeRunRepository
	nodeSnapshots repository.NodeSnapshotRepository
	executors     map[string]WorkflowNodeExecutor
	broker        *sse.Broker
	auditSvc      *AuditService
	typeRegistry  *TypeRegistry
}

func NewWorkflowRunnerService(
	jobRepo repository.JobRepository,
	folderRepo repository.FolderRepository,
	snapshotRepo repository.SnapshotRepository,
	workflowDefRepo repository.WorkflowDefinitionRepository,
	workflowRunRepo repository.WorkflowRunRepository,
	nodeRunRepo repository.NodeRunRepository,
	nodeSnapshotRepo repository.NodeSnapshotRepository,
	fsAdapter fs.FSAdapter,
	broker *sse.Broker,
	auditSvc *AuditService,
) *WorkflowRunnerService {
	svc := &WorkflowRunnerService{
		jobs:          jobRepo,
		folders:       folderRepo,
		snapshots:     snapshotRepo,
		workflowDefs:  workflowDefRepo,
		workflowRuns:  workflowRunRepo,
		nodeRuns:      nodeRunRepo,
		nodeSnapshots: nodeSnapshotRepo,
		executors:     make(map[string]WorkflowNodeExecutor),
		broker:        broker,
		auditSvc:      auditSvc,
		typeRegistry:  NewTypeRegistry(),
	}

	svc.RegisterExecutor(&triggerNodeExecutor{})
	svc.RegisterExecutor(newFolderTreeScannerExecutor(fsAdapter))
	svc.RegisterExecutor(newNameKeywordClassifierExecutor())
	svc.RegisterExecutor(newFileTreeClassifierExecutor())
	svc.RegisterExecutor(newConfidenceCheckExecutor())
	svc.RegisterExecutor(&extRatioClassifierNodeExecutor{fs: fsAdapter})
	svc.RegisterExecutor(newManualClassifierExecutor())
	svc.RegisterExecutor(newSubtreeAggregatorExecutor(folderRepo, snapshotRepo, auditSvc))
	svc.RegisterExecutor(newClassificationReaderExecutor())
	svc.RegisterExecutor(newDBSubtreeReaderExecutor(folderRepo))
	svc.RegisterExecutor(newFolderSplitterExecutor())
	svc.RegisterExecutor(newCategoryRouterExecutor())
	svc.RegisterExecutor(newRenameNodeExecutor())
	svc.RegisterExecutor(newPhase4MoveNodeExecutor(fsAdapter, folderRepo))
	svc.RegisterExecutor(newThumbnailNodeExecutor(fsAdapter, folderRepo))
	svc.RegisterExecutor(newCompressNodeExecutor(fsAdapter))
	svc.RegisterExecutor(newAuditLogNodeExecutor(auditSvc))
	svc.RegisterExecutor(newClassificationPreviewNodeExecutor())
	svc.RegisterExecutor(newFolderSelectorNodeExecutor())
	svc.RegisterExecutor(newFolderPickerNodeExecutor(fsAdapter, folderRepo))

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
		return schemas[i].TypeName() < schemas[j].TypeName()
	})

	return schemas
}

func (s *WorkflowRunnerService) StartJob(ctx context.Context, input StartWorkflowJobInput) (string, error) {
	if input.WorkflowDefID == "" {
		return "", fmt.Errorf("workflowRunner.StartJob: workflow_def_id is required")
	}
	if _, err := s.workflowDefs.GetByID(ctx, input.WorkflowDefID); err != nil {
		return "", fmt.Errorf("workflowRunner.StartJob get workflow def %q: %w", input.WorkflowDefID, err)
	}

	folderIDsJSON, err := json.Marshal([]string{})
	if err != nil {
		return "", fmt.Errorf("workflowRunner.StartJob marshal folder_ids: %w", err)
	}

	sourceDir := strings.TrimSpace(input.SourceDir)

	jobID := uuid.NewString()
	if err := s.jobs.Create(ctx, &repository.Job{
		ID:            jobID,
		Type:          "workflow",
		WorkflowDefID: input.WorkflowDefID,
		SourceDir:     sourceDir,
		Status:        "pending",
		FolderIDs:     string(folderIDsJSON),
		Total:         1,
	}); err != nil {
		return "", fmt.Errorf("workflowRunner.StartJob create job: %w", err)
	}

	go s.runJob(context.Background(), jobID, input.WorkflowDefID, sourceDir)
	return jobID, nil
}

func (s *WorkflowRunnerService) runJob(ctx context.Context, jobID, workflowDefID, sourceDir string) {
	_ = s.jobs.UpdateStatus(ctx, jobID, "running", "")

	run := &repository.WorkflowRun{
		ID:            uuid.NewString(),
		JobID:         jobID,
		SourceDir:     sourceDir,
		WorkflowDefID: workflowDefID,
		Status:        "pending",
	}
	if err := s.workflowRuns.Create(ctx, run); err != nil {
		_ = s.jobs.IncrementProgress(ctx, jobID, 0, 1)
		_ = s.jobs.UpdateStatus(ctx, jobID, "failed", "")
		return
	}
	if err := s.executeWorkflowRun(ctx, run.ID, false); err != nil {
		_ = s.jobs.IncrementProgress(ctx, jobID, 0, 1)
		_ = s.jobs.UpdateStatus(ctx, jobID, "failed", "")
		return
	}
	_ = s.jobs.IncrementProgress(ctx, jobID, 1, 0)
	_ = s.jobs.UpdateStatus(ctx, jobID, "succeeded", "")
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
	if resumeData != nil {
		waitingNodeRun, err := s.nodeRuns.GetWaitingInputByWorkflowRunID(ctx, workflowRunID)
		if err != nil {
			return fmt.Errorf("workflowRunner.ResumeWorkflowRunWithData get waiting node run for %q: %w", workflowRunID, err)
		}

		persisted := make(map[string]any)
		if strings.TrimSpace(waitingNodeRun.ResumeData) != "" {
			if err := json.Unmarshal([]byte(waitingNodeRun.ResumeData), &persisted); err != nil {
				return fmt.Errorf("workflowRunner.ResumeWorkflowRunWithData unmarshal resume data for node run %q: %w", waitingNodeRun.ID, err)
			}
		}
		for key, value := range resumeData {
			persisted[key] = value
		}

		encoded, err := json.Marshal(persisted)
		if err != nil {
			return fmt.Errorf("workflowRunner.ResumeWorkflowRunWithData marshal resume data for node run %q: %w", waitingNodeRun.ID, err)
		}
		if err := s.nodeRuns.UpdateResumeData(ctx, waitingNodeRun.ID, string(encoded)); err != nil {
			return fmt.Errorf("workflowRunner.ResumeWorkflowRunWithData persist resume data for node run %q: %w", waitingNodeRun.ID, err)
		}
	}

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

	var folder *repository.Folder
	if strings.TrimSpace(run.FolderID) != "" {
		folder, err = s.folders.GetByID(ctx, run.FolderID)
		if err != nil {
			return fmt.Errorf("workflowRunner.RollbackWorkflowRun get folder %q: %w", run.FolderID, err)
		}
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
		_ = s.writeNodeRollbackAudit(ctx, run, nodeRun, folder, snaps)

		if strings.TrimSpace(run.FolderID) != "" {
			folder, err = s.folders.GetByID(ctx, run.FolderID)
			if err != nil {
				return fmt.Errorf("workflowRunner.RollbackWorkflowRun reload folder %q: %w", run.FolderID, err)
			}
		}
	}

	if err := s.workflowRuns.UpdateStatus(ctx, workflowRunID, "rolled_back", ""); err != nil {
		return fmt.Errorf("workflowRunner.RollbackWorkflowRun update workflow run status %q: %w", workflowRunID, err)
	}

	return nil
}

func (s *WorkflowRunnerService) executeWorkflowRun(ctx context.Context, workflowRunID string, resume bool) (retErr error) {
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
	s.normalizeGraphPortReferences(graph)

	nodes, err := topologicalNodes(graph)
	if err != nil {
		return fmt.Errorf("workflowRunner.executeWorkflowRun topo sort for workflow def %q: %w", def.ID, err)
	}

	var folder *repository.Folder
	if strings.TrimSpace(run.FolderID) != "" {
		folder, err = s.folders.GetByID(ctx, run.FolderID)
		if err != nil {
			return fmt.Errorf("workflowRunner.executeWorkflowRun get folder %q: %w", run.FolderID, err)
		}
	}

	runStartedAt := time.Now()
	if !resume {
		_ = s.writeWorkflowRunAudit(ctx, run, folder, "workflow.run.start", "success", 0, nil)
	}
	defer func() {
		if retErr != nil {
			_ = s.writeWorkflowRunAudit(ctx, run, folder, "workflow.run.failed", "failed", time.Since(runStartedAt).Milliseconds(), retErr)
		}
	}()

	existingRuns, _, err := s.nodeRuns.List(ctx, repository.NodeRunListFilter{WorkflowRunID: run.ID, Page: 1, Limit: 2000})
	if err != nil {
		return fmt.Errorf("workflowRunner.executeWorkflowRun list node runs for workflow run %q: %w", run.ID, err)
	}
	outputCache := make(map[string]map[string]TypedValue)
	for _, existingRun := range existingRuns {
		if existingRun.NodeID == "" || strings.TrimSpace(existingRun.OutputJSON) == "" {
			continue
		}
		schema := s.schemaForNode(existingRun.NodeType)
		outputs, parseErr := s.parseNodeOutputsForSchema(existingRun.OutputJSON, schema)
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
		waitingNodeRun, waitErr := s.nodeRuns.GetWaitingInputByWorkflowRunID(ctx, workflowRunID)
		if waitErr == nil && strings.TrimSpace(waitingNodeRun.ResumeData) != "" {
			if err := json.Unmarshal([]byte(waitingNodeRun.ResumeData), &resumeData); err != nil {
				return fmt.Errorf("workflowRunner.executeWorkflowRun unmarshal resume data for node run %q: %w", waitingNodeRun.ID, err)
			}
		}
	}

	for _, node := range nodes {
		if !startNow {
			if node.ID == resumeNodeID {
				startNow = true
			} else {
				continue
			}
		}

		inputs := s.resolveNodeInputs(node, outputCache, run.SourceDir)

		if shouldSkipNode(node, inputs, s.schemaForNode(node.Type)) {
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
			outputCache[node.ID] = map[string]TypedValue{}
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

		inputPayload := map[string]any{
			"workflow_run_id": run.ID,
			"source_dir":      run.SourceDir,
			"node":            node,
			"inputs":          typedInputValuesForJSON(inputs),
		}
		if folder != nil {
			inputPayload["folder_id"] = folder.ID
			inputPayload["folder_path"] = folder.Path
		}
		inputJSON, err := json.Marshal(inputPayload)
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
			_ = s.createNodeSnapshot(ctx, run, nodeRun, "post", folder, map[string]TypedValue{"error": {Type: PortTypeString, Value: err.Error()}})
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
			SourceDir:   run.SourceDir,
			Inputs:      inputs,
		}

		nodeStartedAt := time.Now()
		var execOutput NodeExecutionOutput
		var execErr error
		if resume && node.ID == resumeNodeID {
			execOutput, execErr = executor.Resume(ctx, execInput, resumeData)
		} else {
			execOutput, execErr = executor.Execute(ctx, execInput)
		}
		if execErr != nil {
			_ = s.nodeRuns.UpdateFinish(ctx, nodeRun.ID, "failed", "", execErr.Error())
			_ = s.createNodeSnapshot(ctx, run, nodeRun, "post", folder, map[string]TypedValue{"error": {Type: PortTypeString, Value: execErr.Error()}})
			_ = s.writeNodeExecutionAudit(ctx, execInput, folder, nil, execErr, nodeStartedAt)
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
		if execOutput.Outputs == nil {
			execOutput.Outputs = map[string]TypedValue{}
		}

		outputJSON, marshalErr := s.marshalTypedValuesJSON(execOutput.Outputs)
		if marshalErr != nil {
			_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
			return fmt.Errorf("workflowRunner.executeWorkflowRun marshal node output for node %q: %w", node.ID, marshalErr)
		}

		if execOutput.Status == ExecutionPending {
			errMsg := execOutput.PendingReason
			persistedResumeData := make(map[string]any)
			for key, value := range resumeData {
				persistedResumeData[key] = value
			}
			if pendingState := pendingResumeState(execOutput.Outputs); len(pendingState) > 0 {
				for key, value := range pendingState {
					persistedResumeData[key] = value
				}
			}

			if len(persistedResumeData) > 0 {
				encodedResumeData, encodeErr := json.Marshal(persistedResumeData)
				if encodeErr != nil {
					_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
					return fmt.Errorf("workflowRunner.executeWorkflowRun marshal resume data for node %q: %w", node.ID, encodeErr)
				}
				if err := s.nodeRuns.UpdateResumeData(ctx, nodeRun.ID, string(encodedResumeData)); err != nil {
					_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
					return fmt.Errorf("workflowRunner.executeWorkflowRun persist resume data for node run %q: %w", nodeRun.ID, err)
				}
			}
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
			_ = s.createNodeSnapshot(ctx, run, nodeRun, "post", folder, map[string]TypedValue{"error": {Type: PortTypeString, Value: errMsg}})
			_ = s.writeNodeExecutionAudit(ctx, execInput, folder, execOutput.Outputs, fmt.Errorf("%s", errMsg), nodeStartedAt)
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

		outputCache[node.ID] = cloneTypedValueMap(execOutput.Outputs)

		if strings.TrimSpace(run.FolderID) != "" {
			folder, err = s.folders.GetByID(ctx, run.FolderID)
			if err != nil {
				_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
				return fmt.Errorf("workflowRunner.executeWorkflowRun reload folder %q after node %q: %w", run.FolderID, node.ID, err)
			}
		}

		if err := s.createNodeSnapshot(ctx, run, nodeRun, "post", folder, execOutput.Outputs); err != nil {
			_ = s.workflowRuns.UpdateStatus(ctx, run.ID, "failed", node.ID)
			return fmt.Errorf("workflowRunner.executeWorkflowRun create post snapshot for node %q: %w", node.ID, err)
		}
		_ = s.writeNodeExecutionAudit(ctx, execInput, folder, execOutput.Outputs, nil, nodeStartedAt)

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

	_ = s.writeWorkflowRunAudit(ctx, run, folder, "workflow.run.complete", "success", time.Since(runStartedAt).Milliseconds(), nil)
	return nil
}

func pendingResumeState(outputs map[string]TypedValue) map[string]any {
	if len(outputs) == 0 {
		return nil
	}

	stateOutput, ok := outputs["state"]
	if !ok {
		return nil
	}
	state, ok := stateOutput.Value.(map[string]any)
	if !ok {
		return nil
	}

	copyState := make(map[string]any, len(state))
	for key, value := range state {
		copyState[key] = value
	}

	return copyState
}

func (s *WorkflowRunnerService) writeNodeExecutionAudit(ctx context.Context, input NodeExecutionInput, currentFolder *repository.Folder, outputs map[string]TypedValue, execErr error, startedAt time.Time) error {
	if s.auditSvc == nil {
		return nil
	}

	detail := map[string]any{
		"workflow_run_id": workflowRunIDFromInput(input.WorkflowRun),
		"node_run_id":     nodeRunIDFromInput(input.NodeRun),
		"node_id":         input.Node.ID,
		"node_type":       input.Node.Type,
	}
	action := "workflow." + input.Node.Type
	folderPath := folderPathForAudit(input.Folder, currentFolder)
	result := "success"

	switch input.Node.Type {
	case "move":
		detail["source_path"] = folderPathForAudit(input.Folder, nil)
		detail["target_path"] = folderPathForAudit(currentFolder, nil)
	case phase4MoveNodeExecutorType:
		results := moveResultsForAudit(outputs)
		detail["results"] = results
		if len(results) > 0 {
			folderPath = strings.TrimSpace(results[0].TargetPath)
			if folderPath == "" {
				folderPath = strings.TrimSpace(results[0].SourcePath)
			}
			if strings.TrimSpace(results[0].Status) != "" {
				result = strings.TrimSpace(results[0].Status)
			}
		}
	case compressNodeExecutorType:
		archives := stringSliceOutput(outputs, "archives")
		detail["archive_paths"] = archives
		if len(archives) > 0 {
			folderPath = archives[0]
		}
	case thumbnailNodeExecutorType:
		thumbnails := stringSliceOutput(outputs, "thumbnail_paths")
		detail["thumbnail_paths"] = thumbnails
		if len(thumbnails) > 0 {
			folderPath = thumbnails[0]
		}
	}

	if execErr != nil {
		result = "failed"
		detail["error"] = execErr.Error()
	}

	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("workflowRunner.writeNodeExecutionAudit marshal detail: %w", err)
	}

	return s.auditSvc.Write(ctx, &repository.AuditLog{
		JobID:      workflowJobIDFromInput(input.WorkflowRun),
		FolderID:   folderIDForAudit(input.Folder, currentFolder),
		FolderPath: folderPath,
		Action:     action,
		Result:     result,
		Detail:     detailJSON,
		DurationMs: time.Since(startedAt).Milliseconds(),
		ErrorMsg:   errorString(execErr),
	})
}

func (s *WorkflowRunnerService) writeNodeRollbackAudit(ctx context.Context, run *repository.WorkflowRun, nodeRun *repository.NodeRun, folder *repository.Folder, snapshots []*repository.NodeSnapshot) error {
	if s.auditSvc == nil || nodeRun == nil {
		return nil
	}

	detailJSON, err := json.Marshal(map[string]any{
		"workflow_run_id": workflowRunIDFromInput(run),
		"node_run_id":     nodeRun.ID,
		"node_id":         nodeRun.NodeID,
		"node_type":       nodeRun.NodeType,
		"snapshot_count":  len(snapshots),
	})
	if err != nil {
		return fmt.Errorf("workflowRunner.writeNodeRollbackAudit marshal detail: %w", err)
	}

	return s.auditSvc.Write(ctx, &repository.AuditLog{
		JobID:      workflowJobIDFromInput(run),
		FolderID:   folderIDForAudit(folder, nil),
		FolderPath: folderPathForAudit(folder, nil),
		Action:     "workflow." + nodeRun.NodeType + ".rollback",
		Result:     "success",
		Detail:     detailJSON,
	})
}

func (s *WorkflowRunnerService) writeWorkflowRunAudit(ctx context.Context, run *repository.WorkflowRun, folder *repository.Folder, action, result string, durationMs int64, auditErr error) error {
	if s.auditSvc == nil {
		return nil
	}

	detail := map[string]any{
		"workflow_run_id": workflowRunIDFromInput(run),
		"workflow_def_id": run.WorkflowDefID,
	}
	if auditErr != nil {
		detail["error"] = auditErr.Error()
	}

	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("workflowRunner.writeWorkflowRunAudit marshal detail: %w", err)
	}

	return s.auditSvc.Write(ctx, &repository.AuditLog{
		JobID:      workflowJobIDFromInput(run),
		FolderID:   folderIDForAudit(folder, nil),
		FolderPath: folderPathForAudit(folder, nil),
		Action:     action,
		Result:     result,
		Detail:     detailJSON,
		DurationMs: durationMs,
		ErrorMsg:   errorString(auditErr),
	})
}

func nodeRunIDFromInput(run *repository.NodeRun) string {
	if run == nil {
		return ""
	}

	return run.ID
}

func folderIDForAudit(primary *repository.Folder, fallback *repository.Folder) string {
	if primary != nil && strings.TrimSpace(primary.ID) != "" {
		return strings.TrimSpace(primary.ID)
	}
	if fallback != nil {
		return strings.TrimSpace(fallback.ID)
	}

	return ""
}

func folderPathForAudit(primary *repository.Folder, fallback *repository.Folder) string {
	if primary != nil && strings.TrimSpace(primary.Path) != "" {
		return strings.TrimSpace(primary.Path)
	}
	if fallback != nil {
		return strings.TrimSpace(fallback.Path)
	}

	return ""
}

func moveResultsForAudit(outputs map[string]TypedValue) []MoveResult {
	if len(outputs) == 0 {
		return nil
	}

	resultOutput, ok := outputs["results"]
	if !ok {
		return nil
	}

	switch typed := resultOutput.Value.(type) {
	case []MoveResult:
		return append([]MoveResult(nil), typed...)
	case MoveResult:
		return []MoveResult{typed}
	default:
		return nil
	}
}

func stringSliceOutput(outputs map[string]TypedValue, key string) []string {
	output, ok := outputs[key]
	if !ok {
		return nil
	}

	return uniqueCompactStringSlice(anyToStringSlice(output.Value))
}

func errorString(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}

func (s *WorkflowRunnerService) schemaForNode(nodeType string) NodeSchema {
	executor, ok := s.executors[nodeType]
	if !ok {
		return NodeSchema{}
	}
	return executor.Schema()
}

func (s *WorkflowRunnerService) resolveNodeInputs(node repository.WorkflowGraphNode, outputCache map[string]map[string]TypedValue, sourceDir string) map[string]*TypedValue {
	schema := s.schemaForNode(node.Type)
	inputs := make(map[string]*TypedValue, len(schema.InputDefs()))
	for _, port := range schema.InputDefs() {
		inputs[port.Name] = nil
	}

	for portName, spec := range node.Inputs {
		if spec.ConstValue != nil {
			portType := inferPortTypeForInput(schema, portName)
			inputs[portName] = &TypedValue{Type: portType, Value: *spec.ConstValue}
			continue
		}
		if spec.LinkSource == nil {
			continue
		}

		sourceOutputs := outputCache[spec.LinkSource.SourceNodeID]
		sourcePort := strings.TrimSpace(spec.LinkSource.SourcePort)
		if sourcePort == "" {
			inputs[portName] = nil
			continue
		}
		value, ok := sourceOutputs[sourcePort]
		if !ok {
			inputs[portName] = nil
			continue
		}
		copied := value
		inputs[portName] = &copied
	}

	if strings.TrimSpace(sourceDir) != "" {
		if existing, exists := inputs["source_dir"]; exists && existing != nil {
			if text, ok := existing.Value.(string); ok && strings.TrimSpace(text) == "" {
				inputs["source_dir"] = nil
			}
		}
		if _, exists := inputs["source_dir"]; exists && inputs["source_dir"] == nil {
			inputs["source_dir"] = &TypedValue{Type: PortTypePath, Value: sourceDir}
		}
	}

	return inputs
}

func shouldSkipNode(node repository.WorkflowGraphNode, inputs map[string]*TypedValue, schema NodeSchema) bool {
	for _, port := range schema.InputDefs() {
		if !port.Required || port.Lazy {
			continue
		}
		if _, exists := node.Inputs[port.Name]; !exists {
			continue
		}
		if value, ok := inputs[port.Name]; !ok || value == nil || value.Value == nil {
			return true
		}
	}
	return false
}

func (s *WorkflowRunnerService) createNodeSnapshot(
	ctx context.Context,
	run *repository.WorkflowRun,
	nodeRun *repository.NodeRun,
	kind string,
	folder *repository.Folder,
	outputs map[string]TypedValue,
) error {
	manifest := map[string]any{}
	if folder != nil {
		manifest["folder_id"] = folder.ID
		manifest["folder_path"] = folder.Path
		manifest["name"] = folder.Name
		manifest["category"] = folder.Category
		manifest["status"] = folder.Status
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("workflowRunner.createNodeSnapshot marshal fs manifest for node run %q: %w", nodeRun.ID, err)
	}

	outputJSON := ""
	if outputs != nil {
		encodedOutputs, encodeErr := typedValueMapToJSON(outputs, s.typeRegistry)
		if encodeErr != nil {
			return fmt.Errorf("workflowRunner.createNodeSnapshot encode typed outputs for node run %q: %w", nodeRun.ID, encodeErr)
		}
		data, marshalErr := json.Marshal(map[string]any{"outputs": encodedOutputs})
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

func (s *WorkflowRunnerService) normalizeGraphPortReferences(graph *repository.WorkflowGraph) {
	if graph == nil {
		return
	}

	nodeTypeByID := make(map[string]string, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodeTypeByID[node.ID] = node.Type
	}

	for nodeIndex := range graph.Nodes {
		node := &graph.Nodes[nodeIndex]
		for inputName, spec := range node.Inputs {
			if spec.LinkSource == nil {
				continue
			}
			if strings.TrimSpace(spec.LinkSource.SourcePort) != "" {
				spec.LinkSource.OutputPortIndex = 0
				node.Inputs[inputName] = spec
				continue
			}

			sourceType := nodeTypeByID[spec.LinkSource.SourceNodeID]
			sourceSchema := s.schemaForNode(sourceType)
			index := spec.LinkSource.OutputPortIndex
			if index < 0 || index >= len(sourceSchema.OutputDefs()) {
				continue
			}

			spec.LinkSource.SourcePort = sourceSchema.OutputDefs()[index].Name
			spec.LinkSource.OutputPortIndex = 0
			node.Inputs[inputName] = spec
		}
	}
}

func typedInputValuesForJSON(inputs map[string]*TypedValue) map[string]any {
	if len(inputs) == 0 {
		return map[string]any{}
	}

	out := make(map[string]any, len(inputs))
	for key, value := range inputs {
		if value == nil {
			out[key] = nil
			continue
		}
		out[key] = value.Value
	}

	return out
}

func cloneTypedValueMap(values map[string]TypedValue) map[string]TypedValue {
	if len(values) == 0 {
		return map[string]TypedValue{}
	}

	out := make(map[string]TypedValue, len(values))
	for key, value := range values {
		out[key] = value
	}

	return out
}

func inferPortTypeForInput(schema NodeSchema, portName string) PortType {
	if port := schema.InputPort(portName); port != nil {
		if port.Type != "" {
			return port.Type
		}
	}

	return PortTypeJSON
}

func inferPortTypeForOutput(schema NodeSchema, portName string) PortType {
	if port := schema.OutputPort(portName); port != nil {
		if port.Type != "" {
			return port.Type
		}
	}

	return PortTypeJSON
}

func typedValueMapToJSON(values map[string]TypedValue, registry *TypeRegistry) (map[string]TypedValueJSON, error) {
	out := make(map[string]TypedValueJSON, len(values))
	for key, value := range values {
		encoded, err := registry.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("encode output %q: %w", key, err)
		}
		out[key] = encoded
	}

	return out, nil
}

func typedValueMapFromJSON(values map[string]TypedValueJSON, registry *TypeRegistry) (map[string]TypedValue, error) {
	out := make(map[string]TypedValue, len(values))
	for key, value := range values {
		decoded, err := registry.Unmarshal(value)
		if err != nil {
			return nil, fmt.Errorf("decode output %q: %w", key, err)
		}
		out[key] = decoded
	}

	return out, nil
}

func (s *WorkflowRunnerService) marshalTypedValuesJSON(values map[string]TypedValue) (string, error) {
	encoded, err := typedValueMapToJSON(values, s.typeRegistry)
	if err != nil {
		return "", err
	}

	raw, err := json.Marshal(encoded)
	if err != nil {
		return "", err
	}

	return string(raw), nil
}

func (s *WorkflowRunnerService) parseNodeOutputsForSchema(rawOutput string, _ NodeSchema) (map[string]TypedValue, error) {
	var typedEncoded map[string]TypedValueJSON
	if err := json.Unmarshal([]byte(rawOutput), &typedEncoded); err != nil {
		return nil, err
	}

	out, err := typedValueMapFromJSON(typedEncoded, s.typeRegistry)
	if err != nil {
		return nil, fmt.Errorf("parse typed node outputs: %w", err)
	}
	return out, nil
}

func parseTypedNodeOutputs(rawOutput string) (map[string]TypedValue, bool, error) {
	registry := NewTypeRegistry()

	var direct map[string]TypedValueJSON
	if err := json.Unmarshal([]byte(rawOutput), &direct); err == nil && len(direct) > 0 {
		decoded, decodeErr := typedValueMapFromJSON(direct, registry)
		if decodeErr != nil {
			return nil, false, decodeErr
		}
		return decoded, true, nil
	}

	var wrapped struct {
		Outputs map[string]TypedValueJSON `json:"outputs"`
	}
	if err := json.Unmarshal([]byte(rawOutput), &wrapped); err == nil && len(wrapped.Outputs) > 0 {
		decoded, decodeErr := typedValueMapFromJSON(wrapped.Outputs, registry)
		if decodeErr != nil {
			return nil, false, decodeErr
		}
		return decoded, true, nil
	}

	return nil, false, nil
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
	return NodeSchema{
		Type:        e.Type(),
		Label:       "触发器",
		Description: "触发节点，启动工作流并将当前处理文件夹传递给下游",
		Outputs: []PortDef{{
			Name:        "folder",
			Type:        PortTypeJSON,
			Description: "当前处理的文件夹数据",
		}},
	}
}

func (e *triggerNodeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	if input.Folder == nil {
		return NodeExecutionOutput{Outputs: map[string]TypedValue{"folder": {Type: PortTypeJSON, Value: nil}}, Status: ExecutionSuccess}, nil
	}

	return NodeExecutionOutput{Outputs: map[string]TypedValue{"folder": {Type: PortTypeJSON, Value: input.Folder}}, Status: ExecutionSuccess}, nil
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
	return NodeSchema{
		Type:        e.Type(),
		Label:       "扩展名分类器",
		Description: "根据目录内文件扩展名比例判断分类类别",
		Inputs: []PortDef{
			{Name: "trees", Type: PortTypeFolderTreeList, Description: "目录树列表", Required: false},
		},
		Outputs: []PortDef{{
			Name:        "signal",
			Type:        PortTypeClassificationSignalList,
			Description: "分类信号列表",
		}},
	}
}

func (e *extRatioClassifierNodeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	rawTrees, ok := firstPresentTyped(input.Inputs, "trees")
	if !ok {
		return NodeExecutionOutput{Outputs: map[string]TypedValue{"signal": {Type: PortTypeClassificationSignalList, Value: nil}}, Status: ExecutionSuccess}, nil
	}

	trees, found, err := parseFolderTreesInput(rawTrees)
	if err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("extRatioClassifier.Execute parse trees: %w", err)
	}
	if !found {
		return NodeExecutionOutput{Outputs: map[string]TypedValue{"signal": {Type: PortTypeClassificationSignalList, Value: nil}}, Status: ExecutionSuccess}, nil
	}

	signals := make([]ClassificationSignal, 0, len(trees))
	for _, tree := range trees {
		fileNames := make([]string, 0, len(tree.Files))
		for _, file := range tree.Files {
			fileNames = append(fileNames, file.Name)
		}

		category := Classify(tree.Name, fileNames)
		confidence := 0.85
		if category == "other" {
			confidence = 0.5
		}
		reason := fmt.Sprintf("ext-ratio: %s", category)

		signals = append(signals, ClassificationSignal{
			SourcePath: tree.Path,
			Category:   category,
			Confidence: confidence,
			Reason:     reason,
		})
	}

	return NodeExecutionOutput{Outputs: map[string]TypedValue{"signal": {Type: PortTypeClassificationSignalList, Value: signals}}, Status: ExecutionSuccess}, nil
}

func (e *extRatioClassifierNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *extRatioClassifierNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func firstPresentTyped(inputs map[string]*TypedValue, keys ...string) (any, bool) {
	for _, key := range keys {
		value, ok := inputs[key]
		if !ok || value == nil || value.Value == nil {
			continue
		}
		return value.Value, true
	}

	return nil, false
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
