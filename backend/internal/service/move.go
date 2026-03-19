package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
	"github.com/liqiye/classifier/internal/sse"
)

type MoveFolderInput struct {
	FolderIDs   []string
	TargetDir   string
	OperationID string
}

type SnapshotRecorder interface {
	CreateBefore(ctx context.Context, jobID, folderID, operationType string) (string, error)
	CommitAfter(ctx context.Context, snapshotID string, after json.RawMessage) error
}

type AuditWriter interface {
	Write(ctx context.Context, log *repository.AuditLog) error
}

type MoveService struct {
	fs        fs.FSAdapter
	folders   repository.FolderRepository
	snapshots SnapshotRecorder
	audit     AuditWriter
	broker    *sse.Broker
}

func NewMoveService(
	fsAdapter fs.FSAdapter,
	folderRepo repository.FolderRepository,
	snapshots SnapshotRecorder,
	audit AuditWriter,
	broker *sse.Broker,
) *MoveService {
	return &MoveService{
		fs:        fsAdapter,
		folders:   folderRepo,
		snapshots: snapshots,
		audit:     audit,
		broker:    broker,
	}
}

func (s *MoveService) MoveFolders(ctx context.Context, input MoveFolderInput) error {
	if input.TargetDir == "" {
		return fmt.Errorf("moveService.MoveFolders: target dir is required")
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	total := len(input.FolderIDs)
	for i, folderID := range input.FolderIDs {
		if err := s.moveOne(ctx, input, i, total, folderID); err != nil {
			s.publish("job.error", map[string]any{
				"operation_id": input.OperationID,
				"folder_id":    folderID,
				"error":        err.Error(),
				"done":         i + 1,
				"total":        total,
			})
		}
	}

	s.publish("job.done", map[string]any{
		"operation_id": input.OperationID,
		"total":        total,
	})

	return nil
}

func (s *MoveService) moveOne(ctx context.Context, input MoveFolderInput, i, total int, folderID string) error {
	folder, err := s.folders.GetByID(ctx, folderID)
	if err != nil {
		s.writeFailureAudit(ctx, input.OperationID, folderID, "", err)
		return fmt.Errorf("moveService.MoveFolders get folder %q: %w", folderID, err)
	}

	snapshotID, err := s.snapshots.CreateBefore(ctx, input.OperationID, folder.ID, "move")
	if err != nil {
		s.writeFailureAudit(ctx, input.OperationID, folder.ID, folder.Path, err)
		return fmt.Errorf("moveService.MoveFolders create snapshot for folder %q: %w", folder.ID, err)
	}

	if err := s.fs.MkdirAll(ctx, input.TargetDir, 0o755); err != nil {
		s.writeFailureAudit(ctx, input.OperationID, folder.ID, folder.Path, err)
		return fmt.Errorf("moveService.MoveFolders create target dir %q: %w", input.TargetDir, err)
	}

	dst := filepath.Join(input.TargetDir, folder.Name)
	if err := s.fs.MoveDir(ctx, folder.Path, dst); err != nil {
		s.writeFailureAudit(ctx, input.OperationID, folder.ID, folder.Path, err)
		return fmt.Errorf("moveService.MoveFolders move folder %q to %q: %w", folder.Path, dst, err)
	}

	afterPayload := []map[string]string{{
		"original_path": folder.Path,
		"current_path":  dst,
	}}
	afterJSON, err := json.Marshal(afterPayload)
	if err != nil {
		s.writeFailureAudit(ctx, input.OperationID, folder.ID, dst, err)
		return fmt.Errorf("moveService.MoveFolders marshal after payload for folder %q: %w", folder.ID, err)
	}

	if err := s.snapshots.CommitAfter(ctx, snapshotID, afterJSON); err != nil {
		s.writeFailureAudit(ctx, input.OperationID, folder.ID, dst, err)
		return fmt.Errorf("moveService.MoveFolders commit snapshot %q: %w", snapshotID, err)
	}

	if err := s.folders.UpdatePath(ctx, folder.ID, dst); err != nil {
		s.writeFailureAudit(ctx, input.OperationID, folder.ID, dst, err)
		return fmt.Errorf("moveService.MoveFolders update folder path %q: %w", folder.ID, err)
	}

	s.publish("job.progress", map[string]any{
		"operation_id": input.OperationID,
		"folder_id":    folder.ID,
		"done":         i + 1,
		"total":        total,
	})

	if err := s.audit.Write(ctx, &repository.AuditLog{
		ID:         fmt.Sprintf("audit-move-success-%s-%d", folder.ID, time.Now().UTC().UnixNano()),
		JobID:      input.OperationID,
		FolderID:   folder.ID,
		FolderPath: dst,
		Action:     "move",
		Level:      "info",
		Result:     "success",
	}); err != nil {
		return fmt.Errorf("moveService.MoveFolders write success audit for folder %q: %w", folder.ID, err)
	}

	return nil
}

func (s *MoveService) writeFailureAudit(ctx context.Context, operationID, folderID, folderPath string, moveErr error) {
	if s.audit == nil {
		return
	}

	_ = s.audit.Write(ctx, &repository.AuditLog{
		ID:         fmt.Sprintf("audit-move-failed-%s-%d", folderID, time.Now().UTC().UnixNano()),
		JobID:      operationID,
		FolderID:   folderID,
		FolderPath: folderPath,
		Action:     "move",
		Level:      "error",
		Result:     "failed",
		ErrorMsg:   moveErr.Error(),
	})
}

func (s *MoveService) publish(eventType string, payload any) {
	if s.broker == nil {
		return
	}

	_ = s.broker.Publish(eventType, payload)
}
