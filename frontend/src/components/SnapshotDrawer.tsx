import { useEffect, useMemo, useRef, useState } from 'react'
import { AlertTriangle, ChevronRight, RotateCcw, X } from 'lucide-react'

import { revertSnapshot, type RevertResult } from '@/api/snapshots'
import { listAuditLogs } from '@/api/auditLogs'
import { ApiRequestError } from '@/api/client'
import { cn } from '@/lib/utils'
import { useSnapshotStore } from '@/store/snapshotStore'
import type { AuditLog, Snapshot } from '@/types'

export interface SnapshotDrawerProps {
  open: boolean
  folderId: string | null
  onClose: () => void
}

interface DrawerState {
  revertingId: string | null
  lastAttemptedId: string | null
  localError: string | null
  failureDetail: RevertResult | null
}

const OP_LABELS: Record<string, string> = {
  classify: '分类记录',
  move: '移动记录',
  rename: '重命名记录',
}

const AUDIT_RESULT_LABELS: Record<string, string> = {
  success: '成功',
  succeeded: '成功',
  moved: '已移动',
  renamed: '已重命名',
  skipped: '已跳过',
  partial: '部分完成',
  failed: '失败',
}

const AUDIT_RESULT_CLASSES: Record<string, string> = {
  success: 'bg-green-300 text-black border-2 border-black',
  succeeded: 'bg-green-300 text-black border-2 border-black',
  moved: 'bg-primary text-primary-foreground border-2 border-black',
  renamed: 'bg-primary text-primary-foreground border-2 border-black',
  skipped: 'bg-muted text-muted-foreground border-2 border-black',
  partial: 'bg-yellow-300 text-black border-2 border-black',
  failed: 'bg-red-300 text-red-950 border-2 border-black',
}

const STATUS_LABELS: Record<Snapshot['status'], string> = {
  pending: '待完成',
  committed: '已提交',
  reverted: '已回退',
}

const STATUS_CLASSES: Record<Snapshot['status'], string> = {
  pending: 'bg-yellow-300 text-black border-2 border-black',
  committed: 'bg-primary text-primary-foreground border-2 border-black',
  reverted: 'bg-muted text-muted-foreground border-2 border-black',
}

function formatDate(value: string): string {
  if (!value) return '未知时间'
  return new Date(value).toLocaleString('zh-CN')
}

function parseTime(value: string): number {
  const ts = Date.parse(value)
  return Number.isNaN(ts) ? 0 : ts
}

function isProcessingAudit(log: AuditLog): boolean {
  const action = log.action.trim().toLowerCase()
  if (action.startsWith('phase4.processing.')) return true
  if (!action.startsWith('workflow.')) return false

  return (
    action.startsWith('workflow.move-node') ||
    action.startsWith('workflow.rename-node') ||
    action.startsWith('workflow.compress-node') ||
    action.startsWith('workflow.thumbnail-node') ||
    action.startsWith('workflow.audit-log')
  )
}

function resolveAuditActionLabel(action: string): string {
  const normalized = action.trim().toLowerCase()
  if (normalized.startsWith('workflow.move-node.rollback')) return '回滚记录（移动）'
  if (normalized.startsWith('workflow.rename-node.rollback')) return '回滚记录（重命名）'
  if (normalized.startsWith('workflow.compress-node.rollback')) return '回滚记录（压缩）'
  if (normalized.startsWith('workflow.thumbnail-node.rollback')) return '回滚记录（缩略图）'
  if (normalized.startsWith('workflow.move-node')) return '处理记录（移动）'
  if (normalized.startsWith('workflow.rename-node')) return '处理记录（重命名）'
  if (normalized.startsWith('workflow.compress-node')) return '处理记录（压缩）'
  if (normalized.startsWith('workflow.thumbnail-node')) return '处理记录（缩略图）'
  if (normalized.startsWith('workflow.audit-log')) return '处理记录（审计）'
  if (normalized.startsWith('phase4.processing.')) return '处理记录'

  return '处理记录'
}

function renderDetail(detail: Record<string, unknown> | null): Array<[string, string]> {
  if (detail == null) return []

  const entries: Array<[string, string]> = []
  const sourceDir = typeof detail.source_dir === 'string' ? detail.source_dir : null
  const relativePath = typeof detail.relative_path === 'string' ? detail.relative_path : null
  const category = typeof detail.category === 'string' ? detail.category : null
  const categorySource = typeof detail.category_source === 'string' ? detail.category_source : null
  const beforeCategory = typeof detail.before_category === 'string' ? detail.before_category : null
  const beforeCategorySource =
    typeof detail.before_category_source === 'string' ? detail.before_category_source : null
  const afterCategory = typeof detail.after_category === 'string' ? detail.after_category : null
  const afterCategorySource =
    typeof detail.after_category_source === 'string' ? detail.after_category_source : null
  const targetDir = typeof detail.target_dir === 'string' ? detail.target_dir : null
  const sourcePath = typeof detail.source_path === 'string' ? detail.source_path : null
  const folderPath = typeof detail.folder_path === 'string' ? detail.folder_path : null
  const workflowRunId = typeof detail.workflow_run_id === 'string' ? detail.workflow_run_id : null
  const nodeRunId = typeof detail.node_run_id === 'string' ? detail.node_run_id : null
  const nodeType = typeof detail.node_type === 'string' ? detail.node_type : null

  if (sourceDir) entries.push(['扫描目录', sourceDir])
  if (relativePath) entries.push(['相对路径', relativePath])
  if (category) entries.push(['分类结果', category])
  if (categorySource) entries.push(['分类来源', categorySource])
  if (beforeCategory) {
    entries.push([
      '变更前分类',
      beforeCategorySource ? `${beforeCategory}（${beforeCategorySource}）` : beforeCategory,
    ])
  }
  if (afterCategory) {
    entries.push([
      '变更后分类',
      afterCategorySource ? `${afterCategory}（${afterCategorySource}）` : afterCategory,
    ])
  }
  if (targetDir) entries.push(['输出目录', targetDir])
  if (sourcePath) entries.push(['原始路径', sourcePath])
  if (folderPath) entries.push(['目录路径', folderPath])
  if (workflowRunId) entries.push(['来源', '工作流'])
  if (workflowRunId) entries.push(['工作流运行ID', workflowRunId])
  if (nodeType) entries.push(['节点类型', nodeType])
  if (nodeRunId) entries.push(['节点运行ID', nodeRunId])

  return entries
}

function RevertFailurePanel({ detail }: { detail: RevertResult }) {
  return (
    <div className="mt-4 space-y-3 border-2 border-foreground bg-red-100 p-4 shadow-hard">
      <div className="flex items-start gap-2">
        <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-red-600" />
        <div className="space-y-1">
          <p className="text-sm font-bold text-red-900">回退失败</p>
          {detail.preflight_error && (
            <p className="text-xs font-medium text-red-800">{detail.preflight_error}</p>
          )}
          {detail.error_message && !detail.preflight_error && (
            <p className="text-xs font-medium text-red-800">{detail.error_message}</p>
          )}
        </div>
      </div>

      {detail.current_state.length > 0 && (
        <div className="space-y-3">
          <p className="text-xs font-bold text-red-900">目前文件状态（未变动）：</p>
          {detail.current_state.map((s, i) => (
            <div key={i} className="border-2 border-red-900 bg-white px-3 py-2 text-xs">
              <p className="font-bold text-red-900">当前位置</p>
              <p className="break-all font-mono text-foreground">{s.current_path}</p>
              {s.current_path !== s.original_path && (
                <>
                  <p className="mt-2 font-bold text-red-900">目标位置（未到达）</p>
                  <p className="break-all font-mono text-foreground">{s.original_path}</p>
                </>
              )}
            </div>
          ))}
        </div>
      )}

      <p className="text-xs font-bold text-red-700">✓ 回退失败不会导致文件丢失，所有文件保持在回退前位置。</p>
    </div>
  )
}

export function SnapshotDrawer({ open, folderId, onClose }: SnapshotDrawerProps) {
  const snapshots = useSnapshotStore((store) => store.snapshots)
  const isLoading = useSnapshotStore((store) => store.isLoading)
  const storeError = useSnapshotStore((store) => store.error)
  const fetchSnapshots = useSnapshotStore((store) => store.fetchSnapshots)
  const handleRevertDone = useSnapshotStore((store) => store.handleRevertDone)
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([])
  const [isAuditLoading, setIsAuditLoading] = useState(false)
  const [auditError, setAuditError] = useState<string | null>(null)
  const [state, setState] = useState<DrawerState>({
    revertingId: null,
    lastAttemptedId: null,
    localError: null,
    failureDetail: null,
  })

  const prevKeyRef = useRef<string | null>(null)
  const openKey = open ? folderId : null
  if (prevKeyRef.current !== openKey) {
    prevKeyRef.current = openKey
    if (openKey !== null) {
      setState({ revertingId: null, lastAttemptedId: null, localError: null, failureDetail: null })
      setAuditLogs([])
      setIsAuditLoading(false)
      setAuditError(null)
    }
  }

  useEffect(() => {
    if (open && folderId) {
      void fetchSnapshots(folderId)
    }
  }, [fetchSnapshots, folderId, open])

  useEffect(() => {
    if (!open || !folderId) return

    let cancelled = false
    setIsAuditLoading(true)
    setAuditError(null)

    void (async () => {
      try {
        const response = await listAuditLogs({ folderId, page: 1, limit: 200 })
        if (cancelled) return
        setAuditLogs((response.data ?? []).filter(isProcessingAudit))
      } catch (error) {
        if (cancelled) return
        setAuditLogs([])
        setAuditError(error instanceof Error ? error.message : '处理记录加载失败')
      } finally {
        if (!cancelled) {
          setIsAuditLoading(false)
        }
      }
    })()

    return () => {
      cancelled = true
    }
  }, [folderId, open])

  async function handleRevert(snapshotId: string) {
    if (!folderId) return

    setState({ revertingId: snapshotId, lastAttemptedId: snapshotId, localError: null, failureDetail: null })

    try {
      await revertSnapshot(snapshotId)
      handleRevertDone(snapshotId)
      await fetchSnapshots(folderId)
      setState({ revertingId: null, lastAttemptedId: snapshotId, localError: null, failureDetail: null })
    } catch (error) {
      if (error instanceof ApiRequestError && error.status === 422 && error.body) {
        const revertResult = error.body.revert_result as RevertResult | undefined
        setState({
          revertingId: null,
          lastAttemptedId: snapshotId,
          localError: error.message,
          failureDetail: revertResult ?? null,
        })
      } else {
        setState({
          revertingId: null,
          lastAttemptedId: snapshotId,
          localError: error instanceof Error ? error.message : '回退失败',
          failureDetail: null,
        })
      }
    }
  }

  const error = state.localError ?? storeError
  const timelineItems = useMemo(() => {
    const snapshotItems = snapshots.map((snapshot) => ({
      id: `snapshot-${snapshot.id}`,
      createdAt: snapshot.created_at,
      kind: 'snapshot' as const,
      snapshot,
    }))
    const auditItems = auditLogs.map((audit) => ({
      id: `audit-${audit.id}`,
      createdAt: audit.created_at,
      kind: 'audit' as const,
      audit,
    }))

    return [...snapshotItems, ...auditItems].sort((a, b) => parseTime(b.createdAt) - parseTime(a.createdAt))
  }, [auditLogs, snapshots])

  return (
    <>
      <div
        className={cn(
          'fixed inset-0 z-40 bg-black/50 transition-opacity',
          open ? 'pointer-events-auto opacity-100' : 'pointer-events-none opacity-0',
        )}
        onClick={onClose}
        aria-hidden="true"
      />

      <aside
        className={cn(
          'fixed right-0 top-0 z-50 flex h-full w-full max-w-xl flex-col border-l-4 border-foreground bg-background shadow-[-8px_0_0_rgba(0,0,0,1)] transition-transform duration-300 ease-out',
          open ? 'translate-x-0' : 'translate-x-full',
        )}
        aria-label="快照时间线"
      >
        <div className="border-b-2 border-foreground bg-primary px-6 py-5 text-primary-foreground">
          <div className="flex items-start justify-between gap-4">
            <div>
              <p className="text-xs font-bold uppercase tracking-[0.24em]">Snapshots</p>
              <h2 className="mt-2 text-xl font-black tracking-tight">文件夹操作时间线</h2>
              <p className="mt-1 text-sm font-medium">
                按时间查看分类、移动和回退记录。
              </p>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="border-2 border-transparent p-2 transition-all hover:border-primary-foreground hover:bg-foreground hover:text-background"
              aria-label="关闭快照抽屉"
            >
              <X className="h-5 w-5" />
            </button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto px-6 py-6 bg-background">
          {error && !state.failureDetail && (
            <div className="mb-6 border-2 border-foreground bg-red-100 px-4 py-3 text-sm font-bold text-red-900 shadow-hard">
              {error}
            </div>
          )}
          {auditError && (
            <div className="mb-6 border-2 border-foreground bg-yellow-100 px-4 py-3 text-sm font-bold text-yellow-900 shadow-hard">
              处理记录加载失败：{auditError}
            </div>
          )}

          {(isLoading || isAuditLoading) && (
            <p className="text-sm font-bold text-muted-foreground">正在加载时间线记录...</p>
          )}

          {!isLoading && !isAuditLoading && !error && timelineItems.length === 0 && (
            <div className="border-2 border-dashed border-foreground px-4 py-12 text-center text-sm font-bold text-muted-foreground">
              这个文件夹还没有快照记录。
            </div>
          )}

          {timelineItems.length > 0 && (
            <ol className="relative space-y-8 pl-8 before:absolute before:left-[15px] before:top-2 before:h-[calc(100%-0.5rem)] before:w-0.5 before:bg-foreground">
              {timelineItems.map((item) => {
                if (item.kind === 'snapshot') {
                  const snapshot = item.snapshot
                  const detailItems = renderDetail(snapshot.detail)
                  const operationLabel = OP_LABELS[snapshot.operation_type] ?? snapshot.operation_type
                  const isReverting = state.revertingId === snapshot.id

                  return (
                    <li key={item.id} className="relative">
                      <span className="absolute left-[-32px] top-1.5 h-4 w-4 rounded-full border-2 border-foreground bg-primary" />
                      <div className="border-2 border-foreground bg-card p-5 shadow-hard transition-all hover:-translate-y-1 hover:shadow-hard-hover">
                        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                          <div className="space-y-2">
                            <div className="flex flex-wrap items-center gap-3">
                              <span className="text-base font-black tracking-tight">{operationLabel}</span>
                              <span
                                className={cn(
                                  'px-2 py-0.5 text-xs font-bold',
                                  STATUS_CLASSES[snapshot.status],
                                )}
                              >
                                {STATUS_LABELS[snapshot.status]}
                              </span>
                            </div>
                            <p className="text-xs font-mono font-bold text-muted-foreground">{formatDate(snapshot.created_at)}</p>
                            <div className="flex flex-wrap items-center gap-3 text-xs font-bold text-muted-foreground">
                              <span>前状态 {snapshot.before.length} 条</span>
                              {snapshot.after != null && <span>后状态 {snapshot.after.length} 条</span>}
                            </div>
                          </div>

                          {snapshot.status === 'committed' && (
                            <button
                              type="button"
                              disabled={state.revertingId !== null}
                              onClick={() => void handleRevert(snapshot.id)}
                              className="inline-flex items-center gap-1.5 border-2 border-foreground bg-background px-3 py-2 text-xs font-bold transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-background disabled:hover:text-foreground disabled:hover:shadow-none disabled:hover:translate-y-0"
                            >
                              <RotateCcw className="h-4 w-4" />
                              {isReverting ? '回退中...' : '回退到此节点'}
                            </button>
                          )}
                        </div>

                        {state.failureDetail && state.revertingId === null && snapshot.id === state.lastAttemptedId && (
                          <RevertFailurePanel detail={state.failureDetail} />
                        )}

                        {detailItems.length > 0 && (
                          <dl className="mt-5 grid gap-3 border-2 border-foreground bg-muted/30 p-4 text-xs sm:grid-cols-2">
                            {detailItems.map(([label, value]) => (
                              <div key={`${snapshot.id}-${label}`}>
                                <dt className="font-bold text-muted-foreground">{label}</dt>
                                <dd className="mt-1 break-all font-mono font-medium text-foreground">{value}</dd>
                              </div>
                            ))}
                          </dl>
                        )}

                        {snapshot.after != null && snapshot.after.length > 0 && (
                          <div className="mt-5 space-y-3 text-xs">
                            <p className="font-black text-foreground">路径变化</p>
                            {snapshot.after.slice(0, 3).map((record, index) => (
                              <div key={`${snapshot.id}-${record.current_path}-${index}`} className="border-2 border-foreground bg-background px-3 py-3 font-mono">
                                <div className="break-all text-muted-foreground">{record.original_path}</div>
                                <div className="my-2 flex items-center gap-1 font-bold text-primary">
                                  <ChevronRight className="h-4 w-4" />
                                  <span>变更后</span>
                                </div>
                                <div className="break-all font-bold text-foreground">{record.current_path}</div>
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    </li>
                  )
                }

                const audit = item.audit
                const result = audit.result.trim().toLowerCase()
                const resultLabel = AUDIT_RESULT_LABELS[result] ?? (audit.result.trim() || '已记录')
                const resultClass = AUDIT_RESULT_CLASSES[result] ?? 'bg-muted text-muted-foreground border-2 border-black'
                const operationLabel = resolveAuditActionLabel(audit.action)
                const detail = audit.detail != null ? { ...audit.detail } : {}
                if (audit.folder_path !== '') {
                  detail.folder_path = audit.folder_path
                }
                const detailItems = renderDetail(detail)

                return (
                  <li key={item.id} className="relative">
                    <span className="absolute left-[-32px] top-1.5 h-4 w-4 rounded-full border-2 border-foreground bg-primary" />
                    <div className="border-2 border-foreground bg-card p-5 shadow-hard transition-all hover:-translate-y-1 hover:shadow-hard-hover">
                      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                        <div className="space-y-2">
                          <div className="flex flex-wrap items-center gap-3">
                            <span className="text-base font-black tracking-tight">{operationLabel}</span>
                            <span
                              className={cn(
                                'px-2 py-0.5 text-xs font-bold',
                                resultClass,
                              )}
                            >
                              {resultLabel}
                            </span>
                          </div>
                          <p className="text-xs font-mono font-bold text-muted-foreground">{formatDate(audit.created_at)}</p>
                        </div>
                      </div>

                      {detailItems.length > 0 && (
                        <dl className="mt-5 grid gap-3 border-2 border-foreground bg-muted/30 p-4 text-xs sm:grid-cols-2">
                          {detailItems.map(([label, value]) => (
                            <div key={`${audit.id}-${label}`}>
                              <dt className="font-bold text-muted-foreground">{label}</dt>
                              <dd className="mt-1 break-all font-mono font-medium text-foreground">{value}</dd>
                            </div>
                          ))}
                        </dl>
                      )}

                      {audit.error_msg !== '' && (
                        <div className="mt-5 border-2 border-foreground bg-red-100 px-3 py-2 text-xs font-bold text-red-900">
                          {audit.error_msg}
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
