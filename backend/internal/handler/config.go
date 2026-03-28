package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/liqiye/classifier/internal/repository"
	"github.com/robfig/cron/v3"
)

type ConfigSyncer interface {
	Sync(ctx context.Context) error
}

type ConfigHandler struct {
	config repository.ConfigRepository
	syncer ConfigSyncer
}

type appConfigOutputDirsPatch struct {
	Video *string `json:"video"`
	Manga *string `json:"manga"`
	Photo *string `json:"photo"`
	Other *string `json:"other"`
	Mixed *string `json:"mixed"`
}

type appConfigPatchRequest struct {
	Version       *int                      `json:"version"`
	ScanInputDirs *[]string                 `json:"scan_input_dirs"`
	ScanCron      *string                   `json:"scan_cron"`
	SourceDir     *string                   `json:"source_dir"`
	TargetDir     *string                   `json:"target_dir"`
	TargetDirs    *[]string                 `json:"target_dirs"`
	OutputDirs    *appConfigOutputDirsPatch `json:"output_dirs"`
}

func NewConfigHandler(configRepo repository.ConfigRepository, syncer ConfigSyncer) *ConfigHandler {
	return &ConfigHandler{config: configRepo, syncer: syncer}
}

func (h *ConfigHandler) Get(c *gin.Context) {
	value, err := h.config.GetAppConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": value})
}

func (h *ConfigHandler) Put(c *gin.Context) {
	var patch appConfigPatchRequest
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	existing, err := h.config.GetAppConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load current config"})
		return
	}

	payload := *existing
	applyAppConfigPatch(&payload, patch)

	if payload.ScanCron != "" {
		if _, err := cron.ParseStandard(payload.ScanCron); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scan_cron"})
			return
		}
	}

	if err := h.config.SaveAppConfig(c.Request.Context(), &payload); err != nil {
		if errors.Is(err, repository.ErrInvalidConfig) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}
	if h.syncer != nil {
		if err := h.syncer.Sync(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to apply config"})
			return
		}
	}

	stored, err := h.config.GetAppConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load saved config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"saved": true, "data": stored})
}

func applyAppConfigPatch(target *repository.AppConfig, patch appConfigPatchRequest) {
	if target == nil {
		return
	}

	if patch.Version != nil {
		target.Version = *patch.Version
	}
	if patch.ScanInputDirs != nil {
		target.ScanInputDirs = *patch.ScanInputDirs
	}
	if patch.ScanCron != nil {
		target.ScanCron = *patch.ScanCron
	}
	if patch.SourceDir != nil {
		target.SourceDir = *patch.SourceDir
	}
	if patch.TargetDir != nil {
		target.TargetDir = *patch.TargetDir
	}
	if patch.TargetDirs != nil {
		target.TargetDirs = *patch.TargetDirs
	}
	if patch.OutputDirs != nil {
		if patch.OutputDirs.Video != nil {
			target.OutputDirs.Video = *patch.OutputDirs.Video
		}
		if patch.OutputDirs.Manga != nil {
			target.OutputDirs.Manga = *patch.OutputDirs.Manga
		}
		if patch.OutputDirs.Photo != nil {
			target.OutputDirs.Photo = *patch.OutputDirs.Photo
		}
		if patch.OutputDirs.Other != nil {
			target.OutputDirs.Other = *patch.OutputDirs.Other
		}
		if patch.OutputDirs.Mixed != nil {
			target.OutputDirs.Mixed = *patch.OutputDirs.Mixed
		}
	}
}
