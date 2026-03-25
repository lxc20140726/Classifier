package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
)

func TestPhase3WorkflowE2E_KeywordHitSkipsFileTree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	adapter := fs.NewMockAdapter()
	adapter.AddDir("/source", []fs.DirEntry{{Name: "进击的巨人漫画", IsDir: true}})
	adapter.AddDir("/source/进击的巨人漫画", []fs.DirEntry{{Name: "001.jpg", IsDir: false, Size: 100}})

	svc, jobRepo, folderRepo, workflowDefRepo, workflowRunRepo, nodeRunRepo, _ := newPhase3WorkflowTestEnv(t, adapter)

	folder := &repository.Folder{
		ID:             "folder-keyword",
		Path:           "/source/进击的巨人漫画",
		Name:           "进击的巨人漫画",
		Category:       "other",
		CategorySource: "auto",
		Status:         "pending",
	}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	def := createPhase3WorkflowDef(t, workflowDefRepo, "wf-keyword", repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "scanner", Type: folderTreeScannerExecutorType, Enabled: true, Config: map[string]any{"source_dir": "/source"}},
			{ID: "kw", Type: "name-keyword-classifier", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"folder": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "scanner", OutputPortIndex: 0}},
			}},
			{ID: "ft", Type: "file-tree-classifier", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"folder": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "kw", OutputPortIndex: 1}},
			}},
			{ID: "agg", Type: subtreeAggregatorExecutorType, Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"node":      {LinkSource: &repository.NodeLinkSource{SourceNodeID: "scanner", OutputPortIndex: 0}},
				"signal_kw": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "kw", OutputPortIndex: 0}},
			}},
		},
		Edges: []repository.WorkflowGraphEdge{
			{ID: "e1", Source: "scanner", SourcePort: 0, Target: "kw", TargetPort: 0},
			{ID: "e2", Source: "kw", SourcePort: 1, Target: "ft", TargetPort: 0},
			{ID: "e3", Source: "scanner", SourcePort: 0, Target: "agg", TargetPort: 0},
			{ID: "e4", Source: "kw", SourcePort: 0, Target: "agg", TargetPort: 1},
		},
	})

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID, FolderIDs: []string{folder.ID}})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	job := waitJobDone(t, jobRepo, jobID)
	if job.Status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", job.Status)
	}

	updatedFolder, err := folderRepo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("folderRepo.GetByID() error = %v", err)
	}
	if updatedFolder.Category != "manga" {
		t.Fatalf("folder category = %q, want manga", updatedFolder.Category)
	}

	run := waitWorkflowRunByJob(t, workflowRunRepo, jobID)
	nodeRuns, _, err := nodeRunRepo.List(ctx, repository.NodeRunListFilter{WorkflowRunID: run.ID, Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("nodeRunRepo.List() error = %v", err)
	}
	if got := nodeRunStatusByID(nodeRuns, "ft"); got != "skipped" {
		t.Fatalf("file-tree node status = %q, want skipped", got)
	}
}

func TestPhase3WorkflowE2E_FileTreeHit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	adapter := fs.NewMockAdapter()
	adapter.AddDir("/source", []fs.DirEntry{{Name: "cbz-only", IsDir: true}})
	adapter.AddDir("/source/cbz-only", []fs.DirEntry{{Name: "vol1.cbz", IsDir: false, Size: 100}})

	svc, jobRepo, folderRepo, workflowDefRepo, _, _, _ := newPhase3WorkflowTestEnv(t, adapter)

	folder := &repository.Folder{
		ID:             "folder-ft",
		Path:           "/source/cbz-only",
		Name:           "cbz-only",
		Category:       "other",
		CategorySource: "auto",
		Status:         "pending",
	}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	def := createPhase3WorkflowDef(t, workflowDefRepo, "wf-file-tree", repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "scanner", Type: folderTreeScannerExecutorType, Enabled: true, Config: map[string]any{"source_dir": "/source"}},
			{ID: "kw", Type: "name-keyword-classifier", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"folder": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "scanner", OutputPortIndex: 0}},
			}},
			{ID: "ft", Type: "file-tree-classifier", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"folder": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "kw", OutputPortIndex: 1}},
			}},
			{ID: "agg", Type: subtreeAggregatorExecutorType, Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"node":      {LinkSource: &repository.NodeLinkSource{SourceNodeID: "scanner", OutputPortIndex: 0}},
				"signal_ft": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "ft", OutputPortIndex: 0}},
			}},
		},
		Edges: []repository.WorkflowGraphEdge{
			{ID: "e1", Source: "scanner", SourcePort: 0, Target: "kw", TargetPort: 0},
			{ID: "e2", Source: "kw", SourcePort: 1, Target: "ft", TargetPort: 0},
			{ID: "e3", Source: "scanner", SourcePort: 0, Target: "agg", TargetPort: 0},
			{ID: "e4", Source: "ft", SourcePort: 0, Target: "agg", TargetPort: 2},
		},
	})

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID, FolderIDs: []string{folder.ID}})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	job := waitJobDone(t, jobRepo, jobID)
	if job.Status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", job.Status)
	}

	updatedFolder, err := folderRepo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("folderRepo.GetByID() error = %v", err)
	}
	if updatedFolder.Category != "manga" {
		t.Fatalf("folder category = %q, want manga", updatedFolder.Category)
	}
}

func TestPhase3WorkflowE2E_ManualResume(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	adapter := fs.NewMockAdapter()
	adapter.AddDir("/source", []fs.DirEntry{{Name: "needs-manual", IsDir: true}})
	adapter.AddDir("/source/needs-manual", []fs.DirEntry{{Name: "notes.txt", IsDir: false, Size: 10}})

	svc, _, folderRepo, workflowDefRepo, workflowRunRepo, nodeRunRepo, _ := newPhase3WorkflowTestEnv(t, adapter)

	folder := &repository.Folder{
		ID:             "folder-manual",
		Path:           "/source/needs-manual",
		Name:           "needs-manual",
		Category:       "other",
		CategorySource: "auto",
		Status:         "pending",
	}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	def := createPhase3WorkflowDef(t, workflowDefRepo, "wf-manual", repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "scanner", Type: folderTreeScannerExecutorType, Enabled: true, Config: map[string]any{"source_dir": "/source"}},
			{ID: "manual", Type: "manual-classifier", Enabled: true},
			{ID: "agg", Type: subtreeAggregatorExecutorType, Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"node":          {LinkSource: &repository.NodeLinkSource{SourceNodeID: "scanner", OutputPortIndex: 0}},
				"signal_manual": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "manual", OutputPortIndex: 0}},
			}},
		},
		Edges: []repository.WorkflowGraphEdge{
			{ID: "e1", Source: "scanner", SourcePort: 0, Target: "agg", TargetPort: 0},
			{ID: "e2", Source: "manual", SourcePort: 0, Target: "agg", TargetPort: 5},
		},
	})

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID, FolderIDs: []string{folder.ID}})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	run := waitWorkflowRunStatus(t, workflowRunRepo, jobID, "waiting_input")
	if run.ResumeNodeID != "manual" {
		t.Fatalf("resume node id = %q, want manual", run.ResumeNodeID)
	}

	nodeRuns, _, err := nodeRunRepo.List(ctx, repository.NodeRunListFilter{WorkflowRunID: run.ID, Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("nodeRunRepo.List() error = %v", err)
	}
	if got := nodeRunStatusByID(nodeRuns, "manual"); got != "waiting_input" {
		t.Fatalf("manual node status = %q, want waiting_input", got)
	}

	if err := svc.ResumeWorkflowRunWithData(ctx, run.ID, map[string]any{"category": "video"}); err != nil {
		t.Fatalf("ResumeWorkflowRunWithData() error = %v", err)
	}

	run = waitWorkflowRunIDStatus(t, workflowRunRepo, run.ID, "succeeded")
	_ = run

	updatedFolder, err := folderRepo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("folderRepo.GetByID() error = %v", err)
	}
	if updatedFolder.Category != "video" {
		t.Fatalf("folder category = %q, want video", updatedFolder.Category)
	}
}

func TestPhase3WorkflowE2E_MoveRollback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	adapter := fs.NewMockAdapter()
	adapter.AddDir("/source", []fs.DirEntry{{Name: "rollback-me", IsDir: true}})
	adapter.AddDir("/source/rollback-me", []fs.DirEntry{{Name: "001.jpg", IsDir: false, Size: 100}})

	svc, jobRepo, folderRepo, workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo := newPhase3WorkflowTestEnv(t, adapter)

	folder := &repository.Folder{
		ID:             "folder-move-rollback",
		Path:           "/source/rollback-me",
		Name:           "rollback-me",
		Category:       "photo",
		CategorySource: "workflow",
		Status:         "pending",
	}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	def := createPhase3WorkflowDef(t, workflowDefRepo, "wf-move-rollback", repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "move", Type: "move", Enabled: true, Config: map[string]any{"target_dir": "/target"}},
		},
	})

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID, FolderIDs: []string{folder.ID}})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	job := waitJobDone(t, jobRepo, jobID)
	if job.Status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", job.Status)
	}

	run := waitWorkflowRunByJob(t, workflowRunRepo, jobID)
	if run.Status != "succeeded" {
		t.Fatalf("workflow run status = %q, want succeeded", run.Status)
	}

	movedFolder, err := folderRepo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("folderRepo.GetByID() after move error = %v", err)
	}
	if movedFolder.Path != "/target/rollback-me" {
		t.Fatalf("folder path after move = %q, want /target/rollback-me", movedFolder.Path)
	}

	if exists, err := adapter.Exists(ctx, "/target/rollback-me"); err != nil {
		t.Fatalf("adapter.Exists(target) error = %v", err)
	} else if !exists {
		t.Fatalf("target path missing after move")
	}
	if exists, err := adapter.Exists(ctx, "/source/rollback-me"); err != nil {
		t.Fatalf("adapter.Exists(source) error = %v", err)
	} else if exists {
		t.Fatalf("source path still exists after move, want moved away")
	}

	nodeRuns, _, err := nodeRunRepo.List(ctx, repository.NodeRunListFilter{WorkflowRunID: run.ID, Page: 1, Limit: 20})
	if err != nil {
		t.Fatalf("nodeRunRepo.List() error = %v", err)
	}
	if len(nodeRuns) != 1 {
		t.Fatalf("node run count = %d, want 1", len(nodeRuns))
	}

	snaps, err := nodeSnapshotRepo.ListByNodeRunID(ctx, nodeRuns[0].ID)
	if err != nil {
		t.Fatalf("nodeSnapshotRepo.ListByNodeRunID() error = %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("snapshot count = %d, want 2", len(snaps))
	}

	if err := svc.RollbackWorkflowRun(ctx, run.ID); err != nil {
		t.Fatalf("RollbackWorkflowRun() error = %v", err)
	}

	rolledBackRun := waitWorkflowRunIDStatus(t, workflowRunRepo, run.ID, "rolled_back")
	if rolledBackRun.Status != "rolled_back" {
		t.Fatalf("workflow run status after rollback = %q, want rolled_back", rolledBackRun.Status)
	}

	rolledBackFolder, err := folderRepo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("folderRepo.GetByID() after rollback error = %v", err)
	}
	if rolledBackFolder.Path != "/source/rollback-me" {
		t.Fatalf("folder path after rollback = %q, want /source/rollback-me", rolledBackFolder.Path)
	}

	if exists, err := adapter.Exists(ctx, "/source/rollback-me"); err != nil {
		t.Fatalf("adapter.Exists(source after rollback) error = %v", err)
	} else if !exists {
		t.Fatalf("source path missing after rollback")
	}
	if exists, err := adapter.Exists(ctx, "/target/rollback-me"); err != nil {
		t.Fatalf("adapter.Exists(target after rollback) error = %v", err)
	} else if exists {
		t.Fatalf("target path still exists after rollback, want moved back")
	}
}

func newPhase3WorkflowTestEnv(t *testing.T, adapter *fs.MockAdapter) (*WorkflowRunnerService, repository.JobRepository, repository.FolderRepository, repository.WorkflowDefinitionRepository, repository.WorkflowRunRepository, repository.NodeRunRepository, repository.NodeSnapshotRepository) {
	t.Helper()

	database := newServiceTestDB(t)
	jobRepo := repository.NewJobRepository(database)
	folderRepo := repository.NewFolderRepository(database)
	workflowDefRepo := repository.NewWorkflowDefinitionRepository(database)
	workflowRunRepo := repository.NewWorkflowRunRepository(database)
	nodeRunRepo := repository.NewNodeRunRepository(database)
	nodeSnapshotRepo := repository.NewNodeSnapshotRepository(database)
	auditRepo := repository.NewAuditRepository(database)

	svc := NewWorkflowRunnerService(jobRepo, folderRepo, workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo, adapter, nil, nil)
	svc.RegisterExecutor(NewFolderTreeScannerExecutor(adapter))
	svc.RegisterExecutor(NewNameKeywordClassifierExecutor())
	svc.RegisterExecutor(NewFileTreeClassifierExecutor())
	svc.RegisterExecutor(NewConfidenceCheckExecutor())
	svc.RegisterExecutor(NewManualClassifierExecutor())
	svc.RegisterExecutor(NewSubtreeAggregatorExecutor(folderRepo, NewAuditService(auditRepo)))

	return svc, jobRepo, folderRepo, workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo
}

func createPhase3WorkflowDef(t *testing.T, repo repository.WorkflowDefinitionRepository, id string, graph repository.WorkflowGraph) *repository.WorkflowDefinition {
	t.Helper()

	graphJSON, err := json.Marshal(graph)
	if err != nil {
		t.Fatalf("json.Marshal(graph) error = %v", err)
	}

	def := &repository.WorkflowDefinition{ID: id, Name: id, GraphJSON: string(graphJSON), IsActive: true, Version: 1}
	if err := repo.Create(context.Background(), def); err != nil {
		t.Fatalf("workflowDefRepo.Create() error = %v", err)
	}

	return def
}

func waitWorkflowRunByJob(t *testing.T, repo repository.WorkflowRunRepository, jobID string) *repository.WorkflowRun {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runs, _, err := repo.List(context.Background(), repository.WorkflowRunListFilter{JobID: jobID, Page: 1, Limit: 10})
		if err != nil {
			t.Fatalf("workflowRunRepo.List() error = %v", err)
		}
		if len(runs) > 0 {
			return runs[0]
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timeout waiting workflow run for job %q", jobID)
	return nil
}

func waitWorkflowRunStatus(t *testing.T, repo repository.WorkflowRunRepository, jobID, status string) *repository.WorkflowRun {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		run := waitWorkflowRunByJob(t, repo, jobID)
		if run.Status == status {
			return run
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timeout waiting workflow run for job %q to reach status %q", jobID, status)
	return nil
}

func waitWorkflowRunIDStatus(t *testing.T, repo repository.WorkflowRunRepository, runID, status string) *repository.WorkflowRun {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		run, err := repo.GetByID(context.Background(), runID)
		if err != nil {
			t.Fatalf("workflowRunRepo.GetByID() error = %v", err)
		}
		if run.Status == status {
			return run
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timeout waiting workflow run %q to reach status %q", runID, status)
	return nil
}

func nodeRunStatusByID(nodeRuns []*repository.NodeRun, nodeID string) string {
	for _, nodeRun := range nodeRuns {
		if nodeRun.NodeID == nodeID {
			return nodeRun.Status
		}
	}

	return ""
}
