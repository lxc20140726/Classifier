import { useEffect, useMemo, useState } from 'react'

import { SnapshotDrawer } from '@/components/SnapshotDrawer'
import { cn } from '@/lib/utils'
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

function MoveModal({
  selectedIds,
  onConfirm,
  onCancel,
}: {
  selectedIds: string[]
  onConfirm: (targetDir: string) => void
  onCancel: () => void
}) {
  const [targetDir, setTargetDir] = useState('')

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
      <div className="w-full max-w-md rounded-xl border border-border bg-card p-6 shadow-lg">
        <h2 className="text-lg font-semibold">Move Folders</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          Moving {selectedIds.length} folder{selectedIds.length !== 1 ? 's' : ''} to a new location.
        </p>
        <div className="mt-4 space-y-2">
          <label htmlFor="target-dir" className="text-sm font-medium">
            Target directory
          </label>
          <input
            id="target-dir"
            type="text"
            value={targetDir}
            onChange={(e) => setTargetDir(e.target.value)}
            placeholder="/path/to/target"
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
          />
        </div>
        <div className="mt-6 flex justify-end gap-3">
          <button
            type="button"
            onClick={onCancel}
            className="rounded-md border border-input bg-background px-4 py-2 text-sm font-medium hover:bg-accent"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={() => onConfirm(targetDir)}
            disabled={!targetDir.trim()}
            className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
          >
            Move
          </button>
        </div>
      </div>
    </div>
  )
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
    restoreFolder,
    moveFolders,
  } = useFolderStore()

  const [search, setSearch] = useState(filters.q ?? '')
  const [activeFolderId, setActiveFolderId] = useState<string | null>(null)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [showMoveModal, setShowMoveModal] = useState(false)
  const [isMoveLoading, setIsMoveLoading] = useState(false)

  useEffect(() => {
    void fetchFolders()
  }, [fetchFolders, filters])

  // Reset selection when filters change
  useEffect(() => {
    setSelectedIds(new Set())
  }, [filters])

  const progressLabel = useMemo(() => {
    if (!scanProgress || scanProgress.total === 0) {
      return 'Waiting for scan updates'
    }
    return `${scanProgress.scanned}/${scanProgress.total} folders scanned`
  }, [scanProgress])

  const visibleFolderIds = folders.map((f) => f.id)
  const allSelected =
    visibleFolderIds.length > 0 && visibleFolderIds.every((id) => selectedIds.has(id))
  const someSelected = selectedIds.size > 0

  function toggleAll() {
    if (allSelected) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(visibleFolderIds))
    }
  }

  function toggleOne(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  async function handleMove(targetDir: string) {
    setIsMoveLoading(true)
    try {
      await moveFolders([...selectedIds], targetDir)
      setSelectedIds(new Set())
      setShowMoveModal(false)
    } finally {
      setIsMoveLoading(false)
    }
  }

  return (
    <>
      <section className="mx-auto max-w-7xl space-y-6">
        <header className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
          <div className="space-y-2">
            <p className="text-sm uppercase tracking-[0.24em] text-muted-foreground">Folders</p>
            <h2 className="text-3xl font-semibold tracking-tight">
              Scan results and manual corrections
            </h2>
            <p className="text-sm text-muted-foreground">{total} folders in the current view</p>
          </div>

          <div className="flex flex-col items-start gap-2 md:items-end">
            <button
              type="button"
              onClick={() => void triggerScan()}
              disabled={isScanning}
              className="rounded-lg bg-primary px-5 py-2.5 text-sm font-medium text-primary-foreground shadow-sm transition hover:bg-primary/90 disabled:opacity-50"
            >
              {isScanning ? 'Scanning…' : 'Scan source directory'}
            </button>

            {isScanning && (
              <p className="text-xs text-muted-foreground">{progressLabel}</p>
            )}
          </div>
        </header>

        {error && (
          <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">
            {error}
          </div>
        )}

        {/* Filters */}
        <div className="flex flex-wrap items-center gap-3">
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') setFilters({ q: search || undefined })
            }}
            placeholder="Search folders…"
            className="rounded-lg border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
          />

          <select
            value={filters.category ?? ''}
            onChange={(e) =>
              setFilters({ category: (e.target.value as Category) || undefined })
            }
            className="rounded-lg border border-input bg-background px-3 py-2 text-sm"
          >
            {categories.map((c) => (
              <option key={c} value={c}>
                {c === '' ? 'All categories' : c}
              </option>
            ))}
          </select>

          <select
            value={filters.status ?? ''}
            onChange={(e) =>
              setFilters({ status: (e.target.value as FolderStatus) || undefined })
            }
            className="rounded-lg border border-input bg-background px-3 py-2 text-sm"
          >
            {statuses.map((s) => (
              <option key={s} value={s}>
                {s === '' ? 'All statuses' : s}
              </option>
            ))}
          </select>

          {/* Bulk actions */}
          {someSelected && (
            <button
              type="button"
              onClick={() => setShowMoveModal(true)}
              disabled={isMoveLoading}
              className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground shadow-sm transition hover:bg-primary/90 disabled:opacity-50"
            >
              Move {selectedIds.size} selected
            </button>
          )}
          {someSelected && (
            <button
              type="button"
              onClick={() => setSelectedIds(new Set())}
              className="rounded-lg border border-input px-3 py-2 text-sm font-medium hover:bg-accent"
            >
              Clear selection
            </button>
          )}
        </div>

        {/* Table */}
        <div className="overflow-hidden rounded-xl border border-border bg-card">
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b border-border bg-muted/50">
                  <th className="w-10 px-4 py-4">
                    <input
                      type="checkbox"
                      checked={allSelected}
                      ref={(el) => {
                        if (el) {
                          el.indeterminate =
                            someSelected && !allSelected
                        }
                      }}
                      onChange={toggleAll}
                      className="rounded border-input"
                      aria-label="Select all"
                    />
                  </th>
                  <th className="px-4 py-4 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    Name
                  </th>
                  <th className="px-4 py-4 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    Category
                  </th>
                  <th className="px-4 py-4 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    Status
                  </th>
                  <th className="px-4 py-4 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    Files
                  </th>
                  <th className="px-4 py-4 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    Size
                  </th>
                  <th className="px-4 py-4 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody>
                {isLoading ? (
                  <tr>
                    <td
                      colSpan={7}
                      className="px-4 py-12 text-center text-sm text-muted-foreground"
                    >
                      Loading…
                    </td>
                  </tr>
                ) : folders.length === 0 ? (
                  <tr>
                    <td
                      colSpan={7}
                      className="px-4 py-12 text-center text-sm text-muted-foreground"
                    >
                      No folders found. Try scanning or adjusting filters.
                    </td>
                  </tr>
                ) : (
                  folders.map((folder) => (
                    <tr
                      key={folder.id}
                      className={cn(
                        'border-b border-border transition-colors hover:bg-muted/30',
                        selectedIds.has(folder.id) && 'bg-primary/5',
                        folder.deleted_at && 'opacity-60',
                      )}
                    >
                      <td className="px-4 py-4">
                        <input
                          type="checkbox"
                          checked={selectedIds.has(folder.id)}
                          onChange={() => toggleOne(folder.id)}
                          className="rounded border-input"
                          aria-label={`Select ${folder.name}`}
                        />
                      </td>
                      <td className="px-4 py-4">
                        <span
                          className={cn(
                            'font-medium',
                            folder.deleted_at && 'line-through text-muted-foreground',
                          )}
                        >
                          {folder.name}
                        </span>
                      </td>
                      <td className="px-4 py-4">
                        <select
                          value={folder.category}
                          onChange={(e) =>
                            void updateFolderCategory(folder.id, e.target.value as Category)
                          }
                          disabled={!!folder.deleted_at}
                          className="rounded-lg border border-input bg-background px-2 py-1 text-sm disabled:opacity-50"
                        >
                          {categories
                            .filter((c) => c !== '')
                            .map((c) => (
                              <option key={c} value={c}>
                                {c}
                              </option>
                            ))}
                        </select>
                      </td>
                      <td className="px-4 py-4">
                        <select
                          value={folder.status}
                          onChange={(e) =>
                            void updateFolderStatus(folder.id, e.target.value as FolderStatus)
                          }
                          disabled={!!folder.deleted_at}
                          className="rounded-lg border border-input bg-background px-2 py-1 text-sm disabled:opacity-50"
                        >
                          {statuses
                            .filter((s) => s !== '')
                            .map((s) => (
                              <option key={s} value={s}>
                                {s}
                              </option>
                            ))}
                        </select>
                      </td>
                      <td className="px-4 py-4 text-muted-foreground">{folder.file_count}</td>
                      <td className="px-4 py-4 text-muted-foreground">
                        {formatBytes(folder.total_size)}
                      </td>
                      <td className="px-4 py-4">
                        <div className="flex flex-col gap-2">
                          {folder.deleted_at ? (
                            <button
                              type="button"
                              onClick={() => void restoreFolder(folder.id)}
                              className="rounded-lg border border-green-200 px-3 py-1 text-xs font-medium text-green-700 transition hover:bg-green-50"
                            >
                              Restore
                            </button>
                          ) : (
                            <>
                              <button
                                type="button"
                                onClick={() => setActiveFolderId(folder.id)}
                                className="rounded-lg border border-border px-3 py-1 text-xs font-medium transition hover:bg-accent"
                              >
                                View snapshots
                              </button>
                              <button
                                type="button"
                                onClick={() => void removeFolder(folder.id)}
                                className="rounded-lg border border-red-200 px-3 py-1 text-xs font-medium text-red-700 transition hover:bg-red-50"
                              >
                                Remove record
                              </button>
                            </>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))
                )}
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

      {showMoveModal && (
        <MoveModal
          selectedIds={[...selectedIds]}
          onConfirm={(targetDir) => void handleMove(targetDir)}
          onCancel={() => setShowMoveModal(false)}
        />
      )}
    </>
  )
}
