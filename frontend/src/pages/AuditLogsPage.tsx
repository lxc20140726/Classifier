import { useEffect, useState } from 'react'
import { RefreshCw, Search } from 'lucide-react'

import { listAuditLogs } from '@/api/auditLogs'
import { cn } from '@/lib/utils'
import type { AuditLog } from '@/types'

interface AuditFilterState {
  jobId: string
  action: string
  result: string
  folderPath: string
  from: string
  to: string
}

const INITIAL_FILTERS: AuditFilterState = {
  jobId: '',
  action: '',
  result: '',
  folderPath: '',
  from: '',
  to: '',
}

const PAGE_SIZE = 50

export default function AuditLogsPage() {
  const [filters, setFilters] = useState<AuditFilterState>(INITIAL_FILTERS)
  const [appliedFilters, setAppliedFilters] = useState<AuditFilterState>(INITIAL_FILTERS)
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void fetchLogs(page, appliedFilters)
  }, [appliedFilters, page])

  async function fetchLogs(nextPage: number, nextFilters: AuditFilterState) {
    setIsLoading(true)
    setError(null)
    try {
        const response = await listAuditLogs({
        jobId: nextFilters.jobId || undefined,
        action: nextFilters.action || undefined,
        result: nextFilters.result || undefined,
        folderPath: nextFilters.folderPath || undefined,
        from: nextFilters.from || undefined,
        to: nextFilters.to || undefined,
        page: nextPage,
        limit: PAGE_SIZE,
      })
      setLogs(response.data)
      setTotal(response.total)
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : '加载审计日志失败')
    } finally {
      setIsLoading(false)
    }
  }

  async function handleSearch() {
    setAppliedFilters(filters)
    setPage(1)
    await fetchLogs(1, filters)
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <section className="mx-auto flex max-w-[1400px] flex-col gap-6 px-4 py-8">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <p className="text-xs uppercase tracking-[0.24em] text-muted-foreground">Audit Trail</p>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight">审计日志</h1>
          <p className="mt-1 text-sm text-muted-foreground">支持按任务、时间范围、动作、结果和路径关键词检索。</p>
        </div>
        <button
          type="button"
          onClick={() => void fetchLogs(page, appliedFilters)}
          disabled={isLoading}
          className="inline-flex items-center gap-2 rounded-xl border border-border bg-background px-3 py-2 text-sm transition hover:bg-accent disabled:opacity-60"
        >
          <RefreshCw className={cn('h-4 w-4', isLoading && 'animate-spin')} />
          刷新
        </button>
      </div>

      <div className="rounded-3xl border border-border bg-card p-4 shadow-sm">
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-6">
          <input
            value={filters.jobId}
            onChange={(event) => setFilters((prev) => ({ ...prev, jobId: event.target.value }))}
            placeholder="任务 ID"
            className="rounded-xl border border-border bg-background px-3 py-2 text-sm outline-none ring-primary focus:ring-2"
          />
          <input
            value={filters.folderPath}
            onChange={(event) => setFilters((prev) => ({ ...prev, folderPath: event.target.value }))}
            placeholder="路径关键词"
            className="rounded-xl border border-border bg-background px-3 py-2 text-sm outline-none ring-primary focus:ring-2"
          />
          <input
            value={filters.action}
            onChange={(event) => setFilters((prev) => ({ ...prev, action: event.target.value }))}
            placeholder="动作，如 move-node"
            className="rounded-xl border border-border bg-background px-3 py-2 text-sm outline-none ring-primary focus:ring-2"
          />
          <input
            value={filters.result}
            onChange={(event) => setFilters((prev) => ({ ...prev, result: event.target.value }))}
            placeholder="结果，如 success"
            className="rounded-xl border border-border bg-background px-3 py-2 text-sm outline-none ring-primary focus:ring-2"
          />
          <input
            type="datetime-local"
            value={filters.from}
            onChange={(event) => setFilters((prev) => ({ ...prev, from: event.target.value }))}
            className="rounded-xl border border-border bg-background px-3 py-2 text-sm outline-none ring-primary focus:ring-2"
          />
          <input
            type="datetime-local"
            value={filters.to}
            onChange={(event) => setFilters((prev) => ({ ...prev, to: event.target.value }))}
            className="rounded-xl border border-border bg-background px-3 py-2 text-sm outline-none ring-primary focus:ring-2"
          />
        </div>
        <div className="mt-3 flex justify-end">
          <button
            type="button"
            onClick={() => void handleSearch()}
            className="inline-flex items-center gap-2 rounded-xl bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition hover:opacity-90"
          >
            <Search className="h-4 w-4" />
            搜索
          </button>
        </div>
      </div>

      {error && (
        <div className="rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      )}

      <div className="overflow-hidden rounded-3xl border border-border bg-card shadow-sm">
        <table className="w-full text-sm">
          <thead className="bg-muted/40">
            <tr>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">时间</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">动作</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">结果</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">路径</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">目录ID</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">耗时</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr>
                <td colSpan={6} className="px-4 py-10 text-center text-muted-foreground">正在加载审计日志...</td>
              </tr>
            ) : logs.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-4 py-10 text-center text-muted-foreground">没有匹配的审计记录。</td>
              </tr>
            ) : (
              logs.map((log) => (
                <tr key={log.id} className="border-t border-border/60 align-top">
                  <td className="px-4 py-3 text-xs text-muted-foreground">{new Date(log.created_at).toLocaleString('zh-CN')}</td>
                  <td className="px-4 py-3 font-medium">{log.action}</td>
                  <td className="px-4 py-3">
                    <span className={cn(
                      'rounded-full px-2.5 py-1 text-xs font-medium',
                      log.result === 'success' || log.result === 'moved'
                        ? 'bg-emerald-100 text-emerald-700'
                        : 'bg-amber-100 text-amber-700',
                    )}>
                      {log.result || 'unknown'}
                    </span>
                  </td>
                  <td className="max-w-[420px] px-4 py-3 font-mono text-xs text-muted-foreground">{log.folder_path || '—'}</td>
                  <td className="px-4 py-3 font-mono text-xs text-muted-foreground">{log.folder_id || '—'}</td>
                  <td className="px-4 py-3 text-xs text-muted-foreground">{log.duration_ms} ms</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between rounded-2xl border border-border bg-card px-4 py-3 text-sm">
        <p className="text-muted-foreground">第 {page} / {totalPages} 页，共 {total} 条</p>
        <div className="flex gap-2">
          <button
            type="button"
            disabled={page <= 1 || isLoading}
            onClick={() => setPage((prev) => Math.max(1, prev - 1))}
            className="rounded-xl border border-border px-3 py-2 transition hover:bg-accent disabled:opacity-50"
          >
            上一页
          </button>
          <button
            type="button"
            disabled={page >= totalPages || isLoading}
            onClick={() => setPage((prev) => Math.min(totalPages, prev + 1))}
            className="rounded-xl border border-border px-3 py-2 transition hover:bg-accent disabled:opacity-50"
          >
            下一页
          </button>
        </div>
      </div>
    </section>
  )
}
