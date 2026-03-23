import { create } from 'zustand'

import {
  deleteFolder,
  listFolders,
  restoreFolder,
  scanFolders,
  updateFolderCategory,
  updateFolderStatus,
  type FolderQueryParams,
} from '@/api/folders'
import { useJobStore } from '@/store/jobStore'
import type { Category, Folder, FolderStatus, ScanStartResponse, ScanProgressEvent } from '@/types'

export interface FolderFilters {
  status?: FolderStatus
  category?: Category
  q?: string
}

interface ScanProgressState {
  jobId: string
  scanned: number
  total: number
  failed: number
  currentFolderName: string | null
  sourceDirs: string[]
}

type FolderViewMode = 'grid' | 'list'

interface FolderStore {
  folders: Folder[]
  total: number
  page: number
  limit: number
  isLoading: boolean
  error: string | null
  filters: FolderFilters
  scanProgress: ScanProgressState | null
  isScanning: boolean
  viewMode: FolderViewMode
  fetchFolders: () => Promise<void>
  setFilters: (filters: FolderFilters) => void
  setPage: (page: number) => void
  setViewMode: (mode: FolderViewMode) => void
  triggerScan: () => Promise<void>
  handleScanStarted: (payload: ScanStartResponse) => void
  handleScanProgress: (progress: ScanProgressEvent) => void
  handleScanError: (progress: ScanProgressEvent) => void
  handleScanDone: () => void
  updateFolderCategory: (id: string, category: Category) => Promise<void>
  updateFolderStatus: (id: string, status: FolderStatus) => Promise<void>
  removeFolder: (id: string) => Promise<void>
  restoreFolder: (id: string) => Promise<void>
}

function buildQuery(filters: FolderFilters, page: number, limit: number): FolderQueryParams {
  return {
    ...filters,
    page,
    limit,
  }
}

export const useFolderStore = create<FolderStore>((set, get) => ({
  folders: [],
  total: 0,
  page: 1,
  limit: 20,
  isLoading: false,
  error: null,
  filters: {},
  scanProgress: null,
  isScanning: false,
  viewMode: 'list',
  async fetchFolders() {
    const { filters, page, limit } = get()
    set({ isLoading: true, error: null })

    try {
      const response = await listFolders(buildQuery(filters, page, limit))
      set({
        folders: response.data,
        total: response.total,
        page: response.page,
        limit: response.limit,
        isLoading: false,
      })
    } catch (error) {
      set({
        isLoading: false,
        error: error instanceof Error ? error.message : '加载目录失败',
      })
    }
  },
  setFilters(filters) {
    set({ filters, page: 1 })
  },
  setPage(page) {
    set({ page })
  },
  setViewMode(mode) {
    set({ viewMode: mode })
  },
  async triggerScan() {
    set({ isScanning: true, error: null })
    let fallbackTimer: number | undefined
    fallbackTimer = window.setTimeout(() => {
      if (get().isScanning) {
        set({ isScanning: false, scanProgress: null })
      }
    }, 120_000)
    try {
      const response = await scanFolders()
      get().handleScanStarted(response)
      // Start polling as SSE fallback: if SSE events are missed the poll loop
      // will detect completion and call handleScanDone via jobStore coordination.
      useJobStore.getState().startScanPolling(response.job_id)
    } catch (error) {
      window.clearTimeout(fallbackTimer)
      set({
        isScanning: false,
        error: error instanceof Error ? error.message : '启动扫描失败',
      })
    }
  },
  handleScanStarted(payload) {
    // Guard: if isScanning is already false, the scan completed via SSE before this
    // HTTP response handler ran (race condition with fast scans). Do not re-enable.
    if (!get().isScanning) return
    set({
      isScanning: true,
      scanProgress: {
        jobId: payload.job_id,
        scanned: 0,
        total: 0,
        failed: 0,
        currentFolderName: null,
        sourceDirs: payload.source_dirs,
      },
    })
  },
  handleScanProgress(progress) {
    set((state) => ({
      isScanning: true,
      scanProgress: {
        jobId: progress.job_id,
        scanned: progress.done,
        total: progress.total,
        failed: state.scanProgress?.failed ?? 0,
        currentFolderName: progress.folder_name ?? null,
        sourceDirs: state.scanProgress?.sourceDirs ?? [],
      },
    }))
  },
  handleScanError(progress) {
    set((state) => ({
      isScanning: true,
      error: progress.error ?? '扫描过程中出现错误',
      scanProgress: {
        jobId: progress.job_id,
        scanned: progress.done,
        total: progress.total,
        failed: (state.scanProgress?.failed ?? 0) + 1,
        currentFolderName: progress.folder_name ?? null,
        sourceDirs: state.scanProgress?.sourceDirs ?? [],
      },
    }))
  },
  handleScanDone() {
    if (!get().isScanning) return
    set({ isScanning: false, scanProgress: null })
  },
  async updateFolderCategory(id, category) {
    try {
      const response = await updateFolderCategory(id, category)
      set((state) => ({
        folders: state.folders.map((folder) => (folder.id === id ? response.data : folder)),
      }))
    } catch (error) {
      set({ error: error instanceof Error ? error.message : '更新分类失败' })
    }
  },
  async updateFolderStatus(id, status) {
    try {
      const response = await updateFolderStatus(id, status)
      set((state) => ({
        folders: state.folders.map((folder) => (folder.id === id ? response.data : folder)),
      }))
    } catch (error) {
      set({ error: error instanceof Error ? error.message : '更新状态失败' })
    }
  },
  async removeFolder(id) {
    try {
      await deleteFolder(id)
      set((state) => ({
        folders: state.folders.filter((folder) => folder.id !== id),
        total: Math.max(0, state.total - 1),
      }))
    } catch (error) {
      set({ error: error instanceof Error ? error.message : '删除目录记录失败' })
    }
  },
  async restoreFolder(id) {
    try {
      const response = await restoreFolder(id)
      set((state) => ({
        folders: state.folders.map((folder) => (folder.id === id ? response.data : folder)),
      }))
    } catch (error) {
      set({ error: error instanceof Error ? error.message : '恢复目录失败' })
    }
  },
}))
