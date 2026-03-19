package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/liqiye/classifier/internal/repository"
)

var errNilAuditLog = errors.New("audit log is nil")

type AuditService struct {
	repo repository.AuditRepository
}

func NewAuditService(repo repository.AuditRepository) *AuditService {
	return &AuditService{repo: repo}
}

func (s *AuditService) Write(ctx context.Context, log *repository.AuditLog) error {
	if log == nil {
		return errNilAuditLog
	}

	if log.ID == "" {
		log.ID = uuid.NewString()
	}

	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}

	if log.Level == "" {
		log.Level = "info"
	}

	return s.repo.Write(ctx, log)
}

func (s *AuditService) List(ctx context.Context, filter repository.AuditListFilter) ([]*repository.AuditLog, int, error) {
	return s.repo.List(ctx, filter)
}
