package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
	"github.com/liqiye/classifier/internal/sse"
)

type ScanInput struct {
	JobID      string
	SourceDirs []string
}

type ScanSnapshotRecorder interface {
	CreateBefore(ctx context.Context, jobID, folderID, operationType string) (string, error)
	CommitAfter(ctx context.Context, snapshotID string, after json.RawMessage) error
	UpdateDetail(ctx context.Context, snapshotID string, detail json.RawMessage) error
}

type ScannerService struct {
	fs        fs.FSAdapter
	folders   repository.FolderRepository
	jobs      repository.JobRepository
	snapshots ScanSnapshotRecorder
	audit     AuditWriter
	broker    *sse.Broker
}

type scanTarget struct {
	sourceDir    string
	folderPath   string
	folderName   string
	relativePath string
}

func NewScannerService(
	fsAdapter fs.FSAdapter,
	folderRepo repository.FolderRepository,
	jobRepo repository.JobRepository,
	snapshots ScanSnapshotRecorder,
	audit AuditWriter,
	broker *sse.Broker,
) *ScannerService {
	return &ScannerService{
		fs:        fsAdapter,
		folders:   folderRepo,
		jobs:      jobRepo,
		snapshots: snapshots,
		audit:     audit,
		broker:    broker,
	}
}

func (s *ScannerService) Scan(ctx context.Context, input ScanInput) (int, error) {
	sourceDirs := normalizeSourceDirs(input.SourceDirs)
	if len(sourceDirs) == 0 {
		return 0, fmt.Errorf("scanner.Scan: source dirs are required")
	}

	targets, err := s.discoverTargets(ctx, sourceDirs)
	if err != nil {
		if input.JobID != "" && s.jobs != nil {
			_ = s.jobs.UpdateStatus(ctx, input.JobID, "failed", err.Error())
		}
		s.publish("scan.failed", map[string]any{
			"job_id": input.JobID,
			"error":  err.Error(),
		})
		return 0, err
	}

	total := len(targets)
	if input.JobID != "" && s.jobs != nil {
		if err := s.jobs.UpdateStatus(ctx, input.JobID, "running", ""); err != nil {
			return 0, fmt.Errorf("scanner.Scan start job %q: %w", input.JobID, err)
		}
		if err := s.jobs.UpdateTotal(ctx, input.JobID, total); err != nil {
			return 0, fmt.Errorf("scanner.Scan set total for job %q: %w", input.JobID, err)
		}
	}

	s.publish("scan.started", map[string]any{
		"job_id":      input.JobID,
		"source_dirs": sourceDirs,
		"total":       total,
	})

	processed := 0
	failed := 0
	for index, target := range targets {
		folder, scanErr := s.scanOne(ctx, input.JobID, target)
		if scanErr != nil {
			failed++
			if input.JobID != "" && s.jobs != nil {
				_ = s.jobs.IncrementProgress(ctx, input.JobID, 0, 1)
			}
			s.publish("scan.error", map[string]any{
				"job_id":      input.JobID,
				"folder_name": target.folderName,
				"folder_path": target.folderPath,
				"source_dir":  target.sourceDir,
				"done":        index + 1,
				"total":       total,
				"error":       scanErr.Error(),
			})
			continue
		}

		processed++
		if input.JobID != "" && s.jobs != nil {
			if err := s.jobs.IncrementProgress(ctx, input.JobID, 1, 0); err != nil {
				return processed, fmt.Errorf("scanner.Scan update progress for job %q: %w", input.JobID, err)
			}
		}

		payload := map[string]any{
			"job_id":        input.JobID,
			"folder_id":     folder.ID,
			"folder_name":   folder.Name,
			"folder_path":   folder.Path,
			"source_dir":    folder.SourceDir,
			"relative_path": folder.RelativePath,
			"category":      folder.Category,
			"done":          index + 1,
			"total":         total,
		}
		s.publish("scan.progress", payload)
		s.publish("job.progress", payload)
	}

	status := "succeeded"
	if failed > 0 {
		status = "partial"
	}
	if input.JobID != "" && s.jobs != nil {
		if err := s.jobs.UpdateStatus(ctx, input.JobID, status, ""); err != nil {
			return processed, fmt.Errorf("scanner.Scan finish job %q: %w", input.JobID, err)
		}
	}

	completion := map[string]any{
		"job_id":    input.JobID,
		"status":    status,
		"processed": processed,
		"failed":    failed,
		"total":     total,
	}
	s.publish("scan.done", completion)
	s.publish("job.done", completion)

	return processed, nil
}

func (s *ScannerService) discoverTargets(ctx context.Context, sourceDirs []string) ([]scanTarget, error) {
	targets := make([]scanTarget, 0)
	for _, sourceDir := range sourceDirs {
		entries, err := s.fs.ReadDir(ctx, sourceDir)
		if err != nil {
			return nil, fmt.Errorf("scanner.discoverTargets read source directory %q: %w", sourceDir, err)
		}

		for _, entry := range entries {
			if !entry.IsDir {
				continue
			}

			folderPath := filepath.Join(sourceDir, entry.Name)
			isSuppressed, err := s.folders.IsSuppressedPath(ctx, folderPath)
			if err != nil {
				return nil, fmt.Errorf("scanner.discoverTargets check suppressed path %q: %w", folderPath, err)
			}
			if isSuppressed {
				continue
			}

			targets = append(targets, scanTarget{
				sourceDir:    sourceDir,
				folderPath:   folderPath,
				folderName:   entry.Name,
				relativePath: entry.Name,
			})
		}
	}

	return targets, nil
}

func (s *ScannerService) scanOne(ctx context.Context, jobID string, target scanTarget) (*repository.Folder, error) {
	childEntries, err := s.fs.ReadDir(ctx, target.folderPath)
	if err != nil {
		s.writeScanAudit(ctx, jobID, "", target.folderPath, target.sourceDir, target.relativePath, "failed", "", err)
		return nil, fmt.Errorf("scanner.scanOne read folder %q: %w", target.folderPath, err)
	}

	fileNames := make([]string, 0, len(childEntries))
	totalFiles := 0
	var totalSize int64

	for _, child := range childEntries {
		childPath := filepath.Join(target.folderPath, child.Name)
		info, err := s.fs.Stat(ctx, childPath)
		if err != nil {
			s.writeScanAudit(ctx, jobID, "", target.folderPath, target.sourceDir, target.relativePath, "failed", "", err)
			return nil, fmt.Errorf("scanner.scanOne stat %q: %w", childPath, err)
		}

		totalSize += info.Size
		if child.IsDir || info.IsDir {
			continue
		}

		fileNames = append(fileNames, child.Name)
		totalFiles++
	}

	imageCount, videoCount := countMediaFiles(fileNames)
	category := Classify(target.folderName, fileNames)
	now := time.Now().UTC()

	folder := &repository.Folder{
		ID:             deterministicFolderID(target.folderPath),
		Path:           target.folderPath,
		SourceDir:      target.sourceDir,
		RelativePath:   target.relativePath,
		Name:           target.folderName,
		Category:       category,
		CategorySource: "auto",
		Status:         "pending",
		ImageCount:     imageCount,
		VideoCount:     videoCount,
		TotalFiles:     totalFiles,
		TotalSize:      totalSize,
		MarkedForMove:  false,
		ScannedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.folders.Upsert(ctx, folder); err != nil {
		s.writeScanAudit(ctx, jobID, folder.ID, target.folderPath, target.sourceDir, target.relativePath, "failed", category, err)
		return nil, fmt.Errorf("scanner.scanOne upsert %q: %w", target.folderPath, err)
	}

	if err := s.recordClassificationSnapshot(ctx, jobID, folder); err != nil {
		return nil, err
	}

	s.writeScanAudit(ctx, jobID, folder.ID, folder.Path, folder.SourceDir, folder.RelativePath, "success", folder.Category, nil)
	return folder, nil
}

func (s *ScannerService) recordClassificationSnapshot(ctx context.Context, jobID string, folder *repository.Folder) error {
	if s.snapshots == nil || folder == nil {
		return nil
	}

	stateJSON, err := json.Marshal([]snapshotPathState{{
		OriginalPath: folder.Path,
		CurrentPath:  folder.Path,
	}})
	if err != nil {
		return fmt.Errorf("scanner.recordClassificationSnapshot marshal state for folder %q: %w", folder.ID, err)
	}

	snapshotID, err := s.snapshots.CreateBefore(ctx, jobID, folder.ID, "classify")
	if err != nil {
		return fmt.Errorf("scanner.recordClassificationSnapshot create snapshot for folder %q: %w", folder.ID, err)
	}

	detailJSON, err := json.Marshal(map[string]any{
		"source_dir":      folder.SourceDir,
		"relative_path":   folder.RelativePath,
		"category":        folder.Category,
		"category_source": folder.CategorySource,
		"total_files":     folder.TotalFiles,
		"image_count":     folder.ImageCount,
		"video_count":     folder.VideoCount,
	})
	if err != nil {
		return fmt.Errorf("scanner.recordClassificationSnapshot marshal detail for folder %q: %w", folder.ID, err)
	}

	if err := s.snapshots.UpdateDetail(ctx, snapshotID, detailJSON); err != nil {
		return fmt.Errorf("scanner.recordClassificationSnapshot update detail for folder %q: %w", folder.ID, err)
	}

	if err := s.snapshots.CommitAfter(ctx, snapshotID, stateJSON); err != nil {
		return fmt.Errorf("scanner.recordClassificationSnapshot commit snapshot for folder %q: %w", folder.ID, err)
	}

	return nil
}

func (s *ScannerService) writeScanAudit(ctx context.Context, jobID, folderID, folderPath, sourceDir, relativePath, result, category string, scanErr error) {
	if s.audit == nil {
		return
	}

	detail, err := json.Marshal(map[string]any{
		"source_dir":    sourceDir,
		"relative_path": relativePath,
		"category":      category,
	})
	if err != nil {
		detail = nil
	}

	logItem := &repository.AuditLog{
		ID:         fmt.Sprintf("audit-scan-%s-%d", folderID, time.Now().UTC().UnixNano()),
		JobID:      jobID,
		FolderID:   folderID,
		FolderPath: folderPath,
		Action:     "scan",
		Level:      "info",
		Detail:     detail,
		Result:     result,
	}
	if scanErr != nil {
		logItem.Level = "error"
		logItem.ErrorMsg = scanErr.Error()
	}

	_ = s.audit.Write(ctx, logItem)
}

func (s *ScannerService) publish(eventType string, payload any) {
	if s.broker == nil {
		return
	}

	_ = s.broker.Publish(eventType, payload)
}

func normalizeSourceDirs(sourceDirs []string) []string {
	seen := make(map[string]struct{}, len(sourceDirs))
	result := make([]string, 0, len(sourceDirs))
	for _, item := range sourceDirs {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func deterministicFolderID(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	h := sha1.Sum([]byte(absPath))
	return hex.EncodeToString(h[:])
}

func countMediaFiles(fileNames []string) (int, int) {
	imageCount := 0
	videoCount := 0

	for _, name := range fileNames {
		ext := strings.ToLower(filepath.Ext(name))
		if imageExts[ext] {
			imageCount++
		}

		if videoExts[ext] {
			videoCount++
		}
	}

	return imageCount, videoCount
}
