package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
)

func TestPhase3WorkflowE2E_BatchSourceDirClassification(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	adapter := fs.NewMockAdapter()
	adapter.AddDir("/source", []fs.DirEntry{{Name: "漫画合集", IsDir: true}, {Name: "cbz-only", IsDir: true}, {Name: "video-pack", IsDir: true}})
	adapter.AddDir("/source/漫画合集", []fs.DirEntry{{Name: "001.jpg", IsDir: false, Size: 100}})
	adapter.AddDir("/source/cbz-only", []fs.DirEntry{{Name: "vol1.cbz", IsDir: false, Size: 100}})
	adapter.AddDir("/source/video-pack", []fs.DirEntry{{Name: "ep1.mp4", IsDir: false, Size: 200}, {Name: "ep2.mkv", IsDir: false, Size: 220}})

	svc, jobRepo, folderRepo, workflowDefRepo, workflowRunRepo, nodeRunRepo, _ := newPhase3WorkflowTestEnv(t, adapter)

	def := createPhase3WorkflowDef(t, workflowDefRepo, "wf-batch-source", repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "scanner", Type: folderTreeScannerExecutorType, Enabled: true, Config: map[string]any{"source_dir": "/source"}},
			{ID: "kw", Type: "name-keyword-classifier", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"folder": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "scanner", OutputPortIndex: 0}},
			}},
			{ID: "ft", Type: "file-tree-classifier", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"folder": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "scanner", OutputPortIndex: 0}},
			}},
			{ID: "ext", Type: "ext-ratio-classifier", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"folder": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "scanner", OutputPortIndex: 0}},
			}},
			{ID: "cc", Type: "confidence-check", Enabled: true, Config: map[string]any{"threshold": 0.75}, Inputs: map[string]repository.NodeInputSpec{
				"signal": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "ext", OutputPortIndex: 0}},
			}},
			{ID: "manual", Type: "manual-classifier", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"trees": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "scanner", OutputPortIndex: 0}},
				"hint":  {LinkSource: &repository.NodeLinkSource{SourceNodeID: "cc", OutputPortIndex: 1}},
			}},
			{ID: "agg", Type: subtreeAggregatorExecutorType, Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"node":          {LinkSource: &repository.NodeLinkSource{SourceNodeID: "scanner", OutputPortIndex: 0}},
				"signal_kw":     {LinkSource: &repository.NodeLinkSource{SourceNodeID: "kw", OutputPortIndex: 0}},
				"signal_ft":     {LinkSource: &repository.NodeLinkSource{SourceNodeID: "ft", OutputPortIndex: 0}},
				"signal_high":   {LinkSource: &repository.NodeLinkSource{SourceNodeID: "cc", OutputPortIndex: 0}},
				"signal_manual": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "manual", OutputPortIndex: 0}},
			}},
		},
	})

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID, SourceDir: "/source"})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	job := waitJobDone(t, jobRepo, jobID)
	if job.Status != "succeeded" {
		run := waitWorkflowRunByJob(t, workflowRunRepo, jobID)
		nodeRuns, _, listErr := nodeRunRepo.List(ctx, repository.NodeRunListFilter{WorkflowRunID: run.ID, Page: 1, Limit: 50})
		if listErr != nil {
			t.Fatalf("job status = %q, want succeeded; failed listing node runs: %v", job.Status, listErr)
		}
		t.Fatalf("job status = %q, want succeeded (workflow_run_status=%q resume_node_id=%q node_runs=%v)", job.Status, run.Status, run.ResumeNodeID, compactNodeRuns(nodeRuns))
	}

	check := []struct {
		path string
		want string
	}{
		{path: "/source/漫画合集", want: "manga"},
		{path: "/source/cbz-only", want: "manga"},
		{path: "/source/video-pack", want: "video"},
	}
	for _, tc := range check {
		folder, getErr := folderRepo.GetByPath(ctx, tc.path)
		if getErr != nil {
			t.Fatalf("folderRepo.GetByPath(%q) error = %v", tc.path, getErr)
		}
		if folder.Category != tc.want {
			t.Fatalf("folder %q category = %q, want %q", tc.path, folder.Category, tc.want)
		}
	}

	run := waitWorkflowRunByJob(t, workflowRunRepo, jobID)
	nodeRuns, _, err := nodeRunRepo.List(ctx, repository.NodeRunListFilter{WorkflowRunID: run.ID, Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("nodeRunRepo.List() error = %v", err)
	}
	if got := nodeRunStatusByID(nodeRuns, "manual"); got != "succeeded" {
		t.Fatalf("manual node status = %q, want succeeded", got)
	}
}

func TestPhase3WorkflowE2E_BatchManualResume(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	adapter := fs.NewMockAdapter()
	adapter.AddDir("/source", []fs.DirEntry{{Name: "needs-a", IsDir: true}, {Name: "needs-b", IsDir: true}})
	adapter.AddDir("/source/needs-a", []fs.DirEntry{{Name: "notes.txt", IsDir: false, Size: 10}})
	adapter.AddDir("/source/needs-b", []fs.DirEntry{{Name: "readme.md", IsDir: false, Size: 10}})

	svc, _, folderRepo, workflowDefRepo, workflowRunRepo, nodeRunRepo, _ := newPhase3WorkflowTestEnv(t, adapter)

	def := createPhase3WorkflowDef(t, workflowDefRepo, "wf-batch-manual", repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "scanner", Type: folderTreeScannerExecutorType, Enabled: true, Config: map[string]any{"source_dir": "/source"}},
			{ID: "kw", Type: "name-keyword-classifier", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"folder": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "scanner", OutputPortIndex: 0}},
			}},
			{ID: "ft", Type: "file-tree-classifier", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"folder": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "scanner", OutputPortIndex: 0}},
			}},
			{ID: "ext", Type: "ext-ratio-classifier", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"folder": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "scanner", OutputPortIndex: 0}},
			}},
			{ID: "cc", Type: "confidence-check", Enabled: true, Config: map[string]any{"threshold": 0.9}, Inputs: map[string]repository.NodeInputSpec{
				"signal": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "ext", OutputPortIndex: 0}},
			}},
			{ID: "manual", Type: "manual-classifier", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"trees": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "scanner", OutputPortIndex: 0}},
				"hint":  {LinkSource: &repository.NodeLinkSource{SourceNodeID: "cc", OutputPortIndex: 1}},
			}},
			{ID: "agg", Type: subtreeAggregatorExecutorType, Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"node":          {LinkSource: &repository.NodeLinkSource{SourceNodeID: "scanner", OutputPortIndex: 0}},
				"signal_kw":     {LinkSource: &repository.NodeLinkSource{SourceNodeID: "kw", OutputPortIndex: 0}},
				"signal_ft":     {LinkSource: &repository.NodeLinkSource{SourceNodeID: "ft", OutputPortIndex: 0}},
				"signal_high":   {LinkSource: &repository.NodeLinkSource{SourceNodeID: "cc", OutputPortIndex: 0}},
				"signal_manual": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "manual", OutputPortIndex: 0}},
			}},
		},
	})

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID, SourceDir: "/source"})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	run := waitWorkflowRunByJob(t, workflowRunRepo, jobID)
	deadline := time.Now().Add(2 * time.Second)
	for run.Status != "waiting_input" && time.Now().Before(deadline) {
		if run.Status == "failed" || run.Status == "succeeded" {
			break
		}
		time.Sleep(10 * time.Millisecond)
		fresh, getErr := workflowRunRepo.GetByID(ctx, run.ID)
		if getErr != nil {
			t.Fatalf("workflowRunRepo.GetByID() error = %v", getErr)
		}
		run = fresh
	}
	if run.Status != "waiting_input" {
		nodeRuns, _, listErr := nodeRunRepo.List(ctx, repository.NodeRunListFilter{WorkflowRunID: run.ID, Page: 1, Limit: 50})
		if listErr != nil {
			t.Fatalf("workflow run status = %q, want waiting_input; failed listing node runs: %v", run.Status, listErr)
		}
		t.Fatalf("workflow run status = %q, want waiting_input (resume_node_id=%q node_runs=%v)", run.Status, run.ResumeNodeID, compactNodeRuns(nodeRuns))
	}
	if run.ResumeNodeID != "manual" {
		t.Fatalf("resume node id = %q, want manual", run.ResumeNodeID)
	}

	err = svc.ResumeWorkflowRunWithData(ctx, run.ID, map[string]any{
		"classifications": []any{
			map[string]any{"source_path": "/source/needs-a", "category": "video"},
			map[string]any{"source_path": "/source/needs-b", "category": "photo"},
		},
	})
	if err != nil {
		t.Fatalf("ResumeWorkflowRunWithData() error = %v", err)
	}

	_ = waitWorkflowRunIDStatus(t, workflowRunRepo, run.ID, "succeeded")

	folderA, err := folderRepo.GetByPath(ctx, "/source/needs-a")
	if err != nil {
		t.Fatalf("folderRepo.GetByPath(needs-a) error = %v", err)
	}
	folderB, err := folderRepo.GetByPath(ctx, "/source/needs-b")
	if err != nil {
		t.Fatalf("folderRepo.GetByPath(needs-b) error = %v", err)
	}
	if folderA.Category != "video" {
		t.Fatalf("needs-a category = %q, want video", folderA.Category)
	}
	if folderB.Category != "photo" {
		t.Fatalf("needs-b category = %q, want photo", folderB.Category)
	}
}

func TestPhase3WorkflowE2E_MoveRollback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	adapter := fs.NewMockAdapter()
	adapter.AddDir("/source", []fs.DirEntry{{Name: "rollback-me", IsDir: true}})
	adapter.AddDir("/source/rollback-me", []fs.DirEntry{{Name: "001.jpg", IsDir: false, Size: 100}})

	svc, jobRepo, folderRepo, workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo := newPhase3WorkflowTestEnv(t, adapter)

	folder := &repository.Folder{ID: "folder-move-rollback", Path: "/source/rollback-me", Name: "rollback-me", Category: "photo", CategorySource: "workflow", Status: "pending"}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	def := createPhase3WorkflowDef(t, workflowDefRepo, "wf-move-rollback", repository.WorkflowGraph{Nodes: []repository.WorkflowGraphNode{{ID: "move", Type: "move", Enabled: true, Config: map[string]any{"target_dir": "/target"}}}})

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID, FolderIDs: []string{folder.ID}})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	job := waitJobDone(t, jobRepo, jobID)
	if job.Status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", job.Status)
	}

	run := waitWorkflowRunByJob(t, workflowRunRepo, jobID)
	movedFolder, err := folderRepo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("folderRepo.GetByID() after move error = %v", err)
	}
	if movedFolder.Path != "/target/rollback-me" {
		t.Fatalf("folder path after move = %q, want /target/rollback-me", movedFolder.Path)
	}

	nodeRuns, _, err := nodeRunRepo.List(ctx, repository.NodeRunListFilter{WorkflowRunID: run.ID, Page: 1, Limit: 20})
	if err != nil {
		t.Fatalf("nodeRunRepo.List() error = %v", err)
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
	rolledBackFolder, err := folderRepo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("folderRepo.GetByID() after rollback error = %v", err)
	}
	if rolledBackFolder.Path != "/source/rollback-me" {
		t.Fatalf("folder path after rollback = %q, want /source/rollback-me", rolledBackFolder.Path)
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

func compactNodeRuns(nodeRuns []*repository.NodeRun) []string {
	items := make([]string, 0, len(nodeRuns))
	for _, nodeRun := range nodeRuns {
		items = append(items, nodeRun.NodeID+":"+nodeRun.Status+":"+nodeRun.Error)
	}

	return items
}
