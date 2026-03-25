package service

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
)

type stubCustomExecutor struct{}

func (e *stubCustomExecutor) Type() string {
	return "custom-test"
}

func (e *stubCustomExecutor) Schema() NodeSchema {
	return NodeSchema{Type: e.Type(), Label: "Custom Test", Description: "Custom test executor"}
}

func (e *stubCustomExecutor) Execute(_ context.Context, _ NodeExecutionInput) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{Outputs: []any{true}, Status: ExecutionSuccess}, nil
}

func (e *stubCustomExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, nil
}

func (e *stubCustomExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

type produceInputExecutor struct{}

func (e *produceInputExecutor) Type() string {
	return "produce-input"
}

func (e *produceInputExecutor) Schema() NodeSchema {
	return NodeSchema{Type: e.Type(), Label: "Produce Input", Description: "Produce a fixed output for tests"}
}

func (e *produceInputExecutor) Execute(_ context.Context, _ NodeExecutionInput) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{Outputs: []any{"hello-port"}, Status: ExecutionSuccess}, nil
}

func (e *produceInputExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, nil
}

func (e *produceInputExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

type consumeInputExecutor struct {
	seen string
}

type slowParallelExecutor struct {
	active    int32
	maxActive int32
	mu        sync.Mutex
	visited   []string
}

type auditOutputExecutor struct {
	nodeType string
	outputs  []any
}

func (e *consumeInputExecutor) Type() string {
	return "consume-input"
}

func (e *consumeInputExecutor) Schema() NodeSchema {
	return NodeSchema{Type: e.Type(), Label: "Consume Input", Description: "Consume upstream output for tests"}
}

func (e *consumeInputExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	raw, ok := input.Inputs["upstream"]
	if !ok {
		return NodeExecutionOutput{}, nil
	}
	text, _ := raw.(string)
	e.seen = text
	return NodeExecutionOutput{Outputs: []any{text}, Status: ExecutionSuccess}, nil
}

func (e *consumeInputExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, nil
}

func (e *consumeInputExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func (e *slowParallelExecutor) Type() string {
	return "slow-parallel"
}

func (e *slowParallelExecutor) Schema() NodeSchema {
	return NodeSchema{Type: e.Type(), Label: "Slow Parallel", Description: "Track folder-level parallelism"}
}

func (e *slowParallelExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	current := atomic.AddInt32(&e.active, 1)
	for {
		observed := atomic.LoadInt32(&e.maxActive)
		if current <= observed {
			break
		}
		if atomic.CompareAndSwapInt32(&e.maxActive, observed, current) {
			break
		}
	}

	e.mu.Lock()
	e.visited = append(e.visited, input.Folder.ID)
	e.mu.Unlock()

	time.Sleep(50 * time.Millisecond)
	atomic.AddInt32(&e.active, -1)
	return NodeExecutionOutput{Outputs: []any{input.Folder.ID}, Status: ExecutionSuccess}, nil
}

func (e *slowParallelExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, nil
}

func (e *slowParallelExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func (e *auditOutputExecutor) Type() string {
	return e.nodeType
}

func (e *auditOutputExecutor) Schema() NodeSchema {
	return NodeSchema{Type: e.Type(), Label: e.Type(), Description: "Emit outputs for audit tests"}
}

func (e *auditOutputExecutor) Execute(_ context.Context, _ NodeExecutionInput) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{Outputs: e.outputs, Status: ExecutionSuccess}, nil
}

func (e *auditOutputExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, nil
}

func (e *auditOutputExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func TestWorkflowRunnerServiceStartAndResume(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := newServiceTestDB(t)
	jobRepo := repository.NewJobRepository(database)
	folderRepo := repository.NewFolderRepository(database)
	workflowDefRepo := repository.NewWorkflowDefinitionRepository(database)
	workflowRunRepo := repository.NewWorkflowRunRepository(database)
	nodeRunRepo := repository.NewNodeRunRepository(database)
	nodeSnapshotRepo := repository.NewNodeSnapshotRepository(database)

	adapter := fs.NewMockAdapter()
	adapter.AddDir("/source/album", []fs.DirEntry{{Name: "a.jpg", IsDir: false}})

	folder := &repository.Folder{
		ID:             "folder-1",
		Path:           "/source/album",
		Name:           "album",
		Category:       "other",
		CategorySource: "auto",
		Status:         "pending",
	}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	graph := repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "n1", Type: "trigger", Enabled: true},
			{ID: "n2", Type: "custom-test", Enabled: true},
		},
		Edges: []repository.WorkflowGraphEdge{{Source: "n1", Target: "n2"}},
	}
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		t.Fatalf("json.Marshal(graph) error = %v", err)
	}

	def := &repository.WorkflowDefinition{
		ID:        "wf-1",
		Name:      "wf-test",
		GraphJSON: string(graphJSON),
		IsActive:  true,
		Version:   1,
	}
	if err := workflowDefRepo.Create(ctx, def); err != nil {
		t.Fatalf("workflowDefRepo.Create() error = %v", err)
	}

	svc := NewWorkflowRunnerService(
		jobRepo,
		folderRepo,
		workflowDefRepo,
		workflowRunRepo,
		nodeRunRepo,
		nodeSnapshotRepo,
		adapter,
		nil,
		nil,
	)

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID, FolderIDs: []string{folder.ID}})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	job := waitJobDone(t, jobRepo, jobID)
	if job.Status != "failed" {
		t.Fatalf("job status = %q, want failed", job.Status)
	}

	runs, total, err := workflowRunRepo.List(ctx, repository.WorkflowRunListFilter{JobID: jobID, Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("workflowRunRepo.List() error = %v", err)
	}
	if total != 1 || len(runs) != 1 {
		t.Fatalf("workflow runs total/len = %d/%d, want 1/1", total, len(runs))
	}
	if runs[0].ResumeNodeID != "n2" {
		t.Fatalf("resume_node_id = %q, want n2", runs[0].ResumeNodeID)
	}

	svc.RegisterExecutor(&stubCustomExecutor{})
	if err := svc.ResumeWorkflowRun(ctx, runs[0].ID); err != nil {
		t.Fatalf("ResumeWorkflowRun() error = %v", err)
	}

	updatedRun, err := workflowRunRepo.GetByID(ctx, runs[0].ID)
	if err != nil {
		t.Fatalf("workflowRunRepo.GetByID() error = %v", err)
	}
	if updatedRun.Status != "succeeded" {
		t.Fatalf("workflow run status = %q, want succeeded", updatedRun.Status)
	}

	nodeRuns, _, err := nodeRunRepo.List(ctx, repository.NodeRunListFilter{WorkflowRunID: runs[0].ID, Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("nodeRunRepo.List() error = %v", err)
	}
	if len(nodeRuns) < 3 {
		t.Fatalf("node runs len = %d, want >= 3", len(nodeRuns))
	}

	last := nodeRuns[len(nodeRuns)-1]
	if last.NodeID != "n2" || last.Status != "succeeded" {
		t.Fatalf("last node run = node_id %q status %q, want n2/succeeded", last.NodeID, last.Status)
	}
}

func TestWorkflowRunnerServicePortInputPropagation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := newServiceTestDB(t)
	jobRepo := repository.NewJobRepository(database)
	folderRepo := repository.NewFolderRepository(database)
	workflowDefRepo := repository.NewWorkflowDefinitionRepository(database)
	workflowRunRepo := repository.NewWorkflowRunRepository(database)
	nodeRunRepo := repository.NewNodeRunRepository(database)
	nodeSnapshotRepo := repository.NewNodeSnapshotRepository(database)

	adapter := fs.NewMockAdapter()
	adapter.AddDir("/source/album-port", []fs.DirEntry{{Name: "a.jpg", IsDir: false}})

	folder := &repository.Folder{
		ID:             "folder-port",
		Path:           "/source/album-port",
		Name:           "album-port",
		Category:       "other",
		CategorySource: "auto",
		Status:         "pending",
	}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	graph := repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "n1", Type: "produce-input", Enabled: true},
			{ID: "n2", Type: "consume-input", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"upstream": {
					LinkSource: &repository.NodeLinkSource{SourceNodeID: "n1", OutputPortIndex: 0},
				},
			}},
		},
		Edges: []repository.WorkflowGraphEdge{{Source: "n1", SourcePort: 0, Target: "n2", TargetPort: 0}},
	}
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		t.Fatalf("json.Marshal(graph) error = %v", err)
	}

	def := &repository.WorkflowDefinition{
		ID:        "wf-port",
		Name:      "wf-port",
		GraphJSON: string(graphJSON),
		IsActive:  true,
		Version:   1,
	}
	if err := workflowDefRepo.Create(ctx, def); err != nil {
		t.Fatalf("workflowDefRepo.Create() error = %v", err)
	}

	consume := &consumeInputExecutor{}
	svc := NewWorkflowRunnerService(
		jobRepo,
		folderRepo,
		workflowDefRepo,
		workflowRunRepo,
		nodeRunRepo,
		nodeSnapshotRepo,
		adapter,
		nil,
		nil,
	)
	svc.RegisterExecutor(&produceInputExecutor{})
	svc.RegisterExecutor(consume)

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID, FolderIDs: []string{folder.ID}})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	job := waitJobDone(t, jobRepo, jobID)
	if job.Status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", job.Status)
	}

	if consume.seen != "hello-port" {
		t.Fatalf("consume input = %q, want hello-port", consume.seen)
	}
}

func TestWorkflowRunnerServiceRunsFoldersInParallel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := newServiceTestDB(t)
	jobRepo := repository.NewJobRepository(database)
	folderRepo := repository.NewFolderRepository(database)
	workflowDefRepo := repository.NewWorkflowDefinitionRepository(database)
	workflowRunRepo := repository.NewWorkflowRunRepository(database)
	nodeRunRepo := repository.NewNodeRunRepository(database)
	nodeSnapshotRepo := repository.NewNodeSnapshotRepository(database)

	adapter := fs.NewMockAdapter()
	folders := []*repository.Folder{
		{ID: "folder-a", Path: "/source/folder-a", Name: "folder-a", Category: "other", CategorySource: "auto", Status: "pending"},
		{ID: "folder-b", Path: "/source/folder-b", Name: "folder-b", Category: "other", CategorySource: "auto", Status: "pending"},
	}
	for _, folder := range folders {
		if err := folderRepo.Upsert(ctx, folder); err != nil {
			t.Fatalf("folderRepo.Upsert(%s) error = %v", folder.ID, err)
		}
	}

	graph := repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "n1", Type: "trigger", Enabled: true},
			{ID: "n2", Type: "slow-parallel", Enabled: true},
		},
		Edges: []repository.WorkflowGraphEdge{{Source: "n1", Target: "n2"}},
	}
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		t.Fatalf("json.Marshal(graph) error = %v", err)
	}

	def := &repository.WorkflowDefinition{ID: "wf-parallel", Name: "wf-parallel", GraphJSON: string(graphJSON), IsActive: true, Version: 1}
	if err := workflowDefRepo.Create(ctx, def); err != nil {
		t.Fatalf("workflowDefRepo.Create() error = %v", err)
	}

	executor := &slowParallelExecutor{}
	svc := NewWorkflowRunnerService(jobRepo, folderRepo, workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo, adapter, nil, nil)
	svc.RegisterExecutor(executor)

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID, FolderIDs: []string{"folder-a", "folder-b"}})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	job := waitJobDone(t, jobRepo, jobID)
	if job.Status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", job.Status)
	}
	if atomic.LoadInt32(&executor.maxActive) < 2 {
		t.Fatalf("maxActive = %d, want at least 2", atomic.LoadInt32(&executor.maxActive))
	}
	if len(executor.visited) != 2 {
		t.Fatalf("visited len = %d, want 2", len(executor.visited))
	}
}

func TestWorkflowRunnerServiceWritesAuditForMutatingNodes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := newServiceTestDB(t)
	jobRepo := repository.NewJobRepository(database)
	folderRepo := repository.NewFolderRepository(database)
	workflowDefRepo := repository.NewWorkflowDefinitionRepository(database)
	workflowRunRepo := repository.NewWorkflowRunRepository(database)
	nodeRunRepo := repository.NewNodeRunRepository(database)
	nodeSnapshotRepo := repository.NewNodeSnapshotRepository(database)
	auditRepo := repository.NewAuditRepository(database)
	auditSvc := NewAuditService(auditRepo)

	folder := &repository.Folder{ID: "folder-audit", Path: "/source/folder-audit", Name: "folder-audit", Category: "other", CategorySource: "auto", Status: "pending"}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	testCases := []struct {
		name       string
		nodeType   string
		outputs    []any
		action     string
		result     string
		folderPath string
	}{
		{
			name:       "move-node",
			nodeType:   "move-node",
			outputs:    []any{[]ProcessingItem{{FolderID: folder.ID, SourcePath: "/target/folder-audit", FolderName: folder.Name}}, []MoveResult{{SourcePath: folder.Path, TargetPath: "/target/folder-audit", Status: "moved"}}},
			action:     "workflow.move-node",
			result:     "moved",
			folderPath: "/target/folder-audit",
		},
		{
			name:       "compress-node",
			nodeType:   "compress-node",
			outputs:    []any{[]ProcessingItem{{FolderID: folder.ID, SourcePath: folder.Path, FolderName: folder.Name}}, []string{"/archives/folder-audit.cbz"}},
			action:     "workflow.compress-node",
			result:     "success",
			folderPath: "/archives/folder-audit.cbz",
		},
		{
			name:       "thumbnail-node",
			nodeType:   "thumbnail-node",
			outputs:    []any{[]ProcessingItem{{FolderID: folder.ID, SourcePath: folder.Path, FolderName: folder.Name}}, []string{"/thumbs/folder-audit.jpg"}},
			action:     "workflow.thumbnail-node",
			result:     "success",
			folderPath: "/thumbs/folder-audit.jpg",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			graph := repository.WorkflowGraph{
				Nodes: []repository.WorkflowGraphNode{
					{ID: "n1", Type: "trigger", Enabled: true},
					{ID: "n2", Type: tc.nodeType, Enabled: true},
				},
				Edges: []repository.WorkflowGraphEdge{{Source: "n1", Target: "n2"}},
			}
			graphJSON, err := json.Marshal(graph)
			if err != nil {
				t.Fatalf("json.Marshal(graph) error = %v", err)
			}

			def := &repository.WorkflowDefinition{ID: "wf-" + tc.nodeType, Name: tc.nodeType, GraphJSON: string(graphJSON), IsActive: true, Version: 1}
			if err := workflowDefRepo.Create(ctx, def); err != nil {
				t.Fatalf("workflowDefRepo.Create() error = %v", err)
			}

			svc := NewWorkflowRunnerService(jobRepo, folderRepo, workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo, fs.NewMockAdapter(), nil, auditSvc)
			svc.RegisterExecutor(&auditOutputExecutor{nodeType: tc.nodeType, outputs: tc.outputs})

			jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID, FolderIDs: []string{folder.ID}})
			if err != nil {
				t.Fatalf("StartJob() error = %v", err)
			}
			waitJobDone(t, jobRepo, jobID)

			logs, _, err := auditRepo.List(ctx, repository.AuditListFilter{JobID: jobID, Action: tc.action, Page: 1, Limit: 20})
			if err != nil {
				t.Fatalf("auditRepo.List() error = %v", err)
			}
			if len(logs) == 0 {
				t.Fatalf("audit logs len = 0, want at least 1")
			}
			if logs[0].Result != tc.result {
				t.Fatalf("audit result = %q, want %q", logs[0].Result, tc.result)
			}
			if logs[0].FolderPath != tc.folderPath {
				t.Fatalf("audit folder_path = %q, want %q", logs[0].FolderPath, tc.folderPath)
			}
		})
	}
}

func waitJobDone(t *testing.T, jobRepo repository.JobRepository, jobID string) *repository.Job {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := jobRepo.GetByID(context.Background(), jobID)
		if err != nil {
			t.Fatalf("jobRepo.GetByID() error = %v", err)
		}
		if job.Status == "succeeded" || job.Status == "failed" || job.Status == "partial" {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timeout waiting job %q done", jobID)
	return nil
}
