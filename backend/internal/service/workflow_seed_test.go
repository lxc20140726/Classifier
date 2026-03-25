package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/liqiye/classifier/internal/repository"
)

func TestSeedDefaultWorkflow_CreatesExpectedGraphAndIsIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := newServiceTestDB(t)
	repo := repository.NewWorkflowDefinitionRepository(database)

	if err := SeedDefaultWorkflow(ctx, repo); err != nil {
		t.Fatalf("SeedDefaultWorkflow() error = %v", err)
	}

	items, total, err := repo.List(ctx, repository.WorkflowDefListFilter{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("repo.List() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("workflow count = total:%d len:%d, want 1", total, len(items))
	}

	seeded := items[0]
	if seeded.Name != "默认分类流程" {
		t.Fatalf("seeded.Name = %q, want 默认分类流程", seeded.Name)
	}
	if !seeded.IsActive {
		t.Fatalf("seeded.IsActive = false, want true")
	}
	if seeded.Version != 1 {
		t.Fatalf("seeded.Version = %d, want 1", seeded.Version)
	}

	var graph repository.WorkflowGraph
	if err := json.Unmarshal([]byte(seeded.GraphJSON), &graph); err != nil {
		t.Fatalf("json.Unmarshal(GraphJSON) error = %v", err)
	}

	gotTypes := make([]string, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		gotTypes = append(gotTypes, node.Type)
	}
	wantTypes := []string{
		"trigger",
		"folder-tree-scanner",
		"name-keyword-classifier",
		"file-tree-classifier",
		"ext-ratio-classifier",
		"confidence-check",
		"manual-classifier",
		"subtree-aggregator",
	}
	if len(gotTypes) != len(wantTypes) {
		t.Fatalf("node count = %d, want %d; got %v", len(gotTypes), len(wantTypes), gotTypes)
	}
	for i, want := range wantTypes {
		if gotTypes[i] != want {
			t.Fatalf("node[%d].Type = %q, want %q; full=%v", i, gotTypes[i], want, gotTypes)
		}
	}

	if err := SeedDefaultWorkflow(ctx, repo); err != nil {
		t.Fatalf("second SeedDefaultWorkflow() error = %v", err)
	}
	items, total, err = repo.List(ctx, repository.WorkflowDefListFilter{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("repo.List() after second seed error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("after second seed workflow count = total:%d len:%d, want 1", total, len(items))
	}
	if items[0].ID != seeded.ID {
		t.Fatalf("seeded workflow id changed from %q to %q on second seed", seeded.ID, items[0].ID)
	}
}

func TestSeedDefaultProcessingWorkflow_CreatesExpectedGraphAndIsIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := newServiceTestDB(t)
	repo := repository.NewWorkflowDefinitionRepository(database)

	if err := SeedDefaultProcessingWorkflow(ctx, repo); err != nil {
		t.Fatalf("SeedDefaultProcessingWorkflow() error = %v", err)
	}

	items, total, err := repo.List(ctx, repository.WorkflowDefListFilter{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("repo.List() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("workflow count = total:%d len:%d, want 1", total, len(items))
	}

	seeded := items[0]
	if seeded.Name != "default-processing" {
		t.Fatalf("seeded.Name = %q, want default-processing", seeded.Name)
	}
	if !seeded.IsActive {
		t.Fatalf("seeded.IsActive = false, want true")
	}
	if seeded.Version != 1 {
		t.Fatalf("seeded.Version = %d, want 1", seeded.Version)
	}

	var graph repository.WorkflowGraph
	if err := json.Unmarshal([]byte(seeded.GraphJSON), &graph); err != nil {
		t.Fatalf("json.Unmarshal(GraphJSON) error = %v", err)
	}

	gotTypes := make([]string, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		gotTypes = append(gotTypes, node.Type)
	}
	wantTypes := []string{
		"classification-reader",
		"folder-splitter",
		"category-router",
		"thumbnail-node",
		"rename-node",
		"compress-node",
		"move-node",
		"audit-log",
	}
	if len(gotTypes) != len(wantTypes) {
		t.Fatalf("node count = %d, want %d; got %v", len(gotTypes), len(wantTypes), gotTypes)
	}
	for i, want := range wantTypes {
		if gotTypes[i] != want {
			t.Fatalf("node[%d].Type = %q, want %q; full=%v", i, gotTypes[i], want, gotTypes)
		}
	}

	if len(graph.Edges) == 0 {
		t.Fatalf("edges count = 0, want > 0")
	}

	if err := SeedDefaultProcessingWorkflow(ctx, repo); err != nil {
		t.Fatalf("second SeedDefaultProcessingWorkflow() error = %v", err)
	}
	items, total, err = repo.List(ctx, repository.WorkflowDefListFilter{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("repo.List() after second seed error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("after second seed workflow count = total:%d len:%d, want 1", total, len(items))
	}
	if items[0].ID != seeded.ID {
		t.Fatalf("seeded workflow id changed from %q to %q on second seed", seeded.ID, items[0].ID)
	}
}

func TestSeedDefaultProcessingWorkflow_DoesNotDuplicateWhenDefaultWorkflowExists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := newServiceTestDB(t)
	repo := repository.NewWorkflowDefinitionRepository(database)

	if err := SeedDefaultWorkflow(ctx, repo); err != nil {
		t.Fatalf("SeedDefaultWorkflow() error = %v", err)
	}
	if err := SeedDefaultProcessingWorkflow(ctx, repo); err != nil {
		t.Fatalf("SeedDefaultProcessingWorkflow() error = %v", err)
	}
	if err := SeedDefaultProcessingWorkflow(ctx, repo); err != nil {
		t.Fatalf("second SeedDefaultProcessingWorkflow() error = %v", err)
	}

	items, total, err := repo.List(ctx, repository.WorkflowDefListFilter{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("repo.List() error = %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("workflow count = total:%d len:%d, want 2", total, len(items))
	}
}
