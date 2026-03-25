package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/liqiye/classifier/internal/repository"
	"github.com/liqiye/classifier/internal/service"
)

type stubWorkflowRunReader struct {
	resumeWithDataCalled bool
	resumeWithDataRunID  string
	resumeWithData       map[string]any
	resumeWithDataErr    error
}

func (s *stubWorkflowRunReader) ListWorkflowRuns(_ context.Context, _ string, _ int, _ int) ([]*repository.WorkflowRun, int, error) {
	return nil, 0, nil
}

func (s *stubWorkflowRunReader) GetWorkflowRunDetail(_ context.Context, _ string) (*service.WorkflowRunDetail, error) {
	return nil, nil
}

func (s *stubWorkflowRunReader) ResumeWorkflowRun(_ context.Context, _ string) error {
	return nil
}

func (s *stubWorkflowRunReader) ResumeWorkflowRunWithData(_ context.Context, runID string, resumeData map[string]any) error {
	s.resumeWithDataCalled = true
	s.resumeWithDataRunID = runID
	s.resumeWithData = resumeData
	return s.resumeWithDataErr
}

func (s *stubWorkflowRunReader) RollbackWorkflowRun(_ context.Context, _ string) error {
	return nil
}

func setupWorkflowRunRouter(reader WorkflowRunReader) *gin.Engine {
	r := gin.New()
	h := NewWorkflowRunHandler(reader)
	r.POST("/workflow-runs/:id/provide-input", h.ProvideInput)
	return r
}

func TestWorkflowRunHandler_ProvideInput_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reader := &stubWorkflowRunReader{}
	router := setupWorkflowRunRouter(reader)

	req := httptest.NewRequest(http.MethodPost, "/workflow-runs/run-123/provide-input", bytes.NewBufferString(`{"category":"manga"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if !reader.resumeWithDataCalled {
		t.Fatalf("ResumeWorkflowRunWithData called = false, want true")
	}
	if reader.resumeWithDataRunID != "run-123" {
		t.Fatalf("runID = %q, want run-123", reader.resumeWithDataRunID)
	}
	rawCategory, ok := reader.resumeWithData["category"]
	if !ok {
		t.Fatalf("resume data missing category")
	}
	category, ok := rawCategory.(string)
	if !ok || category != "manga" {
		t.Fatalf("category = %#v, want manga", rawCategory)
	}
}

func TestWorkflowRunHandler_ProvideInput_InvalidCategory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reader := &stubWorkflowRunReader{}
	router := setupWorkflowRunRouter(reader)

	req := httptest.NewRequest(http.MethodPost, "/workflow-runs/run-123/provide-input", bytes.NewBufferString(`{"category":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if reader.resumeWithDataCalled {
		t.Fatalf("ResumeWorkflowRunWithData called = true, want false")
	}
}
