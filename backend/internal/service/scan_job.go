package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/liqiye/classifier/internal/repository"
)

type ScanRunner interface {
	Scan(ctx context.Context, input ScanInput) (int, error)
}

type ScanJobStarterService struct {
	jobs    repository.JobRepository
	scanner ScanRunner

	mu      sync.Mutex
	running map[string]struct{}
}

func NewScanJobStarterService(jobRepo repository.JobRepository, scanner ScanRunner) *ScanJobStarterService {
	return &ScanJobStarterService{
		jobs:    jobRepo,
		scanner: scanner,
		running: make(map[string]struct{}),
	}
}

func (s *ScanJobStarterService) StartJob(ctx context.Context, sourceDirs []string) (string, error) {
	jobID, _, err := s.start(ctx, sourceDirs, false)
	if err != nil {
		return "", err
	}

	return jobID, nil
}

func (s *ScanJobStarterService) StartScheduledJob(ctx context.Context, sourceDirs []string) (string, bool, error) {
	return s.start(ctx, sourceDirs, true)
}

func (s *ScanJobStarterService) start(ctx context.Context, sourceDirs []string, dedupe bool) (string, bool, error) {
	normalized := normalizeScanSourceDirs(sourceDirs)
	if len(normalized) == 0 {
		return "", false, fmt.Errorf("scanJobStarter.start: source dirs are required")
	}

	key := scanSourceDirsKey(normalized)
	if dedupe {
		s.mu.Lock()
		if _, ok := s.running[key]; ok {
			s.mu.Unlock()
			return "", false, nil
		}
		s.running[key] = struct{}{}
		s.mu.Unlock()
	}

	jobID := uuid.NewString()
	folderIDsJSON, err := json.Marshal([]string{})
	if err != nil {
		if dedupe {
			s.finish(key)
		}
		return "", false, fmt.Errorf("scanJobStarter.start marshal folder_ids: %w", err)
	}

	if s.jobs != nil {
		if err := s.jobs.Create(ctx, &repository.Job{
			ID:        jobID,
			Type:      "scan",
			Status:    "pending",
			FolderIDs: string(folderIDsJSON),
			Total:     0,
		}); err != nil {
			if dedupe {
				s.finish(key)
			}
			return "", false, fmt.Errorf("scanJobStarter.start create job: %w", err)
		}
	}

	if s.scanner != nil {
		go func(jobID string, dirs []string, runningKey string, tracked bool) {
			if tracked {
				defer s.finish(runningKey)
			}
			_, _ = s.scanner.Scan(context.Background(), ScanInput{
				JobID:      jobID,
				SourceDirs: dirs,
			})
		}(jobID, append([]string(nil), normalized...), key, dedupe)
	}

	return jobID, true, nil
}

func (s *ScanJobStarterService) finish(key string) {
	s.mu.Lock()
	delete(s.running, key)
	s.mu.Unlock()
}

func normalizeScanSourceDirs(sourceDirs []string) []string {
	cleaned := make([]string, 0, len(sourceDirs))
	seen := make(map[string]struct{}, len(sourceDirs))
	for _, dir := range sourceDirs {
		trimmed := strings.TrimSpace(dir)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		cleaned = append(cleaned, trimmed)
	}
	sort.Strings(cleaned)

	return cleaned
}

func scanSourceDirsKey(sourceDirs []string) string {
	return strings.Join(sourceDirs, "\n")
}
