package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/liqiye/classifier/internal/repository"
)

func TestNormalizeWorkflowDefinitionGraphs_RemovesAuditAndStepResults(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := newServiceTestDB(t)
	repo := repository.NewWorkflowDefinitionRepository(database)

	graph := repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "n-rename", Type: "rename-node", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "n-reader", SourcePort: "items"}},
			}},
			{ID: "n-collect", Type: "collect-node", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"items_1":        {LinkSource: &repository.NodeLinkSource{SourceNodeID: "n-rename", SourcePort: "items"}},
				"step_results_1": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "n-rename", SourcePort: "step_results"}},
			}},
			{ID: "n-move", Type: "move-node", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"items":        {LinkSource: &repository.NodeLinkSource{SourceNodeID: "n-collect", SourcePort: "items"}},
				"step_results": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "n-collect", SourcePort: "step_results"}},
			}},
			{ID: "n-audit", Type: "audit-log", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "n-move", SourcePort: "items"}},
			}},
		},
		Edges: []repository.WorkflowGraphEdge{
			{ID: "e1", Source: "n-rename", SourcePort: "items", Target: "n-collect", TargetPort: "items_1"},
			{ID: "e2", Source: "n-rename", SourcePort: "step_results", Target: "n-collect", TargetPort: "step_results_1"},
			{ID: "e3", Source: "n-collect", SourcePort: "items", Target: "n-move", TargetPort: "items"},
			{ID: "e4", Source: "n-collect", SourcePort: "step_results", Target: "n-move", TargetPort: "step_results"},
			{ID: "e5", Source: "n-move", SourcePort: "items", Target: "n-audit", TargetPort: "items"},
		},
	}
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		t.Fatalf("json.Marshal(graph) error = %v", err)
	}

	def := &repository.WorkflowDefinition{
		ID:        "wf-graph-migration",
		Name:      "wf-graph-migration",
		GraphJSON: string(graphJSON),
		IsActive:  true,
		Version:   1,
	}
	if err := repo.Create(ctx, def); err != nil {
		t.Fatalf("repo.Create() error = %v", err)
	}

	if err := NormalizeWorkflowDefinitionGraphs(ctx, repo); err != nil {
		t.Fatalf("NormalizeWorkflowDefinitionGraphs() error = %v", err)
	}

	updated, err := repo.GetByID(ctx, def.ID)
	if err != nil {
		t.Fatalf("repo.GetByID() error = %v", err)
	}

	var normalized repository.WorkflowGraph
	if err := json.Unmarshal([]byte(updated.GraphJSON), &normalized); err != nil {
		t.Fatalf("json.Unmarshal(normalized) error = %v", err)
	}

	for _, node := range normalized.Nodes {
		if node.Type == "audit-log" {
			t.Fatalf("normalized graph still has audit-log node")
		}
		for inputName, spec := range node.Inputs {
			if strings.HasPrefix(inputName, "step_results") {
				t.Fatalf("input %q should be removed", inputName)
			}
			if spec.LinkSource != nil && strings.HasPrefix(spec.LinkSource.SourcePort, "step_results") {
				t.Fatalf("link source port %q should be removed", spec.LinkSource.SourcePort)
			}
		}
	}
	for _, edge := range normalized.Edges {
		if edge.Source == "n-audit" || edge.Target == "n-audit" {
			t.Fatalf("edge connected to removed audit node still exists: %+v", edge)
		}
		if strings.HasPrefix(edge.SourcePort, "step_results") || strings.HasPrefix(edge.TargetPort, "step_results") {
			t.Fatalf("step_results edge should be removed: %+v", edge)
		}
	}
}

func TestNormalizeWorkflowDefinitionGraphs_IsIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := newServiceTestDB(t)
	repo := repository.NewWorkflowDefinitionRepository(database)

	graph := repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{ID: "n1", Type: "collect-node", Enabled: true, Inputs: map[string]repository.NodeInputSpec{
				"items_1": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "n0", SourcePort: "items"}},
			}},
		},
		Edges: []repository.WorkflowGraphEdge{
			{ID: "e1", Source: "n0", SourcePort: "items", Target: "n1", TargetPort: "items_1"},
		},
	}
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		t.Fatalf("json.Marshal(graph) error = %v", err)
	}
	def := &repository.WorkflowDefinition{ID: "wf-graph-idempotent", Name: "wf-graph-idempotent", GraphJSON: string(graphJSON), IsActive: true, Version: 1}
	if err := repo.Create(ctx, def); err != nil {
		t.Fatalf("repo.Create() error = %v", err)
	}

	if err := NormalizeWorkflowDefinitionGraphs(ctx, repo); err != nil {
		t.Fatalf("first NormalizeWorkflowDefinitionGraphs() error = %v", err)
	}
	first, err := repo.GetByID(ctx, def.ID)
	if err != nil {
		t.Fatalf("repo.GetByID(first) error = %v", err)
	}
	firstJSON := first.GraphJSON

	if err := NormalizeWorkflowDefinitionGraphs(ctx, repo); err != nil {
		t.Fatalf("second NormalizeWorkflowDefinitionGraphs() error = %v", err)
	}
	second, err := repo.GetByID(ctx, def.ID)
	if err != nil {
		t.Fatalf("repo.GetByID(second) error = %v", err)
	}

	if second.GraphJSON != firstJSON {
		t.Fatalf("graph json changed after second normalization")
	}
}
