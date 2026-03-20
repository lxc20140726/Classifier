package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	internalfs "github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
	"github.com/liqiye/classifier/internal/service"
)

var validCategories = map[string]struct{}{
	"photo": {},
	"video": {},
	"mixed": {},
	"manga": {},
	"other": {},
}

var validStatuses = map[string]struct{}{
	"pending": {},
	"done":    {},
	"skip":    {},
}

type FolderScanService interface {
	Scan(ctx context.Context, input service.ScanInput) (int, error)
}

type FolderHandler struct {
	folders          repository.FolderRepository
	jobs             repository.JobRepository
	config           repository.ConfigRepository
	scanner          FolderScanService
	fs               internalfs.FSAdapter
	defaultSourceDir string
	deleteStagingDir string
}

func NewFolderHandler(folderRepo repository.FolderRepository, jobRepo repository.JobRepository, configRepo repository.ConfigRepository, scanner FolderScanService, fsAdapter internalfs.FSAdapter, sourceDir, deleteStagingDir string) *FolderHandler {
	return &FolderHandler{
		folders:          folderRepo,
		jobs:             jobRepo,
		config:           configRepo,
		scanner:          scanner,
		fs:               fsAdapter,
		defaultSourceDir: sourceDir,
		deleteStagingDir: deleteStagingDir,
	}
}

func (h *FolderHandler) List(c *gin.Context) {
	page := 1
	if rawPage := c.Query("page"); rawPage != "" {
		parsedPage, err := strconv.Atoi(rawPage)
		if err != nil || parsedPage <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page"})
			return
		}
		page = parsedPage
	}

	limit := 20
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = parsedLimit
	}

	filter := repository.FolderListFilter{
		Status:         c.Query("status"),
		Category:       c.Query("category"),
		Q:              c.Query("q"),
		Page:           page,
		Limit:          limit,
		IncludeDeleted: c.Query("include_deleted") == "true",
		OnlyDeleted:    c.Query("only_deleted") == "true",
	}

	items, total, err := h.folders.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list folders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  items,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *FolderHandler) Get(c *gin.Context) {
	id := c.Param("id")

	folder, err := h.folders.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "folder not found"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get folder"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": folder})
}

func (h *FolderHandler) Scan(c *gin.Context) {
	sourceDirs, err := h.loadScanSourceDirs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(sourceDirs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no scan directories configured"})
		return
	}

	jobID := uuid.NewString()
	if h.jobs != nil {
		folderIDsJSON, marshalErr := json.Marshal([]string{})
		if marshalErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode folder_ids"})
			return
		}
		if err := h.jobs.Create(c.Request.Context(), &repository.Job{
			ID:        jobID,
			Type:      "scan",
			Status:    "pending",
			FolderIDs: string(folderIDsJSON),
			Total:     0,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create scan job"})
			return
		}
	}

	if h.scanner != nil {
		go func(dirs []string, currentJobID string) {
			_, _ = h.scanner.Scan(context.Background(), service.ScanInput{
				JobID:      currentJobID,
				SourceDirs: dirs,
			})
		}(append([]string(nil), sourceDirs...), jobID)
	}

	c.JSON(http.StatusAccepted, gin.H{"started": true, "job_id": jobID, "source_dirs": sourceDirs})
}

func (h *FolderHandler) loadScanSourceDirs(ctx context.Context) ([]string, error) {
	if h.config != nil {
		if raw, err := h.config.Get(ctx, "scan_input_dirs"); err == nil {
			var dirs []string
			if unmarshalErr := json.Unmarshal([]byte(raw), &dirs); unmarshalErr != nil {
				return nil, errors.New("invalid config value for scan_input_dirs")
			}
			if len(dirs) > 0 {
				return dirs, nil
			}
		} else if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}

		if raw, err := h.config.Get(ctx, "source_dir"); err == nil && raw != "" {
			return []string{raw}, nil
		} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
	}

	if h.defaultSourceDir == "" {
		return nil, nil
	}
	return []string{h.defaultSourceDir}, nil
}

func (h *FolderHandler) UpdateCategory(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Category string `json:"category"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if _, ok := validCategories[req.Category]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category"})
		return
	}

	err := h.folders.UpdateCategory(c.Request.Context(), id, req.Category, "manual")
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "folder not found"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update category"})
		return
	}

	folder, err := h.folders.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "folder not found"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get folder"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": folder})
}

func (h *FolderHandler) UpdateStatus(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Status string `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if _, ok := validStatuses[req.Status]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}

	err := h.folders.UpdateStatus(c.Request.Context(), id, req.Status)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "folder not found"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update status"})
		return
	}

	folder, err := h.folders.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "folder not found"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get folder"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": folder})
}

func (h *FolderHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	folder, err := h.folders.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "folder not found"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get folder"})
		return
	}

	if folder.DeletedAt != nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": true}})
		return
	}

	stagingPath := filepath.Join(h.deleteStagingDir, folder.ID+"-"+folder.Name)
	if h.fs != nil {
		if err := h.fs.MkdirAll(c.Request.Context(), h.deleteStagingDir, 0o755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare delete staging dir"})
			return
		}
		if err := h.fs.MoveDir(c.Request.Context(), folder.Path, stagingPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to move folder to delete staging"})
			return
		}
	}

	err = h.folders.SoftDelete(c.Request.Context(), id, stagingPath, folder.Path)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "folder not found"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete folder"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": true}})
}

func (h *FolderHandler) Restore(c *gin.Context) {
	id := c.Param("id")
	folder, err := h.folders.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "folder not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get folder"})
		return
	}
	if folder.DeletedAt == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "folder is not deleted"})
		return
	}
	if h.fs != nil && folder.DeleteStagingPath != "" {
		if err := h.fs.MoveDir(c.Request.Context(), folder.Path, folder.DeleteStagingPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to restore folder"})
			return
		}
	}
	if err := h.folders.Restore(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to restore folder"})
		return
	}
	restored, err := h.folders.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get folder"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": restored})
}
