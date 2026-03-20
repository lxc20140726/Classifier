# 数据模型设计 v3.0

> 版本：v3.0 | 日期：2026-03-20
> **重大变更**：引入 Job-Workflow-Node 三层模型，节点粒度快照，软删除支持，配置版本化

## 核心模型关系

```
Job (顶层任务)
 ├── WorkflowRun (每个文件夹的工作流实例)
 │    ├── NodeRun (节点执行记录)
 │    │    └── NodeSnapshot (节点快照)
 │    └── AuditLog (审计日志)
 └── Folder (被处理的文件夹)
```

## Go Struct 定义

### Job 模型

```go
type JobStatus string
const (
    JobStatusPending   JobStatus = "pending"
    JobStatusRunning   JobStatus = "running"
    JobStatusSucceeded JobStatus = "succeeded"
    JobStatusFailed    JobStatus = "failed"
    JobStatusPartial   JobStatus = "partial"   // 部分成功
    JobStatusCancelled JobStatus = "cancelled"
)

type Job struct {
    ID         string    `json:"id" db:"id"`
    Type       string    `json:"type" db:"type"`           // "process_folders"
    Status     JobStatus `json:"status" db:"status"`
    FolderIDs  []string  `json:"folder_ids" db:"-"`
    FolderIDsJSON string `json:"-" db:"folder_ids"`       // JSON array
    Total      int       `json:"total" db:"total"`
    Done       int       `json:"done" db:"done"`
  Failed     int       `json:"failed" db:"failed"`
    Error      string    `json:"error,omitempty" db:"error"`
    StartedAt  *time.Time `json:"started_at,omitempty" db:"started_at"`
    FinishedAt *time.Time `json:"finished_at,omitempty" db:"finished_at"`
  CreatedAt  time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt  time.Time  `json:"updated_at" db:"updated_at"`
}
```

### WorkflowRun 模型

```go
type WorkflowRunStatus string
const (
    WorkflowRunPending   WorkflowRunStatus = "pending"
    WorkflowRunRunning   WorkflowRunStatus = "running"
    WorkflowRunSucceeded WorkflowRunStatus = "succeeded"
    WorkflowRunFailed    WorkflowRunStatus = "failed"
    WorkflowRunPartial   WorkflowRunStatus = "partial"  // 部分节点成功，已回退
)

type WorkflowRun struct {
    ID              string       `json:"id" db:"id"`
    JobID           string            `json:"job_id" db:"job_id"`
    FolderID        string            `json:"folder_id" db:"folder_id"`
    WorkflowDefID   string            `json:"workflow_def_id" db:"workflow_def_id"`
    Status      WorkflowRunStatus `json:"status" db:"status"`
    ResumeNodeID    string        `json:"resume_node_id,omitempty" db:"resume_node_id"`  // 断点续传
    LastNodeID      string            `json:"last_node_id,omitempty" db:"last_node_id"`      // 最后成功节点
    Error           string            `json:"error,omitempty" db:"error"`
    StartedAt       *time.Time        `json:"started_at,omitempty" db:"started_at"`
    FinishedAt      *time.Time        `json:"finished_at,omitempty" db:"finished_at"`
    CreatedAt       time.Time         `json:"created_at" db:"created_at"`
    UpdatedAt       time.Time         `json:"updated_at" db:"updated_at"`
}
```

### NodeRun 模型

```go
type NodeRunStatus string
const (
    NodeRunPending    NodeRunStatus = "pending"
    NodeRunRunning    NodeRunStatus = "running"
    NodeRunSucceeded  NodeRunStatus = "succeeded"
    NodeRunFailed     NodeRunStatus = "failed"
    NodeRunSkipped    NodeRunStatus = "skipped"
    NodeRunRolledBack NodeRunStatus = "rolled_back"
)

type NodeRun struct {
    ID           string        `json:"id" db:"id"`
    WorkflowRunID  string        `json:"workflow_run_id" db:"workflow_run_id"`
    NodeDefID      string        `json:"node_def_id" db:"node_def_id"`        // 节点定义 ID
    NodeType    string        `json:"node_type" db:"node_type"`            // classifier|rename|compress|thumbnail|move
    Sequence       int           `json:"sequence" db:"sequence"`              // 执行顺序
  Status         NodeRunStatus `json:"status" db:"status"`
    Attempt        int           `json:"attempt" db:"attempt"`
    InputJSON      string        `json:"input_json,omitempty" db:"input_json"`
    OutputJSON     string    `json:"output_json,omitempty" db:"output_json"`
    Error          string        `json:"error,omitempty" db:"error"`
    RollbackStatus string        `json:"rollback_status,omitempty" db:"rollback_status"` // not_needed|pending|succeeded|failed
    StartedAt      *time.Time    `json:"started_at,omitempty" db:"started_at"`
    FinishedAt     *time.Time    `json:"finished_at,omitempty" db:"finished_at"`
    CreatedAt      time.Time     `json:"created_at" db:"created_at"`
}
```

### NodeSnapshot 模型

```go
type SnapshotKind string
const (
    SnapshotPre       SnapshotKind = "pre"           // 节点执行前
    SnapshotPost         SnapshotKind = "post"          // 节点执行后
    SnapshotRollbackBase SnapshotKind = "rollback_base" // 回退基准
)

type NodeSnapshot struct {
    ID             string       `json:"id" db:"id"`
    NodeRunID      string       `json:"node_run_id" db:"node_run_id"`
    WorkflowRunID  string       `json:"workflow_run_id" db:"workflow_run_id"`
    Kind           SnapshotKind `json:"kind" db:"kind"`
    FSManifest     string       `json:"fs_manifest" db:"fs_manifest"`         // JSON: 文件系统状态
    OutputJSON     string       `json:"output_json,omitempty" db:"output_json"` // 节点输出
    Compensation   string       `json:"compensation,omitempty" db:"compensation"` // 回退操作描述
    CreatedAt      time.Time    `json:"created_at" db:"created_at"`
}

// FSManifest 结构
type FSManifest struct {
    Files   []FileEntry   `json:"files"`
    Folders []FolderEntry `json:"folders"`
}

type FileEntry struct {
    Path  string    `json:"path"`
    Size  int64     `json:"size"`
    MTime time.Time `json:"mtime"`
}

type FolderEntry struct {
    Path string `json:"path"`
}
```

### Folder 模型（扩展）

```go
type Folder struct {
    ID            string     `json:"id" db:"id"`
    Path              string     `json:"path" db:"path"`
    Name         string     `json:"name" db:"name"`
    Category          string     `json:"category" db:"category"`
    CategorySource    string     `json:"category_source" db:"category_source"`
    Status          string     `json:"status" db:"status"`
    ImageCount        int        `json:"image_count" db:"image_count"`
    VideoCount        int        `json:"video_count" db:"video_count"`
    TotalFiles        int        `json:"total_files" db:"total_files"`
    TotalSize         int64      `json:"total_size" db:"total_size"`
    MarkedForMove     bool       `json:"marked_for_move" db:"marked_for_move"`
    DeletedAt      *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`              // 软删除
    DeleteStagingPath string     `json:"delete_staging_path,omitempty" db:"delete_staging_path"` // 软删除暂存路径
    ScannedAt         time.Time  `json:"scanned_at" db:"scanned_at"`
    UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}
```

### WorkflowDefinition 模型

```go
type WorkflowDefinition struct {
    ID       string    `json:"id" db:"id"`
    Name     string    `json:"name" db:"name"`
    Description string    `json:"description" db:"description"`
    GraphJSON   string    `json:"graph_json" db:"graph_json"` // React Flow 节点图
    Version     int       `json:"version" db:"version"`
    IsActive    bool      `json:"is_active" db:"is_active"`
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
```

### AppConfig 模型（版本化）

```go
type AppConfig struct {
    Version  int            `json:"version"`
    Server   ServerConfig   `json:"server" validate:"required"`
    Scanner  ScannerConfig  `json:"scanner" validate:"required"`
    Workflow WorkflowConfig `json:"workflow" validate:"required"`
    Storage  StorageConfig  `json:"storage" validate:"required"`
}

type ServerConfig struct {
    Port       int `json:"port" validate:"min=1,max=65535" default:"8080"`
    MaxConcurrency int `json:"max_concurrency" validate:"min=1,max=100" default:"4"`
    SSEBufferSize  int `json:"sse_buffer_size" validate:"min=1" default:"16"`
}

type ScannerConfig struct {
    AutoScanOnStart bool     `json:"auto_scan_on_start" default:"false"`
    ScanInterval    int      `json:"scan_interval_minutes" validate:"min=0" default:"0"`
    ExcludePatterns []string `json:"exclude_patterns" default:"[\".DS_Store\",\"Thumbs.db\"]"`
}

type WorkflowConfig struct {
    DefaultPhotoWorkflow string               `json:"default_photo_workflow"`
    DefaultVideoWorkflow string       `json:"default_video_workflow"`
    DefaultMixedWorkflow string               `json:"default_mixed_workflow"`
    DefaultMangaWorkflow string             `json:"default_manga_workflow"`
    ClassifierThresholds ClassifierThresholds `json:"classifier_thresholds"`
}

type ClassifierThresholds struct {
    PhotoRatio float64 `json:"photo_ratio" validate:"min=0,max=1" default:"0.85"`
    VideoRatio float64 `json:"video_ratio" validate:"min=0,max=1" default:"0.85"`
}

type StorageConfig struct {
    SourceDir           string `json:"source_dir" validate:"required"`
    TargetDir             string `json:"target_dir" validate:"required"`
    DeleteStagingDir      string `json:"delete_staging_dir" validate:"required"`
    SnapshotRetentionDays int    `json:"snapshot_retention_days" validate:"min=1" default:"30"`
    AuditRetentionDays    int    `json:"audit_retention_days" validate:"min=1" default:"90"`
}
```

### AuditLog 模型（保持不变）

```go
type AuditLog struct {
    ID         string    `json:"id" db:"id"`
    JobID      string    `json:"job_id" db:"job_id"`
    FolderID   string    `json:"folder_id" db:"folder_id"`
    FolderPath string    `json:"folder_path" db:"folder_path"`
    Action     string    `json:"action" db:"action"`
    Level      string    `json:"level" db:"level"`
    Detail     string    `json:"detail,omitempty" db:"detail"`
    Result     string    `json:"result" db:"result"`
    ErrorMsg   string    `json:"error_msg,omitempty" db:"error_msg"`
    DurationMs int64     `json:"duration_ms" db:"duration_ms"`
    CreatedAt  time.Time `json:"created_at" db:"created_at"`
}
```

## SQLite Schema

```sql
-- Jobs 表
CREATE TABLE IF NOT EXISTS jobs (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    folder_ids  TEXT NOT NULL,          -- JSON array
    total       INTEGER NOT NULL DEFAULT 0,
    done        INTEGER NOT NULL DEFAULT 0,
    failed      INTEGER NOT NULL DEFAULT 0,
    error       TEXT,
    started_at  TEXT,
    finished_at TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- WorkflowRuns 表
CREATE TABLE IF NOT EXISTS workflow_runs (
    id            TEXT PRIMARY KEY,
    job_id            TEXT NOT NULL,
    folder_id         TEXT NOT NULL,
    workflow_def_id   TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'pending',
    resume_node_id    TEXT,
    last_node_id      TEXT,
    error             TEXT,
    started_at        TEXT,
    finished_at       TEXT,
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (job_id) REFERENCES jobs(id),
    FOREIGN KEY (folder_id) REFERENCES folders(id),
    FOREIGN KEY (workflow_def_id) REFERENCES workflow_definitions(id)
);

-- NodeRuns 表
CREATE TABLE IF NOT EXISTS node_runs (
    id              TEXT PRIMARY KEY,
    workflow_run_id TEXT NOT NULL,
    node_def_id     TEXT NOT NULL,
    node_type       TEXT NOT NULL,
    sequence        INTEGER NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    attempt      INTEGER NOT NULL DEFAULT 1,
    input_json      TEXT,
    output_json     TEXT,
    error        TEXT,
    rollback_status TEXT,
    started_at      TEXT,
    finished_at     TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (workflow_run_id) REFERENCES workflow_runs(id)
);

-- NodeSnapshots 表
CREATE TABLE IF NOT EXISTS node_snapshots (
    id              TEXT PRIMARY KEY,
    node_run_id     TEXT NOT NULL,
    workflow_run_id TEXT NOT NULL,
    kind            TEXT NOT NULL,
    fs_manifest     TEXT NOT NULL,
    output_json     TEXT,
    compensation    TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (node_run_id) REFERENCES node_runs(id),
    FOREIGN KEY (workflow_run_id) REFERENCES workflow_runs(id)
);

-- Folders 表（扩展软删除）
CREATE TABLE IF NOT EXISTS folders (
    id               TEXT PRIMARY KEY,
    path           TEXT NOT NULL,
    name           TEXT NOT NULL,
    category            TEXT NOT NULL DEFAULT 'unknown',
    category_source     TEXT NOT NULL DEFAULT 'auto',
    status              TEXT NOT NULL DEFAULT 'pending',
    image_count         INTEGER NOT NULL DEFAULT 0,
  video_count         INTEGER NOT NULL DEFAULT 0,
    total_files         INTEGER NOT NULL DEFAULT 0,
    total_size          INTEGER NOT NULL DEFAULT 0,
    marked_for_move     INTEGER NOT NULL DEFAULT 0,
    deleted_at          TEXT,
    delete_staging_path TEXT,
    scanned_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);

-- WorkflowDefinitions 表
CREATE TABLE IF NOT EXISTS workflow_definitions (
    id       TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    graph_json  TEXT NOT NULL,
    version     INTEGER NOT NULL DEFAULT 1,
    is_active   INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- AppConfig 表（版本化配置）
CREATE TABLE IF NOT EXISTS app_config (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    version   INTEGER NOT NULL,
    value       TEXT NOT NULL,
    checksum    TEXT NOT NULL,
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- AuditLogs 表（保持不变）
CREATE TABLE IF NOT EXISTS audit_logs (
    id          TEXT PRIMARY KEY,
    job_id      TEXT,
    folder_id   TEXT,
    folder_path TEXT NOT NULL,
    action      TEXT NOT NULL,
    level       TEXT NOT NULL DEFAULT 'info',
    detail      TEXT,
    result      TEXT NOT NULL,
    error_msg   TEXT,
    duration_ms INTEGER,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_created ON jobs(created_at);

CREATE INDEX IF NOT EXISTS idx_workflow_runs_job ON workflow_runs(job_id);
CREATE INDEX IF NOT EXISTS idx_workflow_runs_folder ON workflow_runs(folder_id);
CREATE INDEX IF NOT EXISTS idx_workflow_runs_status ON workflow_runs(status);

CREATE INDEX IF NOT EXISTS idx_node_runs_workflow ON node_runs(workflow_run_id);
CREATE INDEX IF NOT EXISTS idx_node_runs_sequence ON node_runs(workflow_run_id, sequence);

CREATE INDEX IF NOT EXISTS idx_node_snapshots_node ON node_snapshots(node_run_id);
CREATE INDEX IF NOT EXISTS idx_node_snapshots_workflow ON node_snapshots(workflow_run_id);

CREATE INDEX IF NOT EXISTS idx_folders_status ON folders(status);
CREATE INDEX IF NOT EXISTS idx_folders_category ON folders(category);
CREATE INDEX IF NOT EXISTS idx_folders_active ON folders(id) WHERE deleted_at IS NULL;

-- 唯一约束（只约束未删除记录）
CREATE UNIQUE INDEX IF NOT EXISTS ux_folders_path_active 
ON folders(path) 
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_audit_folder ON audit_logs(folder_id);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at);
```

## 迁移策略

从 v2.0 迁移到 v3.0 需要执行以下步骤：

1. 创建新表：`jobs`, `workflow_runs`, `node_runs`, `node_snapshots`, `workflow_definitions`, `app_config`
2. 扩展 `folders` 表：添加 `deleted_at`, `delete_staging_path` 字段
3. 迁移现有 `snapshots` 数据到 `node_snapshots`（如果需要保留历史）
4. 删除旧的 `config` 表，迁移数据到 `app_config`
5. 更新所有外键引用

详见 `backend/internal/db/migrations/002_v3_schema.sql`
