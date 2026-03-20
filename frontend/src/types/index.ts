export type Category = 'photo' | 'video' | 'mixed' | 'manga' | 'other'
export type FolderStatus = 'pending' | 'done' | 'skip'
export type CategorySource = 'auto' | 'manual'
export type JobStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'partial' | 'cancelled'

export interface Folder {
  id: string
  path: string
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
  operation_type: 'rename' | 'move'
  before: FileRecord[]
  after: FileRecord[] | null
  status: 'pending' | 'committed' | 'reverted'
  created_at: string
}

export interface AppConfig {
  source_dir: string
  target_dir: string
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
