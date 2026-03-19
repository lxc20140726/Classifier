package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/liqiye/classifier/internal/repository"
)

type SnapshotReverter interface {
	Revert(ctx context.Context, snapshotID string) error
}

type SnapshotHandler struct {
	snapshots repository.SnapshotRepository
	reverter  SnapshotReverter
}

func NewSnapshotHandler(snapshotRepo repository.SnapshotRepository, reverter SnapshotReverter) *SnapshotHandler {
	return &SnapshotHandler{
		snapshots: snapshotRepo,
		reverter:  reverter,
	}
}

func (h *SnapshotHandler) List(c *gin.Context) {
	folderID := c.Query("folder_id")
	if folderID != "" {
		items, err := h.snapshots.ListByFolderID(c.Request.Context(), folderID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list snapshots"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": items})
		return
	}

	jobID := c.Query("job_id")
	if jobID != "" {
		items, err := h.snapshots.ListByJobID(c.Request.Context(), jobID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list snapshots"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": items})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "folder_id or job_id is required"})
}

func (h *SnapshotHandler) Revert(c *gin.Context) {
	id := c.Param("id")

	if err := h.reverter.Revert(c.Request.Context(), id); err != nil {
		if strings.Contains(err.Error(), "already reverted") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reverted": true})
}
