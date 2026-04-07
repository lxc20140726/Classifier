package repository

import (
	"context"
	"errors"
	"testing"
)

func TestFolderRepositoryUpsertAndGetters(t *testing.T) {
	t.Parallel()

	database := newTestDB(t)
	repo := NewFolderRepository(database)
	ctx := context.Background()

	folder := &Folder{
		ID:             "folder-1",
		Path:           "/media/a",
		Name:           "a",
		Category:       "photo",
		CategorySource: "auto",
		Status:         "pending",
		ImageCount:     10,
		VideoCount:     1,
		OtherFileCount: 2,
		HasOtherFiles:  true,
		TotalFiles:     11,
		TotalSize:      1024,
		MarkedForMove:  true,
		CoverImagePath: "/covers/a.jpg",
	}

	if err := repo.Upsert(ctx, folder); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	updated := *folder
	updated.Name = "renamed"
	updated.Status = "done"
	updated.MarkedForMove = false
	if err := repo.Upsert(ctx, &updated); err != nil {
		t.Fatalf("Upsert(updated) error = %v", err)
	}

	byID, err := repo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if byID.Name != updated.Name {
		t.Fatalf("GetByID().Name = %q, want %q", byID.Name, updated.Name)
	}

	if byID.Status != updated.Status {
		t.Fatalf("GetByID().Status = %q, want %q", byID.Status, updated.Status)
	}

	if byID.MarkedForMove != updated.MarkedForMove {
		t.Fatalf("GetByID().MarkedForMove = %v, want %v", byID.MarkedForMove, updated.MarkedForMove)
	}
	if byID.OtherFileCount != updated.OtherFileCount || byID.HasOtherFiles != updated.HasOtherFiles {
		t.Fatalf("GetByID() other stats = %d/%v, want %d/%v", byID.OtherFileCount, byID.HasOtherFiles, updated.OtherFileCount, updated.HasOtherFiles)
	}
	if byID.CoverImagePath != updated.CoverImagePath {
		t.Fatalf("GetByID().CoverImagePath = %q, want %q", byID.CoverImagePath, updated.CoverImagePath)
	}

	if byID.ScannedAt.IsZero() || byID.UpdatedAt.IsZero() {
		t.Fatalf("expected non-zero timestamps, got scanned_at=%v updated_at=%v", byID.ScannedAt, byID.UpdatedAt)
	}

	byPath, err := repo.GetByPath(ctx, folder.Path)
	if err != nil {
		t.Fatalf("GetByPath() error = %v", err)
	}

	if byPath.ID != folder.ID {
		t.Fatalf("GetByPath().ID = %q, want %q", byPath.ID, folder.ID)
	}
}

func TestFolderRepositoryList(t *testing.T) {
	t.Parallel()

	database := newTestDB(t)
	repo := NewFolderRepository(database)
	ctx := context.Background()

	fixtures := []*Folder{
		{ID: "f1", Path: "/media/photo-a", Name: "photo-a", Category: "photo", CategorySource: "auto", Status: "pending"},
		{ID: "f2", Path: "/media/video-a", Name: "video-a", Category: "video", CategorySource: "auto", Status: "done"},
		{ID: "f3", Path: "/media/photo-b", Name: "photo-b", Category: "photo", CategorySource: "manual", Status: "done"},
	}

	for _, fixture := range fixtures {
		if err := repo.Upsert(ctx, fixture); err != nil {
			t.Fatalf("Upsert(%s) error = %v", fixture.ID, err)
		}
	}

	tests := []struct {
		name      string
		filter    FolderListFilter
		wantTotal int
		wantLen   int
	}{
		{
			name:      "no filter returns all",
			filter:    FolderListFilter{Page: 1, Limit: 10},
			wantTotal: 3,
			wantLen:   3,
		},
		{
			name:      "filter by status",
			filter:    FolderListFilter{Status: "done", Page: 1, Limit: 10},
			wantTotal: 2,
			wantLen:   2,
		},
		{
			name:      "filter by category",
			filter:    FolderListFilter{Category: "photo", Page: 1, Limit: 10},
			wantTotal: 2,
			wantLen:   2,
		},
		{
			name:      "query filter",
			filter:    FolderListFilter{Q: "video", Page: 1, Limit: 10},
			wantTotal: 1,
			wantLen:   1,
		},
		{
			name:      "pagination",
			filter:    FolderListFilter{Page: 2, Limit: 2},
			wantTotal: 3,
			wantLen:   1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			items, total, err := repo.List(ctx, tc.filter)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}

			if total != tc.wantTotal {
				t.Fatalf("List() total = %d, want %d", total, tc.wantTotal)
			}

			if len(items) != tc.wantLen {
				t.Fatalf("List() len = %d, want %d", len(items), tc.wantLen)
			}
		})
	}
}

func TestFolderRepositoryUpdatesAndDelete(t *testing.T) {
	t.Parallel()

	database := newTestDB(t)
	repo := NewFolderRepository(database)
	ctx := context.Background()

	folder := &Folder{
		ID:             "folder-update",
		Path:           "/media/update",
		Name:           "update",
		Category:       "other",
		CategorySource: "auto",
		Status:         "pending",
	}

	if err := repo.Upsert(ctx, folder); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	if err := repo.UpdateCategory(ctx, folder.ID, "video", "manual"); err != nil {
		t.Fatalf("UpdateCategory() error = %v", err)
	}

	if err := repo.UpdateStatus(ctx, folder.ID, "done"); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if err := repo.UpdatePath(ctx, folder.ID, "/media/new-path"); err != nil {
		t.Fatalf("UpdatePath() error = %v", err)
	}

	if err := repo.UpdateCoverImagePath(ctx, folder.ID, "/covers/new-path.jpg"); err != nil {
		t.Fatalf("UpdateCoverImagePath() error = %v", err)
	}

	got, err := repo.GetByID(ctx, folder.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.Category != "video" || got.CategorySource != "manual" {
		t.Fatalf("category/source = %q/%q, want video/manual", got.Category, got.CategorySource)
	}

	if got.Status != "done" {
		t.Fatalf("status = %q, want done", got.Status)
	}

	if got.Path != "/media/new-path" {
		t.Fatalf("path = %q, want /media/new-path", got.Path)
	}

	if got.CoverImagePath != "/covers/new-path.jpg" {
		t.Fatalf("cover_image_path = %q, want /covers/new-path.jpg", got.CoverImagePath)
	}

	if err := repo.Delete(ctx, folder.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = repo.GetByID(ctx, folder.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByID(after delete) error = %v, want ErrNotFound", err)
	}
}

func TestFolderRepositoryNotFoundMutations(t *testing.T) {
	t.Parallel()

	database := newTestDB(t)
	repo := NewFolderRepository(database)
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{name: "UpdateCategory missing", fn: func() error { return repo.UpdateCategory(ctx, "missing", "photo", "auto") }},
		{name: "UpdateStatus missing", fn: func() error { return repo.UpdateStatus(ctx, "missing", "done") }},
		{name: "UpdatePath missing", fn: func() error { return repo.UpdatePath(ctx, "missing", "/new") }},
		{name: "UpdateCoverImagePath missing", fn: func() error { return repo.UpdateCoverImagePath(ctx, "missing", "/cover.jpg") }},
		{name: "Delete missing", fn: func() error { return repo.Delete(ctx, "missing") }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn()
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("error = %v, want ErrNotFound", err)
			}
		})
	}
}
