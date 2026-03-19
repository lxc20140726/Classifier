import { request } from '@/api/client'
import type { Category, Folder, FolderStatus, PaginatedResponse } from '@/types'

export interface FolderQueryParams {
  status?: FolderStatus
  category?: Category
  q?: string
  page?: number
  limit?: number
}

export function listFolders(params: FolderQueryParams = {}) {
  const search = new URLSearchParams()

  if (params.status) search.set('status', params.status)
  if (params.category) search.set('category', params.category)
  if (params.q) search.set('q', params.q)
  if (params.page) search.set('page', String(params.page))
  if (params.limit) search.set('limit', String(params.limit))

  const suffix = search.toString() ? `?${search.toString()}` : ''
  return request<PaginatedResponse<Folder>>(`/folders${suffix}`)
}

export function getFolder(id: string) {
  return request<{ data: Folder }>(`/folders/${id}`)
}

export function scanFolders() {
  return request<{ started: boolean }>('/folders/scan', { method: 'POST' })
}

export function updateFolderCategory(id: string, category: Category) {
  return request<{ data: Folder }>(`/folders/${id}/category`, {
    method: 'PATCH',
    body: JSON.stringify({ category }),
  })
}

export function updateFolderStatus(id: string, status: FolderStatus) {
  return request<{ data: Folder }>(`/folders/${id}/status`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  })
}

export function deleteFolder(id: string) {
  return request<{ data: { deleted: boolean } }>(`/folders/${id}`, {
    method: 'DELETE',
  })
}

export function moveFolders(folderIds: string[], targetDir: string) {
  return request<{ operation_id: string }>('/jobs/move', {
    method: 'POST',
    body: JSON.stringify({ folder_ids: folderIds, target_dir: targetDir }),
  })
}
