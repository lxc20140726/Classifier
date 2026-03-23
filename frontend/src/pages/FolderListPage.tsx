import { useEffect, useState } from 'react'
import {
  Clock,
  FileText,
  FolderOpen,
  Grid2X2,
  History,
  List,
  Loader2,
  Search,
  X,
} from 'lucide-react'

import { moveFolders } from '@/api/folders'
import { SnapshotDrawer } from '@/components/SnapshotDrawer'
import { cn } from '@/lib/utils'
import { useActivityStore } from '@/store/activityStore'
import { useFolderStore } from '@/store/folderStore'
import { useJobStore } from '@/store/jobStore'
import type { Category, Folder, FolderStatus, Job } from '@/types'

const CATEGORY_LABEL: Record<Category | '', string> = {
  '': '全部分类',
  photo: '写真',
  video: '视频',
  mixed: '混合',
  manga: '漫画',
  other: '其他',
}

const CATEGORY_COLOR: Record<Category, string> = {
  photo: 'bg-pink-100 text-pink-800',
  video: 'bg-blue-100 text-blue-800',
  mixed: 'bg-purple-100 text-purple-800',
  manga: 'bg-orange-100 text-orange-800',
  other: 'bg-gray-100 text-gray-700',
}

const STATUS_LABEL: Record<FolderStatus | '', string> = {
  '': '全部状态',
  pending: '待处理',
  done: '已完成',
  skip: '跳过',
}

const STATUS_COLOR: Record<FolderStatus, string> = {
  pending: 'bg-yellow-100 text-yellow-800',
  done: 'bg-green-100 text-green-800',
  skip: 'bg-gray-100 text-gray-500',
}

const JOB_STATUS_LABEL: Record<string, string> = {
  pending: '等待中',
  running: '进行中',
  succeeded: '已完成',
  failed: '失败',
  partial: '部分成功',
  cancelled: '已取消',
}

const JOB_STATUS_COLOR: Record<string, string> = {
  pending: 'bg-gray-100 text-gray-600',
  running: 'bg-blue-100 text-blue-700',
  succeeded: 'bg-green-100 text-green-700',
  failed: 'bg-red-100 text-red-700',
  partial: 'bg-yellow-100 text-yellow-700',
  cancelled: 'bg-gray-100 text-gray-500',
}

const ALL_CATEGORIES: Array<Category | ''> = ['', 'photo', 'video', 'mixed', 'manga', 'other']
const ALL_STATUSES: Array<FolderStatus | ''> = ['', 'pending', 'done', 'skip']

function formatBytes(value: number): string {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MB`
  return `${(value / (1024 * 1024 * 1024)).toFixed(1)} GB`
}

function formatRelativeTime(iso: string): string {
  if (!iso) return ''
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return '刚刚'
  if (mins < 60) return `${mins} 分钟前`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs} 小时前`
  return `${Math.floor(hrs / 24)} 天前`
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
        <h2 className="text-lg font-semibold">移动文件夹</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          将 {selectedIds.length} 个文件夹移动到新位置。
        </p>
        <div className="mt-4 space-y-2">
          <label htmlFor="target-dir" className="text-sm font-medium">
            目标目录
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
            className="rounded-lg border border-border px-4 py-2 text-sm font-medium transition hover:bg-accent"
          >
            取消
          </button>
          <button
            type="button"
            disabled={!targetDir.trim()}
            onClick={() => onConfirm(targetDir.trim())}
            className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition hover:bg-primary/90 disabled:opacity-50"
          >
            确认移动
          </button>
        </div>
      </div>
    </div>
  )
}

function ScanProgressBanner() {
  const isScanning = useFolderStore((s) => s.isScanning)
  const scanProgress = useFolderStore((s) => s.scanProgress)

  if (!isScanning) return null

  const scanned = scanProgress?.scanned ?? 0
  const total = scanProgress?.total ?? 0
  const pct = total > 0 ? Math.round((scanned / total) * 100) : 0

  return (
    <div className="rounded-lg border border-blue-200 bg-blue-50 px-4 py-3">
      <div className="flex items-center gap-2 text-sm text-blue-800">
        <Loader2 className="h-4 w-4 shrink-0 animate-spin" />
        <span className="font-medium">正在扫描</span>
        {scanProgress?.currentFolderName != null && (
          <span className="truncate text-blue-600">{scanProgress.currentFolderName}</span>
        )}
        <span className="ml-auto shrink-0 text-xs tabular-nums">
          {scanned}&nbsp;/&nbsp;{total > 0 ? total : '?'}
        </span>
      </div>
      {total > 0 && (
        <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-blue-200">
          <div
            className="h-full rounded-full bg-blue-500 transition-all duration-300"
            style={{ width: `${pct}%` }}
          />
        </div>
      )}
    </div>
  )
}

function JobItem({ job }: { job: Job }) {
  const pct = job.total > 0 ? Math.round((job.done / job.total) * 100) : 0
  const statusLabel = JOB_STATUS_LABEL[job.status] ?? job.status
  const statusColor = JOB_STATUS_COLOR[job.status] ?? 'bg-gray-100 text-gray-600'

  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between gap-2">
        <span className="truncate text-xs font-medium">
          {job.type === 'move' ? '移动任务' : job.type}
        </span>
        <span className={cn('shrink-0 rounded-full px-2 py-0.5 text-xs font-medium', statusColor)}>
          {statusLabel}
        </span>
      </div>
      {(job.status === 'running' || job.status === 'partial') && (
        <div className="h-1 w-full overflow-hidden rounded-full bg-muted">
          <div
            className="h-full rounded-full bg-primary transition-all duration-300"
            style={{ width: `${pct}%` }}
          />
        </div>
      )}
      <p className="text-xs text-muted-foreground">
        <span className="tabular-nums">{job.done}/{job.total} 个</span>
        {job.failed > 0 && <span className="text-red-500"> · {job.failed} 失败</span>}
        {job.created_at ? <span> · {formatRelativeTime(job.created_at)}</span> : null}
      </p>
    </div>
  )
}

function RecentJobsPanel() {
  const jobs = useJobStore((s) => s.jobs)
  const fetchJobs = useJobStore((s) => s.fetchJobs)

  useEffect(() => {
    void fetchJobs({ limit: 5 })
  }, [fetchJobs])

  return (
    <section className="rounded-lg border border-border bg-card p-4">
      <div className="mb-3 flex items-center gap-2">
        <Clock className="h-4 w-4 text-muted-foreground" />
        <h3 className="text-sm font-semibold">最近任务</h3>
      </div>
      {jobs.length === 0 ? (
        <p className="text-xs text-muted-foreground">暂无任务记录</p>
      ) : (
        <ul className="divide-y divide-border">
          {jobs.slice(0, 5).map((job) => (
            <li key={job.id} className="py-3 first:pt-0 last:pb-0">
              <JobItem job={job} />
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

function RecentLogsPanel() {
  const logs = useActivityStore((s) => s.logs)
  const fetchLogs = useActivityStore((s) => s.fetchLogs)

  useEffect(() => {
    void fetchLogs({ limit: 5 })
  }, [fetchLogs])

  return (
    <section className="rounded-lg border border-border bg-card p-4">
      <div className="mb-3 flex items-center gap-2">
        <FileText className="h-4 w-4 text-muted-foreground" />
        <h3 className="text-sm font-semibold">最近日志</h3>
      </div>
      {logs.length === 0 ? (
        <p className="text-xs text-muted-foreground">暂无操作日志</p>
      ) : (
        <ul className="divide-y divide-border">
          {logs.slice(0, 5).map((log) => (
            <li key={log.id} className="space-y-0.5 py-2 first:pt-0 last:pb-0">
              <div className="flex items-center justify-between gap-2">
                <span className="truncate text-xs font-medium">{log.action}</span>
                <span
                  className={cn(
                    'shrink-0 rounded px-1.5 py-0.5 text-xs',
                    log.result === 'success'
                      ? 'bg-green-100 text-green-700'
                      : log.result === 'failed'
                        ? 'bg-red-100 text-red-700'
                        : 'bg-gray-100 text-gray-600',
                  )}
                >
                  {log.result === 'success' ? '成功' : log.result === 'failed' ? '失败' : log.result}
                </span>
              </div>
              {log.folder_path ? (
                <p className="truncate text-xs text-muted-foreground">{log.folder_path}</p>
              ) : null}
              <p className="text-xs text-muted-foreground">{formatRelativeTime(log.created_at)}</p>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

interface FolderActionProps {
  folder: Folder
  selected: boolean
  onToggleSelect: () => void
  onSnapshot: () => void
  onUpdateCategory: (c: Category) => void
  onUpdateStatus: (s: FolderStatus) => void
  onRemove: () => void
  onRestore: () => void
}

function FolderCard({
  folder,
  selected,
  onToggleSelect,
  onSnapshot,
  onUpdateCategory,
  onUpdateStatus,
  onRemove,
  onRestore,
}: FolderActionProps) {
  const isDeleted = folder.deleted_at !== null

  return (
    <div
      className={cn(
        'flex flex-col rounded-xl border bg-card p-4 transition',
        selected ? 'border-primary ring-1 ring-primary' : 'border-border hover:border-primary/40',
        isDeleted && 'opacity-60',
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <label className="flex min-w-0 cursor-pointer items-center gap-2">
          <input
            type="checkbox"
            checked={selected}
            onChange={onToggleSelect}
            className="h-4 w-4 shrink-0 rounded border-gray-300"
          />
          <FolderOpen className="h-4 w-4 shrink-0 text-muted-foreground" />
          <span className="truncate text-sm font-semibold" title={folder.name}>
            {folder.name}
          </span>
        </label>
        {isDeleted && (
          <span className="shrink-0 rounded bg-red-100 px-1.5 py-0.5 text-xs text-red-700">已隐藏</span>
        )}
      </div>

      <div className="mt-2 flex flex-wrap gap-1.5">
        <span className={cn('rounded-full px-2 py-0.5 text-xs font-medium', CATEGORY_COLOR[folder.category])}>
          {CATEGORY_LABEL[folder.category]}
        </span>
        <span className={cn('rounded-full px-2 py-0.5 text-xs font-medium', STATUS_COLOR[folder.status])}>
          {STATUS_LABEL[folder.status]}
        </span>
        {folder.category_source === 'manual' && (
          <span className="rounded-full bg-indigo-100 px-2 py-0.5 text-xs text-indigo-700">手动</span>
        )}
      </div>

      <div className="mt-3 grid grid-cols-3 gap-1 text-center">
        <div>
          <p className="text-xs text-muted-foreground">图片</p>
          <p className="text-sm font-semibold tabular-nums">{folder.image_count}</p>
        </div>
        <div>
          <p className="text-xs text-muted-foreground">视频</p>
          <p className="text-sm font-semibold tabular-nums">{folder.video_count}</p>
        </div>
        <div>
          <p className="text-xs text-muted-foreground">大小</p>
          <p className="text-sm font-semibold">{formatBytes(folder.total_size)}</p>
        </div>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-1.5 border-t border-border pt-3">
        {isDeleted ? (
          <button
            type="button"
            onClick={onRestore}
            className="flex-1 rounded-lg border border-border px-2 py-1.5 text-xs font-medium transition hover:bg-accent"
          >
            恢复扫描
          </button>
        ) : (
          <>
            <select
              value={folder.category}
              onChange={(e) => onUpdateCategory(e.target.value as Category)}
              className="flex-1 rounded-lg border border-border bg-background px-2 py-1.5 text-xs outline-none focus:ring-1 focus:ring-primary"
              aria-label="更改分类"
            >
              {(['photo', 'video', 'mixed', 'manga', 'other'] as Category[]).map((c) => (
                <option key={c} value={c}>{CATEGORY_LABEL[c]}</option>
              ))}
            </select>
            <select
              value={folder.status}
              onChange={(e) => onUpdateStatus(e.target.value as FolderStatus)}
              className="flex-1 rounded-lg border border-border bg-background px-2 py-1.5 text-xs outline-none focus:ring-1 focus:ring-primary"
              aria-label="更改状态"
            >
              {(['pending', 'done', 'skip'] as FolderStatus[]).map((s) => (
                <option key={s} value={s}>{STATUS_LABEL[s]}</option>
              ))}
            </select>
            <button
              type="button"
              onClick={onSnapshot}
              title="查看快照时间线"
              className="rounded-lg border border-border p-1.5 text-xs transition hover:bg-accent"
            >
              <History className="h-3.5 w-3.5" />
            </button>
            <button
              type="button"
              onClick={onRemove}
              title="从软件中隐藏，不改动实际文件"
              className="rounded-lg border border-red-200 p-1.5 text-xs text-red-600 transition hover:bg-red-50"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </>
        )}
      </div>
    </div>
  )
}

function FolderRow({
  folder,
  selected,
  onToggleSelect,
  onSnapshot,
  onUpdateCategory,
  onUpdateStatus,
  onRemove,
  onRestore,
}: FolderActionProps) {
  const isDeleted = folder.deleted_at !== null

  return (
    <tr
      className={cn(
        'border-b border-border transition hover:bg-muted/40',
        isDeleted && 'opacity-60',
      )}
    >
      <td className="w-8 px-3 py-3">
        <input
          type="checkbox"
          checked={selected}
          onChange={onToggleSelect}
          className="h-4 w-4 rounded border-gray-300"
        />
      </td>
      <td className="px-3 py-3">
        <div className="flex items-center gap-2">
          <FolderOpen className="h-4 w-4 shrink-0 text-muted-foreground" />
          <span className="max-w-xs truncate text-sm font-medium" title={folder.name}>
            {folder.name}
          </span>
        </div>
      </td>
      <td className="px-3 py-3">
        <div className="flex flex-wrap gap-1">
          <span className={cn('rounded-full px-2 py-0.5 text-xs font-medium', CATEGORY_COLOR[folder.category])}>
            {CATEGORY_LABEL[folder.category]}
          </span>
          <span className={cn('rounded-full px-2 py-0.5 text-xs font-medium', STATUS_COLOR[folder.status])}>
            {STATUS_LABEL[folder.status]}
          </span>
        </div>
      </td>
      <td className="hidden px-3 py-3 text-xs text-muted-foreground sm:table-cell">
        <span className="tabular-nums">{folder.image_count} 图</span>
        <span className="mx-1">·</span>
        <span className="tabular-nums">{folder.video_count} 视</span>
      </td>
      <td className="hidden px-3 py-3 text-xs text-muted-foreground md:table-cell">
        {formatBytes(folder.total_size)}
      </td>
      <td className="px-3 py-3">
        <div className="flex items-center gap-1.5">
          {isDeleted ? (
            <button
              type="button"
              onClick={onRestore}
              className="rounded border border-border px-2 py-1 text-xs transition hover:bg-accent"
            >
              恢复扫描
            </button>
          ) : (
            <>
              <select
                value={folder.category}
                onChange={(e) => onUpdateCategory(e.target.value as Category)}
                className="rounded border border-border bg-background px-1.5 py-1 text-xs outline-none focus:ring-1 focus:ring-primary"
                aria-label="更改分类"
              >
                {(['photo', 'video', 'mixed', 'manga', 'other'] as Category[]).map((c) => (
                  <option key={c} value={c}>{CATEGORY_LABEL[c]}</option>
                ))}
              </select>
              <select
                value={folder.status}
                onChange={(e) => onUpdateStatus(e.target.value as FolderStatus)}
                className="rounded border border-border bg-background px-1.5 py-1 text-xs outline-none focus:ring-1 focus:ring-primary"
                aria-label="更改状态"
              >
                {(['pending', 'done', 'skip'] as FolderStatus[]).map((s) => (
                  <option key={s} value={s}>{STATUS_LABEL[s]}</option>
                ))}
              </select>
              <button
                type="button"
                onClick={onSnapshot}
                title="查看快照时间线"
                className="rounded border border-border p-1 text-xs transition hover:bg-accent"
              >
                <History className="h-3.5 w-3.5" />
              </button>
              <button
                type="button"
                onClick={onRemove}
                title="从软件中隐藏，不改动实际文件"
                className="rounded border border-red-200 p-1 text-xs text-red-600 transition hover:bg-red-50"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </>
          )}
        </div>
      </td>
    </tr>
  )
}

export default function FolderListPage() {
  const folders = useFolderStore((s) => s.folders)
  const total = useFolderStore((s) => s.total)
  const page = useFolderStore((s) => s.page)
  const limit = useFolderStore((s) => s.limit)
  const isLoading = useFolderStore((s) => s.isLoading)
  const error = useFolderStore((s) => s.error)
  const filters = useFolderStore((s) => s.filters)
  const isScanning = useFolderStore((s) => s.isScanning)
  const viewMode = useFolderStore((s) => s.viewMode)
  const fetchFolders = useFolderStore((s) => s.fetchFolders)
  const setFilters = useFolderStore((s) => s.setFilters)
  const setPage = useFolderStore((s) => s.setPage)
  const triggerScan = useFolderStore((s) => s.triggerScan)
  const setViewMode = useFolderStore((s) => s.setViewMode)
  const updateFolderCategory = useFolderStore((s) => s.updateFolderCategory)
  const updateFolderStatus = useFolderStore((s) => s.updateFolderStatus)
  const suppressFolder = useFolderStore((s) => s.suppressFolder)
  const unsuppressFolder = useFolderStore((s) => s.unsuppressFolder)

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [activeFolderId, setActiveFolderId] = useState<string | null>(null)
  const [showMoveModal, setShowMoveModal] = useState(false)
  const [moveError, setMoveError] = useState<string | null>(null)

  useEffect(() => {
    void fetchFolders()
  }, [fetchFolders, filters, page])

  const totalPages = Math.max(1, Math.ceil(total / limit))

  function toggleSelect(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function toggleSelectAll() {
    if (selectedIds.size === folders.length) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(folders.map((f) => f.id)))
    }
  }

  async function handleMove(targetDir: string) {
    setMoveError(null)
    try {
      await moveFolders([...selectedIds], targetDir)
      setSelectedIds(new Set())
      setShowMoveModal(false)
      void fetchFolders()
    } catch (e) {
      setMoveError(e instanceof Error ? e.message : '移动失败')
    }
  }

  return (
    <>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold tracking-tight">媒体文件夹</h1>
          <p className="mt-0.5 text-sm text-muted-foreground">
            共 {total} 个文件夹
            {selectedIds.size > 0 && <span className="ml-2 text-primary">· 已选 {selectedIds.size} 个</span>}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {selectedIds.size > 0 && (
            <button
              type="button"
              onClick={() => { setMoveError(null); setShowMoveModal(true) }}
              className="flex items-center gap-1.5 rounded-lg bg-primary px-3 py-2 text-sm font-medium text-primary-foreground transition hover:bg-primary/90"
            >
              移动所选
            </button>
          )}
          <button
            type="button"
            onClick={() => void triggerScan()}
            disabled={isScanning}
            className="flex items-center gap-1.5 rounded-lg border border-border px-3 py-2 text-sm font-medium transition hover:bg-accent disabled:opacity-50"
          >
            {isScanning ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Search className="h-4 w-4" />
            )}
            {isScanning ? '扫描中' : '扫描'}
          </button>
          <div className="flex rounded-lg border border-border">
            <button
              type="button"
              onClick={() => setViewMode('grid')}
              className={cn(
                'rounded-l-lg px-2.5 py-2 text-sm transition',
                viewMode === 'grid' ? 'bg-accent' : 'hover:bg-accent',
              )}
              title="网格视图"
            >
              <Grid2X2 className="h-4 w-4" />
            </button>
            <button
              type="button"
              onClick={() => setViewMode('list')}
              className={cn(
                'rounded-r-lg px-2.5 py-2 text-sm transition',
                viewMode === 'list' ? 'bg-accent' : 'hover:bg-accent',
              )}
              title="列表视图"
            >
              <List className="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>

      <ScanProgressBanner />

      <div className="mt-4 flex flex-wrap gap-2">
        {ALL_CATEGORIES.map((c) => (
          <button
            key={c}
            type="button"
            onClick={() => { setPage(1); setFilters({ ...filters, category: c === '' ? undefined : c }) }}
            className={cn(
              'rounded-full border px-3 py-1 text-xs font-medium transition',
              filters.category === (c === '' ? undefined : c)
                ? 'border-primary bg-primary text-primary-foreground'
                : 'border-border hover:bg-accent',
            )}
          >
            {CATEGORY_LABEL[c]}
          </button>
        ))}
        <div className="mx-1 w-px bg-border" />
        {ALL_STATUSES.map((s) => (
          <button
            key={s}
            type="button"
            onClick={() => { setPage(1); setFilters({ ...filters, status: s === '' ? undefined : s, onlyDeleted: undefined }) }}
            className={cn(
              'rounded-full border px-3 py-1 text-xs font-medium transition',
              !filters.onlyDeleted && filters.status === (s === '' ? undefined : s)
                ? 'border-primary bg-primary text-primary-foreground'
                : 'border-border hover:bg-accent',
            )}
          >
            {STATUS_LABEL[s]}
          </button>
        ))}
        <div className="mx-1 w-px bg-border" />
        <button
          type="button"
          onClick={() => { setPage(1); setFilters({ onlyDeleted: filters.onlyDeleted ? undefined : true }) }}
          className={cn(
            'rounded-full border px-3 py-1 text-xs font-medium transition',
            filters.onlyDeleted
              ? 'border-red-400 bg-red-100 text-red-700'
              : 'border-border hover:bg-accent',
          )}
        >
          已隐藏
        </button>
      </div>

      <div className="mt-4 flex flex-col gap-4 xl:flex-row">
        <div className="min-w-0 flex-1">
          {error != null && (
            <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
              {error}
            </div>
          )}
          {moveError != null && (
            <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
              {moveError}
            </div>
          )}

          {isLoading && folders.length === 0 ? (
            <div className="flex items-center justify-center py-20 text-muted-foreground">
              <Loader2 className="h-6 w-6 animate-spin" />
              <span className="ml-2 text-sm">加载中...</span>
            </div>
          ) : folders.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20 text-muted-foreground">
              <FolderOpen className="h-10 w-10 opacity-30" />
              <p className="mt-2 text-sm">暂无文件夹，请先扫描</p>
            </div>
          ) : viewMode === 'grid' ? (
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {folders.map((folder) => (
                <FolderCard
                  key={folder.id}
                  folder={folder}
                  selected={selectedIds.has(folder.id)}
                  onToggleSelect={() => toggleSelect(folder.id)}
                  onSnapshot={() => setActiveFolderId(folder.id)}
                  onUpdateCategory={(c) => void updateFolderCategory(folder.id, c)}
                  onUpdateStatus={(s) => void updateFolderStatus(folder.id, s)}
                  onRemove={() => void suppressFolder(folder.id)}
                  onRestore={() => void unsuppressFolder(folder.id)}
                />
              ))}
            </div>
          ) : (
            <div className="overflow-x-auto rounded-xl border border-border">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border bg-muted/40">
                    <th className="w-8 px-3 py-3">
                      <input
                        type="checkbox"
                        checked={selectedIds.size === folders.length && folders.length > 0}
                        onChange={toggleSelectAll}
                        className="h-4 w-4 rounded border-gray-300"
                        aria-label="全选"
                      />
                    </th>
                    <th className="px-3 py-3 text-left font-medium">名称</th>
                    <th className="px-3 py-3 text-left font-medium">分类 / 状态</th>
                    <th className="hidden px-3 py-3 text-left font-medium sm:table-cell">文件数</th>
                    <th className="hidden px-3 py-3 text-left font-medium md:table-cell">大小</th>
                    <th className="px-3 py-3 text-left font-medium">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {folders.map((folder) => (
                    <FolderRow
                      key={folder.id}
                      folder={folder}
                      selected={selectedIds.has(folder.id)}
                      onToggleSelect={() => toggleSelect(folder.id)}
                      onSnapshot={() => setActiveFolderId(folder.id)}
                      onUpdateCategory={(c) => void updateFolderCategory(folder.id, c)}
                      onUpdateStatus={(s) => void updateFolderStatus(folder.id, s)}
                      onRemove={() => void suppressFolder(folder.id)}
                      onRestore={() => void unsuppressFolder(folder.id)}
                    />
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {totalPages > 1 && (
            <div className="mt-4 flex items-center justify-center gap-2">
              <button
                type="button"
                disabled={page <= 1}
                onClick={() => setPage(page - 1)}
                className="rounded-lg border border-border px-3 py-1.5 text-sm transition hover:bg-accent disabled:opacity-40"
              >
                上一页
              </button>
              <span className="text-sm text-muted-foreground">
                第 {page} / {totalPages} 页
              </span>
              <button
                type="button"
                disabled={page >= totalPages}
                onClick={() => setPage(page + 1)}
                className="rounded-lg border border-border px-3 py-1.5 text-sm transition hover:bg-accent disabled:opacity-40"
              >
                下一页
              </button>
            </div>
          )}
        </div>

        <div className="flex w-full flex-col gap-4 xl:w-72 xl:shrink-0">
          <RecentJobsPanel />
          <RecentLogsPanel />
        </div>
      </div>

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
