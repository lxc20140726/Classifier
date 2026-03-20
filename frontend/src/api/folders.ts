import { request } from '@/api/client'
import type { Category, Folder, FolderStatus, PaginatedResponse, ScanStartResponse } from '@/types'

export interface FolderQueryParams {
  status?: FolderStatus
  category?: Category
  q?: string
  page?: number
  limit?: number
}

interface RawFolder {
  id?: string
  ID?: string
  path?: string
  Path?: string
  source_dir?: string
  SourceDir?: string
  relative_path?: string
  RelativePath?: string
  name?: string
  Name?: string
  category?: Category
  Category?: Category
  category_source?: 'auto' | 'manual'
  CategorySource?: 'auto' | 'manual'
  status?: FolderStatus
  Status?: FolderStatus
  image_count?: number
  ImageCount?: number
  video_count?: number
  VideoCount?: number
  total_files?: number
  TotalFiles?: number
  total_size?: number
  TotalSize?: number
  marked_for_move?: boolean
  MarkedForMove?: boolean
  deleted_at?: string | null
  DeletedAt?: string | null
  delete_staging_path?: string | null
  DeleteStagingPath?: string | null
  scanned_at?: string
  ScannedAt?: string
  updated_at?: string
  UpdatedAt?: string
}

function parseFolder(raw: RawFolder): Folder {
  return {
    id: raw.id ?? raw.ID ?? '',
    path: raw.path ?? raw.Path ?? '',
    source_dir: raw.source_dir ?? raw.SourceDir ?? '',
    relative_path: raw.relative_path ?? raw.RelativePath ?? '',
    name: raw.name ?? raw.Name ?? '',
    category: raw.category ?? raw.Category ?? 'other',
    category_source: raw.category_source ?? raw.CategorySource ?? 'auto',
    status: raw.status ?? raw.Status ?? 'pending',
    image_count: raw.image_count ?? raw.ImageCount ?? 0,
    video_count: raw.video_count ?? raw.VideoCount ?? 0,
    total_files: raw.total_files ?? raw.TotalFiles ?? 0,
    total_size: raw.total_size ?? raw.TotalSize ?? 0,
    marked_for_move: raw.marked_for_move ?? raw.MarkedForMove ?? false,
    deleted_at: raw.deleted_at ?? raw.DeletedAt ?? null,
    delete_staging_path: raw.delete_staging_path ?? raw.DeleteStagingPath ?? null,
    scanned_at: raw.scanned_at ?? raw.ScannedAt ?? '',
    updated_at: raw.updated_at ?? raw.UpdatedAt ?? '',
  }
}

export async function listFolders(params: FolderQueryParams = {}): Promise<PaginatedResponse<Folder>> {
  const search = new URLSearchParams()

  if (params.status) search.set('status', params.status)
  if (params.category) search.set('category', params.category)
  if (params.q) search.set('q', params.q)
  if (params.page) search.set('page', String(params.page))
  if (params.limit) search.set('limit', String(params.limit))

  const suffix = search.toString() ? `?${search.toString()}` : ''
  const response = await request<PaginatedResponse<RawFolder>>(`/folders${suffix}`)
  return {
    ...response,
    data: (response.data ?? []).map(parseFolder),
  }
}

export async function getFolder(id: string): Promise<{ data: Folder }> {
  const response = await request<{ data: RawFolder }>(`/folders/${id}`)
  return { data: parseFolder(response.data) }
}

export function scanFolders() {
  return request<ScanStartResponse>('/folders/scan', { method: 'POST' })
}

export async function updateFolderCategory(id: string, category: Category): Promise<{ data: Folder }> {
  const response = await request<{ data: RawFolder }>(`/folders/${id}/category`, {
    method: 'PATCH',
    body: JSON.stringify({ category }),
  })
  return { data: parseFolder(response.data) }
}

export async function updateFolderStatus(id: string, status: FolderStatus): Promise<{ data: Folder }> {
  const response = await request<{ data: RawFolder }>(`/folders/${id}/status`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  })
  return { data: parseFolder(response.data) }
}

export function deleteFolder(id: string) {
  return request<{ data: { deleted: boolean } }>(`/folders/${id}`, {
    method: 'DELETE',
  })
}

export function moveFolders(folderIds: string[], targetDir: string) {
  return request<{ job_id: string }>('/jobs/move', {
    method: 'POST',
    body: JSON.stringify({ folder_ids: folderIds, target_dir: targetDir }),
  })
}

export async function restoreFolder(id: string): Promise<{ data: Folder }> {
  const response = await request<{ data: RawFolder }>(`/folders/${id}/restore`, {
    method: 'POST',
  })
  return { data: parseFolder(response.data) }
}
