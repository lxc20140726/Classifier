package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const folderSplitterExecutorType = "folder-splitter"

type folderSplitterNodeExecutor struct{}

func newFolderSplitterExecutor() *folderSplitterNodeExecutor {
	return &folderSplitterNodeExecutor{}
}

func NewFolderSplitterExecutor() WorkflowNodeExecutor {
	return newFolderSplitterExecutor()
}

func (e *folderSplitterNodeExecutor) Type() string {
	return folderSplitterExecutorType
}

func (e *folderSplitterNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "Folder Splitter",
		Description: "Split mixed classified entry into processing items",
		InputPorts: []NodeSchemaPort{
			{Name: "entry", Description: "CLASSIFIED_ENTRY", Required: true},
		},
		OutputPorts: []NodeSchemaPort{
			{Name: "items", Description: "PROCESSING_ITEM[]", Required: true},
		},
	}
}

func (e *folderSplitterNodeExecutor) Execute(_ context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	rawInputs := typedInputsToAny(input.Inputs)
	entry, ok := classificationReaderResolveInputEntry(rawInputs)
	if !ok {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: entry is required", e.Type())
	}

	splitMixed := folderSplitterBoolConfig(input.Node.Config, "split_mixed", true)
	splitDepth := intConfig(input.Node.Config, "split_depth", 1)

	items := []ProcessingItem{folderSplitterBuildSelfItem(entry)}
	if strings.EqualFold(entry.Category, "mixed") && splitMixed && splitDepth > 0 {
		splitItems := folderSplitterBuildFirstLevelItems(entry)
		if len(splitItems) > 0 {
			items = splitItems
		}
	}

	return NodeExecutionOutput{Outputs: map[string]TypedValue{"items": {Type: PortTypeProcessingItemList, Value: items}}, Status: ExecutionSuccess}, nil
}

func (e *folderSplitterNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *folderSplitterNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func folderSplitterBoolConfig(config map[string]any, key string, fallback bool) bool {
	if config == nil {
		return fallback
	}

	raw, ok := config[key]
	if !ok {
		return fallback
	}

	switch value := raw.(type) {
	case bool:
		return value
	case string:
		trimmed := strings.TrimSpace(strings.ToLower(value))
		if trimmed == "true" {
			return true
		}
		if trimmed == "false" {
			return false
		}
	}

	return fallback
}

func folderSplitterBuildSelfItem(entry ClassifiedEntry) ProcessingItem {
	return ProcessingItem{
		SourcePath: entry.Path,
		FolderID:   entry.FolderID,
		FolderName: entry.Name,
		TargetName: entry.Name,
		Category:   entry.Category,
		Files:      append([]FileEntry(nil), entry.Files...),
		ParentPath: filepath.Dir(entry.Path),
	}
}

func folderSplitterBuildFirstLevelItems(entry ClassifiedEntry) []ProcessingItem {
	if len(entry.Subtree) == 0 {
		return nil
	}

	out := make([]ProcessingItem, 0, len(entry.Subtree))
	for _, child := range entry.Subtree {
		if !folderSplitterIsFirstLevelChild(entry, child) {
			continue
		}

		category := child.Category
		if category == "" {
			category = entry.Category
		}
		folderID := child.FolderID
		if folderID == "" {
			folderID = entry.FolderID
		}

		out = append(out, ProcessingItem{
			SourcePath: child.Path,
			FolderID:   folderID,
			FolderName: child.Name,
			TargetName: child.Name,
			Category:   category,
			Files:      append([]FileEntry(nil), child.Files...),
			ParentPath: entry.Path,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].SourcePath == out[j].SourcePath {
			return out[i].FolderName < out[j].FolderName
		}
		return out[i].SourcePath < out[j].SourcePath
	})

	return out
}

func folderSplitterIsFirstLevelChild(entry ClassifiedEntry, child ClassifiedEntry) bool {
	if entry.Path != "" && child.Path != "" {
		rel, err := filepath.Rel(entry.Path, child.Path)
		if err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return !strings.ContainsRune(rel, os.PathSeparator)
		}
	}

	return false
}
