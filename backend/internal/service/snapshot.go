package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
)

type snapshotPathState struct {
	OriginalPath string `json:"original_path"`
	CurrentPath  string `json:"current_path"`
}

var errSnapshotAlreadyReverted = errors.New("snapshot already reverted")

type SnapshotService struct {
	fs        fs.FSAdapter
	snapshots repository.SnapshotRepository
	folders   repository.FolderRepository
}

func NewSnapshotService(fsAdapter fs.FSAdapter, snapshotRepo repository.SnapshotRepository, folderRepo repository.FolderRepository) *SnapshotService {
	return &SnapshotService{fs: fsAdapter, snapshots: snapshotRepo, folders: folderRepo}
}

func (s *SnapshotService) CreateBefore(ctx context.Context, jobID, folderID, operationType string) (string, error) {
	folder, err := s.folders.GetByID(ctx, folderID)
	if err != nil {
		return "", fmt.Errorf("snapshot.CreateBefore load folder %q: %w", folderID, err)
	}

	beforeState, err := json.Marshal([]snapshotPathState{{
		OriginalPath: folder.Path,
		CurrentPath:  folder.Path,
	}})
	if err != nil {
		return "", fmt.Errorf("snapshot.CreateBefore marshal before state: %w", err)
	}

	snapshotID := uuid.NewString()
	if err := s.snapshots.Create(ctx, &repository.Snapshot{
		ID:            snapshotID,
		JobID:         jobID,
		FolderID:      folderID,
		OperationType: operationType,
		Before:        beforeState,
		Status:        "pending",
	}); err != nil {
		return "", fmt.Errorf("snapshot.CreateBefore create snapshot: %w", err)
	}

	return snapshotID, nil
}

func (s *SnapshotService) CommitAfter(ctx context.Context, snapshotID string, after json.RawMessage) error {
	if err := s.snapshots.CommitAfter(ctx, snapshotID, after); err != nil {
		return fmt.Errorf("snapshot.CommitAfter persist after state: %w", err)
	}

	if err := s.snapshots.UpdateStatus(ctx, snapshotID, "committed"); err != nil {
		return fmt.Errorf("snapshot.CommitAfter update status: %w", err)
	}

	return nil
}

func (s *SnapshotService) UpdateDetail(ctx context.Context, snapshotID string, detail json.RawMessage) error {
	if err := s.snapshots.UpdateDetail(ctx, snapshotID, detail); err != nil {
		return fmt.Errorf("snapshot.UpdateDetail persist detail: %w", err)
	}

	return nil
}

func (s *SnapshotService) Revert(ctx context.Context, snapshotID string) error {
	snapshot, err := s.snapshots.GetByID(ctx, snapshotID)
	if err != nil {
		return fmt.Errorf("snapshot.Revert load snapshot %q: %w", snapshotID, err)
	}

	if snapshot.Status == "reverted" {
		return errSnapshotAlreadyReverted
	}

	stateJSON := snapshot.After
	if len(stateJSON) == 0 {
		stateJSON = snapshot.Before
	}

	var states []snapshotPathState
	if err := json.Unmarshal(stateJSON, &states); err != nil {
		return fmt.Errorf("snapshot.Revert parse snapshot state: %w", err)
	}

	for _, state := range states {
		if state.CurrentPath != state.OriginalPath {
			if err := s.fs.MoveDir(ctx, state.CurrentPath, state.OriginalPath); err != nil {
				return fmt.Errorf("snapshot.Revert move %q to %q: %w", state.CurrentPath, state.OriginalPath, err)
			}
		}

		folder, err := s.folders.GetByID(ctx, snapshot.FolderID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				continue
			}
			return fmt.Errorf("snapshot.Revert load folder %q: %w", snapshot.FolderID, err)
		}

		if err := s.folders.UpdatePath(ctx, folder.ID, state.OriginalPath); err != nil {
			return fmt.Errorf("snapshot.Revert update folder path for %q: %w", folder.ID, err)
		}
	}

	if err := s.snapshots.UpdateStatus(ctx, snapshot.ID, "reverted"); err != nil {
		return fmt.Errorf("snapshot.Revert update snapshot status: %w", err)
	}

	return nil
}
