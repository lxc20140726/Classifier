package repository

import (
	"context"
	"encoding/json"
)

type FolderRepository interface {
	Upsert(ctx context.Context, f *Folder) error
	GetByID(ctx context.Context, id string) (*Folder, error)
	GetByPath(ctx context.Context, path string) (*Folder, error)
	List(ctx context.Context, filter FolderListFilter) ([]*Folder, int, error)
	UpdateCategory(ctx context.Context, id, category, source string) error
	UpdateStatus(ctx context.Context, id, status string) error
	UpdatePath(ctx context.Context, id, newPath string) error
	Delete(ctx context.Context, id string) error
}

type SnapshotRepository interface {
	Create(ctx context.Context, s *Snapshot) error
	GetByID(ctx context.Context, id string) (*Snapshot, error)
	ListByFolderID(ctx context.Context, folderID string) ([]*Snapshot, error)
	ListByJobID(ctx context.Context, jobID string) ([]*Snapshot, error)
	CommitAfter(ctx context.Context, id string, after json.RawMessage) error
	UpdateStatus(ctx context.Context, id, status string) error
}

type AuditRepository interface {
	Write(ctx context.Context, log *AuditLog) error
	List(ctx context.Context, filter AuditListFilter) ([]*AuditLog, int, error)
	GetByID(ctx context.Context, id string) (*AuditLog, error)
}

type ConfigRepository interface {
	Set(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) (string, error)
	GetAll(ctx context.Context) (map[string]string, error)
}
