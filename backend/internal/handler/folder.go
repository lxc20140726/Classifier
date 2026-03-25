package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	internalfs "github.com/liqiye/classifier/internal/fs"
	"github.com/liqiye/classifier/internal/repository"
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

type FolderScanStarter interface {
	StartJob(ctx context.Context, sourceDirs []string) (string, error)
}

type FolderHandler struct {
	folders          repository.FolderRepository
	config           repository.ConfigRepository
	scanStarter      FolderScanStarter
	fs               internalfs.FSAdapter
	defaultSourceDir string
	deleteStagingDir string
}

func NewFolderHandler(folderRepo repository.FolderRepository, configRepo repository.ConfigRepository, scanStarter FolderScanStarter, fsAdapter internalfs.FSAdapter, sourceDir, deleteStagingDir string) *FolderHandler {
	return &FolderHandler{
		folders:          folderRepo,
		config:           configRepo,
		scanStarter:      scanStarter,
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

	if h.scanStarter == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "scan starter not configured"})
		return
	}

	jobID, err := h.scanStarter.StartJob(c.Request.Context(), sourceDirs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create scan job"})
		return
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
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"suppressed": true}})
		return
	}

	err = h.folders.Suppress(c.Request.Context(), id, "", "")
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "folder not found"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to suppress folder record"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"suppressed": true}})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "folder is not suppressed"})
		return
	}
	if err := h.folders.Unsuppress(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to restore suppressed folder"})
		return
	}
	restored, err := h.folders.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get folder"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": restored})
}
