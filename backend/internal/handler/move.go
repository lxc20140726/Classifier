package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/liqiye/classifier/internal/repository"
	"github.com/liqiye/classifier/internal/service"
)

type MoveExecutor interface {
	MoveFolders(ctx context.Context, input service.MoveFolderInput) error
}

type MoveHandler struct {
	mover MoveExecutor
	jobs  repository.JobRepository
}

func NewMoveHandler(mover MoveExecutor, jobs repository.JobRepository) *MoveHandler {
	return &MoveHandler{mover: mover, jobs: jobs}
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

	jobID := uuid.NewString()
	folderIDsJSON, err := json.Marshal(req.FolderIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode folder_ids"})
		return
	}

	if h.jobs != nil {
		err = h.jobs.Create(c.Request.Context(), &repository.Job{
			ID:        jobID,
			Type:      "move",
			Status:    "pending",
			FolderIDs: string(folderIDsJSON),
			Total:     len(req.FolderIDs),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create job"})
			return
		}
	}

	go func(folderIDs []string, targetDir, currentJobID string) {
		if h.mover == nil {
			return
		}

		_ = h.mover.MoveFolders(context.Background(), service.MoveFolderInput{
			FolderIDs: folderIDs,
			TargetDir: targetDir,
			JobID:     currentJobID,
		})
	}(append([]string(nil), req.FolderIDs...), req.TargetDir, jobID)

	c.JSON(http.StatusAccepted, gin.H{"job_id": jobID})
}
