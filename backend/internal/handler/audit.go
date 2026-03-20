package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/liqiye/classifier/internal/repository"
)

type AuditHandler struct {
	audit repository.AuditRepository
}

func NewAuditHandler(auditRepo repository.AuditRepository) *AuditHandler {
	return &AuditHandler{audit: auditRepo}
}

func (h *AuditHandler) List(c *gin.Context) {
	page := 1
	if rawPage := c.Query("page"); rawPage != "" {
		parsedPage, err := strconv.Atoi(rawPage)
		if err != nil || parsedPage <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page"})
			return
		}
		page = parsedPage
	}

	limit := 50
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = parsedLimit
	}

	items, total, err := h.audit.List(c.Request.Context(), repository.AuditListFilter{
		JobID:    c.Query("job_id"),
		Action:   c.Query("action"),
		Result:   c.Query("result"),
		FolderID: c.Query("folder_id"),
		Page:     page,
		Limit:    limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list audit logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": items, "total": total, "page": page, "limit": limit})
}
