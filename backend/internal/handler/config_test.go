package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
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
		err := repo.SaveAppConfig(context.Background(), &repository.AppConfig{
			ScanInputDirs: []string{"/media/source", "/media/source-2"},
			SourceDir:     "/media/source",
			TargetDir:     "/media/target",
			OutputDirs: repository.AppConfigOutputDirs{
				Video: "/media/target/video",
				Manga: "/media/target/manga",
				Photo: "/media/target/photo",
				Other: "/media/target/other",
				Mixed: "/media/target/mixed",
			},
		})
		if err != nil {
			t.Fatalf("repo.SaveAppConfig() error = %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/config", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
		}

		var payload struct {
			Data repository.AppConfig `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}

		if payload.Data.SourceDir != "/media/source" {
			t.Fatalf("source_dir = %q, want /media/source", payload.Data.SourceDir)
		}
		if payload.Data.TargetDir != "/media/target" {
			t.Fatalf("target_dir = %q, want /media/target", payload.Data.TargetDir)
		}
		if !reflect.DeepEqual(payload.Data.ScanInputDirs, []string{"/media/source", "/media/source-2"}) {
			t.Fatalf("scan_input_dirs = %#v, want [/media/source /media/source-2]", payload.Data.ScanInputDirs)
		}
	})

	t.Run("put saves structured config and syncs legacy keys", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/config", bytes.NewBufferString(`{
			"scan_input_dirs":["/mnt/source","/mnt/source-2"],
			"source_dir":"/mnt/source",
			"target_dir":"/mnt/target",
			"output_dirs":{
				"video":"/mnt/target/video",
				"manga":"/mnt/target/manga",
				"photo":"/mnt/target/photo",
				"other":"/mnt/target/other",
				"mixed":"/mnt/target/mixed"
			}
		}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
		}

		storedConfig, err := repo.GetAppConfig(context.Background())
		if err != nil {
			t.Fatalf("repo.GetAppConfig() error = %v", err)
		}
		if !reflect.DeepEqual(storedConfig.ScanInputDirs, []string{"/mnt/source", "/mnt/source-2"}) {
			t.Fatalf("scan_input_dirs = %#v, want [/mnt/source /mnt/source-2]", storedConfig.ScanInputDirs)
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

		rawScanInputDirs, err := repo.Get(context.Background(), "scan_input_dirs")
		if err != nil {
			t.Fatalf("repo.Get(scan_input_dirs) error = %v", err)
		}
		if rawScanInputDirs != `["/mnt/source","/mnt/source-2"]` {
			t.Fatalf("scan_input_dirs = %q, want %q", rawScanInputDirs, `["/mnt/source","/mnt/source-2"]`)
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

	t.Run("put wrong typed value returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/config", bytes.NewBufferString(`{"scan_input_dirs":"/mnt/source"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("get maps legacy config when app_config is empty", func(t *testing.T) {
		legacyRepo := newConfigHandlerTestRepo(t)
		legacyRouter := setupConfigRouter(legacyRepo)

		if err := legacyRepo.Set(context.Background(), "source_dir", "/legacy/source"); err != nil {
			t.Fatalf("repo.Set(source_dir) error = %v", err)
		}
		if err := legacyRepo.Set(context.Background(), "target_dir", "/legacy/target"); err != nil {
			t.Fatalf("repo.Set(target_dir) error = %v", err)
		}
		if err := legacyRepo.Set(context.Background(), "scan_input_dirs", `["/legacy/source","/legacy/source-2"]`); err != nil {
			t.Fatalf("repo.Set(scan_input_dirs) error = %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/config", nil)
		w := httptest.NewRecorder()
		legacyRouter.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
		}

		var payload struct {
			Data repository.AppConfig `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}

		if payload.Data.SourceDir != "/legacy/source" {
			t.Fatalf("source_dir = %q, want /legacy/source", payload.Data.SourceDir)
		}
		if payload.Data.TargetDir != "/legacy/target" {
			t.Fatalf("target_dir = %q, want /legacy/target", payload.Data.TargetDir)
		}
		if payload.Data.OutputDirs.Video != "/legacy/target/video" {
			t.Fatalf("output_dirs.video = %q, want /legacy/target/video", payload.Data.OutputDirs.Video)
		}
		if !reflect.DeepEqual(payload.Data.ScanInputDirs, []string{"/legacy/source", "/legacy/source-2"}) {
			t.Fatalf("scan_input_dirs = %#v, want [/legacy/source /legacy/source-2]", payload.Data.ScanInputDirs)
		}
	})
}
