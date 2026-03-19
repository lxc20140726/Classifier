package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/liqiye/classifier/internal/service"
)

type MoveExecutor interface {
	MoveFolders(ctx context.Context, input service.MoveFolderInput) error
}

type MoveHandler struct {
	mover MoveExecutor
}

func NewMoveHandler(mover MoveExecutor) *MoveHandler {
	return &MoveHandler{mover: mover}
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

	operationID := uuid.NewString()

	go func(folderIDs []string, targetDir, opID string) {
		if h.mover == nil {
			return
		}

		_ = h.mover.MoveFolders(context.Background(), service.MoveFolderInput{
			FolderIDs:   folderIDs,
			TargetDir:   targetDir,
			OperationID: opID,
		})
	}(append([]string(nil), req.FolderIDs...), req.TargetDir, operationID)

	c.JSON(http.StatusAccepted, gin.H{"operation_id": operationID})
}
