import { useEffect, useState } from 'react'

import { ChevronDown, ChevronRight, RefreshCw } from 'lucide-react'

import { cn } from '@/lib/utils'
import { useJobStore } from '@/store/jobStore'
import { useWorkflowRunStore } from '@/store/workflowRunStore'
import type { Job, JobStatus, NodeRun, NodeRunStatus, WorkflowRun, WorkflowRunStatus } from '@/types'

function formatDate(dateStr: string | null) {
  if (!dateStr) return '—'
  return new Date(dateStr).toLocaleString('zh-CN')
}

function formatDuration(startedAt: string | null, finishedAt: string | null) {
  if (!startedAt) return '—'
  const end = finishedAt ? new Date(finishedAt) : new Date()
  const start = new Date(startedAt)
  const secs = Math.floor((end.getTime() - start.getTime()) / 1000)
  if (secs < 60) return `${secs} 秒`
  if (secs < 3600) return `${Math.floor(secs / 60)} 分 ${secs % 60} 秒`
  return `${Math.floor(secs / 3600)} 小时 ${Math.floor((secs % 3600) / 60)} 分`
}

const JOB_STATUS_LABELS: Record<JobStatus, string> = {
  pending: '等待中',
  running: '进行中',
  succeeded: '已完成',
  failed: '失败',
  partial: '部分完成',
  cancelled: '已取消',
}

const JOB_STATUS_STYLES: Record<JobStatus, string> = {
  pending: 'bg-muted text-muted-foreground',
  running: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300',
  succeeded: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300',
  failed: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300',
  partial: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-300',
  cancelled: 'bg-muted text-muted-foreground',
}

const WF_STATUS_LABELS: Record<WorkflowRunStatus, string> = {
  pending: '等待中',
  running: '进行中',
  succeeded: '已完成',
  failed: '失败',
  partial: '部分完成',
  waiting_input: '待确认',
}

const WF_STATUS_STYLES: Record<WorkflowRunStatus, string> = {
  pending: 'bg-muted text-muted-foreground',
  running: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300',
  succeeded: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300',
  failed: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300',
  partial: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-300',
  waiting_input: 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-300',
}

const NODE_STATUS_LABELS: Record<NodeRunStatus, string> = {
  pending: '等待中',
  running: '进行中',
  succeeded: '已完成',
  failed: '失败',
  skipped: '已跳过',
  waiting_input: '待确认',
}

const NODE_STATUS_STYLES: Record<NodeRunStatus, string> = {
  pending: 'bg-muted text-muted-foreground',
  running: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300',
  succeeded: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300',
  failed: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300',
  skipped: 'bg-muted text-muted-foreground',
  waiting_input: 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-300',
}

const CATEGORY_LABELS: Record<string, string> = {
  photo: '照片',
  video: '视频',
  manga: '漫画',
  mixed: '混合',
  other: '其他',
}

function StatusBadge({ status, labels, styles }: {
  status: string
  labels: Record<string, string>
  styles: Record<string, string>
}) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
        styles[status] ?? 'bg-muted text-muted-foreground',
      )}
    >
      {labels[status] ?? status}
    </span>
  )
}

function ProgressBar({ done, total }: { done: number; total: number }) {
  const pct = total > 0 ? Math.round((done / total) * 100) : 0
  return (
    <div className="flex items-center gap-2">
      <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
        <div
          className="h-full rounded-full bg-primary transition-all duration-300"
          style={{ width: `${pct}%` }}
        />
      </div>
      <span className="min-w-[3rem] text-right text-xs text-muted-foreground">
        {done}/{total}
      </span>
    </div>
  )
}

function NodeRunsPanel({ runId }: { runId: string }) {
  const { nodesByRunId, fetchRunDetail } = useWorkflowRunStore()
  const nodes = nodesByRunId[runId] ?? []

  useEffect(() => {
    void fetchRunDetail(runId)
  }, [runId, fetchRunDetail])

  if (nodes.length === 0) {
    return <p className="py-2 text-xs text-muted-foreground">暂无节点记录</p>
  }

  return (
    <table className="w-full text-xs">
      <thead>
        <tr className="border-b border-border">
          <th className="py-1 pr-4 text-left font-medium text-muted-foreground">节点ID</th>
          <th className="py-1 pr-4 text-left font-medium text-muted-foreground">类型</th>
          <th className="py-1 pr-4 text-left font-medium text-muted-foreground">序号</th>
          <th className="py-1 pr-4 text-left font-medium text-muted-foreground">状态</th>
          <th className="py-1 text-left font-medium text-muted-foreground">耗时</th>
        </tr>
      </thead>
      <tbody>
        {nodes.map((node: NodeRun) => (
          <tr key={node.id || node.node_id} className="border-b border-border/50">
            <td className="py-1 pr-4 font-mono">{node.node_id}</td>
            <td className="py-1 pr-4">{node.node_type}</td>
            <td className="py-1 pr-4">{node.sequence}</td>
            <td className="py-1 pr-4">
              <StatusBadge status={node.status} labels={NODE_STATUS_LABELS} styles={NODE_STATUS_STYLES} />
            </td>
            <td className="py-1">{formatDuration(node.started_at, node.finished_at)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function WorkflowRunRow({ run }: { run: WorkflowRun }) {
  const [expanded, setExpanded] = useState(false)
  const [selectedCategory, setSelectedCategory] = useState<string>('photo')
  const [isActing, setIsActing] = useState(false)
  const { rollbackRun, provideInput } = useWorkflowRunStore()

  async function handleRollback() {
    setIsActing(true)
    try {
      await rollbackRun(run.id)
    } finally {
      setIsActing(false)
    }
  }

  async function handleProvideInput() {
    setIsActing(true)
    try {
      await provideInput(run.id, selectedCategory as 'photo' | 'video' | 'manga' | 'mixed' | 'other')
    } finally {
      setIsActing(false)
    }
  }

  return (
    <>
      <tr
        className="cursor-pointer border-b border-border/50 transition-colors hover:bg-muted/20"
        onClick={() => setExpanded((v) => !v)}
      >
        <td className="py-1.5 pl-2 pr-3">
          {expanded ? <ChevronDown className="h-3 w-3 text-muted-foreground" /> : <ChevronRight className="h-3 w-3 text-muted-foreground" />}
        </td>
        <td className="py-1.5 pr-4 font-mono text-xs">{run.folder_id.slice(0, 8)}</td>
        <td className="py-1.5 pr-4">
          <StatusBadge status={run.status} labels={WF_STATUS_LABELS} styles={WF_STATUS_STYLES} />
        </td>
        <td className="py-1.5 pr-4 text-xs text-muted-foreground">{formatDate(run.created_at)}</td>
        <td className="py-1.5" onClick={(e) => e.stopPropagation()}>
          <div className="flex items-center gap-2">
            {(run.status === 'failed' || run.status === 'partial') && (
              <button
                type="button"
                disabled={isActing}
                onClick={() => void handleRollback()}
                className="rounded bg-red-100 px-2 py-0.5 text-xs text-red-700 hover:bg-red-200 disabled:opacity-50 dark:bg-red-900/30 dark:text-red-300"
              >
                回滚
              </button>
            )}
            {run.status === 'waiting_input' && (
              <div className="flex items-center gap-1">
                <select
                  value={selectedCategory}
                  onChange={(e) => setSelectedCategory(e.target.value)}
                  className="rounded border border-border bg-background px-1 py-0.5 text-xs"
                >
                  {Object.entries(CATEGORY_LABELS).map(([val, label]) => (
                    <option key={val} value={val}>{label}</option>
                  ))}
                </select>
                <button
                  type="button"
                  disabled={isActing}
                  onClick={() => void handleProvideInput()}
                  className="rounded bg-purple-100 px-2 py-0.5 text-xs text-purple-700 hover:bg-purple-200 disabled:opacity-50 dark:bg-purple-900/30 dark:text-purple-300"
                >
                  确认
                </button>
              </div>
            )}
          </div>
        </td>
      </tr>
      {expanded && (
        <tr className="border-b border-border/30 bg-muted/10">
          <td colSpan={5} className="px-6 py-3">
            <NodeRunsPanel runId={run.id} />
          </td>
        </tr>
      )}
    </>
  )
}

function WorkflowRunsPanel({ jobId }: { jobId: string }) {
  const { runsByJobId, fetchRunsForJob } = useWorkflowRunStore()
  const runs = runsByJobId[jobId] ?? []

  useEffect(() => {
    void fetchRunsForJob(jobId)
  }, [jobId, fetchRunsForJob])

  if (runs.length === 0) {
    return <p className="text-xs text-muted-foreground">暂无工作流运行记录</p>
  }

  return (
    <div>
      <p className="mb-2 text-xs font-medium text-muted-foreground">工作流运行（{runs.length}）</p>
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-border">
            <th className="w-6" />
            <th className="py-1 pr-4 text-left font-medium text-muted-foreground">目录ID</th>
            <th className="py-1 pr-4 text-left font-medium text-muted-foreground">状态</th>
            <th className="py-1 pr-4 text-left font-medium text-muted-foreground">创建时间</th>
            <th className="py-1 text-left font-medium text-muted-foreground">操作</th>
          </tr>
        </thead>
        <tbody>
          {runs.map((run: WorkflowRun) => (
            <WorkflowRunRow key={run.id} run={run} />
          ))}
        </tbody>
      </table>
    </div>
  )
}

function JobRow({ job }: { job: Job }) {
  const [expanded, setExpanded] = useState(false)

  return (
    <>
      <tr
        className="cursor-pointer border-b border-border transition-colors hover:bg-muted/40"
        onClick={() => setExpanded((v) => !v)}
      >
        <td className="px-4 py-3">
          <button
            type="button"
            className="flex items-center text-muted-foreground"
            aria-label={expanded ? '收起' : '展开'}
          >
            {expanded ? (
              <ChevronDown className="h-4 w-4" />
            ) : (
              <ChevronRight className="h-4 w-4" />
            )}
          </button>
        </td>
        <td className="px-4 py-3 font-mono text-xs text-muted-foreground">{job.id.slice(0, 8)}</td>
        <td className="px-4 py-3 text-sm">{job.type}</td>
        <td className="px-4 py-3">
          <StatusBadge status={job.status} labels={JOB_STATUS_LABELS} styles={JOB_STATUS_STYLES} />
        </td>
        <td className="w-48 px-4 py-3">
          <ProgressBar done={job.done} total={job.total} />
        </td>
        <td className="px-4 py-3 text-sm text-muted-foreground">{job.folder_ids.length}</td>
        <td className="px-4 py-3 text-sm text-muted-foreground">{formatDate(job.created_at)}</td>
        <td className="px-4 py-3 text-sm text-muted-foreground">
          {formatDuration(job.started_at, job.finished_at)}
        </td>
      </tr>
      {expanded && (
        <tr className="border-b border-border bg-muted/20">
          <td colSpan={8} className="px-8 py-4">
            <WorkflowRunsPanel jobId={job.id} />
          </td>
        </tr>
      )}
    </>
  )
}

export default function JobsPage() {
  const { jobs, total, isLoading, error, fetchJobs } = useJobStore()

  useEffect(() => {
    void fetchJobs()
  }, [fetchJobs])

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">任务历史</h1>
          <p className="mt-0.5 text-sm text-muted-foreground">
            共 {total} 条任务记录
          </p>
        </div>
        <button
          type="button"
          onClick={() => void fetchJobs()}
          disabled={isLoading}
          className="flex items-center gap-2 rounded-md border border-input bg-background px-3 py-1.5 text-sm font-medium shadow-sm transition-colors hover:bg-accent disabled:opacity-50"
        >
          <RefreshCw className={cn('h-3.5 w-3.5', isLoading && 'animate-spin')} />
           刷新
        </button>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      <div className="overflow-hidden rounded-lg border border-border bg-card">
        <table className="w-full">
          <thead>
            <tr className="border-b border-border bg-muted/50">
              <th className="w-10 px-4 py-3" />
              <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                 ID
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                 类型
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                 状态
              </th>
              <th className="w-48 px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                 进度
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                 目录数
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                 创建时间
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                 耗时
              </th>
            </tr>
          </thead>
          <tbody>
            {isLoading && jobs.length === 0 ? (
              <tr>
                <td colSpan={8} className="px-4 py-12 text-center text-sm text-muted-foreground">
                   正在加载任务...
                </td>
              </tr>
            ) : jobs.length === 0 ? (
              <tr>
                <td colSpan={8} className="px-4 py-12 text-center text-sm text-muted-foreground">
                   暂无任务记录，扫描和移动后会显示在这里。
                </td>
              </tr>
            ) : (
              jobs.map((job) => <JobRow key={job.id} job={job} />)
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
