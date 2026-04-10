package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/liqiye/classifier/internal/repository"
)

const auditLogNodeType = "audit-log"

func NormalizeWorkflowDefinitionGraphs(ctx context.Context, repo repository.WorkflowDefinitionRepository) error {
	if repo == nil {
		return nil
	}

	page := 1
	for {
		items, total, err := repo.List(ctx, repository.WorkflowDefListFilter{Page: page, Limit: 100})
		if err != nil {
			return fmt.Errorf("normalizeWorkflowDefinitionGraphs list page %d: %w", page, err)
		}
		if len(items) == 0 {
			return nil
		}

		for _, item := range items {
			if item == nil || strings.TrimSpace(item.GraphJSON) == "" {
				continue
			}
			normalized, changed, err := normalizeWorkflowGraphJSON(item.GraphJSON)
			if err != nil {
				return fmt.Errorf("normalizeWorkflowDefinitionGraphs normalize %q: %w", item.ID, err)
			}
			if !changed {
				continue
			}
			item.GraphJSON = normalized
			if err := repo.Update(ctx, item); err != nil {
				return fmt.Errorf("normalizeWorkflowDefinitionGraphs update %q: %w", item.ID, err)
			}
		}

		if page*100 >= total {
			return nil
		}
		page++
	}
}

func normalizeWorkflowGraphJSON(raw string) (string, bool, error) {
	graph := repository.WorkflowGraph{}
	if err := json.Unmarshal([]byte(raw), &graph); err != nil {
		return "", false, fmt.Errorf("unmarshal graph json: %w", err)
	}

	removedNodeIDs := map[string]struct{}{}
	changed := false

	filteredNodes := make([]repository.WorkflowGraphNode, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if strings.TrimSpace(node.Type) == auditLogNodeType {
			removedNodeIDs[node.ID] = struct{}{}
			changed = true
			continue
		}
		if migrateNodePathConfig(&node) {
			changed = true
		}

		if len(node.Inputs) > 0 {
			cleanInputs := make(map[string]repository.NodeInputSpec, len(node.Inputs))
			for key, spec := range node.Inputs {
				if isStepResultsPort(key) {
					changed = true
					continue
				}
				if spec.LinkSource != nil {
					if _, removed := removedNodeIDs[spec.LinkSource.SourceNodeID]; removed {
						changed = true
						continue
					}
					if isStepResultsPort(spec.LinkSource.SourcePort) {
						changed = true
						continue
					}
				}
				cleanInputs[key] = spec
			}
			node.Inputs = cleanInputs
		}

		filteredNodes = append(filteredNodes, node)
	}
	graph.Nodes = filteredNodes

	filteredEdges := make([]repository.WorkflowGraphEdge, 0, len(graph.Edges))
	for _, edge := range graph.Edges {
		if _, removed := removedNodeIDs[edge.Source]; removed {
			changed = true
			continue
		}
		if _, removed := removedNodeIDs[edge.Target]; removed {
			changed = true
			continue
		}
		if isStepResultsPort(edge.SourcePort) || isStepResultsPort(edge.TargetPort) {
			changed = true
			continue
		}
		filteredEdges = append(filteredEdges, edge)
	}
	graph.Edges = filteredEdges

	remainingNodeIDs := make(map[string]struct{}, len(graph.Nodes))
	for _, node := range graph.Nodes {
		remainingNodeIDs[node.ID] = struct{}{}
	}
	for index := range graph.Nodes {
		node := &graph.Nodes[index]
		if len(node.Inputs) == 0 {
			continue
		}
		cleanInputs := make(map[string]repository.NodeInputSpec, len(node.Inputs))
		for key, spec := range node.Inputs {
			if spec.LinkSource == nil {
				cleanInputs[key] = spec
				continue
			}
			if _, exists := remainingNodeIDs[spec.LinkSource.SourceNodeID]; !exists {
				changed = true
				continue
			}
			if isStepResultsPort(spec.LinkSource.SourcePort) {
				changed = true
				continue
			}
			cleanInputs[key] = spec
		}
		node.Inputs = cleanInputs
	}

	if !changed {
		return raw, false, nil
	}

	data, err := json.Marshal(graph)
	if err != nil {
		return "", false, fmt.Errorf("marshal normalized graph json: %w", err)
	}
	return string(data), true, nil
}

func isStepResultsPort(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), "step_results")
}

func migrateNodePathConfig(node *repository.WorkflowGraphNode) bool {
	if node == nil || node.Config == nil {
		return false
	}

	changed := false
	legacyPath := ""
	if value := normalizeWorkflowPath(stringConfig(node.Config, "target_dir")); value != "" {
		legacyPath = value
	}
	if legacyPath == "" {
		if value := normalizeWorkflowPath(stringConfig(node.Config, "output_dir")); value != "" {
			legacyPath = value
		}
	}

	if strings.TrimSpace(stringConfig(node.Config, "path_ref_type")) == "" {
		switch strings.TrimSpace(node.Type) {
		case "move-node":
			node.Config["path_ref_type"] = workflowPathRefTypeOutput
			node.Config["path_ref_key"] = "mixed"
			changed = true
		case "compress-node":
			node.Config["path_ref_type"] = workflowPathRefTypeOutput
			node.Config["path_ref_key"] = "mixed"
			changed = true
		case "thumbnail-node":
			node.Config["path_ref_type"] = workflowPathRefTypeOutput
			node.Config["path_ref_key"] = "video"
			changed = true
		}
	}

	if legacyPath != "" {
		node.Config["path_ref_type"] = workflowPathRefTypeCustom
		node.Config["path_ref_key"] = legacyPath
		changed = true
	}

	legacyKeys := []string{
		"target_dir",
		"targetDir",
		"output_dir",
		"target_dir_source",
		"output_dir_source",
		"target_dir_option_id",
		"output_dir_option_id",
	}
	for _, key := range legacyKeys {
		if _, ok := node.Config[key]; ok {
			delete(node.Config, key)
			changed = true
		}
	}

	return changed
}
