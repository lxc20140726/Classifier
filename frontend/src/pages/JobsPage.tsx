import { useEffect, useMemo, useState } from 'react'

import { ChevronDown, ChevronRight, FolderSearch, Play, Plus, RefreshCw, Trash2 } from 'lucide-react'

import { listFolders } from '@/api/folders'
import {
  createScheduledWorkflow,
  deleteScheduledWorkflow,
  listScheduledWorkflows,
  runScheduledWorkflowNow,
  updateScheduledWorkflow,
  type ScheduledWorkflowBody,
} from '@/api/scheduledWorkflows'
import { listWorkflowDefs } from '@/api/workflowDefs'
import { DirPicker } from '@/components/DirPicker'
import { cn } from '@/lib/utils'
import { useJobStore } from '@/store/jobStore'
import { useWorkflowRunStore } from '@/store/workflowRunStore'
import type {
  Folder,
  Job,
  JobStatus,
  NodeRun,
  NodeRunStatus,
  ScheduledWorkflow,
  WorkflowDefinition,
  WorkflowRun,
  WorkflowRunStatus,
} from '@/types'

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

type JobsTab = 'scheduled' | 'history'

interface ScheduledWorkflowFormState {
  jobType: 'workflow' | 'scan'
  name: string
  workflowDefId: string
  cronSpec: string
  enabled: boolean
  folderIds: string[]
  sourceDirs: string[]
}

const EMPTY_SCHEDULED_WORKFLOW_FORM: ScheduledWorkflowFormState = {
  jobType: 'workflow',
  name: '',
  workflowDefId: '',
  cronSpec: '0 * * * *',
  enabled: true,
  folderIds: [],
  sourceDirs: [],
}

type ScheduledWorkflowModalMode =
  | { kind: 'create' }
  | { kind: 'edit'; workflow: ScheduledWorkflow }

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
        <div className="h-full rounded-full bg-primary transition-all duration-300" style={{ width: `${pct}%` }} />
      </div>
      <span className="min-w-[3rem] text-right text-xs text-muted-foreground">{done}/{total}</span>
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
      <tr className="cursor-pointer border-b border-border/50 transition-colors hover:bg-muted/20" onClick={() => setExpanded((v) => !v)}>
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
      <tr className="cursor-pointer border-b border-border transition-colors hover:bg-muted/40" onClick={() => setExpanded((v) => !v)}>
        <td className="px-4 py-3">
          <button type="button" className="flex items-center text-muted-foreground" aria-label={expanded ? '收起' : '展开'}>
            {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
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
        <td className="px-4 py-3 text-sm text-muted-foreground">{formatDuration(job.started_at, job.finished_at)}</td>
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

function ScheduledWorkflowTable({
  workflows,
  workflowDefs,
  isLoading,
  runningId,
  onCreate,
  onEdit,
  onDelete,
  onRunNow,
}: {
  workflows: ScheduledWorkflow[]
  workflowDefs: WorkflowDefinition[]
  isLoading: boolean
  runningId: string | null
  onCreate: () => void
  onEdit: (workflow: ScheduledWorkflow) => void
  onDelete: (workflow: ScheduledWorkflow) => Promise<void>
  onRunNow: (workflow: ScheduledWorkflow) => Promise<void>
}) {
  const workflowNameMap = useMemo(() => {
    return workflowDefs.reduce<Record<string, string>>((acc, item) => {
      acc[item.id] = item.name
      return acc
    }, {})
  }, [workflowDefs])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">计划任务</h2>
          <p className="text-sm text-muted-foreground">用 cron 管理工作流执行，并保留下方执行历史。</p>
        </div>
        <button
          type="button"
          onClick={onCreate}
          className="inline-flex items-center gap-2 rounded-xl bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition hover:opacity-90"
        >
          <Plus className="h-4 w-4" />
          新建计划任务
        </button>
      </div>

      <div className="overflow-hidden rounded-2xl border border-border bg-card shadow-sm">
        <table className="w-full text-sm">
          <thead className="bg-muted/40">
            <tr>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">名称</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">类型</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">工作流</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Cron</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">目录数</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">状态</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">上次执行</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">操作</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr>
                <td colSpan={8} className="px-4 py-10 text-center text-muted-foreground">正在加载计划任务...</td>
              </tr>
            ) : workflows.length === 0 ? (
              <tr>
                <td colSpan={8} className="px-4 py-10 text-center text-muted-foreground">暂无计划任务，可创建带 cron 的工作流作业。</td>
              </tr>
            ) : (
              workflows.map((workflow) => (
                <tr key={workflow.id} className="border-t border-border/60">
                  <td className="px-4 py-3 font-medium">{workflow.name}</td>
                  <td className="px-4 py-3 text-muted-foreground">{workflow.job_type === 'scan' ? '扫描' : '工作流'}</td>
                  <td className="px-4 py-3 text-muted-foreground">{workflow.job_type === 'scan' ? '扫描目录' : (workflowNameMap[workflow.workflow_def_id] ?? workflow.workflow_def_id)}</td>
                  <td className="px-4 py-3 font-mono text-xs">{workflow.cron_spec}</td>
                  <td className="px-4 py-3 text-muted-foreground">{workflow.job_type === 'scan' ? workflow.source_dirs.length : workflow.folder_ids.length}</td>
                  <td className="px-4 py-3">
                    <span className={cn(
                      'inline-flex rounded-full px-2.5 py-1 text-xs font-medium',
                      workflow.enabled ? 'bg-emerald-100 text-emerald-700' : 'bg-muted text-muted-foreground',
                    )}>
                      {workflow.enabled ? '已启用' : '已停用'}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">{formatDate(workflow.last_run_at)}</td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => onEdit(workflow)}
                        className="rounded-lg border border-border px-2.5 py-1 text-xs transition hover:bg-accent"
                      >
                        编辑
                      </button>
                      <button
                        type="button"
                        onClick={() => void onRunNow(workflow)}
                        disabled={runningId === workflow.id}
                        className="inline-flex items-center gap-1 rounded-lg border border-sky-200 px-2.5 py-1 text-xs text-sky-700 transition hover:bg-sky-50 disabled:opacity-50"
                      >
                        <Play className="h-3 w-3" />
                        {runningId === workflow.id ? '启动中' : '立即执行'}
                      </button>
                      <button
                        type="button"
                        onClick={() => void onDelete(workflow)}
                        className="inline-flex items-center gap-1 rounded-lg border border-border px-2.5 py-1 text-xs text-red-600 transition hover:bg-red-50"
                      >
                        <Trash2 className="h-3 w-3" />
                        删除
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function ScheduledWorkflowModal({
  modal,
  form,
  workflowDefs,
  folders,
  filterText,
  setFilterText,
  formError,
  isSaving,
  onClose,
  onChange,
  onToggleFolder,
  onSave,
}: {
  modal: ScheduledWorkflowModalMode | null
  form: ScheduledWorkflowFormState
  workflowDefs: WorkflowDefinition[]
  folders: Folder[]
  filterText: string
  setFilterText: (value: string) => void
  formError: string | null
  isSaving: boolean
  onClose: () => void
  onChange: (patch: Partial<ScheduledWorkflowFormState>) => void
  onToggleFolder: (folderId: string) => void
  onSave: () => Promise<void>
}) {
  const [dirPickerOpen, setDirPickerOpen] = useState(false)
  const filteredFolders = useMemo(() => {
    const keyword = filterText.trim().toLowerCase()
    if (!keyword) return folders
    return folders.filter((folder) => {
      return folder.name.toLowerCase().includes(keyword) || folder.path.toLowerCase().includes(keyword)
    })
  }, [filterText, folders])

  if (!modal) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/45 px-4">
      <div className="w-full max-w-4xl rounded-3xl border border-border bg-background p-6 shadow-2xl">
        <div className="flex items-center justify-between gap-4">
          <div>
            <h2 className="text-lg font-semibold">{modal.kind === 'create' ? '新建计划任务' : '编辑计划任务'}</h2>
            <p className="mt-1 text-sm text-muted-foreground">选择工作流、目标目录和 cron 规则，统一在作业页管理。</p>
          </div>
          <button type="button" onClick={onClose} className="rounded-xl border border-border px-3 py-2 text-sm transition hover:bg-accent">
            关闭
          </button>
        </div>

        <div className="mt-6 grid gap-6 lg:grid-cols-[1.2fr,1fr]">
          <div className="space-y-4">
            <div>
              <label className="mb-1.5 block text-sm font-medium">任务类型</label>
              <select
                value={form.jobType}
                onChange={(event) => onChange({ jobType: event.target.value as 'workflow' | 'scan' })}
                className="w-full rounded-xl border border-border bg-background px-3 py-2 text-sm outline-none ring-primary focus:ring-2"
              >
                <option value="workflow">工作流</option>
                <option value="scan">扫描</option>
              </select>
            </div>

            <div>
              <label className="mb-1.5 block text-sm font-medium">任务名称</label>
              <input
                value={form.name}
                onChange={(event) => onChange({ name: event.target.value })}
                className="w-full rounded-xl border border-border bg-background px-3 py-2 text-sm outline-none ring-primary focus:ring-2"
                placeholder="例如：每小时整理已扫描目录"
              />
            </div>

            {form.jobType === 'workflow' && (
              <div>
                <label className="mb-1.5 block text-sm font-medium">工作流定义</label>
                <select
                  value={form.workflowDefId}
                  onChange={(event) => onChange({ workflowDefId: event.target.value })}
                  className="w-full rounded-xl border border-border bg-background px-3 py-2 text-sm outline-none ring-primary focus:ring-2"
                >
                  <option value="">请选择工作流</option>
                  {workflowDefs.map((workflowDef) => (
                    <option key={workflowDef.id} value={workflowDef.id}>{workflowDef.name}</option>
                  ))}
                </select>
              </div>
            )}

            <div>
              <label className="mb-1.5 block text-sm font-medium">Cron 表达式</label>
              <input
                value={form.cronSpec}
                onChange={(event) => onChange({ cronSpec: event.target.value })}
                className="w-full rounded-xl border border-border bg-background px-3 py-2 font-mono text-sm outline-none ring-primary focus:ring-2"
                placeholder="0 * * * *"
              />
              <p className="mt-1 text-xs text-muted-foreground">使用标准 5 段 cron，例如 `*/30 * * * *` 表示每 30 分钟执行一次。</p>
            </div>

            <label className="flex items-center gap-3 rounded-2xl border border-border px-4 py-3 text-sm">
              <input
                type="checkbox"
                checked={form.enabled}
                onChange={(event) => onChange({ enabled: event.target.checked })}
                className="h-4 w-4 rounded border-border"
              />
              <span>创建后立即启用调度</span>
            </label>

            {formError && <div className="rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{formError}</div>}
          </div>

          <div className="space-y-3">
            <div>
              <label className="mb-1.5 block text-sm font-medium">{form.jobType === 'scan' ? '扫描输入目录' : '选择目录'}</label>
              {form.jobType === 'workflow' ? (
                <>
                  <input
                    value={filterText}
                    onChange={(event) => setFilterText(event.target.value)}
                    className="w-full rounded-xl border border-border bg-background px-3 py-2 text-sm outline-none ring-primary focus:ring-2"
                    placeholder="搜索目录名称或路径"
                  />
                  <p className="mt-1 text-xs text-muted-foreground">已选择 {form.folderIds.length} 个目录。</p>
                </>
              ) : (
                <div className="flex items-center justify-between gap-3 rounded-2xl border border-border bg-card px-4 py-3">
                  <div>
                    <p className="text-sm font-medium">已选择 {form.sourceDirs.length} 个扫描输入目录</p>
                    <p className="text-xs text-muted-foreground">每个扫描任务维护自己的目录列表，不再依赖设置页。</p>
                  </div>
                  <button
                    type="button"
                    onClick={() => setDirPickerOpen(true)}
                    className="inline-flex items-center gap-1.5 rounded-lg border border-border px-3 py-2 text-sm transition hover:bg-accent"
                  >
                    <FolderSearch className="h-4 w-4" />
                    添加目录
                  </button>
                </div>
              )}
            </div>
            <div className="max-h-80 overflow-y-auto rounded-2xl border border-border bg-card p-3">
              <div className="space-y-2">
                {form.jobType === 'scan' ? form.sourceDirs.map((dir) => (
                  <div key={dir} className="flex items-center justify-between rounded-xl border border-border/70 px-3 py-2 text-sm transition hover:bg-accent/50">
                    <p className="break-all font-mono text-xs text-muted-foreground">{dir}</p>
                    <button
                      type="button"
                      onClick={() => onToggleFolder(dir)}
                      className="rounded-lg border border-border px-2 py-1 text-xs text-red-600 transition hover:bg-red-50"
                    >
                      移除
                    </button>
                  </div>
                )) : filteredFolders.map((folder) => (
                  <label key={folder.id} className="flex cursor-pointer items-start gap-3 rounded-xl border border-border/70 px-3 py-2 text-sm transition hover:bg-accent/50">
                    <input
                      type="checkbox"
                      checked={form.folderIds.includes(folder.id)}
                      onChange={() => onToggleFolder(folder.id)}
                      className="mt-0.5 h-4 w-4 rounded border-border"
                    />
                    <div className="min-w-0">
                      <p className="truncate font-medium">{folder.name}</p>
                      <p className="truncate font-mono text-xs text-muted-foreground">{folder.path}</p>
                    </div>
                  </label>
                ))}
                {((form.jobType === 'scan' && form.sourceDirs.length === 0) || (form.jobType === 'workflow' && filteredFolders.length === 0)) && (
                  <div className="rounded-xl border border-dashed border-border px-3 py-8 text-center text-sm text-muted-foreground">
                    {form.jobType === 'scan' ? '尚未为这个扫描任务添加输入目录。' : '没有匹配的目录。'}
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>

        <div className="mt-6 flex justify-end gap-2">
          <button type="button" onClick={onClose} disabled={isSaving} className="rounded-xl border border-border px-4 py-2 text-sm transition hover:bg-accent disabled:opacity-50">
            取消
          </button>
          <button type="button" onClick={() => void onSave()} disabled={isSaving} className="rounded-xl bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition hover:opacity-90 disabled:opacity-50">
            {isSaving ? '保存中...' : '保存'}
          </button>
        </div>

        <DirPicker
          open={dirPickerOpen}
          title="选择扫描输入目录"
          onCancel={() => setDirPickerOpen(false)}
          onConfirm={(path) => {
            setDirPickerOpen(false)
            if (!form.sourceDirs.includes(path)) {
              onChange({ sourceDirs: [...form.sourceDirs, path] })
            }
          }}
        />
      </div>
    </div>
  )
}

export default function JobsPage() {
  const { jobs, total, isLoading, error, fetchJobs } = useJobStore()
  const [activeTab, setActiveTab] = useState<JobsTab>('scheduled')
  const [scheduledWorkflows, setScheduledWorkflows] = useState<ScheduledWorkflow[]>([])
  const [workflowDefs, setWorkflowDefs] = useState<WorkflowDefinition[]>([])
  const [folders, setFolders] = useState<Folder[]>([])
  const [isScheduledLoading, setIsScheduledLoading] = useState(false)
  const [scheduledError, setScheduledError] = useState<string | null>(null)
  const [modal, setModal] = useState<ScheduledWorkflowModalMode | null>(null)
  const [form, setForm] = useState<ScheduledWorkflowFormState>(EMPTY_SCHEDULED_WORKFLOW_FORM)
  const [formError, setFormError] = useState<string | null>(null)
  const [isSaving, setIsSaving] = useState(false)
  const [runningId, setRunningId] = useState<string | null>(null)
  const [folderFilterText, setFolderFilterText] = useState('')

  useEffect(() => {
    void fetchJobs()
    void fetchScheduledData()
  }, [fetchJobs])

  async function fetchScheduledData() {
    setIsScheduledLoading(true)
    setScheduledError(null)
    try {
      const [workflowRes, folderRes, scheduledRes] = await Promise.all([
        listWorkflowDefs({ limit: 100 }),
        listFolders({ limit: 200 }),
        listScheduledWorkflows(),
      ])
      setWorkflowDefs(workflowRes.data)
      setFolders(folderRes.data)
      setScheduledWorkflows(scheduledRes.data)
    } catch (loadError) {
      setScheduledError(loadError instanceof Error ? loadError.message : '加载计划任务失败')
    } finally {
      setIsScheduledLoading(false)
    }
  }

  function openCreateModal() {
    setForm(EMPTY_SCHEDULED_WORKFLOW_FORM)
    setFormError(null)
    setFolderFilterText('')
    setModal({ kind: 'create' })
  }

  function openEditModal(workflow: ScheduledWorkflow) {
    setForm({
      jobType: workflow.job_type,
      name: workflow.name,
      workflowDefId: workflow.workflow_def_id,
      cronSpec: workflow.cron_spec,
      enabled: workflow.enabled,
      folderIds: workflow.folder_ids,
      sourceDirs: workflow.source_dirs,
    })
    setFormError(null)
    setFolderFilterText('')
    setModal({ kind: 'edit', workflow })
  }

  function closeModal() {
    setModal(null)
    setFormError(null)
  }

  function toggleFolder(folderId: string) {
    setForm((prev) => {
      if (prev.jobType === 'scan') {
        return {
          ...prev,
          sourceDirs: prev.sourceDirs.includes(folderId)
            ? prev.sourceDirs.filter((item) => item !== folderId)
            : [...prev.sourceDirs, folderId],
        }
      }

      return {
        ...prev,
        folderIds: prev.folderIds.includes(folderId)
          ? prev.folderIds.filter((item) => item !== folderId)
          : [...prev.folderIds, folderId],
      }
    })
  }

  async function handleSaveScheduledWorkflow() {
    if (!form.name.trim()) {
      setFormError('任务名称不能为空')
      return
    }
    if (form.jobType === 'workflow' && !form.workflowDefId) {
      setFormError('请选择工作流定义')
      return
    }
    if (!form.cronSpec.trim()) {
      setFormError('Cron 表达式不能为空')
      return
    }
    if (form.jobType === 'workflow' && form.folderIds.length === 0) {
      setFormError('请至少选择一个目录')
      return
    }
    if (form.jobType === 'scan' && form.sourceDirs.length === 0) {
      setFormError('请至少选择一个扫描输入目录')
      return
    }

    const body: ScheduledWorkflowBody = {
      job_type: form.jobType,
      name: form.name.trim(),
      workflow_def_id: form.jobType === 'workflow' ? form.workflowDefId : '',
      cron_spec: form.cronSpec.trim(),
      enabled: form.enabled,
      folder_ids: form.jobType === 'workflow' ? form.folderIds : [],
      source_dirs: form.jobType === 'scan' ? form.sourceDirs : [],
    }

    setIsSaving(true)
    setFormError(null)
    try {
      if (modal?.kind === 'create') {
        await createScheduledWorkflow(body)
      } else if (modal?.kind === 'edit') {
        await updateScheduledWorkflow(modal.workflow.id, body)
      }
      closeModal()
      await fetchScheduledData()
    } catch (saveError) {
      setFormError(saveError instanceof Error ? saveError.message : '保存计划任务失败')
    } finally {
      setIsSaving(false)
    }
  }

  async function handleDeleteScheduledWorkflow(workflow: ScheduledWorkflow) {
    if (!window.confirm(`确认删除计划任务「${workflow.name}」？`)) return
    await deleteScheduledWorkflow(workflow.id)
    await fetchScheduledData()
  }

  async function handleRunNow(workflow: ScheduledWorkflow) {
    setRunningId(workflow.id)
    try {
      await runScheduledWorkflowNow(workflow.id)
      setActiveTab('history')
      await Promise.all([fetchJobs(), fetchScheduledData()])
    } finally {
      setRunningId(null)
    }
  }

  async function handleRefresh() {
    await Promise.all([fetchJobs(), fetchScheduledData()])
  }

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">任务管理</h1>
          <p className="mt-0.5 text-sm text-muted-foreground">在这里管理带 cron 的工作流作业，并查看执行历史。</p>
        </div>
        <button
          type="button"
          onClick={() => void handleRefresh()}
          disabled={isLoading || isScheduledLoading}
          className="flex items-center gap-2 rounded-md border border-input bg-background px-3 py-1.5 text-sm font-medium shadow-sm transition-colors hover:bg-accent disabled:opacity-50"
        >
          <RefreshCw className={cn('h-3.5 w-3.5', (isLoading || isScheduledLoading) && 'animate-spin')} />
          刷新
        </button>
      </div>

      <div className="inline-flex rounded-2xl border border-border bg-card p-1 shadow-sm">
        <button
          type="button"
          onClick={() => setActiveTab('scheduled')}
          className={cn(
            'rounded-xl px-4 py-2 text-sm font-medium transition',
            activeTab === 'scheduled' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-accent',
          )}
        >
          计划任务
        </button>
        <button
          type="button"
          onClick={() => setActiveTab('history')}
          className={cn(
            'rounded-xl px-4 py-2 text-sm font-medium transition',
            activeTab === 'history' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-accent',
          )}
        >
          执行历史
        </button>
      </div>

      {scheduledError && activeTab === 'scheduled' && (
        <div className="rounded-md border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">{scheduledError}</div>
      )}
      {error && activeTab === 'history' && (
        <div className="rounded-md border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">{error}</div>
      )}

      {activeTab === 'scheduled' ? (
        <ScheduledWorkflowTable
          workflows={scheduledWorkflows}
          workflowDefs={workflowDefs}
          isLoading={isScheduledLoading}
          runningId={runningId}
          onCreate={openCreateModal}
          onEdit={openEditModal}
          onDelete={handleDeleteScheduledWorkflow}
          onRunNow={handleRunNow}
        />
      ) : (
        <div className="space-y-4">
          <div>
            <h2 className="text-lg font-semibold">执行历史</h2>
            <p className="text-sm text-muted-foreground">共 {total} 条任务记录，可展开查看工作流运行与节点细节。</p>
          </div>
          <div className="overflow-hidden rounded-lg border border-border bg-card">
            <table className="w-full">
              <thead>
                <tr className="border-b border-border bg-muted/50">
                  <th className="w-10 px-4 py-3" />
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">ID</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">类型</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">状态</th>
                  <th className="w-48 px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">进度</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">目录数</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">创建时间</th>
                  <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">耗时</th>
                </tr>
              </thead>
              <tbody>
                {isLoading && jobs.length === 0 ? (
                  <tr>
                    <td colSpan={8} className="px-4 py-12 text-center text-sm text-muted-foreground">正在加载任务...</td>
                  </tr>
                ) : jobs.length === 0 ? (
                  <tr>
                    <td colSpan={8} className="px-4 py-12 text-center text-sm text-muted-foreground">暂无任务记录，执行计划任务后会显示在这里。</td>
                  </tr>
                ) : (
                  jobs.map((job) => <JobRow key={job.id} job={job} />)
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <ScheduledWorkflowModal
        modal={modal}
        form={form}
        workflowDefs={workflowDefs}
        folders={folders}
        filterText={folderFilterText}
        setFilterText={setFolderFilterText}
        formError={formError}
        isSaving={isSaving}
        onClose={closeModal}
        onChange={(patch) => setForm((prev) => ({ ...prev, ...patch }))}
        onToggleFolder={toggleFolder}
        onSave={handleSaveScheduledWorkflow}
      />
    </div>
  )
}
