package repository

import (
	"encoding/json"
	"time"
)

type Folder struct {
	ID                string     `db:"id"`
	Path              string     `db:"path"`
	Name              string     `db:"name"`
	Category          string     `db:"category"`
	CategorySource    string     `db:"category_source"`
	Status            string     `db:"status"`
	ImageCount        int        `db:"image_count"`
	VideoCount        int        `db:"video_count"`
	TotalFiles        int        `db:"total_files"`
	TotalSize         int64      `db:"total_size"`
	MarkedForMove     bool       `db:"marked_for_move"`
	DeletedAt         *time.Time `db:"deleted_at"`
	DeleteStagingPath string     `db:"delete_staging_path"`
	ScannedAt         time.Time  `db:"scanned_at"`
	UpdatedAt         time.Time  `db:"updated_at"`
}

type Job struct {
	ID         string     `db:"id"`
	Type       string     `db:"type"`
	Status     string     `db:"status"`
	FolderIDs  string     `db:"folder_ids"`
	Total      int        `db:"total"`
	Done       int        `db:"done"`
	Failed     int        `db:"failed"`
	Error      string     `db:"error"`
	StartedAt  *time.Time `db:"started_at"`
	FinishedAt *time.Time `db:"finished_at"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
}

type Snapshot struct {
	ID            string          `db:"id"`
	JobID         string          `db:"job_id"`
	FolderID      string          `db:"folder_id"`
	OperationType string          `db:"operation_type"`
	Before        json.RawMessage `db:"before_state"`
	After         json.RawMessage `db:"after_state"`
	Status        string          `db:"status"`
	CreatedAt     time.Time       `db:"created_at"`
}

type AuditLog struct {
	ID         string          `db:"id"`
	JobID      string          `db:"job_id"`
	FolderID   string          `db:"folder_id"`
	FolderPath string          `db:"folder_path"`
	Action     string          `db:"action"`
	Level      string          `db:"level"`
	Detail     json.RawMessage `db:"detail"`
	Result     string          `db:"result"`
	ErrorMsg   string          `db:"error_msg"`
	DurationMs int64           `db:"duration_ms"`
	CreatedAt  time.Time       `db:"created_at"`
}

type FolderListFilter struct {
	Status         string
	Category       string
	Q              string
	Page           int
	Limit          int
	IncludeDeleted bool
	OnlyDeleted    bool
}

type JobListFilter struct {
	Status string
	Page   int
	Limit  int
}

type AuditListFilter struct {
	Action   string
	Result   string
	FolderID string
	From     time.Time
	To       time.Time
	Page     int
	Limit    int
}
