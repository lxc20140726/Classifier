package service

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	dbpkg "github.com/liqiye/classifier/internal/db"
	"github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
)

type testFSAdapter struct {
	*fs.MockAdapter
	stats   map[string]fs.FileInfo
	readErr map[string]error
	statErr map[string]error
}

func newTestFSAdapter() *testFSAdapter {
	return &testFSAdapter{
		MockAdapter: fs.NewMockAdapter(),
		stats:       make(map[string]fs.FileInfo),
		readErr:     make(map[string]error),
		statErr:     make(map[string]error),
	}
}

func (a *testFSAdapter) AddFile(path string, size int64) {
	a.stats[path] = fs.FileInfo{
		Name:    filepath.Base(path),
		IsDir:   false,
		Size:    size,
		ModTime: time.Now().UTC(),
	}
}

func (a *testFSAdapter) ReadDir(ctx context.Context, path string) ([]fs.DirEntry, error) {
	if err, ok := a.readErr[path]; ok {
		return nil, err
	}

	return a.MockAdapter.ReadDir(ctx, path)
}

func (a *testFSAdapter) Stat(ctx context.Context, path string) (fs.FileInfo, error) {
	if err, ok := a.statErr[path]; ok {
		return fs.FileInfo{}, err
	}

	if info, ok := a.stats[path]; ok {
		return info, nil
	}

	return a.MockAdapter.Stat(ctx, path)
}

func TestScan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(t *testing.T, adapter *testFSAdapter, sourceDir string)
		wantCount int
		wantErr   bool
		assert    func(t *testing.T, repo repository.FolderRepository, sourceDir string)
	}{
		{
			name: "scans one photo folder successfully",
			setup: func(t *testing.T, adapter *testFSAdapter, sourceDir string) {
				t.Helper()

				photosPath := filepath.Join(sourceDir, "photos")
				adapter.AddDir(sourceDir, []fs.DirEntry{{Name: "photos", IsDir: true}})
				adapter.AddDir(photosPath, []fs.DirEntry{
					{Name: "a.jpg", IsDir: false},
					{Name: "b.png", IsDir: false},
					{Name: "readme.txt", IsDir: false},
				})
				adapter.AddFile(filepath.Join(photosPath, "a.jpg"), 100)
				adapter.AddFile(filepath.Join(photosPath, "b.png"), 200)
				adapter.AddFile(filepath.Join(photosPath, "readme.txt"), 50)
			},
			wantCount: 1,
			wantErr:   false,
			assert: func(t *testing.T, repo repository.FolderRepository, sourceDir string) {
				t.Helper()

				path := filepath.Join(sourceDir, "photos")
				folder, err := repo.GetByPath(context.Background(), path)
				if err != nil {
					t.Fatalf("GetByPath(%q) error = %v", path, err)
				}

				if folder.ID != deterministicFolderID(path) {
					t.Fatalf("folder.ID = %q, want %q", folder.ID, deterministicFolderID(path))
				}

				if folder.Category != "photo" {
					t.Fatalf("folder.Category = %q, want photo", folder.Category)
				}

				if folder.CategorySource != "auto" || folder.Status != "pending" {
					t.Fatalf("source/status = %q/%q, want auto/pending", folder.CategorySource, folder.Status)
				}

				if folder.ImageCount != 2 || folder.VideoCount != 0 || folder.TotalFiles != 3 {
					t.Fatalf("counts = image:%d video:%d total:%d, want 2/0/3", folder.ImageCount, folder.VideoCount, folder.TotalFiles)
				}

				if folder.TotalSize != 350 {
					t.Fatalf("folder.TotalSize = %d, want 350", folder.TotalSize)
				}

				if folder.MarkedForMove {
					t.Fatalf("folder.MarkedForMove = true, want false")
				}
			},
		},
		{
			name: "scans mixed media folder successfully",
			setup: func(t *testing.T, adapter *testFSAdapter, sourceDir string) {
				t.Helper()

				mixedPath := filepath.Join(sourceDir, "events")
				adapter.AddDir(sourceDir, []fs.DirEntry{{Name: "events", IsDir: true}})
				adapter.AddDir(mixedPath, []fs.DirEntry{
					{Name: "clip.mp4", IsDir: false},
					{Name: "cover.jpeg", IsDir: false},
				})
				adapter.AddFile(filepath.Join(mixedPath, "clip.mp4"), 300)
				adapter.AddFile(filepath.Join(mixedPath, "cover.jpeg"), 120)
			},
			wantCount: 1,
			assert: func(t *testing.T, repo repository.FolderRepository, sourceDir string) {
				t.Helper()

				path := filepath.Join(sourceDir, "events")
				folder, err := repo.GetByPath(context.Background(), path)
				if err != nil {
					t.Fatalf("GetByPath(%q) error = %v", path, err)
				}

				if folder.Category != "mixed" {
					t.Fatalf("folder.Category = %q, want mixed", folder.Category)
				}

				if folder.ImageCount != 1 || folder.VideoCount != 1 || folder.TotalFiles != 2 {
					t.Fatalf("counts = image:%d video:%d total:%d, want 1/1/2", folder.ImageCount, folder.VideoCount, folder.TotalFiles)
				}

				if folder.TotalSize != 420 {
					t.Fatalf("folder.TotalSize = %d, want 420", folder.TotalSize)
				}
			},
		},
		{
			name: "skips non-directory entries in source root and returns processed count",
			setup: func(t *testing.T, adapter *testFSAdapter, sourceDir string) {
				t.Helper()

				first := filepath.Join(sourceDir, "folder-a")
				second := filepath.Join(sourceDir, "folder-b")

				adapter.AddDir(sourceDir, []fs.DirEntry{
					{Name: "folder-a", IsDir: true},
					{Name: "readme.txt", IsDir: false},
					{Name: "folder-b", IsDir: true},
				})

				adapter.AddDir(first, []fs.DirEntry{{Name: "a.jpg", IsDir: false}})
				adapter.AddFile(filepath.Join(first, "a.jpg"), 10)

				adapter.AddDir(second, []fs.DirEntry{{Name: "b.mp4", IsDir: false}})
				adapter.AddFile(filepath.Join(second, "b.mp4"), 20)
			},
			wantCount: 2,
			assert: func(t *testing.T, repo repository.FolderRepository, sourceDir string) {
				t.Helper()

				if _, err := repo.GetByPath(context.Background(), filepath.Join(sourceDir, "readme.txt")); err == nil {
					t.Fatalf("expected root file entry to be skipped")
				}
			},
		},
		{
			name: "propagates read errors",
			setup: func(t *testing.T, adapter *testFSAdapter, sourceDir string) {
				t.Helper()

				adapter.AddDir(sourceDir, []fs.DirEntry{{Name: "broken", IsDir: true}})
				adapter.readErr[filepath.Join(sourceDir, "broken")] = fmt.Errorf("boom")
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "propagates stat errors",
			setup: func(t *testing.T, adapter *testFSAdapter, sourceDir string) {
				t.Helper()

				brokenPath := filepath.Join(sourceDir, "broken")
				adapter.AddDir(sourceDir, []fs.DirEntry{{Name: "broken", IsDir: true}})
				adapter.AddDir(brokenPath, []fs.DirEntry{{Name: "missing.jpg", IsDir: false}})
				adapter.statErr[filepath.Join(brokenPath, "missing.jpg")] = fmt.Errorf("missing stat")
			},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			database := newServiceTestDB(t)
			repo := repository.NewFolderRepository(database)
			adapter := newTestFSAdapter()
			sourceDir := filepath.Join("/library", tc.name)

			tc.setup(t, adapter, sourceDir)

			jobRepo := repository.NewJobRepository(database)
			snapshotRepo := repository.NewSnapshotRepository(database)
			auditRepo := repository.NewAuditRepository(database)
			auditSvc := NewAuditService(auditRepo)
			snapshotSvc := NewSnapshotService(adapter, snapshotRepo, repo)
			scanner := NewScannerService(adapter, repo, jobRepo, snapshotSvc, auditSvc, nil)
			gotCount, err := scanner.Scan(context.Background(), ScanInput{SourceDirs: []string{sourceDir}})
			if (err != nil) != tc.wantErr {
				t.Fatalf("Scan() error = %v, wantErr %v", err, tc.wantErr)
			}

			if gotCount != tc.wantCount {
				t.Fatalf("Scan() count = %d, want %d", gotCount, tc.wantCount)
			}

			if tc.assert != nil && err == nil {
				tc.assert(t, repo, sourceDir)
			}
		})
	}
}

var serviceDBCounter uint64

func newServiceTestDB(t *testing.T) *sql.DB {
	t.Helper()

	id := atomic.AddUint64(&serviceDBCounter, 1)
	dsn := fmt.Sprintf("file:classifier_service_%d?cache=shared&mode=memory", id)

	database, err := dbpkg.Open(dsn)
	if err != nil {
		t.Fatalf("db.Open(%q) error = %v", dsn, err)
	}

	t.Cleanup(func() {
		_ = database.Close()
	})

	return database
}
