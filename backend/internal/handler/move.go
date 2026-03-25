package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/liqiye/classifier/internal/service"
)

type MoveJobStarter interface {
	StartJob(ctx context.Context, input service.MoveFolderInput) (string, error)
}

type MoveHandler struct {
	starter MoveJobStarter
}

func NewMoveHandler(starter MoveJobStarter) *MoveHandler {
	return &MoveHandler{starter: starter}
}

func (h *MoveHandler) Start(c *gin.Context) {
	var req struct {
		FolderIDs []string `json:"folder_ids"`
		TargetDir string   `json:"target_dir"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if len(req.FolderIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "folder_ids is required"})
		return
	}

	if req.TargetDir == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_dir is required"})
		return
	}

	if h.starter == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "move starter not configured"})
		return
	}

	jobID, err := h.starter.StartJob(c.Request.Context(), service.MoveFolderInput{
		FolderIDs: req.FolderIDs,
		TargetDir: req.TargetDir,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create job"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"job_id": jobID})
}
