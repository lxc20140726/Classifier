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
		Inputs: testInputs(map[string]any{
			"entry": ClassifiedEntry{
				FolderID: "folder-root",
				Path:     "/root/mixed",
				Name:     "mixed",
				Category: "mixed",
				Subtree: []ClassifiedEntry{
					{
						FolderID: "folder-a",
						Path:     "/root/mixed/child-a",
						Name:     "child-a",
						Category: "video",
					},
					{
						FolderID: "folder-b",
						Path:     "/root/mixed/child-b",
						Name:     "child-b",
						Category: "photo",
					},
					{
						FolderID: "folder-a-nested",
						Path:     "/root/mixed/child-a/nested",
						Name:     "nested",
						Category: "manga",
					},
				},
			},
		}),
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

	items, ok := out.Outputs["items"].Value.([]ProcessingItem)
	if !ok {
		t.Fatalf("output type = %T, want []ProcessingItem", out.Outputs["items"].Value)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].SourcePath != "/root/mixed/child-a" || items[0].Category != "video" {
		t.Fatalf("items[0] = %#v, want child-a video", items[0])
	}
	if items[0].RootPath != "/root/mixed" || items[0].RelativePath != "child-a" || items[0].SourceKind != ProcessingItemSourceKindDirectory {
		t.Fatalf("items[0] root/relative/source_kind = %q/%q/%q, want /root/mixed/child-a/directory", items[0].RootPath, items[0].RelativePath, items[0].SourceKind)
	}
	if items[1].SourcePath != "/root/mixed/child-b" || items[1].Category != "photo" {
		t.Fatalf("items[1] = %#v, want child-b photo", items[1])
	}
	if items[1].RootPath != "/root/mixed" || items[1].RelativePath != "child-b" || items[1].SourceKind != ProcessingItemSourceKindDirectory {
		t.Fatalf("items[1] root/relative/source_kind = %q/%q/%q, want /root/mixed/child-b/directory", items[1].RootPath, items[1].RelativePath, items[1].SourceKind)
	}
}

func TestCategoryRouterExecutorPortPlacement(t *testing.T) {
	t.Parallel()

	executor := newCategoryRouterExecutor()
	out, err := executor.Execute(context.Background(), NodeExecutionInput{
		Inputs: testInputs(map[string]any{
			"items": []ProcessingItem{
				{FolderName: "v", Category: "video"},
				{FolderName: "m", Category: "manga"},
				{FolderName: "p", Category: "photo"},
				{FolderName: "o", Category: "other"},
				{FolderName: "x", Category: "mixed"},
			},
		}),
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

	for _, key := range []string{"video", "manga", "photo", "other", "mixed_leaf"} {
		list, ok := out.Outputs[key].Value.([]ProcessingItem)
		if !ok || len(list) != 1 {
			t.Fatalf("port %s type/count = %T/%d, want []ProcessingItem/1", key, out.Outputs[key].Value, len(list))
		}
	}

	if out.Outputs["video"].Value.([]ProcessingItem)[0].Category != "video" {
		t.Fatalf("video port category = %q, want video", out.Outputs["video"].Value.([]ProcessingItem)[0].Category)
	}
	if out.Outputs["manga"].Value.([]ProcessingItem)[0].Category != "manga" {
		t.Fatalf("manga port category = %q, want manga", out.Outputs["manga"].Value.([]ProcessingItem)[0].Category)
	}
	if out.Outputs["photo"].Value.([]ProcessingItem)[0].Category != "photo" {
		t.Fatalf("photo port category = %q, want photo", out.Outputs["photo"].Value.([]ProcessingItem)[0].Category)
	}
	if out.Outputs["other"].Value.([]ProcessingItem)[0].Category != "other" {
		t.Fatalf("other port category = %q, want other", out.Outputs["other"].Value.([]ProcessingItem)[0].Category)
	}
	if out.Outputs["mixed_leaf"].Value.([]ProcessingItem)[0].Category != "mixed" {
		t.Fatalf("mixed_leaf port category = %q, want mixed", out.Outputs["mixed_leaf"].Value.([]ProcessingItem)[0].Category)
	}
}

func TestCategoryRouterExecutorEmptyBranchesReturnEmptyLists(t *testing.T) {
	t.Parallel()

	executor := newCategoryRouterExecutor()
	out, err := executor.Execute(context.Background(), NodeExecutionInput{
		Inputs: testInputs(map[string]any{
			"items": []ProcessingItem{{FolderName: "v", Category: "video"}},
		}),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, key := range []string{"video", "manga", "photo", "other", "mixed_leaf"} {
		list, ok := out.Outputs[key].Value.([]ProcessingItem)
		if !ok {
			t.Fatalf("port %s type = %T, want []ProcessingItem", key, out.Outputs[key].Value)
		}
		if key == "video" {
			if len(list) != 1 {
				t.Fatalf("video len = %d, want 1", len(list))
			}
			continue
		}
		if len(list) != 0 {
			t.Fatalf("port %s len = %d, want 0", key, len(list))
		}
	}

	out, err = executor.Execute(context.Background(), NodeExecutionInput{})
	if err != nil {
		t.Fatalf("Execute() without input error = %v", err)
	}
	for _, key := range []string{"video", "manga", "photo", "other", "mixed_leaf"} {
		list, ok := out.Outputs[key].Value.([]ProcessingItem)
		if !ok {
			t.Fatalf("empty-input port %s type = %T, want []ProcessingItem", key, out.Outputs[key].Value)
		}
		if len(list) != 0 {
			t.Fatalf("empty-input port %s len = %d, want 0", key, len(list))
		}
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
			Inputs: testInputs(map[string]any{"item": ProcessingItem{FolderName: "Blade Runner 2049", TargetName: "Blade Runner 2049"}}),
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		items, ok := out.Outputs["items"].Value.([]ProcessingItem)
		if !ok || len(items) != 1 {
			t.Fatalf("output type/len = %T/%d, want []ProcessingItem/1", out.Outputs["items"].Value, len(items))
		}
		if items[0].TargetName != "Blade Runner (2049)" {
			t.Fatalf("TargetName = %q, want %q", items[0].TargetName, "Blade Runner (2049)")
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
			Inputs: testInputs(map[string]any{"item": ProcessingItem{FolderName: "Dune[2021]", TargetName: "Dune[2021]"}}),
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		items := out.Outputs["items"].Value.([]ProcessingItem)
		if len(items) != 1 || items[0].TargetName != "Dune (2021)" {
			t.Fatalf("TargetName = %q, want %q", items[0].TargetName, "Dune (2021)")
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
			Inputs: testInputs(map[string]any{"item": ProcessingItem{FolderName: "Sample", TargetName: "Sample", Category: "photo"}}),
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		items := out.Outputs["items"].Value.([]ProcessingItem)
		if len(items) != 1 || items[0].TargetName != "DEFAULT-Sample" {
			t.Fatalf("TargetName = %q, want %q", items[0].TargetName, "DEFAULT-Sample")
		}
	})
}

func TestMoveNodeExecutorMergeConflictPolicies(t *testing.T) {
	t.Parallel()

	t.Run("skip_when_target_file_exists", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		rootPath := filepath.Join(root, "source", "a")
		sourcePath := filepath.Join(rootPath, "b")
		targetDir := filepath.Join(root, "target")
		dstExisting := filepath.Join(targetDir, "a", "001.mp4")

		mustMkdirAll(t, sourcePath)
		mustMkdirAll(t, filepath.Dir(dstExisting))
		if err := os.WriteFile(filepath.Join(sourcePath, "001.mp4"), []byte("video"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(source) error = %v", err)
		}
		if err := os.WriteFile(dstExisting, []byte("seed"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(target) error = %v", err)
		}

		executor := newPhase4MoveNodeExecutor(fs.NewOSAdapter(), nil)
		out, err := executor.Execute(context.Background(), NodeExecutionInput{
			Node: repository.WorkflowGraphNode{Config: map[string]any{
				"target_dir":            targetDir,
				"conflict_policy":       "skip",
				"move_unit":             "folder",
				"preserve_substructure": true,
			}},
			Inputs: testInputs(map[string]any{"item": ProcessingItem{
				SourcePath:   sourcePath,
				RootPath:     rootPath,
				RelativePath: "b",
				SourceKind:   ProcessingItemSourceKindDirectory,
			}}),
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		results, ok := out.Outputs["step_results"].Value.([]ProcessingStepResult)
		if !ok || len(results) != 1 {
			t.Fatalf("step_results output = %T/%d, want []ProcessingStepResult/1", out.Outputs["step_results"].Value, len(results))
		}
		if results[0].Status != "skipped" {
			t.Fatalf("result status = %q, want skipped", results[0].Status)
		}
		if results[0].TargetPath != dstExisting {
			t.Fatalf("target path = %q, want %q", results[0].TargetPath, dstExisting)
		}

		if !pathExists(t, filepath.Join(sourcePath, "001.mp4")) {
			t.Fatalf("source file should still exist after skip")
		}
	})

	t.Run("auto_rename_uses_relative_prefix_and_numeric_suffix", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		rootPath := filepath.Join(root, "source", "a")
		sourcePathA := filepath.Join(rootPath, "b")
		sourcePathB := filepath.Join(rootPath, "aa", "c")
		targetDir := filepath.Join(root, "target")
		dstRoot := filepath.Join(targetDir, "a")
		dstFirst := filepath.Join(dstRoot, "dup.txt")
		dstPrefixed := filepath.Join(dstRoot, "aa-c-dup.txt")
		dstRenamed := filepath.Join(dstRoot, "aa-c-dup-1.txt")

		mustMkdirAll(t, sourcePathA)
		mustMkdirAll(t, sourcePathB)
		mustMkdirAll(t, dstRoot)
		if err := os.WriteFile(filepath.Join(sourcePathA, "dup.txt"), []byte("a"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(source A) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(sourcePathB, "dup.txt"), []byte("b"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(source B) error = %v", err)
		}
		if err := os.WriteFile(dstPrefixed, []byte("seed"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(prefixed) error = %v", err)
		}

		executor := newPhase4MoveNodeExecutor(fs.NewOSAdapter(), nil)
		out, err := executor.Execute(context.Background(), NodeExecutionInput{
			Node: repository.WorkflowGraphNode{Config: map[string]any{
				"target_dir":      targetDir,
				"conflict_policy": "auto_rename",
			}},
			Inputs: testInputs(map[string]any{"items": []ProcessingItem{
				{
					SourcePath:   sourcePathA,
					RootPath:     rootPath,
					RelativePath: "b",
					SourceKind:   ProcessingItemSourceKindDirectory,
				},
				{
					SourcePath:   sourcePathB,
					RootPath:     rootPath,
					RelativePath: "aa/c",
					SourceKind:   ProcessingItemSourceKindDirectory,
				},
			}}),
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		results, ok := out.Outputs["step_results"].Value.([]ProcessingStepResult)
		if !ok || len(results) != 2 {
			t.Fatalf("step_results output = %T/%d, want []ProcessingStepResult/2", out.Outputs["step_results"].Value, len(results))
		}
		targets := map[string]string{}
		for _, result := range results {
			targets[result.SourcePath] = result.TargetPath
			if result.Status != "moved" {
				t.Fatalf("result status = %q, want moved", result.Status)
			}
		}
		if targets[filepath.Join(sourcePathA, "dup.txt")] != dstFirst {
			t.Fatalf("first target = %q, want %q", targets[filepath.Join(sourcePathA, "dup.txt")], dstFirst)
		}
		if targets[filepath.Join(sourcePathB, "dup.txt")] != dstRenamed {
			t.Fatalf("second target = %q, want %q", targets[filepath.Join(sourcePathB, "dup.txt")], dstRenamed)
		}
		if !pathExists(t, dstFirst) || !pathExists(t, dstRenamed) {
			t.Fatalf("expected merged files should exist in target root")
		}
		if pathExists(t, filepath.Join(sourcePathA, "dup.txt")) || pathExists(t, filepath.Join(sourcePathB, "dup.txt")) {
			t.Fatalf("source files should be moved out")
		}
	})
}

func TestPhase4MoveNodeExecutorRollbackMovesArtifactsBackAndCleansTargetRoot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source", "a", "b", "001.jpg")
	targetPath := filepath.Join(root, "target", "a", "001.jpg")
	mustMkdirAll(t, filepath.Dir(sourcePath))
	mustMkdirAll(t, filepath.Dir(targetPath))
	if err := os.WriteFile(targetPath, []byte("img"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(target) error = %v", err)
	}

	encodedOutputs, err := typedValueMapToJSON(map[string]TypedValue{
		"items": {Type: PortTypeProcessingItemList, Value: []ProcessingItem{{
			FolderID:     "folder-move-rb-1",
			SourcePath:   filepath.Join(root, "source", "a", "b"),
			RootPath:     filepath.Join(root, "source", "a"),
			RelativePath: "b",
			SourceKind:   ProcessingItemSourceKindDirectory,
		}}},
		"step_results": {Type: PortTypeProcessingStepResultList, Value: []ProcessingStepResult{{SourcePath: sourcePath, TargetPath: targetPath, NodeType: "move-node", Status: "moved"}}},
	}, NewTypeRegistry())
	if err != nil {
		t.Fatalf("typedValueMapToJSON() error = %v", err)
	}

	executor := newPhase4MoveNodeExecutor(fs.NewOSAdapter(), nil)
	err = executor.Rollback(ctx, NodeRollbackInput{
		NodeRun: &repository.NodeRun{ID: "node-run-move-rb-1", OutputJSON: mustJSONMarshal(t, map[string]any{"outputs": encodedOutputs})},
	})
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}

	if !pathExists(t, sourcePath) {
		t.Fatalf("source path %q should exist after rollback", sourcePath)
	}
	if pathExists(t, targetPath) {
		t.Fatalf("target path %q should not exist after rollback", targetPath)
	}
	targetRoot := filepath.Join(root, "target", "a")
	if pathExists(t, targetRoot) {
		t.Fatalf("target root %q should be removed when empty", targetRoot)
	}
}

func TestMoveNodeExecutorMergeValidationAndArchiveFlatten(t *testing.T) {
	t.Parallel()

	t.Run("multiple_root_path_returns_error", func(t *testing.T) {
		t.Parallel()

		executor := newPhase4MoveNodeExecutor(fs.NewMockAdapter(), nil)
		_, err := executor.Execute(context.Background(), NodeExecutionInput{
			Node: repository.WorkflowGraphNode{Config: map[string]any{"target_dir": "/target"}},
			Inputs: testInputs(map[string]any{"items": []ProcessingItem{
				{SourcePath: "/source/a/b", RootPath: "/source/a", SourceKind: ProcessingItemSourceKindDirectory},
				{SourcePath: "/source/x/y", RootPath: "/source/x", SourceKind: ProcessingItemSourceKindDirectory},
			}}),
		})
		if err == nil {
			t.Fatalf("Execute() error = nil, want multiple root_path error")
		}
		if !stringsContains(err.Error(), "multiple root_path") {
			t.Fatalf("error = %q, want multiple root_path", err.Error())
		}
	})

	t.Run("directory_contains_subdirectory_returns_error", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		rootPath := filepath.Join(root, "source", "a")
		sourcePath := filepath.Join(rootPath, "b")
		subdir := filepath.Join(sourcePath, "nested")
		targetDir := filepath.Join(root, "target")
		mustMkdirAll(t, subdir)
		if err := os.WriteFile(filepath.Join(sourcePath, "001.mp4"), []byte("video"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(source) error = %v", err)
		}

		executor := newPhase4MoveNodeExecutor(fs.NewOSAdapter(), nil)
		_, err := executor.Execute(context.Background(), NodeExecutionInput{
			Node: repository.WorkflowGraphNode{Config: map[string]any{"target_dir": targetDir}},
			Inputs: testInputs(map[string]any{"item": ProcessingItem{
				SourcePath:   sourcePath,
				RootPath:     rootPath,
				RelativePath: "b",
				SourceKind:   ProcessingItemSourceKindDirectory,
			}}),
		})
		if err == nil {
			t.Fatalf("Execute() error = nil, want subdirectory validation error")
		}
		if !stringsContains(err.Error(), "contains subdirectory") {
			t.Fatalf("error = %q, want subdirectory message", err.Error())
		}
	})

	t.Run("archive_items_flatten_relative_path_to_filename", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		rootPath := filepath.Join(root, "source", "a")
		archivePathA := filepath.Join(root, "archives", "c.cbz")
		archivePathB := filepath.Join(root, "archives", "b.cbz")
		targetDir := filepath.Join(root, "target")
		mustMkdirAll(t, filepath.Dir(archivePathA))
		if err := os.WriteFile(archivePathA, []byte("a"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(archive A) error = %v", err)
		}
		if err := os.WriteFile(archivePathB, []byte("b"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(archive B) error = %v", err)
		}

		executor := newPhase4MoveNodeExecutor(fs.NewOSAdapter(), nil)
		out, err := executor.Execute(context.Background(), NodeExecutionInput{
			Node: repository.WorkflowGraphNode{Config: map[string]any{"target_dir": targetDir}},
			Inputs: testInputs(map[string]any{"items": []ProcessingItem{
				{
					SourcePath:         archivePathA,
					RootPath:           rootPath,
					RelativePath:       "aa/c",
					SourceKind:         ProcessingItemSourceKindArchive,
					OriginalSourcePath: filepath.Join(rootPath, "aa", "c"),
				},
				{
					SourcePath:         archivePathB,
					RootPath:           rootPath,
					RelativePath:       "b",
					SourceKind:         ProcessingItemSourceKindArchive,
					OriginalSourcePath: filepath.Join(rootPath, "b"),
				},
			}}),
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		results := out.Outputs["step_results"].Value.([]ProcessingStepResult)
		if len(results) != 2 {
			t.Fatalf("len(step_results) = %d, want 2", len(results))
		}

		targetA := filepath.Join(targetDir, "a", "aa-c.cbz")
		targetB := filepath.Join(targetDir, "a", "b.cbz")
		if !pathExists(t, targetA) || !pathExists(t, targetB) {
			t.Fatalf("flattened archive targets should exist: %q, %q", targetA, targetB)
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
			Inputs: testInputs(map[string]any{"item": ProcessingItem{SourcePath: "/x", FolderName: "x"}}),
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

func TestCompressNodeExecutorOutputsArchiveItemsAndCompatibility(t *testing.T) {
	t.Parallel()

	executor := newCompressNodeExecutor(fs.NewOSAdapter())

	root := t.TempDir()
	sourcePath := filepath.Join(root, "source", "album")
	archiveDir := filepath.Join(root, "archives")
	mustMkdirAll(t, sourcePath)
	if err := os.WriteFile(filepath.Join(sourcePath, "001.jpg"), []byte("img"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(source) error = %v", err)
	}

	output, err := executor.Execute(context.Background(), NodeExecutionInput{
		Node: repository.WorkflowGraphNode{
			Type: "compress-node",
			Config: map[string]any{
				"target_dir": archiveDir,
				"format":     "cbz",
			},
		},
		Inputs: testInputs(map[string]any{
			"items": []ProcessingItem{{
				FolderID:   "folder-1",
				SourcePath: sourcePath,
				FolderName: "album",
				TargetName: "album-final",
				Category:   "manga",
				Files:      []FileEntry{{Name: "001.jpg", Ext: "jpg", SizeBytes: 3}},
				ParentPath: filepath.Dir(sourcePath),
				RootPath:   sourcePath,
				SourceKind: ProcessingItemSourceKindDirectory,
			}},
		}),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.Status != ExecutionSuccess {
		t.Fatalf("status = %q, want %q", output.Status, ExecutionSuccess)
	}

	items, ok := output.Outputs["items"].Value.([]ProcessingItem)
	if !ok || len(items) != 1 {
		t.Fatalf("items output type/len = %T/%d, want []ProcessingItem/1", output.Outputs["items"].Value, len(items))
	}
	if items[0].SourcePath != sourcePath {
		t.Fatalf("items[0].SourcePath = %q, want %q", items[0].SourcePath, sourcePath)
	}

	archiveItems, ok := output.Outputs["archive_items"].Value.([]ProcessingItem)
	if !ok || len(archiveItems) != 1 {
		t.Fatalf("archive_items output type/len = %T/%d, want []ProcessingItem/1", output.Outputs["archive_items"].Value, len(archiveItems))
	}
	archiveItem := archiveItems[0]
	if archiveItem.SourcePath == sourcePath {
		t.Fatalf("archive_items[0].SourcePath should point to archive file, got source path %q", archiveItem.SourcePath)
	}
	if got, want := archiveItem.ParentPath, filepath.Dir(archiveItem.SourcePath); got != want {
		t.Fatalf("archive_items[0].ParentPath = %q, want %q", got, want)
	}
	if got, want := archiveItem.FolderName, filepath.Base(archiveItem.SourcePath); got != want {
		t.Fatalf("archive_items[0].FolderName = %q, want %q", got, want)
	}
	if got, want := archiveItem.TargetName, filepath.Base(archiveItem.SourcePath); got != want {
		t.Fatalf("archive_items[0].TargetName = %q, want %q", got, want)
	}
	if archiveItem.FolderID != "folder-1" {
		t.Fatalf("archive_items[0].FolderID = %q, want folder-1", archiveItem.FolderID)
	}
	if archiveItem.Category != "manga" {
		t.Fatalf("archive_items[0].Category = %q, want manga", archiveItem.Category)
	}
	if archiveItem.RootPath != sourcePath {
		t.Fatalf("archive_items[0].RootPath = %q, want %q", archiveItem.RootPath, sourcePath)
	}
	if archiveItem.SourceKind != ProcessingItemSourceKindArchive {
		t.Fatalf("archive_items[0].SourceKind = %q, want %q", archiveItem.SourceKind, ProcessingItemSourceKindArchive)
	}
	if archiveItem.OriginalSourcePath != sourcePath {
		t.Fatalf("archive_items[0].OriginalSourcePath = %q, want %q", archiveItem.OriginalSourcePath, sourcePath)
	}
	if archiveItem.Files != nil && len(archiveItem.Files) != 0 {
		t.Fatalf("archive_items[0].Files should be nil or empty, got len=%d", len(archiveItem.Files))
	}

	stepResults, ok := output.Outputs["step_results"].Value.([]ProcessingStepResult)
	if !ok || len(stepResults) != 1 {
		t.Fatalf("step_results output type/len = %T/%d, want []ProcessingStepResult/1", output.Outputs["step_results"].Value, len(stepResults))
	}
	if got, want := stepResults[0].TargetPath, normalizeWorkflowPath(archiveItem.SourcePath); got != want {
		t.Fatalf("step_results[0].TargetPath = %q, want %q", got, want)
	}

	compressSchema := executor.Schema()
	moveSchema := newPhase4MoveNodeExecutor(fs.NewMockAdapter(), nil).Schema()
	collectSchema := newCollectNodeExecutor().Schema()

	archivePort := compressSchema.OutputPort("archive_items")
	if archivePort == nil {
		t.Fatalf("compress schema missing output port archive_items")
	}
	moveInputPort := moveSchema.InputPort("items")
	if moveInputPort == nil {
		t.Fatalf("move schema missing input port items")
	}
	if archivePort.Type != moveInputPort.Type {
		t.Fatalf("compress archive_items type = %q, move items type = %q", archivePort.Type, moveInputPort.Type)
	}
	collectInputPort := collectSchema.InputPort("items_1")
	if collectInputPort == nil {
		t.Fatalf("collect schema missing input port items_1")
	}
	if archivePort.Type != collectInputPort.Type {
		t.Fatalf("compress archive_items type = %q, collect items_1 type = %q", archivePort.Type, collectInputPort.Type)
	}
}

func TestCompressNodeIntegrationArchiveItemsAndLegacyItems(t *testing.T) {
	t.Parallel()

	t.Run("archive_items_to_move_moves_archive_file", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		sourcePath := filepath.Join(root, "source", "album")
		archiveDir := filepath.Join(root, "archives")
		moveTargetDir := filepath.Join(root, "final")

		mustMkdirAll(t, sourcePath)
		if err := os.WriteFile(filepath.Join(sourcePath, "001.jpg"), []byte("img"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(source) error = %v", err)
		}

		compressOut, err := newCompressNodeExecutor(fs.NewOSAdapter()).Execute(context.Background(), NodeExecutionInput{
			Node: repository.WorkflowGraphNode{
				Type: "compress-node",
				Config: map[string]any{
					"target_dir": archiveDir,
					"format":     "cbz",
				},
			},
			Inputs: testInputs(map[string]any{
				"items": []ProcessingItem{{SourcePath: sourcePath, FolderName: "album", TargetName: "album"}},
			}),
		})
		if err != nil {
			t.Fatalf("compress Execute() error = %v", err)
		}

		archiveItems := compressOut.Outputs["archive_items"].Value.([]ProcessingItem)
		if len(archiveItems) != 1 {
			t.Fatalf("len(archive_items) = %d, want 1", len(archiveItems))
		}

		moveOut, err := newPhase4MoveNodeExecutor(fs.NewOSAdapter(), nil).Execute(context.Background(), NodeExecutionInput{
			Node: repository.WorkflowGraphNode{
				Type:   "move-node",
				Config: map[string]any{"target_dir": moveTargetDir},
			},
			Inputs: testInputs(map[string]any{
				"items": archiveItems,
			}),
		})
		if err != nil {
			t.Fatalf("move Execute() error = %v", err)
		}

		movedItems := moveOut.Outputs["items"].Value.([]ProcessingItem)
		if len(movedItems) != 1 {
			t.Fatalf("len(movedItems) = %d, want 1", len(movedItems))
		}
		moveSteps := moveOut.Outputs["step_results"].Value.([]ProcessingStepResult)
		if len(moveSteps) != 1 {
			t.Fatalf("len(step_results) = %d, want 1", len(moveSteps))
		}
		movedPath := moveSteps[0].TargetPath
		if filepath.Ext(movedPath) != ".cbz" {
			t.Fatalf("moved archive extension = %q, want .cbz", filepath.Ext(movedPath))
		}
		if !pathExists(t, movedPath) {
			t.Fatalf("moved archive %q should exist", movedPath)
		}
		if pathExists(t, archiveItems[0].SourcePath) {
			t.Fatalf("original archive path %q should not exist after move", archiveItems[0].SourcePath)
		}
	})

	t.Run("legacy_items_to_move_keeps_legacy_semantics", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		sourcePath := filepath.Join(root, "source", "album")
		archiveDir := filepath.Join(root, "archives")
		moveTargetDir := filepath.Join(root, "final")

		mustMkdirAll(t, sourcePath)
		if err := os.WriteFile(filepath.Join(sourcePath, "001.jpg"), []byte("img"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(source) error = %v", err)
		}

		compressOut, err := newCompressNodeExecutor(fs.NewOSAdapter()).Execute(context.Background(), NodeExecutionInput{
			Node: repository.WorkflowGraphNode{
				Type: "compress-node",
				Config: map[string]any{
					"target_dir": archiveDir,
					"format":     "cbz",
				},
			},
			Inputs: testInputs(map[string]any{
				"items": []ProcessingItem{{SourcePath: sourcePath, FolderName: "album", TargetName: "album"}},
			}),
		})
		if err != nil {
			t.Fatalf("compress Execute() error = %v", err)
		}

		legacyItems := compressOut.Outputs["items"].Value.([]ProcessingItem)
		if len(legacyItems) != 1 {
			t.Fatalf("len(items) = %d, want 1", len(legacyItems))
		}
		if legacyItems[0].SourcePath != sourcePath {
			t.Fatalf("items[0].SourcePath = %q, want %q", legacyItems[0].SourcePath, sourcePath)
		}

		moveOut, err := newPhase4MoveNodeExecutor(fs.NewOSAdapter(), nil).Execute(context.Background(), NodeExecutionInput{
			Node: repository.WorkflowGraphNode{
				Type:   "move-node",
				Config: map[string]any{"target_dir": moveTargetDir},
			},
			Inputs: testInputs(map[string]any{
				"items": legacyItems,
			}),
		})
		if err != nil {
			t.Fatalf("move Execute() error = %v", err)
		}

		movedItems := moveOut.Outputs["items"].Value.([]ProcessingItem)
		if len(movedItems) != 1 {
			t.Fatalf("len(movedItems) = %d, want 1", len(movedItems))
		}
		movedSourcePath := movedItems[0].SourcePath
		if filepath.Base(movedSourcePath) != "album" {
			t.Fatalf("moved folder name = %q, want album", filepath.Base(movedSourcePath))
		}
		if !pathExists(t, movedSourcePath) {
			t.Fatalf("moved folder %q should exist", movedSourcePath)
		}
		if pathExists(t, sourcePath) {
			t.Fatalf("source folder %q should not exist after move", sourcePath)
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
			ID:   "snapshot-compress-rb-1",
			Kind: "post",
			OutputJSON: mustJSONMarshal(t, mustTypedOutputsMap(t, map[string]TypedValue{
				"items":        {Type: PortTypeProcessingItemList, Value: []ProcessingItem{{SourcePath: "/source/album"}}},
				"step_results": {Type: PortTypeProcessingStepResultList, Value: []ProcessingStepResult{{SourcePath: "/source/album", TargetPath: archivePath, NodeType: "compress-node", Status: "succeeded"}}},
			})),
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
			Inputs: testInputs(map[string]any{"item": ProcessingItem{SourcePath: "/source/album", FolderName: "album"}}),
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
		Inputs: testInputs(map[string]any{
			"item": ProcessingItem{FolderID: folder.ID, SourcePath: "/source/album", FolderName: "album", Category: "video"},
		}),
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
		Snapshots: []*repository.NodeSnapshot{{
			ID:   "snapshot-thumbnail-rb-1",
			Kind: "post",
			OutputJSON: mustJSONMarshal(t, mustTypedOutputsMap(t, map[string]TypedValue{
				"items":        {Type: PortTypeProcessingItemList, Value: []ProcessingItem{{FolderID: folder.ID, SourcePath: "/source/album"}}},
				"step_results": {Type: PortTypeProcessingStepResultList, Value: []ProcessingStepResult{{SourcePath: "/source/album", TargetPath: thumbPath, NodeType: "thumbnail-node", Status: "succeeded"}}},
			})),
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

// TestEngineV2_AC_ROLL2_ThumbnailNodeRollbackTypedFormat verifies that
// thumbnail-node Rollback deletes the generated thumbnail file when node output
// is stored in the typed-value JSON format used by the engine v2 runtime.
func TestEngineV2_AC_ROLL2_ThumbnailNodeRollbackTypedFormat(t *testing.T) {
	t.Parallel()

	database := newServiceTestDB(t)
	folderRepo := repository.NewFolderRepository(database)

	ctx := context.Background()
	root := t.TempDir()
	thumbPath := filepath.Join(root, "thumbnails", "album.jpg")
	mustMkdirAll(t, filepath.Dir(thumbPath))
	if err := os.WriteFile(thumbPath, []byte("thumb-typed"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", thumbPath, err)
	}

	folder := &repository.Folder{
		ID: "folder-thumbnail-rb-typed", Path: "/source/album", Name: "album",
		Category: "video", CategorySource: "auto", Status: "pending", CoverImagePath: thumbPath,
	}
	if err := folderRepo.Upsert(ctx, folder); err != nil {
		t.Fatalf("folderRepo.Upsert() error = %v", err)
	}

	encodedOutputs, err := typedValueMapToJSON(map[string]TypedValue{
		"items":        {Type: PortTypeProcessingItemList, Value: []ProcessingItem{{FolderID: folder.ID, SourcePath: "/source/album"}}},
		"step_results": {Type: PortTypeProcessingStepResultList, Value: []ProcessingStepResult{{SourcePath: "/source/album", TargetPath: thumbPath, NodeType: "thumbnail-node", Status: "succeeded"}}},
	}, NewTypeRegistry())
	if err != nil {
		t.Fatalf("typedValueMapToJSON() error = %v", err)
	}

	executor := newThumbnailNodeExecutor(fs.NewOSAdapter(), folderRepo)
	err = executor.Rollback(ctx, NodeRollbackInput{
		NodeRun:   &repository.NodeRun{ID: "node-run-thumbnail-rb-typed"},
		Folder:    folder,
		Snapshots: []*repository.NodeSnapshot{{ID: "snap-thumbnail-rb-typed", Kind: "post", OutputJSON: mustJSONMarshal(t, encodedOutputs)}},
	})
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}

	if pathExists(t, thumbPath) {
		t.Fatalf("thumbnail %q should be deleted after typed-format rollback", thumbPath)
	}

	updated, err := folderRepo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("folderRepo.GetByID() error = %v", err)
	}
	if updated.CoverImagePath != "" {
		t.Fatalf("cover_image_path = %q, want empty after rollback", updated.CoverImagePath)
	}
}

// TestEngineV2_AC_ROLL3_CompressNodeRollbackTypedFormat verifies that
// compress-node Rollback deletes the generated archive when node output is
// stored in the typed-value JSON format used by the engine v2 runtime.
func TestEngineV2_AC_ROLL3_CompressNodeRollbackTypedFormat(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	archivePath := filepath.Join(root, "archives", "album.cbz")
	mustMkdirAll(t, filepath.Dir(archivePath))
	if err := os.WriteFile(archivePath, []byte("typed-archive"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", archivePath, err)
	}

	encodedOutputs, err := typedValueMapToJSON(map[string]TypedValue{
		"items":        {Type: PortTypeProcessingItemList, Value: []ProcessingItem{{SourcePath: "/source/album"}}},
		"step_results": {Type: PortTypeProcessingStepResultList, Value: []ProcessingStepResult{{SourcePath: "/source/album", TargetPath: archivePath, NodeType: "compress-node", Status: "succeeded"}}},
	}, NewTypeRegistry())
	if err != nil {
		t.Fatalf("typedValueMapToJSON() error = %v", err)
	}

	executor := newCompressNodeExecutor(fs.NewOSAdapter())
	err = executor.Rollback(context.Background(), NodeRollbackInput{
		NodeRun: &repository.NodeRun{
			ID:        "node-run-compress-rb-typed",
			InputJSON: `{"node":{"config":{"delete_source":false}}}`,
		},
		Snapshots: []*repository.NodeSnapshot{{
			ID:         "snap-compress-rb-typed",
			Kind:       "post",
			OutputJSON: mustJSONMarshal(t, encodedOutputs),
		}},
	})
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}

	if pathExists(t, archivePath) {
		t.Fatalf("archive %q should be deleted after typed-format rollback", archivePath)
	}
}

// TestEngineV2_AC_COMPAT1_LegacyOutputJsonRollbackCompat verifies backward
// compatibility cleanup: rollback rejects legacy output_json array format and
// only accepts typed-value map format.
func TestEngineV2_AC_COMPAT1_LegacyOutputJsonRollbackCompat(t *testing.T) {
	t.Parallel()

	t.Run("compress_node_legacy_array_format", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		archivePath := filepath.Join(root, "legacy-album.cbz")
		if err := os.WriteFile(archivePath, []byte("legacy"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", archivePath, err)
		}

		executor := newCompressNodeExecutor(fs.NewOSAdapter())
		err := executor.Rollback(context.Background(), NodeRollbackInput{
			NodeRun: &repository.NodeRun{
				ID:        "node-run-compress-legacy-compat",
				InputJSON: `{"node":{"config":{"delete_source":false}}}`,
			},
			Snapshots: []*repository.NodeSnapshot{{
				ID:   "snap-compress-legacy-compat",
				Kind: "post",
				// old array format: [<items-value>, <archives-list>]
				OutputJSON: mustJSONMarshal(t, map[string]any{
					"outputs": []any{
						map[string]any{"source_path": "/source/album"},
						[]string{archivePath},
					},
				}),
			}},
		})
		if err == nil {
			t.Fatalf("Rollback() expected error for legacy format, got nil")
		}
		if !pathExists(t, archivePath) {
			t.Fatalf("archive should remain when rollback receives legacy array format")
		}
	})

	t.Run("thumbnail_node_legacy_array_format", func(t *testing.T) {
		t.Parallel()

		database := newServiceTestDB(t)
		folderRepo := repository.NewFolderRepository(database)

		ctx := context.Background()
		root := t.TempDir()
		thumbPath := filepath.Join(root, "legacy-album.jpg")
		if err := os.WriteFile(thumbPath, []byte("legacy-thumb"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", thumbPath, err)
		}

		folder := &repository.Folder{
			ID: "folder-thumbnail-legacy-compat", Path: "/source/album", Name: "album",
			Category: "video", CategorySource: "auto", Status: "pending",
		}
		if err := folderRepo.Upsert(ctx, folder); err != nil {
			t.Fatalf("folderRepo.Upsert() error = %v", err)
		}

		executor := newThumbnailNodeExecutor(fs.NewOSAdapter(), folderRepo)
		err := executor.Rollback(ctx, NodeRollbackInput{
			NodeRun: &repository.NodeRun{ID: "node-run-thumbnail-legacy-compat"},
			Snapshots: []*repository.NodeSnapshot{{
				ID:   "snap-thumbnail-legacy-compat",
				Kind: "post",
				// old array format: [<items-value>, <thumbnail-paths-list>]
				OutputJSON: mustJSONMarshal(t, map[string]any{
					"outputs": []any{
						map[string]any{"folder_id": folder.ID, "source_path": "/source/album"},
						[]string{thumbPath},
					},
				}),
			}},
		})
		if err == nil {
			t.Fatalf("Rollback() expected error for legacy thumbnail format, got nil")
		}
		if !pathExists(t, thumbPath) {
			t.Fatalf("thumbnail should remain when rollback receives legacy array format")
		}
	})
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

func mustTypedOutputsMap(t *testing.T, values map[string]TypedValue) map[string]TypedValueJSON {
	t.Helper()

	encoded, err := typedValueMapToJSON(values, NewTypeRegistry())
	if err != nil {
		t.Fatalf("typedValueMapToJSON() error = %v", err)
	}

	return encoded
}
