package service

import (
	"context"
	"encoding/json"
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
