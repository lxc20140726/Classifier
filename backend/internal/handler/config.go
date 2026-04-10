package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
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
	Version         *int                      `json:"version"`
	ScanInputDirs   *[]string                 `json:"scan_input_dirs"`
	ScanCron        *string                   `json:"scan_cron"`
	OutputDirs      *appConfigOutputDirsPatch `json:"output_dirs"`
	SourceDir       *json.RawMessage          `json:"source_dir"`
	TargetDir       *json.RawMessage          `json:"target_dir"`
	TargetDirs      *json.RawMessage          `json:"target_dirs"`
	PathOptions     *json.RawMessage          `json:"path_options"`
	DeprecatedField map[string]json.RawMessage
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
	logDeprecatedConfigFields(patch)

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

func logDeprecatedConfigFields(patch appConfigPatchRequest) {
	fields := make([]string, 0, 4)
	if patch.SourceDir != nil {
		fields = append(fields, "source_dir")
	}
	if patch.TargetDir != nil {
		fields = append(fields, "target_dir")
	}
	if patch.TargetDirs != nil {
		fields = append(fields, "target_dirs")
	}
	if patch.PathOptions != nil {
		fields = append(fields, "path_options")
	}
	if len(fields) == 0 {
		return
	}
	log.Printf("config handler received deprecated fields (ignored): %v", fields)
}
