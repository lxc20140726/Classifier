package service

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
)

const thumbnailNodeExecutorType = "thumbnail-node"

var thumbnailVideoExtensions = map[string]struct{}{
	".mp4":  {},
	".mkv":  {},
	".mov":  {},
	".avi":  {},
	".wmv":  {},
	".flv":  {},
	".webm": {},
	".m4v":  {},
	".ts":   {},
}

type thumbnailNodeExecutor struct {
	fs        fs.FSAdapter
	folders   repository.FolderRepository
	lookPath  func(string) (string, error)
	runFFmpeg func(ctx context.Context, command string, args ...string) ([]byte, error)
}

func newThumbnailNodeExecutor(fsAdapter fs.FSAdapter, folderRepo repository.FolderRepository) *thumbnailNodeExecutor {
	return &thumbnailNodeExecutor{
		fs:       fsAdapter,
		folders:  folderRepo,
		lookPath: exec.LookPath,
		runFFmpeg: func(ctx context.Context, command string, args ...string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, command, args...)
			return cmd.CombinedOutput()
		},
	}
}

func (e *thumbnailNodeExecutor) Type() string {
	return thumbnailNodeExecutorType
}

func (e *thumbnailNodeExecutor) Schema() NodeSchema {
	return NodeSchema{
		Type:        e.Type(),
		Label:       "Thumbnail Node",
		Description: "Generate a representative thumbnail for video folders",
		InputPorts: []NodeSchemaPort{
			{Name: "items", Description: "PROCESSING_ITEM or PROCESSING_ITEM[]", Required: true},
		},
		OutputPorts: []NodeSchemaPort{
			{Name: "items", Description: "PROCESSING_ITEM or PROCESSING_ITEM[]", Required: true},
			{Name: "thumbnail_paths", Description: "Generated thumbnail file paths", Required: true},
		},
	}
}

func (e *thumbnailNodeExecutor) Execute(ctx context.Context, input NodeExecutionInput) (NodeExecutionOutput, error) {
	items, isList, ok := categoryRouterExtractItems(input.Inputs)
	if !ok || len(items) == 0 {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: item/items input is required", e.Type())
	}

	ffmpegPath, err := e.lookPath("ffmpeg")
	if err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: ffmpeg binary not found: %w", e.Type(), err)
	}

	outputDir := stringConfig(input.Node.Config, "output_dir")
	if outputDir == "" {
		outputDir = stringConfig(input.Node.Config, "target_dir")
	}
	if outputDir == "" {
		outputDir = ".thumbnails"
	}

	createTarget := folderSplitterBoolConfig(input.Node.Config, "create_target_if_missing", true)
	outExists, err := e.fs.Exists(ctx, outputDir)
	if err != nil {
		return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: check output dir %q: %w", e.Type(), outputDir, err)
	}
	if !outExists {
		if !createTarget {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: output dir %q does not exist and create_target_if_missing is false", e.Type(), outputDir)
		}
		if err := e.fs.MkdirAll(ctx, outputDir, 0o755); err != nil {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: create output dir %q: %w", e.Type(), outputDir, err)
		}
	}

	offsetSeconds := intConfig(input.Node.Config, "offset_seconds", 8)
	if offsetSeconds < 0 {
		offsetSeconds = 0
	}
	width := intConfig(input.Node.Config, "width", 640)

	thumbnailPaths := make([]string, 0, len(items))
	for _, item := range items {
		videoPath, err := e.representativeVideoPath(ctx, item)
		if err != nil {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: %w", e.Type(), err)
		}

		outputName := phase4MoveItemName(item)
		if strings.TrimSpace(outputName) == "" {
			outputName = filepath.Base(strings.TrimSpace(item.SourcePath))
		}
		thumbnailPath := filepath.Join(outputDir, outputName+".jpg")

		args := thumbnailNodeBuildArgs(videoPath, thumbnailPath, offsetSeconds, width)
		combined, err := e.runFFmpeg(ctx, ffmpegPath, args...)
		if err != nil {
			return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: ffmpeg failed for %q -> %q: %w: %s", e.Type(), videoPath, thumbnailPath, err, strings.TrimSpace(string(combined)))
		}

		if e.folders != nil && strings.TrimSpace(item.FolderID) != "" {
			if err := e.folders.UpdateCoverImagePath(ctx, item.FolderID, thumbnailPath); err != nil {
				return NodeExecutionOutput{}, fmt.Errorf("%s.Execute: update cover image path for %q: %w", e.Type(), item.FolderID, err)
			}
		}

		thumbnailPaths = append(thumbnailPaths, thumbnailPath)
	}

	if isList {
		return NodeExecutionOutput{Outputs: map[string]TypedValue{"items": {Type: PortTypeProcessingItemList, Value: items}, "thumbnail_paths": {Type: PortTypeStringList, Value: thumbnailPaths}}, Status: ExecutionSuccess}, nil
	}

	return NodeExecutionOutput{Outputs: map[string]TypedValue{"items": {Type: PortTypeJSON, Value: items[0]}, "thumbnail_paths": {Type: PortTypeStringList, Value: thumbnailPaths}}, Status: ExecutionSuccess}, nil
}

func (e *thumbnailNodeExecutor) Resume(_ context.Context, _ NodeExecutionInput, _ map[string]any) (NodeExecutionOutput, error) {
	return NodeExecutionOutput{}, fmt.Errorf("%s: Resume not supported", e.Type())
}

func (e *thumbnailNodeExecutor) Rollback(ctx context.Context, input NodeRollbackInput) error {
	thumbnailPaths, folderIDs, err := thumbnailNodeCollectRollbackData(input)
	if err != nil {
		return fmt.Errorf("%s.Rollback: %w", e.Type(), err)
	}

	for _, thumbnailPath := range thumbnailPaths {
		exists, err := e.fs.Exists(ctx, thumbnailPath)
		if err != nil {
			return fmt.Errorf("%s.Rollback: check thumbnail path %q: %w", e.Type(), thumbnailPath, err)
		}
		if !exists {
			continue
		}
		if err := e.fs.Remove(ctx, thumbnailPath); err != nil {
			return fmt.Errorf("%s.Rollback: remove thumbnail path %q: %w", e.Type(), thumbnailPath, err)
		}
	}

	if input.Folder != nil && strings.TrimSpace(input.Folder.ID) != "" {
		folderIDs = append(folderIDs, input.Folder.ID)
	}
	if err := e.clearCoverImagePathIfNeeded(ctx, folderIDs, thumbnailPaths); err != nil {
		return fmt.Errorf("%s.Rollback: %w", e.Type(), err)
	}

	return nil
}

func thumbnailNodeCollectRollbackData(input NodeRollbackInput) ([]string, []string, error) {
	pathSet := map[string]struct{}{}
	folderIDSet := map[string]struct{}{}

	if input.NodeRun != nil && strings.TrimSpace(input.NodeRun.OutputJSON) != "" {
		typedOutputs, typed, err := parseTypedNodeOutputs(input.NodeRun.OutputJSON)
		if err != nil {
			return nil, nil, fmt.Errorf("parse node output json for node run %q: %w", input.NodeRun.ID, err)
		}
		var paths []string
		var folderIDs []string
		if typed {
			paths = compactStringSlice(anyToStringSlice(typedOutputs["thumbnail_paths"].Value))
			folderIDs = thumbnailNodeExtractFolderIDs(typedOutputs["items"].Value)
		} else {
			outputs, legacyErr := parseNodeOutputs(input.NodeRun.OutputJSON)
			if legacyErr != nil {
				return nil, nil, fmt.Errorf("parse node output json for node run %q: %w", input.NodeRun.ID, legacyErr)
			}
			paths, folderIDs = thumbnailNodeExtractRollbackData(outputs)
		}
		for _, path := range paths {
			pathSet[path] = struct{}{}
		}
		for _, folderID := range folderIDs {
			folderIDSet[folderID] = struct{}{}
		}
	}

	for _, snapshot := range input.Snapshots {
		if snapshot == nil || snapshot.Kind != "post" || strings.TrimSpace(snapshot.OutputJSON) == "" {
			continue
		}
		typedOutputs, typed, err := parseTypedNodeOutputs(snapshot.OutputJSON)
		if err != nil {
			return nil, nil, fmt.Errorf("parse node snapshot output json for snapshot %q: %w", snapshot.ID, err)
		}
		var paths []string
		var folderIDs []string
		if typed {
			paths = compactStringSlice(anyToStringSlice(typedOutputs["thumbnail_paths"].Value))
			folderIDs = thumbnailNodeExtractFolderIDs(typedOutputs["items"].Value)
		} else {
			outputs, legacyErr := parseNodeOutputs(snapshot.OutputJSON)
			if legacyErr != nil {
				return nil, nil, fmt.Errorf("parse node snapshot output json for snapshot %q: %w", snapshot.ID, legacyErr)
			}
			paths, folderIDs = thumbnailNodeExtractRollbackData(outputs)
		}
		for _, path := range paths {
			pathSet[path] = struct{}{}
		}
		for _, folderID := range folderIDs {
			folderIDSet[folderID] = struct{}{}
		}
	}

	thumbnailPaths := make([]string, 0, len(pathSet))
	for path := range pathSet {
		thumbnailPaths = append(thumbnailPaths, path)
	}

	folderIDs := make([]string, 0, len(folderIDSet))
	for folderID := range folderIDSet {
		folderIDs = append(folderIDs, folderID)
	}

	return thumbnailPaths, folderIDs, nil
}

func thumbnailNodeExtractRollbackData(outputs []any) ([]string, []string) {
	if len(outputs) < 2 {
		return nil, nil
	}

	thumbnailPaths := compactStringSlice(anyToStringSlice(outputs[1]))
	folderIDs := thumbnailNodeExtractFolderIDs(outputs[0])

	return thumbnailPaths, folderIDs
}

func thumbnailNodeExtractFolderIDs(raw any) []string {
	items := thumbnailNodeAsMapSlice(raw)
	if len(items) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		folderID := strings.TrimSpace(anyString(item["folder_id"]))
		if folderID == "" {
			continue
		}
		if _, ok := seen[folderID]; ok {
			continue
		}
		seen[folderID] = struct{}{}
		out = append(out, folderID)
	}

	return out
}

func thumbnailNodeAsMapSlice(raw any) []map[string]any {
	switch typed := raw.(type) {
	case map[string]any:
		return []map[string]any{typed}
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, itemMap)
		}
		return out
	default:
		return nil
	}
}

func (e *thumbnailNodeExecutor) clearCoverImagePathIfNeeded(ctx context.Context, folderIDs, thumbnailPaths []string) error {
	if e.folders == nil || len(thumbnailPaths) == 0 {
		return nil
	}

	uniqueFolderIDs := uniqueCompactStringSlice(folderIDs)
	if len(uniqueFolderIDs) != 1 {
		return nil
	}

	folder, err := e.folders.GetByID(ctx, uniqueFolderIDs[0])
	if err != nil {
		return nil
	}

	currentCover := strings.TrimSpace(folder.CoverImagePath)
	if currentCover == "" {
		return nil
	}

	for _, thumbnailPath := range thumbnailPaths {
		if currentCover != thumbnailPath {
			continue
		}
		if err := e.folders.UpdateCoverImagePath(ctx, uniqueFolderIDs[0], ""); err != nil {
			return nil
		}
		return nil
	}

	return nil
}

func (e *thumbnailNodeExecutor) representativeVideoPath(ctx context.Context, item ProcessingItem) (string, error) {
	sourcePath := strings.TrimSpace(item.SourcePath)
	if sourcePath == "" {
		return "", fmt.Errorf("item source_path is required")
	}

	entries, err := e.fs.ReadDir(ctx, sourcePath)
	if err != nil {
		return "", fmt.Errorf("read source dir %q: %w", sourcePath, err)
	}

	bestName := ""
	bestSize := int64(-1)
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}
		if !thumbnailNodeIsVideoFile(entry.Name) {
			continue
		}
		if entry.Size > bestSize {
			bestSize = entry.Size
			bestName = entry.Name
		}
	}

	if bestName == "" {
		return "", fmt.Errorf("no direct video file found in %q", sourcePath)
	}

	return filepath.Join(sourcePath, bestName), nil
}

func thumbnailNodeBuildArgs(videoPath, outputPath string, offsetSeconds, width int) []string {
	args := []string{
		"-y",
		"-ss", strconv.Itoa(offsetSeconds),
		"-i", videoPath,
		"-frames:v", "1",
		"-q:v", "2",
	}
	if width > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=%d:-2", width))
	}
	args = append(args, outputPath)

	return args
}

func thumbnailNodeIsVideoFile(name string) bool {
	_, ok := thumbnailVideoExtensions[strings.ToLower(filepath.Ext(strings.TrimSpace(name)))]
	return ok
}
