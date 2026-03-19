package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/liqiye/classifier/internal/service"
)

type moveCall struct {
	input service.MoveFolderInput
}

type stubMover struct {
	mu sync.Mutex

	calls  []service.MoveFolderInput
	called chan moveCall
}

func newStubMover() *stubMover {
	return &stubMover{called: make(chan moveCall, 10)}
}

func (s *stubMover) MoveFolders(_ context.Context, input service.MoveFolderInput) error {
	s.mu.Lock()
	s.calls = append(s.calls, input)
	s.mu.Unlock()

	s.called <- moveCall{input: input}
	return nil
}

func setupMoveRouter(mover MoveExecutor) *gin.Engine {
	g := gin.New()
	h := NewMoveHandler(mover)
	g.POST("/move", h.Start)
	return g
}

func TestMoveHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("valid request returns 202 and operation_id", func(t *testing.T) {
		mover := newStubMover()
		router := setupMoveRouter(mover)

		req := httptest.NewRequest(http.MethodPost, "/move", bytes.NewBufferString(`{"folder_ids":["f1"],"target_dir":"/data/target"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusAccepted, w.Body.String())
		}

		var payload struct {
			OperationID string `json:"operation_id"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}

		if payload.OperationID == "" {
			t.Fatalf("operation_id = %q, want non-empty", payload.OperationID)
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		mover := newStubMover()
		router := setupMoveRouter(mover)

		req := httptest.NewRequest(http.MethodPost, "/move", bytes.NewBufferString(`{"folder_ids":["f1"],"target_dir":`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}

		var payload map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}

		if payload["error"] != "invalid json" {
			t.Fatalf("error = %q, want invalid json", payload["error"])
		}
	})

	t.Run("empty folder_ids returns 400", func(t *testing.T) {
		mover := newStubMover()
		router := setupMoveRouter(mover)

		req := httptest.NewRequest(http.MethodPost, "/move", bytes.NewBufferString(`{"folder_ids":[],"target_dir":"/data/target"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("empty target_dir returns 400", func(t *testing.T) {
		mover := newStubMover()
		router := setupMoveRouter(mover)

		req := httptest.NewRequest(http.MethodPost, "/move", bytes.NewBufferString(`{"folder_ids":["f1"],"target_dir":""}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("background call receives exact folder_ids and target_dir", func(t *testing.T) {
		mover := newStubMover()
		router := setupMoveRouter(mover)

		folderIDs := []string{"f1", "f2", "f3"}
		targetDir := "/data/target"

		req := httptest.NewRequest(http.MethodPost, "/move", bytes.NewBufferString(`{"folder_ids":["f1","f2","f3"],"target_dir":"/data/target"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
		}

		select {
		case call := <-mover.called:
			if !reflect.DeepEqual(call.input.FolderIDs, folderIDs) {
				t.Fatalf("FolderIDs = %#v, want %#v", call.input.FolderIDs, folderIDs)
			}
			if call.input.TargetDir != targetDir {
				t.Fatalf("TargetDir = %q, want %q", call.input.TargetDir, targetDir)
			}
			if call.input.OperationID == "" {
				t.Fatalf("OperationID = %q, want non-empty", call.input.OperationID)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for MoveFolders call")
		}
	})
}
