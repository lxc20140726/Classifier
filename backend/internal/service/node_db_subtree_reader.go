package service

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/liqiye/classifier/internal/repository"
)

const dbSubtreeReaderExecutorType = "db-subtree-reader"

type dbSubtreeReaderNodeExecutor struct {
	folders repository.FolderRepository
}

func newDBSubtreeReaderExecutor(folderRepo repository.FolderRepository) *dbSubtreeReaderNodeExecutor {
	return &dbSubtreeReaderNodeExecutor{folders: folderRepo}
}

func (e *dbSubtreeReaderNodeExecutor) Type() string {
	return dbSubtreeReaderExecutorType
}

func (e *dbSubtreeReaderNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "子树读取器",
		Description: "从数据库读取指定目录及其子目录的分类结果，重建为分类树供处理流使用",
		Inputs: []PortDef{
			{Name: "path", Type: PortTypePath, Description: "根目录路径", Required: false},
			{Name: "entry", Type: PortTypeJSON, Description: "可选输入，读取其 path 字段作为根路径", Required: false},
		},
		Outputs: []PortDef{
			{Name: "entry", Type: PortTypeJSON, Description: "包含子树分类信息的根条目"},
		},
	}
}

func (e *dbSubtreeReaderNodeExecutor) Execute(ctx context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	if e.folders == nil {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: folder repository is required", e.Type())
	}

	rawInputs := typedInputsToAny(input.Inputs)
	rootPath := strings.TrimSpace(anyString(rawInputs["path"]))
	if rootPath == "" {
		rootPath = dbSubtreeRootPathFromEntry(rawInputs["entry"])
	}
	if rootPath == "" {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: path or entry is required", e.Type())
	}

	items, err := e.folders.ListByPathPrefix(ctx, rootPath)
	if err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute list folders by prefix %q: %w", e.Type(), rootPath, err)
	}
	if len(items) == 0 {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: folder not found for path %q", e.Type(), rootPath)
	}

	rootEntry, ok := dbSubtreeBuildEntry(items, rootPath)
	if !ok {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: failed to build subtree for path %q", e.Type(), rootPath)
	}

	return NodeExecutionOutput{
		Outputs: map[string]TypedValue{
			"entry": {Type: PortTypeJSON, Value: rootEntry},
		},
		Status: ExecutionSuccess,
	}, nil
}

func (e *dbSubtreeReaderNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *dbSubtreeReaderNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func dbSubtreeRootPathFromEntry(raw any) string {
	if raw == nil {
		return ""
	}
	if entry, ok := classificationReaderToEntry(raw); ok {
		return strings.TrimSpace(entry.Path)
	}

	return ""
}

func dbSubtreeBuildEntry(folders []*repository.Folder, rootPath string) (ClassifiedEntry, bool) {
	trimmedRoot := strings.TrimSpace(rootPath)
	if trimmedRoot == "" {
		return ClassifiedEntry{}, false
	}

	sort.Slice(folders, func(i, j int) bool {
		return len(folders[i].Path) < len(folders[j].Path)
	})

	pathToEntry := make(map[string]*ClassifiedEntry, len(folders))
	for _, folder := range folders {
		if folder == nil {
			continue
		}
		path := strings.TrimSpace(folder.Path)
		if path == "" {
			continue
		}
		pathToEntry[path] = &ClassifiedEntry{
			FolderID:      folder.ID,
			Path:          path,
			Name:          strings.TrimSpace(folder.Name),
			Category:      strings.TrimSpace(folder.Category),
			Confidence:    1,
			Reason:        "db:folder",
			Classifier:    dbSubtreeReaderExecutorType,
			HasOtherFiles: folder.HasOtherFiles,
			Files:         []FileEntry{},
			Subtree:       []ClassifiedEntry{},
		}
		if pathToEntry[path].Name == "" {
			pathToEntry[path].Name = filepath.Base(path)
		}
		if pathToEntry[path].Category == "" {
			pathToEntry[path].Category = "other"
		}
	}

	rootEntry, ok := pathToEntry[trimmedRoot]
	if !ok {
		return ClassifiedEntry{}, false
	}

	childrenByParent := make(map[string][]*ClassifiedEntry, len(pathToEntry))
	for path, entry := range pathToEntry {
		if path == trimmedRoot {
			continue
		}
		parentPath := filepath.Dir(path)
		if parentPath == "." || parentPath == path {
			continue
		}
		if _, ok := pathToEntry[parentPath]; !ok {
			continue
		}
		childrenByParent[parentPath] = append(childrenByParent[parentPath], entry)
	}
	for parentPath := range childrenByParent {
		sort.Slice(childrenByParent[parentPath], func(i, j int) bool {
			return childrenByParent[parentPath][i].Path < childrenByParent[parentPath][j].Path
		})
	}

	var build func(*ClassifiedEntry) ClassifiedEntry
	build = func(node *ClassifiedEntry) ClassifiedEntry {
		if node == nil {
			return ClassifiedEntry{}
		}
		out := *node
		children := childrenByParent[node.Path]
		out.Subtree = make([]ClassifiedEntry, 0, len(children))
		for _, child := range children {
			out.Subtree = append(out.Subtree, build(child))
		}

		return out
	}

	return build(rootEntry), true
}
