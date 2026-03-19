package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
)

type ScannerService struct {
	fs      fs.FSAdapter
	folders repository.FolderRepository
}

func NewScannerService(fsAdapter fs.FSAdapter, folderRepo repository.FolderRepository) *ScannerService {
	return &ScannerService{fs: fsAdapter, folders: folderRepo}
}

func (s *ScannerService) Scan(ctx context.Context, sourceDir string) (int, error) {
	entries, err := s.fs.ReadDir(ctx, sourceDir)
	if err != nil {
		return 0, fmt.Errorf("scanner.Scan read source directory %q: %w", sourceDir, err)
	}

	processed := 0
	for _, entry := range entries {
		if !entry.IsDir {
			continue
		}

		folderPath := filepath.Join(sourceDir, entry.Name)
		childEntries, err := s.fs.ReadDir(ctx, folderPath)
		if err != nil {
			return processed, fmt.Errorf("scanner.Scan read folder %q: %w", folderPath, err)
		}

		fileNames := make([]string, 0, len(childEntries))
		totalFiles := 0
		var totalSize int64

		for _, child := range childEntries {
			childPath := filepath.Join(folderPath, child.Name)
			info, err := s.fs.Stat(ctx, childPath)
			if err != nil {
				return processed, fmt.Errorf("scanner.Scan stat %q: %w", childPath, err)
			}

			totalSize += info.Size

			if child.IsDir || info.IsDir {
				continue
			}

			fileNames = append(fileNames, child.Name)
			totalFiles++
		}

		imageCount, videoCount := countMediaFiles(fileNames)
		category := Classify(entry.Name, fileNames)
		now := time.Now().UTC()

		folder := &repository.Folder{
			ID:             deterministicFolderID(folderPath),
			Path:           folderPath,
			Name:           entry.Name,
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
			return processed, fmt.Errorf("scanner.Scan upsert %q: %w", folderPath, err)
		}

		processed++
	}

	return processed, nil
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
