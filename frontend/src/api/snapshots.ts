import { request } from '@/api/client'
import type { Snapshot } from '@/types'

interface SnapshotListParams {
  folderId?: string
  jobId?: string
}

export async function listSnapshots(params: SnapshotListParams): Promise<Snapshot[]> {
  const query = new URLSearchParams()

  if (params.folderId) query.set('folder_id', params.folderId)
  if (params.jobId) query.set('job_id', params.jobId)

  const suffix = query.toString() ? `?${query.toString()}` : ''
  const response = await request<{ data: Snapshot[] }>(`/snapshots${suffix}`)
  return response.data ?? []
}

export function revertSnapshot(snapshotId: string): Promise<{ reverted: boolean }> {
  return request<{ reverted: boolean }>(`/snapshots/${snapshotId}/revert`, {
    method: 'POST',
  })
}
