package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	dbpkg "github.com/liqiye/classifier/internal/db"
	"github.com/liqiye/classifier/internal/repository"
)

var configHandlerDBCounter uint64

func newConfigHandlerTestRepo(t *testing.T) repository.ConfigRepository {
	t.Helper()

	id := atomic.AddUint64(&configHandlerDBCounter, 1)
	dsn := fmt.Sprintf("file:classifier_config_handler_%d?cache=shared&mode=memory", id)

	database, err := dbpkg.Open(dsn)
	if err != nil {
		t.Fatalf("db.Open(%q) error = %v", dsn, err)
	}

	t.Cleanup(func() {
		_ = database.Close()
	})

	return repository.NewConfigRepository(database)
}

func setupConfigRouter(configRepo repository.ConfigRepository) *gin.Engine {
	g := gin.New()
	h := NewConfigHandler(configRepo)

	g.GET("/config", h.Get)
	g.PUT("/config", h.Put)

	return g
}

func TestConfigHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newConfigHandlerTestRepo(t)
	router := setupConfigRouter(repo)

	t.Run("get returns stored config", func(t *testing.T) {
		if err := repo.Set(context.Background(), "source_dir", "/media/source"); err != nil {
			t.Fatalf("repo.Set(source_dir) error = %v", err)
		}
		if err := repo.Set(context.Background(), "target_dir", "/media/target"); err != nil {
			t.Fatalf("repo.Set(target_dir) error = %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/config", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
		}

		var payload struct {
			Data map[string]string `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}

		if payload.Data["source_dir"] != "/media/source" {
			t.Fatalf("source_dir = %q, want /media/source", payload.Data["source_dir"])
		}
		if payload.Data["target_dir"] != "/media/target" {
			t.Fatalf("target_dir = %q, want /media/target", payload.Data["target_dir"])
		}
	})

	t.Run("put saves string values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/config", bytes.NewBufferString(`{"source_dir":"/mnt/source","target_dir":"/mnt/target"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
		}

		sourceDir, err := repo.Get(context.Background(), "source_dir")
		if err != nil {
			t.Fatalf("repo.Get(source_dir) error = %v", err)
		}
		if sourceDir != "/mnt/source" {
			t.Fatalf("source_dir = %q, want /mnt/source", sourceDir)
		}

		targetDir, err := repo.Get(context.Background(), "target_dir")
		if err != nil {
			t.Fatalf("repo.Get(target_dir) error = %v", err)
		}
		if targetDir != "/mnt/target" {
			t.Fatalf("target_dir = %q, want /mnt/target", targetDir)
		}
	})

	t.Run("put invalid json returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/config", bytes.NewBufferString(`{"source_dir"`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("put non-string value returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/config", bytes.NewBufferString(`{"source_dir":123}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})
}
