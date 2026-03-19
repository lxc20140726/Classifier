import { useEffect, useMemo, useState } from 'react'

import { SnapshotDrawer } from '@/components/SnapshotDrawer'
import { useFolderStore } from '@/store/folderStore'
import type { Category, FolderStatus } from '@/types'

const categories: Array<Category | ''> = ['', 'photo', 'video', 'mixed', 'manga', 'other']
const statuses: Array<FolderStatus | ''> = ['', 'pending', 'done', 'skip']

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MB`
  return `${(value / (1024 * 1024 * 1024)).toFixed(1)} GB`
}

export default function FolderListPage() {
  const {
    folders,
    total,
    isLoading,
    error,
    filters,
    isScanning,
    scanProgress,
    fetchFolders,
    setFilters,
    triggerScan,
    updateFolderCategory,
    updateFolderStatus,
    removeFolder,
  } = useFolderStore()

  const [search, setSearch] = useState(filters.q ?? '')
  const [activeFolderId, setActiveFolderId] = useState<string | null>(null)

  useEffect(() => {
    void fetchFolders()
  }, [fetchFolders, filters])

  const progressLabel = useMemo(() => {
    if (!scanProgress || scanProgress.total === 0) {
      return 'Waiting for scan updates'
    }

    return `${scanProgress.scanned}/${scanProgress.total} folders scanned`
  }, [scanProgress])

  return (
    <>
      <section className="mx-auto max-w-7xl space-y-6">
        <header className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
          <div className="space-y-2">
            <p className="text-sm uppercase tracking-[0.24em] text-muted-foreground">Folders</p>
            <h2 className="text-3xl font-semibold tracking-tight">Scan results and manual corrections</h2>
            <p className="text-sm text-muted-foreground">{total} folders in the current view</p>
          </div>

          <div className="flex flex-col items-start gap-2 md:items-end">
            <button
              type="button"
              onClick={() => {
                void triggerScan()
              }}
              className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={isScanning}
            >
              {isScanning ? 'Scanning...' : 'Scan source directory'}
            </button>
            <p className="text-xs text-muted-foreground">{isScanning ? progressLabel : 'Ready to scan'}</p>
          </div>
        </header>

        <div className="grid gap-3 rounded-2xl border border-border bg-white p-4 shadow-sm md:grid-cols-[2fr,1fr,1fr]">
          <label className="space-y-2 text-sm font-medium">
            <span>Search</span>
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  setFilters({ ...filters, q: search || undefined })
                }
              }}
              className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm outline-none transition focus:border-primary"
              placeholder="Folder name or path"
            />
          </label>

          <label className="space-y-2 text-sm font-medium">
            <span>Category</span>
            <select
              value={filters.category ?? ''}
              onChange={(event) => {
                const value = event.target.value as Category | ''
                setFilters({ ...filters, category: value || undefined })
              }}
              className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm"
            >
              <option value="">All categories</option>
              {categories.filter(Boolean).map((category) => (
                <option key={category} value={category}>
                  {category}
                </option>
              ))}
            </select>
          </label>

          <label className="space-y-2 text-sm font-medium">
            <span>Status</span>
            <select
              value={filters.status ?? ''}
              onChange={(event) => {
                const value = event.target.value as FolderStatus | ''
                setFilters({ ...filters, status: value || undefined, q: search || undefined })
              }}
              className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm"
            >
              <option value="">All statuses</option>
              {statuses.filter(Boolean).map((status) => (
                <option key={status} value={status}>
                  {status}
                </option>
              ))}
            </select>
          </label>

          <div className="flex justify-end md:col-span-3">
            <button
              type="button"
              onClick={() => setFilters({ ...filters, q: search || undefined })}
              className="rounded-lg border border-border px-4 py-2 text-sm font-medium text-foreground transition hover:bg-accent"
            >
              Apply filters
            </button>
          </div>
        </div>

        {error && <div className="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>}

        <div className="overflow-hidden rounded-2xl border border-border bg-white shadow-sm">
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-border text-sm">
              <thead className="bg-muted/50 text-left text-xs uppercase tracking-[0.18em] text-muted-foreground">
                <tr>
                  <th className="px-4 py-3">Name</th>
                  <th className="px-4 py-3">Path</th>
                  <th className="px-4 py-3">Category</th>
                  <th className="px-4 py-3">Status</th>
                  <th className="px-4 py-3">Images</th>
                  <th className="px-4 py-3">Videos</th>
                  <th className="px-4 py-3">Files</th>
                  <th className="px-4 py-3">Size</th>
                  <th className="px-4 py-3">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {isLoading && (
                  <tr>
                    <td className="px-4 py-6 text-muted-foreground" colSpan={9}>
                      Loading folders...
                    </td>
                  </tr>
                )}

                {!isLoading && folders.length === 0 && (
                  <tr>
                    <td className="px-4 py-6 text-muted-foreground" colSpan={9}>
                      No folders found for the current filters.
                    </td>
                  </tr>
                )}

                {folders.map((folder) => (
                  <tr key={folder.id} className="align-top">
                    <td className="px-4 py-4 font-medium text-foreground">{folder.name}</td>
                    <td className="px-4 py-4 text-xs text-muted-foreground">{folder.path}</td>
                    <td className="px-4 py-4">
                      <select
                        value={folder.category}
                        onChange={(event) => {
                          void updateFolderCategory(folder.id, event.target.value as Category)
                        }}
                        className="rounded-lg border border-border bg-background px-2 py-1 text-sm"
                      >
                        {categories.filter(Boolean).map((category) => (
                          <option key={category} value={category}>
                            {category}
                          </option>
                        ))}
                      </select>
                    </td>
                    <td className="px-4 py-4">
                      <select
                        value={folder.status}
                        onChange={(event) => {
                          void updateFolderStatus(folder.id, event.target.value as FolderStatus)
                        }}
                        className="rounded-lg border border-border bg-background px-2 py-1 text-sm"
                      >
                        {statuses.filter(Boolean).map((status) => (
                          <option key={status} value={status}>
                            {status}
                          </option>
                        ))}
                      </select>
                    </td>
                    <td className="px-4 py-4 text-muted-foreground">{folder.image_count}</td>
                    <td className="px-4 py-4 text-muted-foreground">{folder.video_count}</td>
                    <td className="px-4 py-4 text-muted-foreground">{folder.total_files}</td>
                    <td className="px-4 py-4 text-muted-foreground">{formatBytes(folder.total_size)}</td>
                    <td className="px-4 py-4">
                      <div className="flex flex-col gap-2">
                        <button
                          type="button"
                          onClick={() => setActiveFolderId(folder.id)}
                          className="rounded-lg border border-border px-3 py-1 text-xs font-medium transition hover:bg-accent"
                        >
                          View snapshots
                        </button>
                        <button
                          type="button"
                          onClick={() => {
                            void removeFolder(folder.id)
                          }}
                          className="rounded-lg border border-red-200 px-3 py-1 text-xs font-medium text-red-700 transition hover:bg-red-50"
                        >
                          Remove record
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </section>

      <SnapshotDrawer
        open={activeFolderId !== null}
        folderId={activeFolderId}
        onClose={() => setActiveFolderId(null)}
      />
    </>
  )
}
