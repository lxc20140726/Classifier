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
	UpdateCoverImagePath(ctx context.Context, id, coverImagePath string) error
	IsSuppressedPath(ctx context.Context, path string) (bool, error)
	Suppress(ctx context.Context, id, currentPath, originalPath string) error
	Unsuppress(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

type JobRepository interface {
	Create(ctx context.Context, job *Job) error
	GetByID(ctx context.Context, id string) (*Job, error)
	List(ctx context.Context, filter JobListFilter) ([]*Job, int, error)
	UpdateTotal(ctx context.Context, id string, total int) error
	UpdateStatus(ctx context.Context, id, status, errMsg string) error
	IncrementProgress(ctx context.Context, id string, successDelta, failedDelta int) error
}

type WorkflowDefinitionRepository interface {
	Create(ctx context.Context, item *WorkflowDefinition) error
	GetByID(ctx context.Context, id string) (*WorkflowDefinition, error)
	List(ctx context.Context, filter WorkflowDefListFilter) ([]*WorkflowDefinition, int, error)
	Update(ctx context.Context, item *WorkflowDefinition) error
	Delete(ctx context.Context, id string) error
}

type WorkflowRunRepository interface {
	Create(ctx context.Context, item *WorkflowRun) error
	GetByID(ctx context.Context, id string) (*WorkflowRun, error)
	List(ctx context.Context, filter WorkflowRunListFilter) ([]*WorkflowRun, int, error)
	UpdateStatus(ctx context.Context, id, status, resumeNodeID string) error
	UpdateBlocks(ctx context.Context, id string, delta int) error
}

type NodeRunRepository interface {
	Create(ctx context.Context, item *NodeRun) error
	GetByID(ctx context.Context, id string) (*NodeRun, error)
	List(ctx context.Context, filter NodeRunListFilter) ([]*NodeRun, int, error)
	GetLatestByNodeID(ctx context.Context, workflowRunID, nodeID string) (*NodeRun, error)
	UpdateStart(ctx context.Context, id, inputJSON string) error
	UpdateFinish(ctx context.Context, id, status, outputJSON, errMsg string) error
	SetResumeToken(ctx context.Context, nodeRunID, token string) error
}

type NodeSnapshotRepository interface {
	Create(ctx context.Context, item *NodeSnapshot) error
	ListByNodeRunID(ctx context.Context, nodeRunID string) ([]*NodeSnapshot, error)
}

type SnapshotRepository interface {
	Create(ctx context.Context, s *Snapshot) error
	GetByID(ctx context.Context, id string) (*Snapshot, error)
	ListByFolderID(ctx context.Context, folderID string) ([]*Snapshot, error)
	ListByJobID(ctx context.Context, jobID string) ([]*Snapshot, error)
	CommitAfter(ctx context.Context, id string, after json.RawMessage) error
	UpdateDetail(ctx context.Context, id string, detail json.RawMessage) error
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
	GetAppConfig(ctx context.Context) (*AppConfig, error)
	SaveAppConfig(ctx context.Context, value *AppConfig) error
	EnsureAppConfig(ctx context.Context) error
}
