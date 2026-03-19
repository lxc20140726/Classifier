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
	values, err := h.config.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": values})
}

func (h *ConfigHandler) Put(c *gin.Context) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	for key, value := range payload {
		stringValue, ok := value.(string)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "all config values must be strings"})
			return
		}

		if err := h.config.Set(c.Request.Context(), key, stringValue); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"saved": true})
}
