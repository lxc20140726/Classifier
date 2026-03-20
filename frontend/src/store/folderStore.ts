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
import type { Category, Folder, FolderStatus } from '@/types'

export interface FolderFilters {
  status?: FolderStatus
  category?: Category
  q?: string
}

interface ScanProgressState {
  scanned: number
  total: number
}

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
  fetchFolders: () => Promise<void>
  setFilters: (filters: FolderFilters) => void
  setPage: (page: number) => void
  triggerScan: () => Promise<void>
  handleScanProgress: (progress: ScanProgressState) => void
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
        error: error instanceof Error ? error.message : 'Failed to load folders',
      })
    }
  },
  setFilters(filters) {
    set({ filters, page: 1 })
  },
  setPage(page) {
    set({ page })
  },
  async triggerScan() {
    set({ isScanning: true, error: null, scanProgress: { scanned: 0, total: 0 } })
    try {
      await scanFolders()
    } catch (error) {
      set({
        isScanning: false,
        error: error instanceof Error ? error.message : 'Failed to start scan',
      })
    }
  },
  handleScanProgress(progress) {
    set({ isScanning: true, scanProgress: progress })
  },
  handleScanDone() {
    set({ isScanning: false, scanProgress: null })
  },
  async updateFolderCategory(id, category) {
    try {
      const response = await updateFolderCategory(id, category)
      set((state) => ({
        folders: state.folders.map((folder) => (folder.id === id ? response.data : folder)),
      }))
    } catch (error) {
      set({ error: error instanceof Error ? error.message : 'Failed to update category' })
    }
  },
  async updateFolderStatus(id, status) {
    try {
      const response = await updateFolderStatus(id, status)
      set((state) => ({
        folders: state.folders.map((folder) => (folder.id === id ? response.data : folder)),
      }))
    } catch (error) {
      set({ error: error instanceof Error ? error.message : 'Failed to update status' })
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
      set({ error: error instanceof Error ? error.message : 'Failed to remove folder' })
    }
  },
  async restoreFolder(id) {
    try {
      const response = await restoreFolder(id)
      set((state) => ({
        folders: state.folders.map((folder) => (folder.id === id ? response.data : folder)),
      }))
    } catch (error) {
      set({ error: error instanceof Error ? error.message : 'Failed to restore folder' })
    }
  },
}))
