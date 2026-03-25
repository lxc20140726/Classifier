package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/liqiye/classifier/internal/repository"
)

type ConfigHandler struct {
	config repository.ConfigRepository
}

func NewConfigHandler(configRepo repository.ConfigRepository) *ConfigHandler {
	return &ConfigHandler{config: configRepo}
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

	if err := h.config.SaveAppConfig(c.Request.Context(), &payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}

	stored, err := h.config.GetAppConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load saved config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"saved": true, "data": stored})
}
