export type Category = 'photo' | 'video' | 'mixed' | 'manga' | 'other'
export type FolderStatus = 'pending' | 'done' | 'skip'
export type CategorySource = 'auto' | 'manual'
export type JobStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'partial' | 'cancelled'

export interface Folder {
  id: string
  path: string
  source_dir: string
  relative_path: string
  name: string
  category: Category
  category_source: CategorySource
  status: FolderStatus
  image_count: number
  video_count: number
  total_files: number
  total_size: number
  marked_for_move: boolean
  deleted_at: string | null
  delete_staging_path?: string | null
  scanned_at: string
  updated_at: string
}

export interface FileRecord {
  original_path: string
  current_path: string
}

export interface Snapshot {
  id: string
  job_id: string
  folder_id: string
  operation_type: string
  before: FileRecord[]
  after: FileRecord[] | null
  detail: Record<string, unknown> | null
  status: 'pending' | 'committed' | 'reverted'
  created_at: string
}

export interface AppConfig {
  source_dir?: string
  target_dir?: string
  scan_input_dirs?: string[]
}

export interface ApiError {
  error: string
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  limit: number
}

export interface Job {
  id: string
  type: string
  workflow_def_id?: string
  status: JobStatus
  folder_ids: string[]
  total: number
  done: number
  failed: number
  error: string
  started_at: string | null
  finished_at: string | null
  created_at: string
  updated_at: string
}

export interface JobProgress {
  job_id: string
  status: JobStatus
  done: number
  total: number
  failed: number
  updated_at: string
}

export interface ScanStartResponse {
  started: boolean
  job_id: string
  source_dirs: string[]
}

export interface ScanProgressEvent {
  job_id: string
  folder_id?: string
  folder_name?: string
  folder_path?: string
  source_dir?: string
  relative_path?: string
  category?: string
  done: number
  total: number
  error?: string
}

export interface JobDoneEvent {
  job_id: string
  status: JobStatus
  processed?: number
  failed?: number
  total: number
}

export interface AuditLog {
  id: string
  job_id: string
  folder_id: string
  folder_path: string
  action: string
  level: string
  detail: Record<string, unknown> | null
  result: string
  error_msg: string
  duration_ms: number
  created_at: string
}

export type WorkflowRunStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'partial'
export type NodeRunStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'skipped'
export type NodeType = 'trigger' | 'ext-ratio-classifier' | 'move'

export interface WorkflowDefinition {
  id: string
  name: string
  graph_json: string
  is_active: boolean
  version: number
  created_at: string
  updated_at: string
}

export interface WorkflowRun {
  id: string
  job_id: string
  folder_id: string
  workflow_def_id: string
  status: WorkflowRunStatus
  resume_node_id: string | null
  created_at: string
  updated_at: string
}

export interface NodeRun {
  id: string
  workflow_run_id: string
  node_id: string
  node_type: NodeType
  sequence: number
  status: NodeRunStatus
  input_json: string
  output_json: string
  error: string
  started_at: string | null
  finished_at: string | null
  created_at: string
}

export interface WorkflowRunDetail {
  data: WorkflowRun
  node_runs: NodeRun[]
}

export interface WorkflowNodeEvent {
  job_id: string
  workflow_run_id: string
  node_id: string
  node_type: string
  status?: NodeRunStatus
  error?: string
}
