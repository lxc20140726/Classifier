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
	return NodeExecutionOutput{Outputs: map[string]TypedValue{"ok": {Type: PortTypeBoolean, Value: true}}, Status: ExecutionSuccess}, nil
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
	return NodeSchema{Type: e.Type(), Label: "Produce Input", Description: "Produce a fixed output for tests", Outputs: []PortDef{{Name: "out", Type: PortTypeString}}}
}

func (e *produceInputExecutor) Execute(_ context.Context, _ NodeExecutionInput) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{Outputs: map[string]TypedValue{"out": {Type: PortTypeString, Value: "hello-port"}}, Status: ExecutionSuccess}, nil
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
	outputs  map[string]TypedValue
}

type namedPortProducerExecutor struct{}

type namedPortConsumerExecutor struct {
	seen string
}

type requiredInputProbeExecutor struct {
	nodeType  string
	portName  string
	lazy      bool
	executed  int32
	lastValue any
}

type resumeDataMergeExecutor struct {
	lastResume map[string]any
}

type processingItemSourceExecutor struct {
	items []ProcessingItem
}

func (e *consumeInputExecutor) Type() string {
	return "consume-input"
}

func (e *consumeInputExecutor) Schema() NodeSchema {
	return NodeSchema{Type: e.Type(), Label: "Consume Input", Description: "Consume upstream output for tests", Inputs: []PortDef{{Name: "upstream", Type: PortTypeString, Required: true}}, Outputs: []PortDef{{Name: "echo", Type: PortTypeString}}}
}

func (e *consumeInputExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	raw, ok := input.Inputs["upstream"]
	if !ok {
		return NodeExecutionOutput{}, nil
	}
	if raw == nil {
		return NodeExecutionOutput{}, nil
	}
	text, _ := raw.Value.(string)
	e.seen = text
	return NodeExecutionOutput{Outputs: map[string]TypedValue{"echo": {Type: PortTypeString, Value: text}}, Status: ExecutionSuccess}, nil
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

	folderID := ""
	if input.Folder != nil {
		folderID = input.Folder.ID
	}

	e.mu.Lock()
	e.visited = append(e.visited, folderID)
	e.mu.Unlock()

	time.Sleep(50 * time.Millisecond)
	atomic.AddInt32(&e.active, -1)
	return NodeExecutionOutput{Outputs: map[string]TypedValue{"folder_id": {Type: PortTypeString, Value: folderID}}, Status: ExecutionSuccess}, nil
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

func (e *namedPortProducerExecutor) Type() string {
	return "named-port-producer"
}

func (e *namedPortProducerExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "Named Port Producer",
		Description: "Emit multi-port outputs for named-port compatibility tests",
		Outputs: []PortDef{{Name: "first"}, {Name: "second"}},
	}
}

func (e *namedPortProducerExecutor) Execute(_ context.Context, _ NodeExecutionInput) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{Outputs: map[string]TypedValue{"first": {Type: PortTypeString, Value: "first-value"}, "second": {Type: PortTypeString, Value: "second-value"}}, Status: ExecutionSuccess}, nil
}

func (e *namedPortProducerExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, nil
}

func (e *namedPortProducerExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func (e *namedPortConsumerExecutor) Type() string {
	return "named-port-consumer"
}

func (e *namedPortConsumerExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "Named Port Consumer",
		Description: "Consume upstream output through named source port",
		Inputs: []PortDef{{Name: "upstream", Required: true}},
	}
}

func (e *namedPortConsumerExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	if input.Inputs["upstream"] != nil {
		if value, ok := input.Inputs["upstream"].Value.(string); ok {
			e.seen = value
		}
	}

	return NodeExecutionOutput{Outputs: map[string]TypedValue{"seen": {Type: PortTypeString, Value: e.seen}}, Status: ExecutionSuccess}, nil
}

func (e *namedPortConsumerExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, nil
}

func (e *namedPortConsumerExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func (e *requiredInputProbeExecutor) Type() string {
	return e.nodeType
}

func (e *requiredInputProbeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       e.Type(),
		Description: "Probe skip behavior for required inputs",
		Inputs: []PortDef{{Name: e.portName, Required: true, Lazy: e.lazy}},
	}
}

func (e *requiredInputProbeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	atomic.AddInt32(&e.executed, 1)
	if input.Inputs[e.portName] != nil {
		e.lastValue = input.Inputs[e.portName].Value
	}
	return NodeExecutionOutput{Outputs: map[string]TypedValue{"value": {Type: PortTypeJSON, Value: e.lastValue}}, Status: ExecutionSuccess}, nil
}

func (e *requiredInputProbeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, nil
}

func (e *requiredInputProbeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func (e *resumeDataMergeExecutor) Type() string {
	return "resume-data-merge"
}

func (e *resumeDataMergeExecutor) Schema() NodeSchema {
	return NodeSchema{Type: e.Type(), Label: "Resume Data Merge", Description: "Verify DB-backed resume-data merge behavior"}
}

func (e *resumeDataMergeExecutor) Execute(_ context.Context, _ NodeExecutionInput) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{Status: ExecutionPending, PendingReason: "need resume data"}, nil
}

func (e *resumeDataMergeExecutor) Resume(_ context.Context, _ NodeExecutionInput, resumeData map[string]any) (NodeExecutionOutput, error) {
	e.lastResume = make(map[string]any, len(resumeData))
	for key, value := range resumeData {
		e.lastResume[key] = value
	}

	if _, ok := resumeData["category"]; !ok {
		return NodeExecutionOutput{Status: ExecutionPending, PendingReason: "category is required"}, nil
	}

	return NodeExecutionOutput{Outputs: map[string]TypedValue{"category": {Type: PortTypeString, Value: resumeData["category"]}}, Status: ExecutionSuccess}, nil
}

func (e *resumeDataMergeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func (e *processingItemSourceExecutor) Type() string {
	return "processing-item-source"
}

func (e *processingItemSourceExecutor) Schema() NodeSchema {
	return NodeSchema{Type: e.Type(), Label: "Processing Item Source", Description: "Emit processing items for rollback tests", Outputs: []PortDef{{Name: "items", Type: PortTypeProcessingItemList}}}
}

func (e *processingItemSourceExecutor) Execute(_ context.Context, _ NodeExecutionInput) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{Outputs: map[string]TypedValue{"items": {Type: PortTypeProcessingItemList, Value: append([]ProcessingItem(nil), e.items...)}}, Status: ExecutionSuccess}, nil
}

func (e *processingItemSourceExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, nil
}

func (e *processingItemSourceExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
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
		repository.NewSnapshotRepository(database),
		workflowDefRepo,
		workflowRunRepo,
		nodeRunRepo,
		nodeSnapshotRepo,
		adapter,
		nil,
		nil,
	)

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID})
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
		Edges: []repository.WorkflowGraphEdge{{Source: "n1", SourcePortIndex: 0, Target: "n2", TargetPortIndex: 0}},
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
		repository.NewSnapshotRepository(database),
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

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID})
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
	svc := NewWorkflowRunnerService(jobRepo, folderRepo, repository.NewSnapshotRepository(database), workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo, adapter, nil, nil)
	svc.RegisterExecutor(executor)

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	job := waitJobDone(t, jobRepo, jobID)
	if job.Status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", job.Status)
	}
	if atomic.LoadInt32(&executor.maxActive) != 1 {
		t.Fatalf("maxActive = %d, want 1 in v2 single-run mode", atomic.LoadInt32(&executor.maxActive))
	}
	if len(executor.visited) != 1 {
		t.Fatalf("visited len = %d, want 1 in v2 single-run mode", len(executor.visited))
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
		outputs    map[string]TypedValue
		action     string
		result     string
		folderPath string
	}{
		{
			name:       "move-node",
			nodeType:   "move-node",
			outputs:    map[string]TypedValue{"items": {Type: PortTypeProcessingItemList, Value: []ProcessingItem{{FolderID: folder.ID, SourcePath: "/target/folder-audit", FolderName: folder.Name}}}, "results": {Type: PortTypeMoveResultList, Value: []MoveResult{{SourcePath: folder.Path, TargetPath: "/target/folder-audit", Status: "moved"}}}},
			action:     "workflow.move-node",
			result:     "moved",
			folderPath: "/target/folder-audit",
		},
		{
			name:       "compress-node",
			nodeType:   "compress-node",
			outputs:    map[string]TypedValue{"items": {Type: PortTypeProcessingItemList, Value: []ProcessingItem{{FolderID: folder.ID, SourcePath: folder.Path, FolderName: folder.Name}}}, "archives": {Type: PortTypeStringList, Value: []string{"/archives/folder-audit.cbz"}}},
			action:     "workflow.compress-node",
			result:     "success",
			folderPath: "/archives/folder-audit.cbz",
		},
		{
			name:       "thumbnail-node",
			nodeType:   "thumbnail-node",
			outputs:    map[string]TypedValue{"items": {Type: PortTypeProcessingItemList, Value: []ProcessingItem{{FolderID: folder.ID, SourcePath: folder.Path, FolderName: folder.Name}}}, "thumbnail_paths": {Type: PortTypeStringList, Value: []string{"/thumbs/folder-audit.jpg"}}},
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

			svc := NewWorkflowRunnerService(jobRepo, folderRepo, repository.NewSnapshotRepository(database), workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo, fs.NewMockAdapter(), nil, auditSvc)
			svc.RegisterExecutor(&auditOutputExecutor{nodeType: tc.nodeType, outputs: tc.outputs})

			jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID})
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

func TestWorkflowRunnerServiceNamedSourcePortCompatibility(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := newServiceTestDB(t)
	jobRepo := repository.NewJobRepository(database)
	folderRepo := repository.NewFolderRepository(database)
	workflowDefRepo := repository.NewWorkflowDefinitionRepository(database)
	workflowRunRepo := repository.NewWorkflowRunRepository(database)
	nodeRunRepo := repository.NewNodeRunRepository(database)
	nodeSnapshotRepo := repository.NewNodeSnapshotRepository(database)

	graph := repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "n1", Type: "named-port-producer", Enabled: true},
			{ID: "n2", Type: "named-port-consumer", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"upstream": {
					LinkSource: &repository.NodeLinkSource{SourceNodeID: "n1", SourcePort: "second", OutputPortIndex: 0},
				},
			}},
		},
		Edges: []repository.WorkflowGraphEdge{{Source: "n1", SourcePort: "second", Target: "n2", TargetPort: "upstream"}},
	}
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		t.Fatalf("json.Marshal(graph) error = %v", err)
	}

	def := &repository.WorkflowDefinition{ID: "wf-named-port", Name: "wf-named-port", GraphJSON: string(graphJSON), IsActive: true, Version: 1}
	if err := workflowDefRepo.Create(ctx, def); err != nil {
		t.Fatalf("workflowDefRepo.Create() error = %v", err)
	}

	consumer := &namedPortConsumerExecutor{}
	svc := NewWorkflowRunnerService(jobRepo, folderRepo, repository.NewSnapshotRepository(database), workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo, fs.NewMockAdapter(), nil, nil)
	svc.RegisterExecutor(&namedPortProducerExecutor{})
	svc.RegisterExecutor(consumer)

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	job := waitJobDone(t, jobRepo, jobID)
	if job.Status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", job.Status)
	}
	if consumer.seen != "second-value" {
		t.Fatalf("consumer input = %q, want second-value", consumer.seen)
	}
}

func TestWorkflowRunnerServiceLazyRequiredInputSkipSemantics(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := newServiceTestDB(t)
	jobRepo := repository.NewJobRepository(database)
	folderRepo := repository.NewFolderRepository(database)
	workflowDefRepo := repository.NewWorkflowDefinitionRepository(database)
	workflowRunRepo := repository.NewWorkflowRunRepository(database)
	nodeRunRepo := repository.NewNodeRunRepository(database)
	nodeSnapshotRepo := repository.NewNodeSnapshotRepository(database)

	strict := &requiredInputProbeExecutor{nodeType: "required-strict", portName: "upstream", lazy: false}
	lazy := &requiredInputProbeExecutor{nodeType: "required-lazy", portName: "upstream", lazy: true}

	graph := repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "trigger", Type: "trigger", Enabled: true},
			{ID: "strict", Type: strict.Type(), Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"upstream": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "trigger", OutputPortIndex: 0}},
			}},
			{ID: "lazy", Type: lazy.Type(), Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"upstream": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "trigger", OutputPortIndex: 0}},
			}},
		},
		Edges: []repository.WorkflowGraphEdge{
			{Source: "trigger", SourcePortIndex: 0, Target: "strict", TargetPortIndex: 0},
			{Source: "trigger", SourcePortIndex: 0, Target: "lazy", TargetPortIndex: 0},
		},
	}
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		t.Fatalf("json.Marshal(graph) error = %v", err)
	}

	def := &repository.WorkflowDefinition{ID: "wf-lazy-skip", Name: "wf-lazy-skip", GraphJSON: string(graphJSON), IsActive: true, Version: 1}
	if err := workflowDefRepo.Create(ctx, def); err != nil {
		t.Fatalf("workflowDefRepo.Create() error = %v", err)
	}

	svc := NewWorkflowRunnerService(jobRepo, folderRepo, repository.NewSnapshotRepository(database), workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo, fs.NewMockAdapter(), nil, nil)
	svc.RegisterExecutor(strict)
	svc.RegisterExecutor(lazy)

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	job := waitJobDone(t, jobRepo, jobID)
	if job.Status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", job.Status)
	}

	run := waitWorkflowRunByJob(t, workflowRunRepo, jobID)
	nodeRuns, _, err := nodeRunRepo.List(ctx, repository.NodeRunListFilter{WorkflowRunID: run.ID, Page: 1, Limit: 20})
	if err != nil {
		t.Fatalf("nodeRunRepo.List() error = %v", err)
	}

	if got := nodeRunStatusByID(nodeRuns, "strict"); got != "skipped" {
		t.Fatalf("strict node status = %q, want skipped", got)
	}
	if got := nodeRunStatusByID(nodeRuns, "lazy"); got != "succeeded" {
		t.Fatalf("lazy node status = %q, want succeeded", got)
	}

	if atomic.LoadInt32(&strict.executed) != 0 {
		t.Fatalf("strict executed = %d, want 0", atomic.LoadInt32(&strict.executed))
	}
	if atomic.LoadInt32(&lazy.executed) != 1 {
		t.Fatalf("lazy executed = %d, want 1", atomic.LoadInt32(&lazy.executed))
	}
}

func TestWorkflowRunnerServiceResumeDataPersistence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := newServiceTestDB(t)
	jobRepo := repository.NewJobRepository(database)
	folderRepo := repository.NewFolderRepository(database)
	workflowDefRepo := repository.NewWorkflowDefinitionRepository(database)
	workflowRunRepo := repository.NewWorkflowRunRepository(database)
	nodeRunRepo := repository.NewNodeRunRepository(database)
	nodeSnapshotRepo := repository.NewNodeSnapshotRepository(database)

	resumeExecutor := &resumeDataMergeExecutor{}
	graph := repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{{ID: "resume", Type: resumeExecutor.Type(), Enabled: true}},
	}
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		t.Fatalf("json.Marshal(graph) error = %v", err)
	}

	def := &repository.WorkflowDefinition{ID: "wf-resume-persist", Name: "wf-resume-persist", GraphJSON: string(graphJSON), IsActive: true, Version: 1}
	if err := workflowDefRepo.Create(ctx, def); err != nil {
		t.Fatalf("workflowDefRepo.Create() error = %v", err)
	}

	svc := NewWorkflowRunnerService(jobRepo, folderRepo, repository.NewSnapshotRepository(database), workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo, fs.NewMockAdapter(), nil, nil)
	svc.RegisterExecutor(resumeExecutor)

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	run := waitWorkflowRunStatus(t, workflowRunRepo, jobID, "waiting_input")
	if run.ResumeNodeID != "resume" {
		t.Fatalf("resume node id = %q, want resume", run.ResumeNodeID)
	}

	if err := svc.ResumeWorkflowRunWithData(ctx, run.ID, map[string]any{"note": "first-pass"}); err != nil {
		t.Fatalf("ResumeWorkflowRunWithData(first) error = %v", err)
	}

	run = waitWorkflowRunIDStatus(t, workflowRunRepo, run.ID, "waiting_input")
	waitingNodeRun, err := nodeRunRepo.GetWaitingInputByWorkflowRunID(ctx, run.ID)
	if err != nil {
		t.Fatalf("nodeRunRepo.GetWaitingInputByWorkflowRunID() error = %v", err)
	}

	var persisted map[string]any
	if err := json.Unmarshal([]byte(waitingNodeRun.ResumeData), &persisted); err != nil {
		t.Fatalf("json.Unmarshal(resume_data) error = %v", err)
	}
	if persisted["note"] != "first-pass" {
		t.Fatalf("persisted note = %#v, want first-pass", persisted["note"])
	}

	if err := svc.ResumeWorkflowRunWithData(ctx, run.ID, map[string]any{"category": "video"}); err != nil {
		t.Fatalf("ResumeWorkflowRunWithData(second) error = %v", err)
	}

	run = waitWorkflowRunIDStatus(t, workflowRunRepo, run.ID, "succeeded")
	if run.Status != "succeeded" {
		t.Fatalf("workflow run status = %q, want succeeded", run.Status)
	}

	if resumeExecutor.lastResume["note"] != "first-pass" {
		t.Fatalf("resume note = %#v, want first-pass", resumeExecutor.lastResume["note"])
	}
	if resumeExecutor.lastResume["category"] != "video" {
		t.Fatalf("resume category = %#v, want video", resumeExecutor.lastResume["category"])
	}

	job := waitJobDone(t, jobRepo, jobID)
	if job.Status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", job.Status)
	}
}

func TestWorkflowRunnerServicePhase4MoveRollback(t *testing.T) {
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
	adapter.AddDir("/source", []fs.DirEntry{{Name: "album", IsDir: true}})
	adapter.AddDir("/source/album", []fs.DirEntry{{Name: "001.jpg", IsDir: false, Size: 10}})

	folder := &repository.Folder{ID: "folder-phase4-move-rb", Path: "/source/album", Name: "album", Category: "photo", CategorySource: "workflow", Status: "pending"}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	producer := &processingItemSourceExecutor{items: []ProcessingItem{{FolderID: folder.ID, SourcePath: folder.Path, FolderName: folder.Name, Category: folder.Category}}}
	graph := repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "source", Type: producer.Type(), Enabled: true},
			{ID: "move", Type: phase4MoveNodeExecutorType, Enabled: true, Config: map[string]any{"target_dir": "/target"}, Inputs: map[string]repository.NodeInputSpec{
				"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "source", SourcePort: "items"}},
			}},
		},
		Edges: []repository.WorkflowGraphEdge{{ID: "e1", Source: "source", SourcePort: "items", Target: "move", TargetPort: "items"}},
	}
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		t.Fatalf("json.Marshal(graph) error = %v", err)
	}
	def := &repository.WorkflowDefinition{ID: "wf-phase4-move-rb", Name: "wf-phase4-move-rb", GraphJSON: string(graphJSON), IsActive: true, Version: 1}
	if err := workflowDefRepo.Create(ctx, def); err != nil {
		t.Fatalf("workflowDefRepo.Create() error = %v", err)
	}

	svc := NewWorkflowRunnerService(jobRepo, folderRepo, repository.NewSnapshotRepository(database), workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo, adapter, nil, nil)
	svc.RegisterExecutor(producer)

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}
	job := waitJobDone(t, jobRepo, jobID)
	if job.Status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", job.Status)
	}

	moved, err := folderRepo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("folderRepo.GetByID() after move error = %v", err)
	}
	if moved.Path != "/target/album" {
		t.Fatalf("folder path after move = %q, want /target/album", moved.Path)
	}

	run := waitWorkflowRunByJob(t, workflowRunRepo, jobID)
	if err := svc.RollbackWorkflowRun(ctx, run.ID); err != nil {
		t.Fatalf("RollbackWorkflowRun() error = %v", err)
	}

	rolledBack, err := folderRepo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("folderRepo.GetByID() after rollback error = %v", err)
	}
	if rolledBack.Path != "/source/album" {
		t.Fatalf("folder path after rollback = %q, want /source/album", rolledBack.Path)
	}

	existsTarget, err := adapter.Exists(ctx, "/target/album")
	if err != nil {
		t.Fatalf("adapter.Exists(target) error = %v", err)
	}
	if existsTarget {
		t.Fatalf("target path should not exist after rollback")
	}
	existsSource, err := adapter.Exists(ctx, "/source/album")
	if err != nil {
		t.Fatalf("adapter.Exists(source) error = %v", err)
	}
	if !existsSource {
		t.Fatalf("source path should exist after rollback")
	}
}

// TestEngineV2_AC_PROC1_ProcessingChainRenameAndMove verifies the complete
// processing chain: category-router → rename-node → move-node runs end-to-end
// and the folder ends up at the correct renamed destination.
func TestEngineV2_AC_PROC1_ProcessingChainRenameAndMove(t *testing.T) {
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
	adapter.AddDir("/source", []fs.DirEntry{{Name: "Dune[2021]", IsDir: true}})
	adapter.AddDir("/source/Dune[2021]", []fs.DirEntry{{Name: "movie.mkv", IsDir: false, Size: 500}})

	folder := &repository.Folder{ID: "folder-proc1", Path: "/source/Dune[2021]", Name: "Dune[2021]", Category: "video", CategorySource: "workflow", Status: "pending"}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	producer := &processingItemSourceExecutor{items: []ProcessingItem{
		{FolderID: folder.ID, SourcePath: folder.Path, FolderName: folder.Name, TargetName: folder.Name, Category: "video"},
	}}

	graph := repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "source", Type: producer.Type(), Enabled: true},
			{ID: "router", Type: categoryRouterExecutorType, Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "source", SourcePort: "items"}},
			}},
			{ID: "rename", Type: renameNodeExecutorType, Enabled: true,
				Config: map[string]any{"strategy": "regex_extract", "regex": `^(?P<title>.+?)\[(?P<year>\d{4})\]$`, "template": "{title} ({year})"},
				Inputs: map[string]repository.NodeInputSpec{
					"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "router", SourcePort: "video"}},
				},
			},
			{ID: "move", Type: phase4MoveNodeExecutorType, Enabled: true,
				Config: map[string]any{"target_dir": "/target"},
				Inputs: map[string]repository.NodeInputSpec{
					"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "rename", SourcePort: "items"}},
				},
			},
		},
		Edges: []repository.WorkflowGraphEdge{
			{ID: "e1", Source: "source", SourcePort: "items", Target: "router", TargetPort: "items"},
			{ID: "e2", Source: "router", SourcePort: "video", Target: "rename", TargetPort: "items"},
			{ID: "e3", Source: "rename", SourcePort: "items", Target: "move", TargetPort: "items"},
		},
	}
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		t.Fatalf("json.Marshal(graph) error = %v", err)
	}
	def := &repository.WorkflowDefinition{ID: "wf-proc1", Name: "wf-proc1", GraphJSON: string(graphJSON), IsActive: true, Version: 1}
	if err := workflowDefRepo.Create(ctx, def); err != nil {
		t.Fatalf("workflowDefRepo.Create() error = %v", err)
	}

	svc := NewWorkflowRunnerService(jobRepo, folderRepo, repository.NewSnapshotRepository(database), workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo, adapter, nil, nil)
	svc.RegisterExecutor(producer)

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}
	job := waitJobDone(t, jobRepo, jobID)
	if job.Status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", job.Status)
	}

	// folder should now be at the renamed destination
	updatedFolder, err := folderRepo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("folderRepo.GetByID() error = %v", err)
	}
	if updatedFolder.Path != "/target/Dune (2021)" {
		t.Fatalf("folder path = %q, want /target/Dune (2021)", updatedFolder.Path)
	}

	dstExists, err := adapter.Exists(ctx, "/target/Dune (2021)")
	if err != nil {
		t.Fatalf("adapter.Exists(dst) error = %v", err)
	}
	if !dstExists {
		t.Fatalf("destination /target/Dune (2021) should exist after move")
	}
}

// TestEngineV2_AC_ROLL4_MultiNodeReverseRollback verifies that RollbackWorkflowRun
// reverses node executions in strict reverse-sequence order: two sequential move-node
// steps both get rolled back, with the later move reversed first.
func TestEngineV2_AC_ROLL4_MultiNodeReverseRollback(t *testing.T) {
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
	adapter.AddDir("/source", []fs.DirEntry{{Name: "album", IsDir: true}})
	adapter.AddDir("/source/album", []fs.DirEntry{{Name: "001.jpg", IsDir: false, Size: 10}})

	folder := &repository.Folder{ID: "folder-roll4", Path: "/source/album", Name: "album", Category: "photo", CategorySource: "workflow", Status: "pending"}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	// Graph: source → move1 (/source/album → /dst1/album) → rename → move2 (/dst1/album → /dst2/album)
	producer := &processingItemSourceExecutor{items: []ProcessingItem{
		{FolderID: folder.ID, SourcePath: folder.Path, FolderName: folder.Name, TargetName: folder.Name, Category: "photo"},
	}}

	graph := repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "source", Type: producer.Type(), Enabled: true},
			{ID: "move1", Type: phase4MoveNodeExecutorType, Enabled: true,
				Config: map[string]any{"target_dir": "/dst1"},
				Inputs: map[string]repository.NodeInputSpec{
					"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "source", SourcePort: "items"}},
				},
			},
			{ID: "rename", Type: renameNodeExecutorType, Enabled: true,
				Config: map[string]any{"strategy": "template", "template": "{name}"},
				Inputs: map[string]repository.NodeInputSpec{
					"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "move1", SourcePort: "items"}},
				},
			},
			{ID: "move2", Type: phase4MoveNodeExecutorType, Enabled: true,
				Config: map[string]any{"target_dir": "/dst2"},
				Inputs: map[string]repository.NodeInputSpec{
					"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "rename", SourcePort: "items"}},
				},
			},
		},
		Edges: []repository.WorkflowGraphEdge{
			{ID: "e1", Source: "source", SourcePort: "items", Target: "move1", TargetPort: "items"},
			{ID: "e2", Source: "move1", SourcePort: "items", Target: "rename", TargetPort: "items"},
			{ID: "e3", Source: "rename", SourcePort: "items", Target: "move2", TargetPort: "items"},
		},
	}
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		t.Fatalf("json.Marshal(graph) error = %v", err)
	}
	def := &repository.WorkflowDefinition{ID: "wf-roll4", Name: "wf-roll4", GraphJSON: string(graphJSON), IsActive: true, Version: 1}
	if err := workflowDefRepo.Create(ctx, def); err != nil {
		t.Fatalf("workflowDefRepo.Create() error = %v", err)
	}

	svc := NewWorkflowRunnerService(jobRepo, folderRepo, repository.NewSnapshotRepository(database), workflowDefRepo, workflowRunRepo, nodeRunRepo, nodeSnapshotRepo, adapter, nil, nil)
	svc.RegisterExecutor(producer)

	jobID, err := svc.StartJob(ctx, StartWorkflowJobInput{WorkflowDefID: def.ID})
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}
	job := waitJobDone(t, jobRepo, jobID)
	if job.Status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", job.Status)
	}

	// after both moves: folder should be at /dst2/album
	movedFolder, err := folderRepo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("folderRepo.GetByID() after moves error = %v", err)
	}
	if movedFolder.Path != "/dst2/album" {
		t.Fatalf("folder path after moves = %q, want /dst2/album", movedFolder.Path)
	}

	run := waitWorkflowRunByJob(t, workflowRunRepo, jobID)
	if err := svc.RollbackWorkflowRun(ctx, run.ID); err != nil {
		t.Fatalf("RollbackWorkflowRun() error = %v", err)
	}

	// after rollback: folder should be back at /source/album
	rolledBack, err := folderRepo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("folderRepo.GetByID() after rollback error = %v", err)
	}
	if rolledBack.Path != "/source/album" {
		t.Fatalf("folder path after rollback = %q, want /source/album", rolledBack.Path)
	}

	// intermediate path /dst1/album should also be gone (moved back through by reverse rollback)
	dst1Exists, err := adapter.Exists(ctx, "/dst1/album")
	if err != nil {
		t.Fatalf("adapter.Exists(/dst1/album) error = %v", err)
	}
	if dst1Exists {
		t.Fatalf("/dst1/album should not exist after complete rollback")
	}

	dst2Exists, err := adapter.Exists(ctx, "/dst2/album")
	if err != nil {
		t.Fatalf("adapter.Exists(/dst2/album) error = %v", err)
	}
	if dst2Exists {
		t.Fatalf("/dst2/album should not exist after rollback")
	}

	srcExists, err := adapter.Exists(ctx, "/source/album")
	if err != nil {
		t.Fatalf("adapter.Exists(/source/album) error = %v", err)
	}
	if !srcExists {
		t.Fatalf("/source/album should exist after rollback")
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
