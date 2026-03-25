package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
)

func TestFolderSplitterExecutorMixedSplitFirstLevel(t *testing.T) {
	t.Parallel()

	executor := newFolderSplitterExecutor()
	out, err := executor.Execute(context.Background(), NodeExecutionInput{
		Node: repository.WorkflowGraphNode{
			Config: map[string]any{"split_mixed": true, "split_depth": 1},
		},
		Inputs: map[string]any{
			"entry": ClassifiedEntry{
				FolderID: "folder-root",
				Path:     "/root/mixed",
				Name:     "mixed",
				Category: "mixed",
				Subtree: map[string]ClassifiedEntry{
					"child-a": {
						FolderID: "folder-a",
						Path:     "/root/mixed/child-a",
						Name:     "child-a",
						Category: "video",
					},
					"child-b": {
						FolderID: "folder-b",
						Path:     "/root/mixed/child-b",
						Name:     "child-b",
						Category: "photo",
					},
					"child-a/nested": {
						FolderID: "folder-a-nested",
						Path:     "/root/mixed/child-a/nested",
						Name:     "nested",
						Category: "manga",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out.Status != ExecutionSuccess {
		t.Fatalf("status = %q, want %q", out.Status, ExecutionSuccess)
	}
	if len(out.Outputs) != 1 {
		t.Fatalf("len(outputs) = %d, want 1", len(out.Outputs))
	}

	items, ok := out.Outputs[0].([]ProcessingItem)
	if !ok {
		t.Fatalf("output type = %T, want []ProcessingItem", out.Outputs[0])
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].SourcePath != "/root/mixed/child-a" || items[0].Category != "video" {
		t.Fatalf("items[0] = %#v, want child-a video", items[0])
	}
	if items[1].SourcePath != "/root/mixed/child-b" || items[1].Category != "photo" {
		t.Fatalf("items[1] = %#v, want child-b photo", items[1])
	}
}

func TestCategoryRouterExecutorPortPlacement(t *testing.T) {
	t.Parallel()

	executor := newCategoryRouterExecutor()
	out, err := executor.Execute(context.Background(), NodeExecutionInput{
		Inputs: map[string]any{
			"items": []ProcessingItem{
				{FolderName: "v", Category: "video"},
				{FolderName: "m", Category: "manga"},
				{FolderName: "p", Category: "photo"},
				{FolderName: "o", Category: "other"},
				{FolderName: "x", Category: "mixed"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out.Status != ExecutionSuccess {
		t.Fatalf("status = %q, want %q", out.Status, ExecutionSuccess)
	}
	if len(out.Outputs) != 5 {
		t.Fatalf("len(outputs) = %d, want 5", len(out.Outputs))
	}

	for i := 0; i < 5; i++ {
		list, ok := out.Outputs[i].([]ProcessingItem)
		if !ok || len(list) != 1 {
			t.Fatalf("port %d type/count = %T/%d, want []ProcessingItem/1", i, out.Outputs[i], len(list))
		}
	}

	if out.Outputs[0].([]ProcessingItem)[0].Category != "video" {
		t.Fatalf("video port category = %q, want video", out.Outputs[0].([]ProcessingItem)[0].Category)
	}
	if out.Outputs[1].([]ProcessingItem)[0].Category != "manga" {
		t.Fatalf("manga port category = %q, want manga", out.Outputs[1].([]ProcessingItem)[0].Category)
	}
	if out.Outputs[2].([]ProcessingItem)[0].Category != "photo" {
		t.Fatalf("photo port category = %q, want photo", out.Outputs[2].([]ProcessingItem)[0].Category)
	}
	if out.Outputs[3].([]ProcessingItem)[0].Category != "other" {
		t.Fatalf("other port category = %q, want other", out.Outputs[3].([]ProcessingItem)[0].Category)
	}
	if out.Outputs[4].([]ProcessingItem)[0].Category != "mixed" {
		t.Fatalf("mixed_leaf port category = %q, want mixed", out.Outputs[4].([]ProcessingItem)[0].Category)
	}
}

func TestRenameNodeExecutorTemplateRegexAndConditionalDefault(t *testing.T) {
	t.Parallel()

	executor := newRenameNodeExecutor()

	t.Run("template_title_year", func(t *testing.T) {
		t.Parallel()

		out, err := executor.Execute(context.Background(), NodeExecutionInput{
			Node: repository.WorkflowGraphNode{Config: map[string]any{
				"strategy": "template",
				"template": "{title} ({year})",
			}},
			Inputs: map[string]any{"item": ProcessingItem{FolderName: "Blade Runner 2049", TargetName: "Blade Runner 2049"}},
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		item, ok := out.Outputs[0].(ProcessingItem)
		if !ok {
			t.Fatalf("output type = %T, want ProcessingItem", out.Outputs[0])
		}
		if item.TargetName != "Blade Runner (2049)" {
			t.Fatalf("TargetName = %q, want %q", item.TargetName, "Blade Runner (2049)")
		}
	})

	t.Run("regex_extract_named_groups", func(t *testing.T) {
		t.Parallel()

		out, err := executor.Execute(context.Background(), NodeExecutionInput{
			Node: repository.WorkflowGraphNode{Config: map[string]any{
				"strategy": "regex_extract",
				"regex":    `^(?P<title>.+?)\[(?P<year>\d{4})\]$`,
				"template": "{title} ({year})",
			}},
			Inputs: map[string]any{"item": ProcessingItem{FolderName: "Dune[2021]", TargetName: "Dune[2021]"}},
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		item := out.Outputs[0].(ProcessingItem)
		if item.TargetName != "Dune (2021)" {
			t.Fatalf("TargetName = %q, want %q", item.TargetName, "Dune (2021)")
		}
	})

	t.Run("conditional_default", func(t *testing.T) {
		t.Parallel()

		out, err := executor.Execute(context.Background(), NodeExecutionInput{
			Node: repository.WorkflowGraphNode{Config: map[string]any{
				"strategy": "conditional",
				"rules": []any{
					map[string]any{"condition": `name CONTAINS "合集"`, "template": "PACK-{name}"},
					map[string]any{"condition": `category == "video"`, "template": "VID-{name}"},
					map[string]any{"condition": "DEFAULT", "template": "DEFAULT-{name}"},
				},
			}},
			Inputs: map[string]any{"item": ProcessingItem{FolderName: "Sample", TargetName: "Sample", Category: "photo"}},
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		item := out.Outputs[0].(ProcessingItem)
		if item.TargetName != "DEFAULT-Sample" {
			t.Fatalf("TargetName = %q, want %q", item.TargetName, "DEFAULT-Sample")
		}
	})
}

func TestMoveNodeExecutorConflictPolicySkipAndAutoRename(t *testing.T) {
	t.Parallel()

	t.Run("skip_when_target_exists", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		sourcePath := filepath.Join(root, "source", "album")
		targetDir := filepath.Join(root, "target")
		dstExisting := filepath.Join(targetDir, "album")

		mustMkdirAll(t, sourcePath)
		mustMkdirAll(t, dstExisting)

		executor := newPhase4MoveNodeExecutor(fs.NewOSAdapter(), nil)
		out, err := executor.Execute(context.Background(), NodeExecutionInput{
			Node: repository.WorkflowGraphNode{Config: map[string]any{
				"target_dir":            targetDir,
				"conflict_policy":       "skip",
				"move_unit":             "folder",
				"preserve_substructure": true,
			}},
			Inputs: map[string]any{"item": ProcessingItem{SourcePath: sourcePath, FolderName: "album", TargetName: "album"}},
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		results, ok := out.Outputs[1].([]MoveResult)
		if !ok || len(results) != 1 {
			t.Fatalf("results output = %T/%d, want []MoveResult/1", out.Outputs[1], len(results))
		}
		if results[0].Status != "skipped" {
			t.Fatalf("result status = %q, want skipped", results[0].Status)
		}

		if !pathExists(t, sourcePath) {
			t.Fatalf("source path %q should still exist after skip", sourcePath)
		}
	})

	t.Run("auto_rename_when_target_exists", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		sourcePath := filepath.Join(root, "source", "album")
		targetDir := filepath.Join(root, "target")
		dstExisting := filepath.Join(targetDir, "album")
		dstRenamed := filepath.Join(targetDir, "album (1)")

		mustMkdirAll(t, sourcePath)
		mustMkdirAll(t, dstExisting)

		executor := newPhase4MoveNodeExecutor(fs.NewOSAdapter(), nil)
		out, err := executor.Execute(context.Background(), NodeExecutionInput{
			Node: repository.WorkflowGraphNode{Config: map[string]any{
				"target_dir":      targetDir,
				"conflict_policy": "auto_rename",
			}},
			Inputs: map[string]any{"item": ProcessingItem{SourcePath: sourcePath, FolderName: "album"}},
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		results, ok := out.Outputs[1].([]MoveResult)
		if !ok || len(results) != 1 {
			t.Fatalf("results output = %T/%d, want []MoveResult/1", out.Outputs[1], len(results))
		}
		if results[0].TargetPath != dstRenamed {
			t.Fatalf("target path = %q, want %q", results[0].TargetPath, dstRenamed)
		}
		if results[0].Status != "moved" {
			t.Fatalf("result status = %q, want moved", results[0].Status)
		}

		if pathExists(t, sourcePath) {
			t.Fatalf("source path %q should not exist after move", sourcePath)
		}
		if !pathExists(t, dstRenamed) {
			t.Fatalf("renamed destination %q should exist", dstRenamed)
		}
	})
}

func TestCompressNodeExecutorUnsupportedAndArchiveNaming(t *testing.T) {
	t.Parallel()

	t.Run("unsupported_scope_returns_error", func(t *testing.T) {
		t.Parallel()

		executor := newCompressNodeExecutor(fs.NewMockAdapter())
		_, err := executor.Execute(context.Background(), NodeExecutionInput{
			Node:   repository.WorkflowGraphNode{Config: map[string]any{"scope": "files"}},
			Inputs: map[string]any{"item": ProcessingItem{SourcePath: "/x", FolderName: "x"}},
		})
		if err == nil {
			t.Fatalf("Execute() error = nil, want unsupported scope error")
		}
	})

	t.Run("archive_name_auto_suffix", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		archiveDir := filepath.Join(root, "archives")
		mustMkdirAll(t, archiveDir)

		existing := filepath.Join(archiveDir, "album.cbz")
		if err := os.WriteFile(existing, []byte("seed"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", existing, err)
		}

		path, err := compressNodeResolveArchivePath(context.Background(), fs.NewOSAdapter(), archiveDir, "album", ".cbz")
		if err != nil {
			t.Fatalf("compressNodeResolveArchivePath() error = %v", err)
		}
		want := filepath.Join(archiveDir, "album (1).cbz")
		if path != want {
			t.Fatalf("archive path = %q, want %q", path, want)
		}
	})
}

func TestCompressNodeExecutorRollbackRemovesGeneratedArchives(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	archivePath := filepath.Join(root, "archives", "album.cbz")
	mustMkdirAll(t, filepath.Dir(archivePath))
	if err := os.WriteFile(archivePath, []byte("archive"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", archivePath, err)
	}

	executor := newCompressNodeExecutor(fs.NewOSAdapter())
	err := executor.Rollback(context.Background(), NodeRollbackInput{
		NodeRun: &repository.NodeRun{
			ID:        "node-run-compress-rb-1",
			InputJSON: `{"node":{"config":{"delete_source":false}}}`,
		},
		Snapshots: []*repository.NodeSnapshot{{
			ID:         "snapshot-compress-rb-1",
			Kind:       "post",
			OutputJSON: mustJSONMarshal(t, map[string]any{"outputs": []any{map[string]any{"ok": true}, []string{archivePath}}}),
		}},
	})
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}

	if pathExists(t, archivePath) {
		t.Fatalf("archive path %q should be removed during rollback", archivePath)
	}
}

func TestCompressNodeExecutorRollbackDeleteSourceUnsupported(t *testing.T) {
	t.Parallel()

	executor := newCompressNodeExecutor(fs.NewMockAdapter())
	err := executor.Rollback(context.Background(), NodeRollbackInput{
		NodeRun: &repository.NodeRun{
			ID:        "node-run-compress-rb-unsupported",
			InputJSON: `{"node":{"config":{"delete_source":true}}}`,
		},
	})
	if err == nil {
		t.Fatalf("Rollback() error = nil, want unsupported delete_source error")
	}
	if !stringsContains(err.Error(), "delete_source=true rollback is not supported") {
		t.Fatalf("error = %q, want message containing %q", err.Error(), "delete_source=true rollback is not supported")
	}
}

func TestAuditLogNodeExecutorWritesLog(t *testing.T) {
	t.Parallel()

	repo := &auditRepoStub{}
	executor := newAuditLogNodeExecutor(NewAuditService(repo))

	_, err := executor.Execute(context.Background(), NodeExecutionInput{
		WorkflowRun: &repository.WorkflowRun{ID: "run-1", JobID: "job-1"},
		Node:        repository.WorkflowGraphNode{ID: "node-audit", Type: "audit-log", Config: map[string]any{"action": "phase4.move"}},
		Inputs: map[string]any{
			"item":   ProcessingItem{FolderID: "folder-1", SourcePath: "/source/folder-1", FolderName: "folder-1"},
			"result": MoveResult{Status: "moved"},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(repo.logs) != 1 {
		t.Fatalf("audit logs count = %d, want 1", len(repo.logs))
	}
	if repo.logs[0].Action != "phase4.move" {
		t.Fatalf("audit action = %q, want %q", repo.logs[0].Action, "phase4.move")
	}
	if repo.logs[0].FolderID != "folder-1" {
		t.Fatalf("audit folder_id = %q, want folder-1", repo.logs[0].FolderID)
	}
}

func TestThumbnailNodeHelpersAndFfmpegMissing(t *testing.T) {
	t.Parallel()

	t.Run("build_args_contains_expected_segments", func(t *testing.T) {
		t.Parallel()

		args := thumbnailNodeBuildArgs("/src/movie.mkv", "/out/thumb.jpg", 8, 640)
		if len(args) == 0 {
			t.Fatalf("thumbnailNodeBuildArgs() returned empty args")
		}
		if args[0] != "-y" {
			t.Fatalf("args[0] = %q, want -y", args[0])
		}
		if args[len(args)-1] != "/out/thumb.jpg" {
			t.Fatalf("last arg = %q, want /out/thumb.jpg", args[len(args)-1])
		}
	})

	t.Run("ffmpeg_missing_returns_clear_error", func(t *testing.T) {
		t.Parallel()

		adapter := fs.NewMockAdapter()
		adapter.AddDir("/source/album", []fs.DirEntry{{Name: "small.mp4", Size: 10}, {Name: "large.mkv", Size: 100}})

		executor := newThumbnailNodeExecutor(adapter, nil)
		executor.lookPath = func(string) (string, error) {
			return "", errors.New("not found")
		}

		_, err := executor.Execute(context.Background(), NodeExecutionInput{
			Inputs: map[string]any{"item": ProcessingItem{SourcePath: "/source/album", FolderName: "album"}},
		})
		if err == nil {
			t.Fatalf("Execute() error = nil, want ffmpeg missing error")
		}
		if !stringsContains(err.Error(), "ffmpeg binary not found") {
			t.Fatalf("error = %q, want message containing %q", err.Error(), "ffmpeg binary not found")
		}
	})
}

func TestThumbnailNodeExecutorPersistsCoverImagePath(t *testing.T) {
	t.Parallel()

	database := newServiceTestDB(t)
	folderRepo := repository.NewFolderRepository(database)

	ctx := context.Background()
	folder := &repository.Folder{
		ID:             "folder-thumbnail-1",
		Path:           "/source/album",
		Name:           "album",
		Category:       "video",
		CategorySource: "auto",
		Status:         "pending",
	}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	adapter := fs.NewMockAdapter()
	adapter.AddDir("/source/album", []fs.DirEntry{{Name: "movie.mkv", Size: 100}})

	executor := newThumbnailNodeExecutor(adapter, folderRepo)
	executor.lookPath = func(string) (string, error) {
		return "/usr/bin/ffmpeg", nil
	}
	executor.runFFmpeg = func(context.Context, string, ...string) ([]byte, error) {
		return nil, nil
	}

	_, err := executor.Execute(ctx, NodeExecutionInput{
		Node: repository.WorkflowGraphNode{Config: map[string]any{"output_dir": "/out"}},
		Inputs: map[string]any{
			"item": ProcessingItem{FolderID: folder.ID, SourcePath: "/source/album", FolderName: "album", Category: "video"},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	updated, err := folderRepo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("folderRepo.GetByID() error = %v", err)
	}
	if updated.CoverImagePath != "/out/album.jpg" {
		t.Fatalf("cover_image_path = %q, want %q", updated.CoverImagePath, "/out/album.jpg")
	}
}

func TestThumbnailNodeExecutorRollbackRemovesFilesAndClearsCover(t *testing.T) {
	t.Parallel()

	database := newServiceTestDB(t)
	folderRepo := repository.NewFolderRepository(database)

	ctx := context.Background()
	root := t.TempDir()
	thumbPath := filepath.Join(root, "thumbnails", "album.jpg")
	mustMkdirAll(t, filepath.Dir(thumbPath))
	if err := os.WriteFile(thumbPath, []byte("thumb"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", thumbPath, err)
	}

	folder := &repository.Folder{
		ID:             "folder-thumbnail-rb-1",
		Path:           "/source/album",
		Name:           "album",
		Category:       "video",
		CategorySource: "auto",
		Status:         "pending",
		CoverImagePath: thumbPath,
	}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	executor := newThumbnailNodeExecutor(fs.NewOSAdapter(), folderRepo)
	err := executor.Rollback(ctx, NodeRollbackInput{
		NodeRun: &repository.NodeRun{ID: "node-run-thumbnail-rb-1"},
		Folder:  folder,
		Snapshots: []*repository.NodeSnapshot{{
			ID:   "snapshot-thumbnail-rb-1",
			Kind: "post",
			OutputJSON: mustJSONMarshal(t, map[string]any{
				"outputs": []any{
					map[string]any{
						"folder_id":   folder.ID,
						"source_path": "/source/album",
					},
					[]string{thumbPath},
				},
			}),
		}},
	})
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}

	if pathExists(t, thumbPath) {
		t.Fatalf("thumbnail path %q should be removed during rollback", thumbPath)
	}

	updated, err := folderRepo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("folderRepo.GetByID() error = %v", err)
	}
	if updated.CoverImagePath != "" {
		t.Fatalf("cover_image_path = %q, want empty after rollback", updated.CoverImagePath)
	}
}

type auditRepoStub struct {
	logs []*repository.AuditLog
}

func (s *auditRepoStub) Write(_ context.Context, log *repository.AuditLog) error {
	s.logs = append(s.logs, log)
	return nil
}

func (s *auditRepoStub) List(_ context.Context, _ repository.AuditListFilter) ([]*repository.AuditLog, int, error) {
	return append([]*repository.AuditLog(nil), s.logs...), len(s.logs), nil
}

func (s *auditRepoStub) GetByID(_ context.Context, id string) (*repository.AuditLog, error) {
	for _, item := range s.logs {
		if item.ID == id {
			return item, nil
		}
	}
	return nil, nil
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", path, err)
	}
}

func pathExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

func stringsContains(text, sub string) bool {
	return strings.Contains(text, sub)
}

func mustJSONMarshal(t *testing.T, value any) string {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	return string(data)
}
