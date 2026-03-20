package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	dbpkg "github.com/liqiye/classifier/internal/db"
	"github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
	"github.com/liqiye/classifier/internal/service"
)

var folderHandlerDBCounter uint64

type scannerCall struct {
	jobID      string
	sourceDirs []string
}

type stubScanner struct {
	called chan scannerCall
}

func (s *stubScanner) Scan(_ context.Context, input service.ScanInput) (int, error) {
	s.called <- scannerCall{jobID: input.JobID, sourceDirs: input.SourceDirs}
	return 1, nil
}

func newHandlerTestDB(t *testing.T) *sql.DB {
	t.Helper()

	id := atomic.AddUint64(&folderHandlerDBCounter, 1)
	dsn := fmt.Sprintf("file:classifier_handler_%d?cache=shared&mode=memory", id)

	database, err := dbpkg.Open(dsn)
	if err != nil {
		t.Fatalf("db.Open(%q) error = %v", dsn, err)
	}

	t.Cleanup(func() {
		_ = database.Close()
	})

	return database
}

func seedFolder(t *testing.T, repo repository.FolderRepository, folder *repository.Folder) {
	t.Helper()

	if err := repo.Upsert(context.Background(), folder); err != nil {
		t.Fatalf("seed Upsert(%s) error = %v", folder.ID, err)
	}
}

func setupRouter(folderRepo repository.FolderRepository, jobRepo repository.JobRepository, configRepo repository.ConfigRepository, scanner FolderScanService, fsAdapter fs.FSAdapter) *gin.Engine {
	g := gin.New()
	h := NewFolderHandler(folderRepo, jobRepo, configRepo, scanner, fsAdapter, "/test/source", "/test/delete-staging")

	g.GET("/folders", h.List)
	g.GET("/folders/:id", h.Get)
	g.POST("/folders/scan", h.Scan)
	g.POST("/folders/:id/restore", h.Restore)
	g.PATCH("/folders/:id/category", h.UpdateCategory)
	g.PATCH("/folders/:id/status", h.UpdateStatus)
	g.DELETE("/folders/:id", h.Delete)

	return g
}

func TestFolderHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	database := newHandlerTestDB(t)
	repo := repository.NewFolderRepository(database)
	jobRepo := repository.NewJobRepository(database)
	configRepo := repository.NewConfigRepository(database)
	scanner := &stubScanner{called: make(chan scannerCall, 1)}
	fsAdapter := fs.NewMockAdapter()
	router := setupRouter(repo, jobRepo, configRepo, scanner, fsAdapter)

	seedFolder(t, repo, &repository.Folder{
		ID:             "f1",
		Path:           "/media/f1",
		Name:           "f1",
		Category:       "photo",
		CategorySource: "auto",
		Status:         "pending",
	})
	seedFolder(t, repo, &repository.Folder{
		ID:             "f2",
		Path:           "/media/f2",
		Name:           "f2",
		Category:       "video",
		CategorySource: "auto",
		Status:         "done",
	})
	fsAdapter.AddDir("/media/f2", []fs.DirEntry{{Name: "a.txt", IsDir: false}})

	t.Run("list folders", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/folders?page=1&limit=10", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
		}

		var payload struct {
			Data  []repository.Folder `json:"data"`
			Total int                 `json:"total"`
			Page  int                 `json:"page"`
			Limit int                 `json:"limit"`
		}

		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}

		if payload.Total != 2 {
			t.Fatalf("total = %d, want 2", payload.Total)
		}

		if len(payload.Data) != 2 {
			t.Fatalf("len(data) = %d, want 2", len(payload.Data))
		}

		if payload.Page != 1 || payload.Limit != 10 {
			t.Fatalf("page/limit = %d/%d, want 1/10", payload.Page, payload.Limit)
		}
	})

	t.Run("get folder by id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/folders/f1", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}

		var payload struct {
			Data repository.Folder `json:"data"`
		}

		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}

		if payload.Data.ID != "f1" {
			t.Fatalf("id = %q, want f1", payload.Data.ID)
		}
	})

	t.Run("scan returns 202", func(t *testing.T) {
		if err := configRepo.Set(context.Background(), "scan_input_dirs", `["/test/source","/test/other"]`); err != nil {
			t.Fatalf("configRepo.Set() error = %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/folders/scan", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
		}

		call := <-scanner.called
		if len(call.sourceDirs) != 2 || call.sourceDirs[0] != "/test/source" || call.sourceDirs[1] != "/test/other" {
			t.Fatalf("sourceDirs = %#v, want configured dirs", call.sourceDirs)
		}
		if call.jobID == "" {
			t.Fatalf("expected scan job id to be set")
		}
	})

	t.Run("update category valid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/folders/f1/category", bytes.NewBufferString(`{"category":"manga"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
		}

		folder, err := repo.GetByID(context.Background(), "f1")
		if err != nil {
			t.Fatalf("repo.GetByID() error = %v", err)
		}

		if folder.Category != "manga" {
			t.Fatalf("category = %q, want manga", folder.Category)
		}

		if folder.CategorySource != "manual" {
			t.Fatalf("category_source = %q, want manual", folder.CategorySource)
		}
	})

	t.Run("update category invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/folders/f1/category", bytes.NewBufferString(`{"category":"unknown"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("update status valid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/folders/f1/status", bytes.NewBufferString(`{"status":"skip"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}

		folder, err := repo.GetByID(context.Background(), "f1")
		if err != nil {
			t.Fatalf("repo.GetByID() error = %v", err)
		}

		if folder.Status != "skip" {
			t.Fatalf("status = %q, want skip", folder.Status)
		}
	})

	t.Run("update status invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/folders/f1/status", bytes.NewBufferString(`{"status":"unknown"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("delete existing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/folders/f2", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}

		folder, err := repo.GetByID(context.Background(), "f2")
		if err != nil {
			t.Fatalf("repo.GetByID() error = %v", err)
		}
		if folder.DeletedAt == nil {
			t.Fatalf("expected folder to be soft-deleted")
		}
	})

	t.Run("get missing returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/folders/missing", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})
}
