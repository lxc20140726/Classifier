package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/liqiye/classifier/internal/repository"
)

func setupWorkflowDefRouter(repo repository.WorkflowDefinitionRepository) *gin.Engine {
	r := gin.New()
	h := NewWorkflowDefHandler(repo)
	r.GET("/workflow-defs", h.List)
	r.POST("/workflow-defs", h.Create)
	r.GET("/workflow-defs/:id", h.Get)
	r.PUT("/workflow-defs/:id", h.Update)
	r.DELETE("/workflow-defs/:id", h.Delete)
	return r
}

func TestWorkflowDefHandler_CRUDRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)

	database := newHandlerTestDB(t)
	repo := repository.NewWorkflowDefinitionRepository(database)
	router := setupWorkflowDefRouter(repo)

	createBody := `{"name":"测试工作流","graph_json":"{\"nodes\":[{\"id\":\"n1\",\"type\":\"trigger\",\"config\":{},\"enabled\":true}],\"edges\":[]}"}`
	createReq := httptest.NewRequest(http.MethodPost, "/workflow-defs", bytes.NewBufferString(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	router.ServeHTTP(createResp, createReq)

	if createResp.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d body=%s", createResp.Code, http.StatusCreated, createResp.Body.String())
	}

	var created struct {
		Data repository.WorkflowDefinition `json:"data"`
	}
	if err := json.Unmarshal(createResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create) error = %v", err)
	}
	if created.Data.ID == "" {
		t.Fatalf("created id = empty, want non-empty")
	}
	if created.Data.GraphJSON == "" {
		t.Fatalf("created graph_json = empty, want non-empty")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/workflow-defs?limit=10", nil)
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d body=%s", listResp.Code, http.StatusOK, listResp.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/workflow-defs/"+created.Data.ID, nil)
	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d body=%s", getResp.Code, http.StatusOK, getResp.Body.String())
	}

	updateBody := `{"name":"已更新工作流","graph_json":"{\"nodes\":[{\"id\":\"n1\",\"type\":\"trigger\",\"config\":{},\"enabled\":true},{\"id\":\"n2\",\"type\":\"move\",\"config\":{},\"enabled\":true}],\"edges\":[{\"id\":\"e1\",\"source\":\"n1\",\"source_port\":0,\"target\":\"n2\",\"target_port\":0}]}"}`
	updateReq := httptest.NewRequest(http.MethodPut, "/workflow-defs/"+created.Data.ID, bytes.NewBufferString(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp := httptest.NewRecorder()
	router.ServeHTTP(updateResp, updateReq)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d body=%s", updateResp.Code, http.StatusOK, updateResp.Body.String())
	}

	var updated struct {
		Data repository.WorkflowDefinition `json:"data"`
	}
	if err := json.Unmarshal(updateResp.Body.Bytes(), &updated); err != nil {
		t.Fatalf("json.Unmarshal(update) error = %v", err)
	}
	if updated.Data.Name != "已更新工作流" {
		t.Fatalf("updated name = %q, want 已更新工作流", updated.Data.Name)
	}
	if updated.Data.ID != created.Data.ID {
		t.Fatalf("updated id = %q, want %q", updated.Data.ID, created.Data.ID)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/workflow-defs/"+created.Data.ID, nil)
	deleteResp := httptest.NewRecorder()
	router.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d body=%s", deleteResp.Code, http.StatusOK, deleteResp.Body.String())
	}

	getDeletedReq := httptest.NewRequest(http.MethodGet, "/workflow-defs/"+created.Data.ID, nil)
	getDeletedResp := httptest.NewRecorder()
	router.ServeHTTP(getDeletedResp, getDeletedReq)
	if getDeletedResp.Code != http.StatusNotFound {
		t.Fatalf("get deleted status = %d, want %d body=%s", getDeletedResp.Code, http.StatusNotFound, getDeletedResp.Body.String())
	}
}
