import { request } from '@/api/client'
import type { Snapshot } from '@/types'

interface SnapshotListParams {
  folderId?: string
  jobId?: string
}

interface RawSnapshotRecord {
  original_path?: string
  current_path?: string
}

interface RawSnapshot {
  id?: string
  ID?: string
  job_id?: string
  JobID?: string
  folder_id?: string
  FolderID?: string
  operation_type?: string
  OperationType?: string
  before?: RawSnapshotRecord[]
  Before?: RawSnapshotRecord[]
  after?: RawSnapshotRecord[] | null
  After?: RawSnapshotRecord[] | null
  detail?: Record<string, unknown> | null
  Detail?: Record<string, unknown> | null
  status?: 'pending' | 'committed' | 'reverted'
  Status?: 'pending' | 'committed' | 'reverted'
  created_at?: string
  CreatedAt?: string
}

function parseRecord(raw: RawSnapshotRecord) {
  return {
    original_path: raw.original_path ?? '',
    current_path: raw.current_path ?? '',
  }
}

function parseSnapshot(raw: RawSnapshot): Snapshot {
  return {
    id: raw.id ?? raw.ID ?? '',
    job_id: raw.job_id ?? raw.JobID ?? '',
    folder_id: raw.folder_id ?? raw.FolderID ?? '',
    operation_type: raw.operation_type ?? raw.OperationType ?? 'move',
    before: (raw.before ?? raw.Before ?? []).map(parseRecord),
    after: raw.after != null || raw.After != null ? (raw.after ?? raw.After ?? []).map(parseRecord) : null,
    detail: raw.detail ?? raw.Detail ?? null,
    status: raw.status ?? raw.Status ?? 'pending',
    created_at: raw.created_at ?? raw.CreatedAt ?? '',
  }
}

export async function listSnapshots(params: SnapshotListParams): Promise<Snapshot[]> {
  const query = new URLSearchParams()

  if (params.folderId) query.set('folder_id', params.folderId)
  if (params.jobId) query.set('job_id', params.jobId)

  const suffix = query.toString() ? `?${query.toString()}` : ''
  const response = await request<{ data: RawSnapshot[] }>(`/snapshots${suffix}`)
  return (response.data ?? []).map(parseSnapshot)
}

export interface RevertPathState {
  original_path: string
  current_path: string
}

export interface RevertResult {
  ok: boolean
  error_message?: string
  preflight_error?: string
  current_state: RevertPathState[]
}

export interface RevertResponse {
  reverted: boolean
  revert_result: RevertResult
}

export async function revertSnapshot(snapshotId: string): Promise<RevertResponse> {
  return request<RevertResponse>(`/snapshots/${snapshotId}/revert`, {
    method: 'POST',
  })
}
