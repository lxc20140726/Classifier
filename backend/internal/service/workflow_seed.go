package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/liqiye/classifier/internal/repository"
)

func SeedDefaultWorkflow(ctx context.Context, repo repository.WorkflowDefinitionRepository) error {
	items, _, err := repo.List(ctx, repository.WorkflowDefListFilter{Limit: 1})
	if err != nil {
		return fmt.Errorf("seedDefaultWorkflow list definitions: %w", err)
	}
	if len(items) > 0 {
		return nil
	}

	graph := repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{
				ID:      "n-trigger",
				Type:    "trigger",
				Config:  map[string]any{},
				Inputs:  map[string]repository.NodeInputSpec{},
				Enabled: true,
			},
			{
				ID:   "n-scanner",
				Type: "folder-tree-scanner",
				Config: map[string]any{
					"source_dir": "",
				},
				Inputs: map[string]repository.NodeInputSpec{
					"source_dir": {
						LinkSource: &repository.NodeLinkSource{SourceNodeID: "n-trigger", OutputPortIndex: 0},
					},
				},
				Enabled: true,
			},
			{
				ID:     "n-kw",
				Type:   "name-keyword-classifier",
				Config: map[string]any{},
				Inputs: map[string]repository.NodeInputSpec{
					"folder": {
						LinkSource: &repository.NodeLinkSource{SourceNodeID: "n-scanner", OutputPortIndex: 0},
					},
				},
				Enabled: true,
			},
			{
				ID:     "n-ft",
				Type:   "file-tree-classifier",
				Config: map[string]any{},
				Inputs: map[string]repository.NodeInputSpec{
					"folder": {
						LinkSource: &repository.NodeLinkSource{SourceNodeID: "n-kw", OutputPortIndex: 1},
					},
				},
				Enabled: true,
			},
			{
				ID:     "n-ext",
				Type:   "ext-ratio-classifier",
				Config: map[string]any{},
				Inputs: map[string]repository.NodeInputSpec{
					"folder": {
						LinkSource: &repository.NodeLinkSource{SourceNodeID: "n-ft", OutputPortIndex: 1},
					},
				},
				Enabled: true,
			},
			{
				ID:   "n-cc",
				Type: "confidence-check",
				Config: map[string]any{
					"threshold": 0.75,
				},
				Inputs: map[string]repository.NodeInputSpec{
					"signal": {
						LinkSource: &repository.NodeLinkSource{SourceNodeID: "n-ext", OutputPortIndex: 0},
					},
				},
				Enabled: true,
			},
			{
				ID:     "n-manual",
				Type:   "manual-classifier",
				Config: map[string]any{},
				Inputs: map[string]repository.NodeInputSpec{
					"signal": {
						LinkSource: &repository.NodeLinkSource{SourceNodeID: "n-cc", OutputPortIndex: 1},
					},
				},
				Enabled: true,
			},
			{
				ID:     "n-agg",
				Type:   "subtree-aggregator",
				Config: map[string]any{},
				Inputs: map[string]repository.NodeInputSpec{
					"signal_kw": {
						LinkSource: &repository.NodeLinkSource{SourceNodeID: "n-kw", OutputPortIndex: 0},
					},
					"signal_ft": {
						LinkSource: &repository.NodeLinkSource{SourceNodeID: "n-ft", OutputPortIndex: 0},
					},
					"signal_ext": {
						LinkSource: &repository.NodeLinkSource{SourceNodeID: "n-cc", OutputPortIndex: 0},
					},
					"signal_manual": {
						LinkSource: &repository.NodeLinkSource{SourceNodeID: "n-manual", OutputPortIndex: 0},
					},
				},
				Enabled: true,
			},
		},
	}

	graphBytes, err := json.Marshal(graph)
	if err != nil {
		return fmt.Errorf("seedDefaultWorkflow marshal graph: %w", err)
	}

	if err := repo.Create(ctx, &repository.WorkflowDefinition{
		ID:        uuid.NewString(),
		Name:      "默认分类流程",
		GraphJSON: string(graphBytes),
		IsActive:  true,
		Version:   1,
	}); err != nil {
		return fmt.Errorf("seedDefaultWorkflow create workflow: %w", err)
	}

	return nil
}

func SeedDefaultProcessingWorkflow(ctx context.Context, repo repository.WorkflowDefinitionRepository) error {
	exists, err := workflowDefinitionExistsByName(ctx, repo, "default-processing")
	if err != nil {
		return fmt.Errorf("seedDefaultProcessingWorkflow check existing: %w", err)
	}
	if exists {
		return nil
	}

	graph := repository.WorkflowGraph{
		Nodes: []repository.WorkflowGraphNode{
			{
				ID:      "p-reader",
				Type:    "classification-reader",
				Config:  map[string]any{},
				Inputs:  map[string]repository.NodeInputSpec{},
				Enabled: true,
			},
			{
				ID:   "p-split",
				Type: "folder-splitter",
				Config: map[string]any{
					"split_mixed": true,
					"split_depth": 1,
				},
				Inputs: map[string]repository.NodeInputSpec{
					"entry": {
						LinkSource: &repository.NodeLinkSource{SourceNodeID: "p-reader", OutputPortIndex: 0},
					},
				},
				Enabled: true,
			},
			{
				ID:      "p-router",
				Type:    "category-router",
				Config:  map[string]any{},
				Inputs:  map[string]repository.NodeInputSpec{"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "p-split", OutputPortIndex: 0}}},
				Enabled: true,
			},
			{
				ID:   "p-thumbnail",
				Type: "thumbnail-node",
				Config: map[string]any{
					"output_dir": ".thumbnails",
				},
				Inputs:  map[string]repository.NodeInputSpec{"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "p-router", OutputPortIndex: 0}}},
				Enabled: true,
			},
			{
				ID:   "p-rename",
				Type: "rename-node",
				Config: map[string]any{
					"strategy": "template",
					"template": "{name}",
				},
				Inputs:  map[string]repository.NodeInputSpec{"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "p-thumbnail", OutputPortIndex: 0}}},
				Enabled: true,
			},
			{
				ID:   "p-compress",
				Type: "compress-node",
				Config: map[string]any{
					"scope":      "folder",
					"format":     "cbz",
					"target_dir": ".archives",
				},
				Inputs:  map[string]repository.NodeInputSpec{"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "p-rename", OutputPortIndex: 0}}},
				Enabled: true,
			},
			{
				ID:   "p-move",
				Type: "move-node",
				Config: map[string]any{
					"target_dir":      ".processed",
					"move_unit":       "folder",
					"conflict_policy": "auto_rename",
				},
				Inputs:  map[string]repository.NodeInputSpec{"items": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "p-compress", OutputPortIndex: 0}}},
				Enabled: true,
			},
			{
				ID:   "p-audit",
				Type: "audit-log",
				Config: map[string]any{
					"action": "phase4.processing",
					"level":  "info",
				},
				Inputs: map[string]repository.NodeInputSpec{
					"item":   {LinkSource: &repository.NodeLinkSource{SourceNodeID: "p-move", OutputPortIndex: 0}},
					"result": {LinkSource: &repository.NodeLinkSource{SourceNodeID: "p-move", OutputPortIndex: 1}},
				},
				Enabled: true,
			},
		},
		Edges: []repository.WorkflowGraphEdge{
			{ID: "e-reader-split", Source: "p-reader", SourcePort: 0, Target: "p-split", TargetPort: 0},
			{ID: "e-split-router", Source: "p-split", SourcePort: 0, Target: "p-router", TargetPort: 0},
			{ID: "e-router-thumbnail", Source: "p-router", SourcePort: 0, Target: "p-thumbnail", TargetPort: 0},
			{ID: "e-thumbnail-rename", Source: "p-thumbnail", SourcePort: 0, Target: "p-rename", TargetPort: 0},
			{ID: "e-rename-compress", Source: "p-rename", SourcePort: 0, Target: "p-compress", TargetPort: 0},
			{ID: "e-compress-move", Source: "p-compress", SourcePort: 0, Target: "p-move", TargetPort: 0},
			{ID: "e-move-audit-item", Source: "p-move", SourcePort: 0, Target: "p-audit", TargetPort: 0},
			{ID: "e-move-audit-result", Source: "p-move", SourcePort: 1, Target: "p-audit", TargetPort: 1},
		},
	}

	graphBytes, err := json.Marshal(graph)
	if err != nil {
		return fmt.Errorf("seedDefaultProcessingWorkflow marshal graph: %w", err)
	}

	if err := repo.Create(ctx, &repository.WorkflowDefinition{
		ID:          uuid.NewString(),
		Name:        "default-processing",
		Description: "Phase 4 processing workflow",
		GraphJSON:   string(graphBytes),
		IsActive:    true,
		Version:     1,
	}); err != nil {
		return fmt.Errorf("seedDefaultProcessingWorkflow create workflow: %w", err)
	}

	return nil
}

func workflowDefinitionExistsByName(ctx context.Context, repo repository.WorkflowDefinitionRepository, name string) (bool, error) {
	page := 1
	for {
		items, total, err := repo.List(ctx, repository.WorkflowDefListFilter{Page: page, Limit: 100})
		if err != nil {
			return false, err
		}
		for _, item := range items {
			if strings.EqualFold(strings.TrimSpace(item.Name), strings.TrimSpace(name)) {
				return true, nil
			}
		}

		if len(items) == 0 || page*100 >= total {
			return false, nil
		}
		page++
	}
}
