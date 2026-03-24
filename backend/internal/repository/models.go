package repository

import (
	"encoding/json"
	"time"
)

type Folder struct {
	ID                string     `db:"id"`
	Path              string     `db:"path"`
	SourceDir         string     `db:"source_dir"`
	RelativePath      string     `db:"relative_path"`
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
	ID            string     `db:"id"`
	Type          string     `db:"type"`
	WorkflowDefID string     `db:"workflow_def_id"`
	Status        string     `db:"status"`
	FolderIDs     string     `db:"folder_ids"`
	Total         int        `db:"total"`
	Done          int        `db:"done"`
	Failed        int        `db:"failed"`
	Error         string     `db:"error"`
	StartedAt     *time.Time `db:"started_at"`
	FinishedAt    *time.Time `db:"finished_at"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
}

type Snapshot struct {
	ID            string          `db:"id"`
	JobID         string          `db:"job_id"`
	FolderID      string          `db:"folder_id"`
	OperationType string          `db:"operation_type"`
	Before        json.RawMessage `db:"before_state"`
	After         json.RawMessage `db:"after_state"`
	Detail        json.RawMessage `db:"detail"`
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
	JobID    string
	Action   string
	Result   string
	FolderID string
	From     time.Time
	To       time.Time
	Page     int
	Limit    int
}

type WorkflowDefinition struct {
	ID          string    `db:"id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	GraphJSON   string    `db:"graph_json"`
	IsActive    bool      `db:"is_active"`
	Version     int       `db:"version"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type WorkflowRun struct {
	ID             string     `db:"id"`
	JobID          string     `db:"job_id"`
	FolderID       string     `db:"folder_id"`
	WorkflowDefID  string     `db:"workflow_def_id"`
	Status         string     `db:"status"`
	ResumeNodeID   string     `db:"resume_node_id"`
	LastNodeID     string     `db:"last_node_id"`
	ExternalBlocks int        `db:"external_blocks"`
	Error          string     `db:"error"`
	StartedAt      *time.Time `db:"started_at"`
	FinishedAt     *time.Time `db:"finished_at"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

type NodeRun struct {
	ID             string     `db:"id"`
	WorkflowRunID  string     `db:"workflow_run_id"`
	NodeID         string     `db:"node_id"`
	NodeType       string     `db:"node_type"`
	Sequence       int        `db:"sequence"`
	Status         string     `db:"status"`
	InputJSON      string     `db:"input_json"`
	OutputJSON     string     `db:"output_json"`
	InputSignature string     `db:"input_signature"`
	ResumeToken    string     `db:"resume_token"`
	Error          string     `db:"error"`
	StartedAt      *time.Time `db:"started_at"`
	FinishedAt     *time.Time `db:"finished_at"`
	CreatedAt      time.Time  `db:"created_at"`
}

type NodeSnapshot struct {
	ID            string    `db:"id"`
	NodeRunID     string    `db:"node_run_id"`
	WorkflowRunID string    `db:"workflow_run_id"`
	Kind          string    `db:"kind"`
	FSManifest    string    `db:"fs_manifest"`
	OutputJSON    string    `db:"output_json"`
	Compensation  string    `db:"compensation"`
	CreatedAt     time.Time `db:"created_at"`
}

type WorkflowGraph struct {
	Nodes []WorkflowGraphNode `json:"nodes"`
	Edges []WorkflowGraphEdge `json:"edges"`
}

type WorkflowGraphNode struct {
	ID         string                   `json:"id"`
	Type       string                   `json:"type"`
	Label      string                   `json:"label,omitempty"`
	Config     map[string]any           `json:"config"`
	Inputs     map[string]NodeInputSpec `json:"inputs,omitempty"`
	UIPosition *NodeUIPosition          `json:"ui_position,omitempty"`
	Enabled    bool                     `json:"enabled"`
}

type WorkflowGraphEdge struct {
	ID         string `json:"id,omitempty"`
	Source     string `json:"source"`
	SourcePort int    `json:"source_port"`
	Target     string `json:"target"`
	TargetPort int    `json:"target_port"`
}

type NodeInputSpec struct {
	ConstValue *any            `json:"const_value,omitempty"`
	LinkSource *NodeLinkSource `json:"link_source,omitempty"`
}

type NodeLinkSource struct {
	SourceNodeID    string `json:"source_node_id"`
	OutputPortIndex int    `json:"output_port_index"`
}

type NodeUIPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type WorkflowDefListFilter struct {
	Page  int
	Limit int
}

type WorkflowRunListFilter struct {
	JobID    string
	FolderID string
	Status   string
	Page     int
	Limit    int
}

type NodeRunListFilter struct {
	WorkflowRunID string
	Page          int
	Limit         int
}
