import { useEffect, useMemo, useState } from 'react'
import { ChevronRight, RotateCcw, X } from 'lucide-react'

import { revertSnapshot } from '@/api/snapshots'
import { cn } from '@/lib/utils'
import { useSnapshotStore } from '@/store/snapshotStore'
import type { Snapshot } from '@/types'

export interface SnapshotDrawerProps {
  open: boolean
  folderId: string | null
  onClose: () => void
}

interface DrawerState {
  revertingId: string | null
  localError: string | null
}

const OP_LABELS: Record<string, string> = {
  classify: '分类记录',
  move: '移动记录',
  rename: '重命名记录',
}

const STATUS_LABELS: Record<Snapshot['status'], string> = {
  pending: '待完成',
  committed: '已提交',
  reverted: '已回退',
}

const STATUS_CLASSES: Record<Snapshot['status'], string> = {
  pending: 'bg-yellow-100 text-yellow-800',
  committed: 'bg-green-100 text-green-800',
  reverted: 'bg-gray-100 text-gray-500',
}

function formatDate(value: string): string {
  if (!value) return '未知时间'
  return new Date(value).toLocaleString('zh-CN')
}

function renderDetail(detail: Record<string, unknown> | null): Array<[string, string]> {
  if (detail == null) {
    return []
  }

  const entries: Array<[string, string]> = []
  const sourceDir = typeof detail.source_dir === 'string' ? detail.source_dir : null
  const relativePath = typeof detail.relative_path === 'string' ? detail.relative_path : null
  const category = typeof detail.category === 'string' ? detail.category : null
  const targetDir = typeof detail.target_dir === 'string' ? detail.target_dir : null
  const sourcePath = typeof detail.source_path === 'string' ? detail.source_path : null

  if (sourceDir) entries.push(['扫描目录', sourceDir])
  if (relativePath) entries.push(['相对路径', relativePath])
  if (category) entries.push(['分类结果', category])
  if (targetDir) entries.push(['输出目录', targetDir])
  if (sourcePath) entries.push(['原始路径', sourcePath])

  return entries
}

export function SnapshotDrawer({ open, folderId, onClose }: SnapshotDrawerProps) {
  const snapshots = useSnapshotStore((store) => store.snapshots)
  const isLoading = useSnapshotStore((store) => store.isLoading)
  const storeError = useSnapshotStore((store) => store.error)
  const fetchSnapshots = useSnapshotStore((store) => store.fetchSnapshots)
  const handleRevertDone = useSnapshotStore((store) => store.handleRevertDone)
  const [state, setState] = useState<DrawerState>({ revertingId: null, localError: null })

  useEffect(() => {
    if (open && folderId) {
      void fetchSnapshots(folderId)
    }
  }, [fetchSnapshots, folderId, open])

  async function handleRevert(snapshotId: string) {
    if (!folderId) return

    setState({ revertingId: snapshotId, localError: null })

    try {
      await revertSnapshot(snapshotId)
      handleRevertDone(snapshotId)
      await fetchSnapshots(folderId)
      setState({ revertingId: null, localError: null })
    } catch (error) {
      setState({
        revertingId: null,
        localError: error instanceof Error ? error.message : '回退失败',
      })
    }
  }

  const error = state.localError ?? storeError
  const orderedSnapshots = useMemo(() => [...snapshots].reverse(), [snapshots])

  return (
    <>
      <div
        className={cn(
          'fixed inset-0 z-40 bg-black/45 transition-opacity',
          open ? 'pointer-events-auto opacity-100' : 'pointer-events-none opacity-0',
        )}
        onClick={onClose}
        aria-hidden="true"
      />

      <aside
        className={cn(
          'fixed right-0 top-0 z-50 flex h-full w-full max-w-xl flex-col border-l border-border bg-background shadow-2xl transition-transform duration-200',
          open ? 'translate-x-0' : 'translate-x-full',
        )}
        aria-label="快照时间线"
      >
        <div className="border-b border-border bg-card px-5 py-4">
          <div className="flex items-start justify-between gap-4">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-muted-foreground">Snapshots</p>
              <h2 className="mt-2 text-lg font-semibold">文件夹操作时间线</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                按时间查看分类、移动和回退记录。
              </p>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg border border-border p-2 transition hover:bg-accent"
              aria-label="关闭快照抽屉"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-5">
          {error && (
            <div className="mb-4 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
              {error}
            </div>
          )}

          {isLoading && <p className="text-sm text-muted-foreground">正在加载快照记录...</p>}

          {!isLoading && !error && orderedSnapshots.length === 0 && (
            <div className="rounded-xl border border-dashed border-border px-4 py-10 text-center text-sm text-muted-foreground">
              这个文件夹还没有快照记录。
            </div>
          )}

          {orderedSnapshots.length > 0 && (
            <ol className="relative space-y-6 pl-6 before:absolute before:left-[9px] before:top-2 before:h-[calc(100%-0.5rem)] before:w-px before:bg-border">
              {orderedSnapshots.map((snapshot) => {
                const detailItems = renderDetail(snapshot.detail)
                const operationLabel = OP_LABELS[snapshot.operation_type] ?? snapshot.operation_type

                return (
                  <li key={snapshot.id} className="relative">
                    <span className="absolute left-[-24px] top-1.5 h-4 w-4 rounded-full border-4 border-background bg-primary" />
                    <div className="rounded-2xl border border-border bg-card p-4 shadow-sm">
                      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                        <div className="space-y-2">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="text-sm font-semibold">{operationLabel}</span>
                            <span
                              className={cn(
                                'rounded-full px-2 py-0.5 text-xs font-medium',
                                STATUS_CLASSES[snapshot.status],
                              )}
                            >
                              {STATUS_LABELS[snapshot.status]}
                            </span>
                          </div>
                          <p className="text-xs text-muted-foreground">{formatDate(snapshot.created_at)}</p>
                          <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                            <span>前状态 {snapshot.before.length} 条</span>
                            {snapshot.after != null && <span>后状态 {snapshot.after.length} 条</span>}
                          </div>
                        </div>

                        {snapshot.status !== 'reverted' && snapshot.operation_type === 'move' && (
                          <button
                            type="button"
                            disabled={state.revertingId !== null}
                            onClick={() => void handleRevert(snapshot.id)}
                            className="inline-flex items-center gap-1 rounded-lg border border-border px-3 py-1.5 text-xs font-medium transition hover:bg-accent disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            <RotateCcw className="h-3.5 w-3.5" />
                            {state.revertingId === snapshot.id ? '回退中...' : '回退到此节点'}
                          </button>
                        )}
                      </div>

                      {detailItems.length > 0 && (
                        <dl className="mt-4 grid gap-2 rounded-xl bg-muted/50 p-3 text-xs sm:grid-cols-2">
                          {detailItems.map(([label, value]) => (
                            <div key={`${snapshot.id}-${label}`}>
                              <dt className="text-muted-foreground">{label}</dt>
                              <dd className="mt-0.5 break-all text-foreground">{value}</dd>
                            </div>
                          ))}
                        </dl>
                      )}

                      {snapshot.after != null && snapshot.after.length > 0 && (
                        <div className="mt-4 space-y-2 text-xs text-muted-foreground">
                          <p className="font-medium text-foreground">路径变化</p>
                          {snapshot.after.slice(0, 3).map((record, index) => (
                            <div key={`${snapshot.id}-${record.current_path}-${index}`} className="rounded-lg border border-border px-3 py-2">
                              <div className="break-all">{record.original_path}</div>
                              <div className="my-1 flex items-center gap-1 text-primary">
                                <ChevronRight className="h-3.5 w-3.5" />
                                <span>变更后</span>
                              </div>
                              <div className="break-all">{record.current_path}</div>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  </li>
                )
              })}
            </ol>
          )}
        </div>
      </aside>
    </>
  )
}
