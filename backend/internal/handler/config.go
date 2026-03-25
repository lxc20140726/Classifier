package handler

import (
	"context"
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
	var payload repository.AppConfig
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if payload.ScanCron != "" {
		if _, err := cron.ParseStandard(payload.ScanCron); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scan_cron"})
			return
		}
	}

	if err := h.config.SaveAppConfig(c.Request.Context(), &payload); err != nil {
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
