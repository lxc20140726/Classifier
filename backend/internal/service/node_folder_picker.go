package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
)

const folderPickerExecutorType = "folder-picker"

type folderPickerNodeExecutor struct {
	fs      fs.FSAdapter
	folders repository.FolderRepository
}

func newFolderPickerNodeExecutor(fsAdapter fs.FSAdapter, folderRepo repository.FolderRepository) *folderPickerNodeExecutor {
	return &folderPickerNodeExecutor{fs: fsAdapter, folders: folderRepo}
}

func (e *folderPickerNodeExecutor) Type() string {
	return folderPickerExecutorType
}

func (e *folderPickerNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "文件夹选择器",
		Description: "静态源节点：支持自选路径目录或从媒体文件夹记录选择，运行时直接输出为目录树",
		Inputs:      []PortDef{},
		Outputs: []PortDef{
			{Name: "folders", Type: PortTypeFolderTreeList, Description: "选定的目录树列表（可直接接分类器）"},
			{Name: "path", Type: PortTypePath, Description: "第一个配置路径（可接目录树扫描器的 source_dir）"},
		},
	}
}

func (e *folderPickerNodeExecutor) Execute(ctx context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	mode := folderPickerSourceMode(input.Node.Config)
	paths := []string{}
	switch mode {
	case "folders":
		folderIDs := folderPickerParseFolderIDs(input.Node.Config)
		paths = e.pathsFromFolderRecords(ctx, folderIDs)
	default:
		paths = folderPickerParsePaths(input.Node.Config)
	}

	trees := make([]FolderTree, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		info, err := e.fs.Stat(ctx, p)
		if err != nil {
			continue
		}
		if !info.IsDir {
			continue
		}

		trees = append(trees, FolderTree{
			Path:    p,
			Name:    filepath.Base(p),
			Files:   []FileEntry{},
			Subdirs: []FolderTree{},
		})
	}

	primaryPath := ""
	if len(paths) > 0 {
		primaryPath = strings.TrimSpace(paths[0])
	}

	return NodeExecutionOutput{
		Outputs: map[string]TypedValue{
			"folders": {Type: PortTypeFolderTreeList, Value: trees},
			"path":    {Type: PortTypePath, Value: primaryPath},
		},
		Status: ExecutionSuccess,
	}, nil
}

func (e *folderPickerNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *folderPickerNodeExecutor) Rollback(_ context.Context, _ NodeRollbackInput) error {
	return nil
}

func (e *folderPickerNodeExecutor) pathsFromFolderRecords(ctx context.Context, folderIDs []string) []string {
	if e.folders == nil || len(folderIDs) == 0 {
		return []string{}
	}

	paths := make([]string, 0, len(folderIDs))
	seen := make(map[string]struct{}, len(folderIDs))
	for _, id := range folderIDs {
		trimmedID := strings.TrimSpace(id)
		if trimmedID == "" {
			continue
		}
		folder, err := e.folders.GetByID(ctx, trimmedID)
		if err != nil || folder == nil {
			continue
		}
		trimmedPath := strings.TrimSpace(folder.Path)
		if trimmedPath == "" {
			continue
		}
		if _, ok := seen[trimmedPath]; ok {
			continue
		}
		seen[trimmedPath] = struct{}{}
		paths = append(paths, trimmedPath)
	}

	return paths
}

func folderPickerSourceMode(config map[string]any) string {
	mode := strings.ToLower(strings.TrimSpace(anyString(config["source_mode"])))
	if mode == "folders" {
		return mode
	}

	return "path"
}

func folderPickerParsePaths(config map[string]any) []string {
	raw, ok := config["paths"]
	if !ok {
		return nil
	}

	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil
		}
		return []string{s}
	}
	return nil
}

func folderPickerParseFolderIDs(config map[string]any) []string {
	raw, ok := config["folder_ids"]
	if !ok {
		return nil
	}

	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil
		}
		return []string{s}
	}

	return nil
}
