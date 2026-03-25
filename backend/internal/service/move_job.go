package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/liqiye/classifier/internal/repository"
)

type MoveRunner interface {
	MoveFolders(ctx context.Context, input MoveFolderInput) error
}

type MoveJobStarterService struct {
	jobs  repository.JobRepository
	mover MoveRunner
}

func NewMoveJobStarterService(jobRepo repository.JobRepository, mover MoveRunner) *MoveJobStarterService {
	return &MoveJobStarterService{jobs: jobRepo, mover: mover}
}

func (s *MoveJobStarterService) StartJob(ctx context.Context, input MoveFolderInput) (string, error) {
	if len(input.FolderIDs) == 0 {
		return "", fmt.Errorf("moveJobStarter.StartJob: folder_ids is required")
	}
	if input.TargetDir == "" {
		return "", fmt.Errorf("moveJobStarter.StartJob: target_dir is required")
	}

	jobID := uuid.NewString()
	folderIDsJSON, err := json.Marshal(input.FolderIDs)
	if err != nil {
		return "", fmt.Errorf("moveJobStarter.StartJob marshal folder_ids: %w", err)
	}

	if s.jobs != nil {
		if err := s.jobs.Create(ctx, &repository.Job{
			ID:        jobID,
			Type:      "move",
			Status:    "pending",
			FolderIDs: string(folderIDsJSON),
			Total:     len(input.FolderIDs),
		}); err != nil {
			return "", fmt.Errorf("moveJobStarter.StartJob create job: %w", err)
		}
	}

	if s.mover != nil {
		go func(jobID string, folderIDs []string, targetDir string) {
			_ = s.mover.MoveFolders(context.Background(), MoveFolderInput{
				FolderIDs: folderIDs,
				TargetDir: targetDir,
				JobID:     jobID,
			})
		}(jobID, append([]string(nil), input.FolderIDs...), input.TargetDir)
	}

	return jobID, nil
}
