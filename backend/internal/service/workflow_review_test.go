package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
	"github.com/liqiye/classifier/internal/sse"
)

func TestWorkflowRunnerServiceProcessingReviewApproveFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := newServiceTestDB(t)
	jobRepo := repository.NewJobRepository(database)
	folderRepo := repository.NewFolderRepository(database)
	workflowDefRepo := repository.NewWorkflowDefinitionRepository(database)
	workflowRunRepo := repository.NewWorkflowRunRepository(database)
	reviewRepo := repository.NewProcessingReviewRepository(database)
	nodeRunRepo := repository.NewNodeRunRepository(database)
	nodeSnapshotRepo := repository.NewNodeSnapshotRepository(database)

	folder := &repository.Folder{
		ID:             "folder-review-approve",
		Path:           "/source/review-approve",
		SourceDir:      "/source",
		RelativePath:   "review-approve",
		Name:           "review-approve",
		Category:       "photo",
		CategorySource: "manual",
		Status:         "pending",
	}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	producer := &processingItemSourceExecutor{items: []ProcessingItem{{
		FolderID:   folder.ID,
		SourcePath: folder.Path,
		FolderName: folder.Name,
		Category:   folder.Category,
	}}}
	graph := repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "source", Type: producer.Type(), Enabled: true},
			{ID: "rename", Type: renameNodeExecutorType, Enabled: true, Config: map[string]any{"strategy": "template", "template": "{name}-ok"}, Inputs: map[string]repository.NodeInputSpec{
				"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "source", SourcePort: "items"}},
			}},
		},
		Edges: []repository.WorkflowGraphEdge{
			{ID: "e1", Source: "source", SourcePort: "items", Target: "rename", TargetPort: "items"},
		},
	}
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		t.Fatalf("json.Marshal(graph) error = %v", err)
	}
	def := &repository.WorkflowDefinition{ID: "wf-review-approve", Name: "wf-review-approve", GraphJSON: string(graphJSON), IsActive: true, Version: 1}
	if err := workflowDefRepo.Create(ctx, def); err != nil {
		t.Fatalf("workflowDefRepo.Create() error = %v", err)
	}

	svc := NewWorkflowRunnerService(jobRepo, folderRepo, repository.NewSnapshotRepository(database), workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo, fs.NewMockAdapter(), nil, nil)
	svc.SetProcessingReviewRepository(reviewRepo)
	svc.RegisterExecutor(producer)

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	run := waitWorkflowRunStatus(t, workflowRunRepo, jobID, "waiting_input")
	job, err := jobRepo.GetByID(ctx, jobID)
	if err != nil {
		t.Fatalf("jobRepo.GetByID() error = %v", err)
	}
	if job.Status != "waiting_input" {
		t.Fatalf("job status = %q, want waiting_input", job.Status)
	}

	reviews, err := svc.ListProcessingReviews(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListProcessingReviews() error = %v", err)
	}
	if reviews.Summary.Total != 1 || reviews.Summary.Pending != 1 {
		t.Fatalf("review summary = %+v, want total=1 pending=1", reviews.Summary)
	}

	if err := svc.ApproveProcessingReview(ctx, run.ID, reviews.Items[0].ID); err != nil {
		t.Fatalf("ApproveProcessingReview() error = %v", err)
	}

	run = waitWorkflowRunIDStatus(t, workflowRunRepo, run.ID, "succeeded")
	if run.Status != "succeeded" {
		t.Fatalf("workflow run status = %q, want succeeded", run.Status)
	}
	job = waitJobDone(t, jobRepo, jobID)
	if job.Status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", job.Status)
	}
	updatedFolder, err := folderRepo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("folderRepo.GetByID() error = %v", err)
	}
	if updatedFolder.Status != "done" {
		t.Fatalf("folder status = %q, want done", updatedFolder.Status)
	}
}

func TestWorkflowRunnerServiceWorkflowRunUpdatedEventOnReviewFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := newServiceTestDB(t)
	jobRepo := repository.NewJobRepository(database)
	folderRepo := repository.NewFolderRepository(database)
	workflowDefRepo := repository.NewWorkflowDefinitionRepository(database)
	workflowRunRepo := repository.NewWorkflowRunRepository(database)
	reviewRepo := repository.NewProcessingReviewRepository(database)
	nodeRunRepo := repository.NewNodeRunRepository(database)
	nodeSnapshotRepo := repository.NewNodeSnapshotRepository(database)
	broker := sse.NewBroker()
	events := broker.Subscribe()
	defer broker.Unsubscribe(events)

	folder := &repository.Folder{
		ID:             "folder-review-updated-event",
		Path:           "/source/review-updated-event",
		SourceDir:      "/source",
		RelativePath:   "review-updated-event",
		Name:           "review-updated-event",
		Category:       "photo",
		CategorySource: "manual",
		Status:         "pending",
	}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	producer := &processingItemSourceExecutor{items: []ProcessingItem{{
		FolderID:   folder.ID,
		SourcePath: folder.Path,
		FolderName: folder.Name,
		Category:   folder.Category,
	}}}
	graph := repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "source", Type: producer.Type(), Enabled: true},
			{ID: "rename", Type: renameNodeExecutorType, Enabled: true, Config: map[string]any{"strategy": "template", "template": "{name}-ok"}, Inputs: map[string]repository.NodeInputSpec{
				"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "source", SourcePort: "items"}},
			}},
		},
		Edges: []repository.WorkflowGraphEdge{
			{ID: "e1", Source: "source", SourcePort: "items", Target: "rename", TargetPort: "items"},
		},
	}
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		t.Fatalf("json.Marshal(graph) error = %v", err)
	}
	def := &repository.WorkflowDefinition{ID: "wf-review-updated-event", Name: "wf-review-updated-event", GraphJSON: string(graphJSON), IsActive: true, Version: 1}
	if err := workflowDefRepo.Create(ctx, def); err != nil {
		t.Fatalf("workflowDefRepo.Create() error = %v", err)
	}

	svc := NewWorkflowRunnerService(jobRepo, folderRepo, repository.NewSnapshotRepository(database), workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo, fs.NewMockAdapter(), broker, nil)
	svc.SetProcessingReviewRepository(reviewRepo)
	svc.RegisterExecutor(producer)

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	run := waitWorkflowRunStatus(t, workflowRunRepo, jobID, "waiting_input")
	waitWorkflowRunUpdatedStatus(t, events, run.ID, "waiting_input", jobID, def.ID)

	reviews, err := svc.ListProcessingReviews(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListProcessingReviews() error = %v", err)
	}
	if len(reviews.Items) == 0 {
		t.Fatalf("review items empty, want at least 1")
	}

	if err := svc.ApproveProcessingReview(ctx, run.ID, reviews.Items[0].ID); err != nil {
		t.Fatalf("ApproveProcessingReview() error = %v", err)
	}

	waitWorkflowRunUpdatedStatus(t, events, run.ID, "succeeded", jobID, def.ID)
}

func waitWorkflowRunUpdatedStatus(t *testing.T, events <-chan sse.Event, runID, status, jobID, workflowDefID string) {
	t.Helper()

	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting workflow_run.updated status=%q run=%q", status, runID)
		case event := <-events:
			if event.Type != "workflow_run.updated" {
				continue
			}
			var payload struct {
				JobID         string  `json:"job_id"`
				WorkflowRunID string  `json:"workflow_run_id"`
				WorkflowDefID string  `json:"workflow_def_id"`
				Status        string  `json:"status"`
				LastNodeID    string  `json:"last_node_id"`
				ResumeNodeID  *string `json:"resume_node_id"`
				Error         string  `json:"error"`
			}
			if err := json.Unmarshal(event.Data, &payload); err != nil {
				t.Fatalf("json.Unmarshal(workflow_run.updated) error = %v", err)
			}
			if payload.WorkflowRunID != runID || payload.Status != status {
				continue
			}
			if payload.JobID != jobID {
				t.Fatalf("workflow_run.updated job_id = %q, want %q", payload.JobID, jobID)
			}
			if payload.WorkflowDefID != workflowDefID {
				t.Fatalf("workflow_run.updated workflow_def_id = %q, want %q", payload.WorkflowDefID, workflowDefID)
			}
			if status == "running" && payload.ResumeNodeID != nil && *payload.ResumeNodeID != "" {
				t.Fatalf("running workflow_run.updated resume_node_id = %q, want empty", *payload.ResumeNodeID)
			}
			return
		}
	}
}

func TestWorkflowRunnerPrepareProcessingReviews_AutoAggregateWithoutStepResultEdges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := newServiceTestDB(t)
	jobRepo := repository.NewJobRepository(database)
	folderRepo := repository.NewFolderRepository(database)
	workflowDefRepo := repository.NewWorkflowDefinitionRepository(database)
	workflowRunRepo := repository.NewWorkflowRunRepository(database)
	reviewRepo := repository.NewProcessingReviewRepository(database)
	nodeRunRepo := repository.NewNodeRunRepository(database)
	nodeSnapshotRepo := repository.NewNodeSnapshotRepository(database)

	folder := &repository.Folder{
		ID:             "folder-review-auto",
		Path:           "/source/review-auto",
		SourceDir:      "/source",
		RelativePath:   "review-auto",
		Name:           "review-auto",
		Category:       "photo",
		CategorySource: "manual",
		Status:         "pending",
	}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	job := &repository.Job{
		ID:            "job-review-auto",
		Type:          "workflow",
		WorkflowDefID: "wf-review-auto",
		Status:        "running",
		FolderIDs:     `["folder-review-auto"]`,
		Total:         1,
	}
	if err := jobRepo.Create(ctx, job); err != nil {
		t.Fatalf("jobRepo.Create() error = %v", err)
	}

	runStartedAt := time.Now()
	run := &repository.WorkflowRun{
		ID:            "run-review-auto",
		JobID:         job.ID,
		FolderID:      folder.ID,
		WorkflowDefID: job.WorkflowDefID,
		Status:        "running",
		StartedAt:     &runStartedAt,
	}
	if err := workflowRunRepo.Create(ctx, run); err != nil {
		t.Fatalf("workflowRunRepo.Create() error = %v", err)
	}

	makeOutputJSON := func(outputs map[string]TypedValue) string {
		encoded, err := typedValueMapToJSON(outputs, NewTypeRegistry())
		if err != nil {
			t.Fatalf("typedValueMapToJSON() error = %v", err)
		}
		raw, err := json.Marshal(encoded)
		if err != nil {
			t.Fatalf("json.Marshal(outputs) error = %v", err)
		}
		return string(raw)
	}
	makeInputJSON := func(nodeType, nodeLabel string, config map[string]any, inputs map[string]any) string {
		raw, err := json.Marshal(map[string]any{
			"node": map[string]any{
				"type":   nodeType,
				"label":  nodeLabel,
				"config": config,
			},
			"inputs": inputs,
		})
		if err != nil {
			t.Fatalf("json.Marshal(inputs) error = %v", err)
		}
		return string(raw)
	}

	renamedName := "review-auto-renamed"
	movedPath := "/target/review-auto-renamed"
	compressDir := "/archives"
	thumbDir := "/thumbs"

	nodeRuns := []*repository.NodeRun{
		{
			ID:            "nr-rename",
			WorkflowRunID: run.ID,
			NodeID:        "rename",
			NodeType:      "rename-node",
			Sequence:      1,
			Status:        "succeeded",
			InputJSON: makeInputJSON("rename-node", "重命名", map[string]any{
				"strategy": "template",
				"template": "{name}-renamed",
			}, map[string]any{
				"items": []ProcessingItem{{FolderID: folder.ID, SourcePath: folder.Path, FolderName: folder.Name}},
			}),
			OutputJSON: makeOutputJSON(map[string]TypedValue{
				"items":        {Type: PortTypeProcessingItemList, Value: []ProcessingItem{{FolderID: folder.ID, SourcePath: folder.Path, FolderName: folder.Name, TargetName: renamedName}}},
				"step_results": {Type: PortTypeProcessingStepResultList, Value: []ProcessingStepResult{{SourcePath: folder.Path, TargetPath: renamedName, NodeType: "rename-node", Status: "renamed"}}},
			}),
		},
		{
			ID:            "nr-move",
			WorkflowRunID: run.ID,
			NodeID:        "move",
			NodeType:      "move-node",
			Sequence:      2,
			Status:        "succeeded",
			InputJSON: makeInputJSON("move-node", "移动", map[string]any{"target_dir": "/target"}, map[string]any{
				"items": []ProcessingItem{{FolderID: folder.ID, SourcePath: folder.Path, FolderName: folder.Name, TargetName: renamedName}},
			}),
			OutputJSON: makeOutputJSON(map[string]TypedValue{
				"items":        {Type: PortTypeProcessingItemList, Value: []ProcessingItem{{FolderID: folder.ID, SourcePath: movedPath, FolderName: folder.Name, TargetName: renamedName}}},
				"step_results": {Type: PortTypeProcessingStepResultList, Value: []ProcessingStepResult{{SourcePath: folder.Path, TargetPath: movedPath, NodeType: "move-node", Status: "moved"}}},
			}),
		},
		{
			ID:            "nr-compress",
			WorkflowRunID: run.ID,
			NodeID:        "compress",
			NodeType:      "compress-node",
			Sequence:      3,
			Status:        "succeeded",
			InputJSON: makeInputJSON("compress-node", "压缩", map[string]any{"target_dir": compressDir, "format": "cbz"}, map[string]any{
				"items": []ProcessingItem{{FolderID: folder.ID, SourcePath: movedPath, FolderName: folder.Name, TargetName: renamedName}},
			}),
			OutputJSON: makeOutputJSON(map[string]TypedValue{
				"items":        {Type: PortTypeProcessingItemList, Value: []ProcessingItem{{FolderID: folder.ID, SourcePath: movedPath, FolderName: folder.Name, TargetName: renamedName}}},
				"step_results": {Type: PortTypeProcessingStepResultList, Value: []ProcessingStepResult{{SourcePath: movedPath, TargetPath: compressDir + "/" + renamedName + ".cbz", NodeType: "compress-node", Status: "succeeded"}}},
			}),
		},
		{
			ID:            "nr-thumbnail",
			WorkflowRunID: run.ID,
			NodeID:        "thumbnail",
			NodeType:      "thumbnail-node",
			Sequence:      4,
			Status:        "succeeded",
			InputJSON: makeInputJSON("thumbnail-node", "缩略图", map[string]any{"output_dir": thumbDir}, map[string]any{
				"items": []ProcessingItem{{FolderID: folder.ID, SourcePath: movedPath, FolderName: folder.Name, TargetName: renamedName}},
			}),
			OutputJSON: makeOutputJSON(map[string]TypedValue{
				"items":        {Type: PortTypeProcessingItemList, Value: []ProcessingItem{{FolderID: folder.ID, SourcePath: movedPath, FolderName: folder.Name, TargetName: renamedName}}},
				"step_results": {Type: PortTypeProcessingStepResultList, Value: []ProcessingStepResult{{SourcePath: movedPath, TargetPath: thumbDir + "/" + renamedName + ".jpg", NodeType: "thumbnail-node", Status: "succeeded"}}},
			}),
		},
	}
	for _, item := range nodeRuns {
		if err := nodeRunRepo.Create(ctx, item); err != nil {
			t.Fatalf("nodeRunRepo.Create(%s) error = %v", item.ID, err)
		}
	}
	storedNodeRuns, _, err := nodeRunRepo.List(ctx, repository.NodeRunListFilter{WorkflowRunID: run.ID, Page: 1, Limit: 20})
	if err != nil {
		t.Fatalf("nodeRunRepo.List() error = %v", err)
	}
	for _, nodeRun := range storedNodeRuns {
		typedOutputs, typed, parseErr := parseTypedNodeOutputs(nodeRun.OutputJSON)
		if parseErr != nil || !typed {
			t.Fatalf("parseTypedNodeOutputs(%s) error = %v typed=%v", nodeRun.NodeID, parseErr, typed)
		}
		if len(processingStepResultsFromNodeRun(nodeRun, typedOutputs)) == 0 {
			t.Fatalf("processingStepResultsFromNodeRun(%s) returned empty", nodeRun.NodeID)
		}
	}

	svc := NewWorkflowRunnerService(jobRepo, folderRepo, repository.NewSnapshotRepository(database), workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo, fs.NewMockAdapter(), nil, nil)
	svc.SetProcessingReviewRepository(reviewRepo)

	prepared, err := svc.prepareProcessingReviews(ctx, run.ID, folder)
	if err != nil {
		t.Fatalf("prepareProcessingReviews() error = %v", err)
	}
	if !prepared {
		t.Fatalf("prepareProcessingReviews() = false, want true")
	}

	reviews, err := svc.ListProcessingReviews(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListProcessingReviews() error = %v", err)
	}
	if reviews.Summary.Total != 1 {
		t.Fatalf("review total = %d, want 1", reviews.Summary.Total)
	}
	if len(reviews.Items) != 1 {
		t.Fatalf("review items len = %d, want 1", len(reviews.Items))
	}

	var steps []ProcessingStepResult
	if err := json.Unmarshal(reviews.Items[0].StepResultsJSON, &steps); err != nil {
		t.Fatalf("json.Unmarshal(step_results) error = %v", err)
	}
	if len(steps) != 4 {
		t.Fatalf("step_results len = %d, want 4", len(steps))
	}
	typeSet := map[string]struct{}{}
	for _, step := range steps {
		typeSet[step.NodeType] = struct{}{}
	}
	for _, nodeType := range []string{"rename-node", "move-node", "compress-node", "thumbnail-node"} {
		if _, ok := typeSet[nodeType]; !ok {
			t.Fatalf("missing step result for node type %q", nodeType)
		}
	}
}
