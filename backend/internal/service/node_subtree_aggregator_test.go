package service

import (
	"context"
	"errors"
	"testing"

	"github.com/liqiye/classifier/internal/repository"
)

type subtreeAggregatorFakeFolderRepo struct {
	updateCategoryCalls []subtreeAggregatorUpdateCategoryCall
	updateCategoryErr   error
}

type subtreeAggregatorUpdateCategoryCall struct {
	id       string
	category string
	source   string
}

func (r *subtreeAggregatorFakeFolderRepo) Upsert(_ context.Context, _ *repository.Folder) error {
	return nil
}

func (r *subtreeAggregatorFakeFolderRepo) GetByID(_ context.Context, _ string) (*repository.Folder, error) {
	return nil, repository.ErrNotFound
}

func (r *subtreeAggregatorFakeFolderRepo) GetByPath(_ context.Context, _ string) (*repository.Folder, error) {
	return nil, repository.ErrNotFound
}

func (r *subtreeAggregatorFakeFolderRepo) List(_ context.Context, _ repository.FolderListFilter) ([]*repository.Folder, int, error) {
	return nil, 0, nil
}

func (r *subtreeAggregatorFakeFolderRepo) UpdateCategory(_ context.Context, id, category, source string) error {
	r.updateCategoryCalls = append(r.updateCategoryCalls, subtreeAggregatorUpdateCategoryCall{
		id:       id,
		category: category,
		source:   source,
	})
	if r.updateCategoryErr != nil {
		return r.updateCategoryErr
	}

	return nil
}

func (r *subtreeAggregatorFakeFolderRepo) UpdateStatus(_ context.Context, _ string, _ string) error {
	return nil
}

func (r *subtreeAggregatorFakeFolderRepo) UpdatePath(_ context.Context, _ string, _ string) error {
	return nil
}

func (r *subtreeAggregatorFakeFolderRepo) UpdateCoverImagePath(_ context.Context, _ string, _ string) error {
	return nil
}

func (r *subtreeAggregatorFakeFolderRepo) IsSuppressedPath(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (r *subtreeAggregatorFakeFolderRepo) Suppress(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (r *subtreeAggregatorFakeFolderRepo) Unsuppress(_ context.Context, _ string) error {
	return nil
}

func (r *subtreeAggregatorFakeFolderRepo) Delete(_ context.Context, _ string) error {
	return nil
}

type subtreeAggregatorFakeAuditRepo struct {
	writeCalls []*repository.AuditLog
	writeErr   error
}

func (r *subtreeAggregatorFakeAuditRepo) Write(_ context.Context, log *repository.AuditLog) error {
	r.writeCalls = append(r.writeCalls, log)
	if r.writeErr != nil {
		return r.writeErr
	}

	return nil
}

func (r *subtreeAggregatorFakeAuditRepo) List(_ context.Context, _ repository.AuditListFilter) ([]*repository.AuditLog, int, error) {
	return nil, 0, nil
}

func (r *subtreeAggregatorFakeAuditRepo) GetByID(_ context.Context, _ string) (*repository.AuditLog, error) {
	return nil, repository.ErrNotFound
}

func TestSubtreeAggregatorExecutorTypeAndSchema(t *testing.T) {
	t.Parallel()

	executor := newSubtreeAggregatorExecutor(&subtreeAggregatorFakeFolderRepo{}, nil)

	if executor.Type() != subtreeAggregatorExecutorType {
		t.Fatalf("Type() = %q, want %q", executor.Type(), subtreeAggregatorExecutorType)
	}

	schema := executor.Schema()
	if schema.Type != subtreeAggregatorExecutorType {
		t.Fatalf("schema.Type = %q, want %q", schema.Type, subtreeAggregatorExecutorType)
	}
	if len(schema.OutputPorts) != 1 || schema.OutputPorts[0].Name != "entry" {
		t.Fatalf("schema.OutputPorts = %#v, want one entry port", schema.OutputPorts)
	}
}

func TestSubtreeAggregatorExecutorExecute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		inputs                map[string]any
		folder                *repository.Folder
		workflowRun           *repository.WorkflowRun
		withAudit             bool
		folderUpdateErr       error
		auditWriteErr         error
		wantErr               bool
		wantCategory          string
		wantConfidence        float64
		wantReason            string
		wantSource            string
		wantAuditCalls        int
		wantAuditAction       string
		wantAuditFolderPath   string
		wantOutputEntryPath   string
		wantOutputEntryName   string
		wantOutputFilesLength int
	}{
		{
			name: "signal_priority_keyword_over_others",
			inputs: map[string]any{
				"node": FolderTree{Path: "/tree/path", Name: "tree-name", Files: []FileEntry{{Name: "a.jpg", Ext: ".jpg"}}},
				"signal_kw": ClassificationSignal{
					Category:   "manga",
					Confidence: 0.95,
					Reason:     "keyword",
				},
				"signal_ft": ClassificationSignal{
					Category:   "video",
					Confidence: 0.8,
					Reason:     "file-tree",
				},
			},
			folder:                &repository.Folder{ID: "folder-1", Path: "/folder/path", Name: "folder-name", Category: "other"},
			workflowRun:           &repository.WorkflowRun{ID: "run-1"},
			withAudit:             true,
			wantCategory:          "manga",
			wantConfidence:        0.95,
			wantReason:            "keyword",
			wantSource:            "workflow",
			wantAuditCalls:        1,
			wantAuditAction:       subtreeAggregatorExecutorType,
			wantAuditFolderPath:   "/tree/path",
			wantOutputEntryPath:   "/tree/path",
			wantOutputEntryName:   "tree-name",
			wantOutputFilesLength: 1,
		},
		{
			name: "fallback_to_folder_category_when_no_signal",
			inputs: map[string]any{
				"node": FolderTree{Path: "/tree/only", Name: "tree-only"},
			},
			folder:              &repository.Folder{ID: "folder-2", Path: "/folder/original", Name: "folder-orig", Category: "photo"},
			wantCategory:        "photo",
			wantSource:          "workflow",
			wantAuditCalls:      0,
			wantOutputEntryPath: "/tree/only",
			wantOutputEntryName: "tree-only",
		},
		{
			name:                "fallback_to_other_when_both_empty",
			inputs:              map[string]any{},
			folder:              &repository.Folder{ID: "folder-3", Path: "/folder/empty", Name: "folder-empty", Category: ""},
			wantCategory:        "other",
			wantSource:          "workflow",
			wantAuditCalls:      0,
			wantOutputEntryPath: "/folder/empty",
			wantOutputEntryName: "folder-empty",
		},
		{
			name: "accept_signal_from_map",
			inputs: map[string]any{
				"signal_manual": map[string]any{
					"category":   "video",
					"confidence": 1.0,
					"reason":     "manual",
				},
			},
			folder:              &repository.Folder{ID: "folder-4", Path: "/folder/map", Name: "folder-map", Category: "other"},
			wantCategory:        "video",
			wantConfidence:      1.0,
			wantReason:          "manual",
			wantSource:          "workflow",
			wantAuditCalls:      0,
			wantOutputEntryPath: "/folder/map",
			wantOutputEntryName: "folder-map",
		},
		{
			name:           "folder_required",
			inputs:         map[string]any{},
			folder:         nil,
			wantErr:        true,
			wantAuditCalls: 0,
		},
		{
			name:            "update_category_error",
			inputs:          map[string]any{"signal_high": ClassificationSignal{Category: "video"}},
			folder:          &repository.Folder{ID: "folder-5", Path: "/folder/fail", Name: "folder-fail", Category: "other"},
			folderUpdateErr: errors.New("update-failed"),
			wantErr:         true,
			wantAuditCalls:  0,
		},
		{
			name:           "audit_error",
			inputs:         map[string]any{"signal_high": ClassificationSignal{Category: "video"}},
			folder:         &repository.Folder{ID: "folder-6", Path: "/folder/audit", Name: "folder-audit", Category: "other"},
			withAudit:      true,
			auditWriteErr:  errors.New("audit-failed"),
			wantErr:        true,
			wantAuditCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			folderRepo := &subtreeAggregatorFakeFolderRepo{updateCategoryErr: tt.folderUpdateErr}
			auditRepo := &subtreeAggregatorFakeAuditRepo{writeErr: tt.auditWriteErr}

			var auditSvc *AuditService
			if tt.withAudit {
				auditSvc = NewAuditService(auditRepo)
			}

			executor := newSubtreeAggregatorExecutor(folderRepo, auditSvc)
			out, err := executor.Execute(context.Background(), NodeExecutionInput{
				WorkflowRun: tt.workflowRun,
				Folder:      tt.folder,
				Inputs:      tt.inputs,
			})

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Execute() error = nil, want non-nil")
				}
				if len(auditRepo.writeCalls) != tt.wantAuditCalls {
					t.Fatalf("audit write calls = %d, want %d", len(auditRepo.writeCalls), tt.wantAuditCalls)
				}
				return
			}

			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if out.Status != ExecutionSuccess {
				t.Fatalf("status = %q, want %q", out.Status, ExecutionSuccess)
			}

			if len(folderRepo.updateCategoryCalls) != 1 {
				t.Fatalf("UpdateCategory calls = %d, want 1", len(folderRepo.updateCategoryCalls))
			}
			call := folderRepo.updateCategoryCalls[0]
			if call.id != tt.folder.ID {
				t.Fatalf("UpdateCategory id = %q, want %q", call.id, tt.folder.ID)
			}
			if call.category != tt.wantCategory {
				t.Fatalf("UpdateCategory category = %q, want %q", call.category, tt.wantCategory)
			}
			if call.source != tt.wantSource {
				t.Fatalf("UpdateCategory source = %q, want %q", call.source, tt.wantSource)
			}

			if len(out.Outputs) != 1 {
				t.Fatalf("len(outputs) = %d, want 1", len(out.Outputs))
			}
			entry, ok := out.Outputs[0].(ClassifiedEntry)
			if !ok {
				t.Fatalf("output type = %T, want ClassifiedEntry", out.Outputs[0])
			}
			if entry.Category != tt.wantCategory {
				t.Fatalf("entry.Category = %q, want %q", entry.Category, tt.wantCategory)
			}
			if entry.Confidence != tt.wantConfidence {
				t.Fatalf("entry.Confidence = %v, want %v", entry.Confidence, tt.wantConfidence)
			}
			if entry.Reason != tt.wantReason {
				t.Fatalf("entry.Reason = %q, want %q", entry.Reason, tt.wantReason)
			}
			if entry.Classifier != subtreeAggregatorExecutorType {
				t.Fatalf("entry.Classifier = %q, want %q", entry.Classifier, subtreeAggregatorExecutorType)
			}
			if entry.Path != tt.wantOutputEntryPath {
				t.Fatalf("entry.Path = %q, want %q", entry.Path, tt.wantOutputEntryPath)
			}
			if entry.Name != tt.wantOutputEntryName {
				t.Fatalf("entry.Name = %q, want %q", entry.Name, tt.wantOutputEntryName)
			}
			if len(entry.Files) != tt.wantOutputFilesLength {
				t.Fatalf("len(entry.Files) = %d, want %d", len(entry.Files), tt.wantOutputFilesLength)
			}

			if len(auditRepo.writeCalls) != tt.wantAuditCalls {
				t.Fatalf("audit write calls = %d, want %d", len(auditRepo.writeCalls), tt.wantAuditCalls)
			}
			if tt.wantAuditCalls > 0 {
				log := auditRepo.writeCalls[0]
				if log.Action != tt.wantAuditAction {
					t.Fatalf("audit.Action = %q, want %q", log.Action, tt.wantAuditAction)
				}
				if log.FolderPath != tt.wantAuditFolderPath {
					t.Fatalf("audit.FolderPath = %q, want %q", log.FolderPath, tt.wantAuditFolderPath)
				}
			}
		})
	}
}

func TestSubtreeAggregatorExecutorResumeAndRollback(t *testing.T) {
	t.Parallel()

	executor := newSubtreeAggregatorExecutor(&subtreeAggregatorFakeFolderRepo{}, nil)
	if _, err := executor.Resume(context.Background(), NodeExecutionInput{}, nil); err == nil {
		t.Fatalf("Resume() error = nil, want non-nil")
	}

	if err := executor.Rollback(context.Background(), NodeRollbackInput{}); err != nil {
		t.Fatalf("Rollback() error = %v, want nil", err)
	}
}
