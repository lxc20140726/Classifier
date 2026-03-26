import { useEffect, useMemo, useState, useRef } from 'react'

import { ChevronDown, ChevronRight, FolderSearch, Play, Plus, RefreshCw, Trash2, X } from 'lucide-react'
import gsap from 'gsap'

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
import { CronExpressionField } from '@/components/CronExpressionField'
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
  pending: 'bg-gray-200 text-gray-900 border-2 border-foreground',
  running: 'bg-blue-300 text-blue-900 border-2 border-foreground',
  succeeded: 'bg-green-300 text-green-900 border-2 border-foreground',
  failed: 'bg-red-300 text-red-900 border-2 border-foreground',
  partial: 'bg-yellow-300 text-yellow-900 border-2 border-foreground',
  cancelled: 'bg-gray-300 text-gray-900 border-2 border-foreground',
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
  pending: 'bg-gray-200 text-gray-900 border-2 border-foreground',
  running: 'bg-blue-300 text-blue-900 border-2 border-foreground',
  succeeded: 'bg-green-300 text-green-900 border-2 border-foreground',
  failed: 'bg-red-300 text-red-900 border-2 border-foreground',
  partial: 'bg-yellow-300 text-yellow-900 border-2 border-foreground',
  waiting_input: 'bg-purple-300 text-purple-900 border-2 border-foreground',
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
  pending: 'bg-gray-200 text-gray-900 border-2 border-foreground',
  running: 'bg-blue-300 text-blue-900 border-2 border-foreground',
  succeeded: 'bg-green-300 text-green-900 border-2 border-foreground',
  failed: 'bg-red-300 text-red-900 border-2 border-foreground',
  skipped: 'bg-gray-300 text-gray-900 border-2 border-foreground',
  waiting_input: 'bg-purple-300 text-purple-900 border-2 border-foreground',
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
        'inline-flex items-center px-2 py-0.5 text-[10px] font-black',
        styles[status] ?? 'bg-muted text-muted-foreground border-2 border-foreground',
      )}
    >
      {labels[status] ?? status}
    </span>
  )
}

function ProgressBar({ done, total }: { done: number; total: number }) {
  const pct = total > 0 ? Math.round((done / total) * 100) : 0
  return (
    <div className="flex items-center gap-3">
      <div className="h-2 flex-1 overflow-hidden border-2 border-foreground bg-muted">
        <div className="h-full bg-foreground transition-all duration-300" style={{ width: `${pct}%` }} />
      </div>
      <span className="min-w-[3rem] text-right text-xs font-black tabular-nums">{done}/{total}</span>
    </div>
  )
}

function NodeRunsPanel({ runId }: { runId: string }) {
  const { nodesByRunId, fetchRunDetail } = useWorkflowRunStore()
  const nodes = nodesByRunId[runId] ?? []
  const listRef = useRef<HTMLTableSectionElement | null>(null)

  useEffect(() => {
    void fetchRunDetail(runId)
  }, [runId, fetchRunDetail])

  useEffect(() => {
    if (nodes.length > 0 && listRef.current) {
      const items = listRef.current.querySelectorAll('.node-item')
      const dots = listRef.current.querySelectorAll('.node-dot')
      const lines = listRef.current.querySelectorAll('.node-line')
      
      gsap.fromTo(items, 
        { opacity: 0, x: -20 }, 
        { opacity: 1, x: 0, duration: 0.4, stagger: 0.1, ease: "back.out(1.5)" }
      )
      
      // 粒子飞入成为节点圆点的动效
      dots.forEach((dot, i) => {
        gsap.fromTo(dot,
          { 
            scale: 0,
            x: () => (Math.random() - 0.5) * 150,
            y: () => (Math.random() - 0.5) * 150,
            opacity: 0
          },
          { 
            scale: 1, 
            x: 0,
            y: 0,
            opacity: 1,
            duration: 0.6, 
            delay: i * 0.1, 
            ease: "expo.out" 
          }
        )
      })
      
      if (lines.length > 0) {
        gsap.fromTo(lines,
          { scaleY: 0 },
          { scaleY: 1, duration: 0.3, stagger: 0.1, transformOrigin: "top", ease: "none", delay: 0.2 }
        )
      }
    }
  }, [nodes.length])

  if (nodes.length === 0) {
    return <p className="py-4 text-xs font-bold text-muted-foreground text-center">暂无节点记录</p>
  }

  return (
    <div className="pl-4 py-2">
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b-2 border-foreground bg-muted/30">
            <th className="w-8 py-2"></th>
            <th className="py-2 pr-4 text-left font-black tracking-widest">节点ID</th>
            <th className="py-2 pr-4 text-left font-black tracking-widest">类型</th>
            <th className="py-2 pr-4 text-left font-black tracking-widest">序号</th>
            <th className="py-2 pr-4 text-left font-black tracking-widest">状态</th>
            <th className="py-2 text-left font-black tracking-widest">耗时</th>
          </tr>
        </thead>
        <tbody ref={listRef}>
          {nodes.map((node: NodeRun, i) => (
            <tr key={node.id || node.node_id} className="node-item border-b-2 border-foreground/20 last:border-0 hover:bg-muted/10">
              <td className="py-3 relative">
                <div className="flex flex-col items-center justify-center h-full">
                  <div className="node-dot w-3 h-3 bg-foreground rounded-full z-10 relative"></div>
                  {i < nodes.length - 1 && (
                    <div className="node-line absolute top-6 bottom-[-12px] w-0.5 bg-foreground z-0"></div>
                  )}
                </div>
              </td>
              <td className="py-3 pr-4 font-mono font-bold">{node.node_id}</td>
              <td className="py-3 pr-4 font-bold">{node.node_type}</td>
              <td className="py-3 pr-4 font-black">{node.sequence}</td>
              <td className="py-3 pr-4">
                <StatusBadge status={node.status} labels={NODE_STATUS_LABELS} styles={NODE_STATUS_STYLES} />
              </td>
              <td className="py-3 font-mono font-bold">{formatDuration(node.started_at, node.finished_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
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

  function toggleExpand() {
    setExpanded((v) => !v)
  }

  return (
    <>
      <tr className="cursor-pointer border-b-2 border-foreground transition-colors hover:bg-muted/20" onClick={toggleExpand}>
        <td className="py-3 pl-4 pr-3">
          <div className="flex items-center justify-center w-6 h-6 border-2 border-foreground bg-background transition-transform hover:scale-110">
            {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
          </div>
        </td>
        <td className="py-3 pr-4 font-mono text-xs font-bold">{run.folder_id.slice(0, 8)}</td>
        <td className="py-3 pr-4">
          <StatusBadge status={run.status} labels={WF_STATUS_LABELS} styles={WF_STATUS_STYLES} />
        </td>
        <td className="py-3 pr-4 text-xs font-mono font-bold text-muted-foreground">{formatDate(run.created_at)}</td>
        <td className="py-3" onClick={(e) => e.stopPropagation()}>
          <div className="flex items-center gap-2">
            {(run.status === 'failed' || run.status === 'partial') && (
              <button
                type="button"
                disabled={isActing}
                onClick={() => void handleRollback()}
                className="border-2 border-red-900 bg-red-200 px-3 py-1 text-xs font-bold text-red-900 transition-all hover:bg-red-900 hover:text-red-100 hover:shadow-hard hover:-translate-y-0.5 disabled:opacity-50"
              >
                回滚
              </button>
            )}
            {run.status === 'waiting_input' && (
              <div className="flex items-center gap-2">
                <select
                  value={selectedCategory}
                  onChange={(e) => setSelectedCategory(e.target.value)}
                  className="border-2 border-foreground bg-background px-2 py-1 text-xs font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-1"
                >
                  {Object.entries(CATEGORY_LABELS).map(([val, label]) => (
                    <option key={val} value={val}>{label}</option>
                  ))}
                </select>
                <button
                  type="button"
                  disabled={isActing}
                  onClick={() => void handleProvideInput()}
                  className="border-2 border-purple-900 bg-purple-200 px-3 py-1 text-xs font-bold text-purple-900 transition-all hover:bg-purple-900 hover:text-purple-100 hover:shadow-hard hover:-translate-y-0.5 disabled:opacity-50"
                >
                  确认
                </button>
              </div>
            )}
          </div>
        </td>
      </tr>
      {expanded && (
        <tr className="border-b-2 border-foreground bg-muted/10">
          <td colSpan={5} className="px-6 py-4">
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
    return <p className="text-xs font-bold text-muted-foreground py-4 text-center">暂无工作流运行记录</p>
  }

  return (
    <div className="border-2 border-foreground bg-card shadow-hard">
      <div className="bg-muted/30 px-4 py-2 border-b-2 border-foreground">
        <p className="text-xs font-black tracking-widest">WORKFLOW RUNS ({runs.length})</p>
      </div>
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b-2 border-foreground bg-muted/10">
            <th className="w-12" />
            <th className="py-2 pr-4 text-left font-black tracking-widest">目录ID</th>
            <th className="py-2 pr-4 text-left font-black tracking-widest">状态</th>
            <th className="py-2 pr-4 text-left font-black tracking-widest">创建时间</th>
            <th className="py-2 text-left font-black tracking-widest">操作</th>
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

  function toggleExpand() {
    setExpanded((v) => !v)
  }

  return (
    <>
      <tr className="job-row cursor-pointer border-b-2 border-foreground transition-colors hover:bg-muted/30" onClick={toggleExpand}>
        <td className="px-4 py-4">
          <div className="flex items-center justify-center w-6 h-6 border-2 border-foreground bg-background transition-transform hover:scale-110">
            {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
          </div>
        </td>
        <td className="px-4 py-4 font-mono text-xs font-bold">{job.id.slice(0, 8)}</td>
        <td className="px-4 py-4 text-sm font-black">{job.type}</td>
        <td className="px-4 py-4">
          <StatusBadge status={job.status} labels={JOB_STATUS_LABELS} styles={JOB_STATUS_STYLES} />
        </td>
        <td className="w-48 px-4 py-4">
          <ProgressBar done={job.done} total={job.total} />
        </td>
        <td className="px-4 py-4 text-sm font-black">{job.folder_ids.length}</td>
        <td className="px-4 py-4 text-xs font-mono font-bold text-muted-foreground">{formatDate(job.created_at)}</td>
        <td className="px-4 py-4 text-xs font-mono font-bold text-muted-foreground">{formatDuration(job.started_at, job.finished_at)}</td>
      </tr>
      {expanded && (
        <tr className="border-b-2 border-foreground bg-muted/10">
          <td colSpan={8} className="px-8 py-6">
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
          <h2 className="text-xl font-black tracking-tight">计划任务</h2>
          <p className="mt-1 text-sm font-medium text-muted-foreground">用 cron 管理工作流执行，并保留下方执行历史。</p>
        </div>
        <button
          type="button"
          onClick={onCreate}
          className="inline-flex items-center gap-2 border-2 border-foreground bg-primary px-4 py-2 text-sm font-bold text-primary-foreground transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5"
        >
          <Plus className="h-4 w-4" />
          新建计划任务
        </button>
      </div>

      <div className="overflow-hidden border-2 border-foreground bg-card shadow-hard">
        <table className="w-full text-sm">
          <thead className="bg-muted/50 border-b-2 border-foreground">
            <tr>
              <th className="px-4 py-4 text-left font-black tracking-widest">名称</th>
              <th className="px-4 py-4 text-left font-black tracking-widest">类型</th>
              <th className="px-4 py-4 text-left font-black tracking-widest">工作流</th>
              <th className="px-4 py-4 text-left font-black tracking-widest">Cron</th>
              <th className="px-4 py-4 text-left font-black tracking-widest">目录数</th>
              <th className="px-4 py-4 text-left font-black tracking-widest">状态</th>
              <th className="px-4 py-4 text-left font-black tracking-widest">上次执行</th>
              <th className="px-4 py-4 text-left font-black tracking-widest">操作</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr>
                <td colSpan={8} className="px-4 py-16 text-center font-bold text-muted-foreground">正在加载计划任务...</td>
              </tr>
            ) : workflows.length === 0 ? (
              <tr>
                <td colSpan={8} className="px-4 py-16 text-center font-bold text-muted-foreground border-2 border-dashed border-foreground m-4">暂无计划任务，可创建带 cron 的工作流作业。</td>
              </tr>
            ) : (
              workflows.map((workflow) => (
                <tr key={workflow.id} className="scheduled-row border-b-2 border-foreground last:border-0 hover:bg-muted/30 transition-colors">
                  <td className="px-4 py-4 font-black">{workflow.name}</td>
                  <td className="px-4 py-4 font-bold text-muted-foreground">{workflow.job_type === 'scan' ? '扫描' : '工作流'}</td>
                  <td className="px-4 py-4 font-bold text-muted-foreground">{workflow.job_type === 'scan' ? '扫描目录' : (workflowNameMap[workflow.workflow_def_id] ?? workflow.workflow_def_id)}</td>
                  <td className="px-4 py-4 font-mono text-xs font-bold bg-muted/50 px-2">{workflow.cron_spec}</td>
                  <td className="px-4 py-4 font-black tabular-nums">{workflow.job_type === 'scan' ? workflow.source_dirs.length : workflow.folder_ids.length}</td>
                  <td className="px-4 py-4">
                    <span className={cn(
                      'inline-flex border-2 border-foreground px-2 py-0.5 text-[10px] font-black',
                      workflow.enabled ? 'bg-green-300 text-green-900' : 'bg-gray-200 text-gray-900',
                    )}>
                      {workflow.enabled ? '已启用' : '已停用'}
                    </span>
                  </td>
                  <td className="px-4 py-4 font-mono text-xs font-bold text-muted-foreground">{formatDate(workflow.last_run_at)}</td>
                  <td className="px-4 py-4">
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => onEdit(workflow)}
                        className="border-2 border-foreground bg-background px-3 py-1.5 text-xs font-bold transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5"
                      >
                        编辑
                      </button>
                      <button
                        type="button"
                        onClick={() => void onRunNow(workflow)}
                        disabled={runningId === workflow.id}
                        className="inline-flex items-center gap-1 border-2 border-foreground bg-primary px-3 py-1.5 text-xs font-bold text-primary-foreground transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5 disabled:opacity-50 disabled:hover:bg-primary disabled:hover:text-primary-foreground disabled:hover:shadow-none disabled:hover:translate-y-0"
                      >
                        <Play className="h-3 w-3" />
                        {runningId === workflow.id ? '启动中' : '立即执行'}
                      </button>
                      <button
                        type="button"
                        onClick={() => void onDelete(workflow)}
                        className="inline-flex items-center gap-1 border-2 border-red-900 bg-red-100 px-3 py-1.5 text-xs font-bold text-red-900 transition-all hover:bg-red-900 hover:text-red-100 hover:shadow-hard hover:-translate-y-0.5"
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
  const overlayRef = useRef<HTMLDivElement | null>(null)
  const modalRef = useRef<HTMLDivElement | null>(null)

  const filteredFolders = useMemo(() => {
    const keyword = filterText.trim().toLowerCase()
    if (!keyword) return folders
    return folders.filter((folder) => {
      return folder.name.toLowerCase().includes(keyword) || folder.path.toLowerCase().includes(keyword)
    })
  }, [filterText, folders])

  useEffect(() => {
    if (modal && overlayRef.current && modalRef.current) {
      gsap.fromTo(overlayRef.current, { opacity: 0 }, { opacity: 1, duration: 0.2 })
      gsap.fromTo(modalRef.current, { scale: 0.8, opacity: 0 }, { scale: 1, opacity: 1, duration: 0.4, ease: "back.out(1.7)" })
    }
  }, [modal])

  if (!modal) return null

  return (
    <div ref={overlayRef} className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4 py-8 overflow-y-auto">
      <div ref={modalRef} className="w-full max-w-4xl border-2 border-foreground bg-background shadow-hard-lg my-auto">
        <div className="flex items-center justify-between border-b-2 border-foreground bg-primary px-6 py-5 text-primary-foreground">
          <div>
            <h2 className="text-xl font-black tracking-tight">{modal.kind === 'create' ? '新建计划任务' : '编辑计划任务'}</h2>
            <p className="mt-1 text-sm font-medium">选择工作流、目标目录和 cron 规则，统一在作业页管理。</p>
          </div>
          <button type="button" onClick={onClose} className="border-2 border-transparent p-2 transition-all hover:border-primary-foreground hover:bg-foreground hover:text-background">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="p-6 grid gap-8 lg:grid-cols-[1fr,1.2fr]">
          <div className="space-y-5 min-w-0">
            <div>
              <label className="mb-2 block text-sm font-black tracking-widest">任务类型</label>
              <select
                value={form.jobType}
                onChange={(event) => onChange({ jobType: event.target.value as 'workflow' | 'scan' })}
                className="w-full border-2 border-foreground bg-background px-4 py-3 text-sm font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-2 focus:ring-offset-background"
              >
                <option value="workflow">工作流</option>
                <option value="scan">扫描</option>
              </select>
            </div>

            <div>
              <label className="mb-2 block text-sm font-black tracking-widest">任务名称</label>
              <input
                value={form.name}
                onChange={(event) => onChange({ name: event.target.value })}
                className="w-full border-2 border-foreground bg-background px-4 py-3 text-sm font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-2 focus:ring-offset-background"
                placeholder="例如：每小时整理已扫描目录"
              />
            </div>

            {form.jobType === 'workflow' && (
              <div>
                <label className="mb-2 block text-sm font-black tracking-widest">工作流定义</label>
                <select
                  value={form.workflowDefId}
                  onChange={(event) => onChange({ workflowDefId: event.target.value })}
                  className="w-full border-2 border-foreground bg-background px-4 py-3 text-sm font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-2 focus:ring-offset-background"
                >
                  <option value="">请选择工作流</option>
                  {workflowDefs.map((workflowDef) => (
                    <option key={workflowDef.id} value={workflowDef.id}>{workflowDef.name}</option>
                  ))}
                </select>
              </div>
            )}

            <div>
              <label className="mb-2 block text-sm font-black tracking-widest">CRON 表达式</label>
              <CronExpressionField
                value={form.cronSpec}
                onChange={(v) => onChange({ cronSpec: v })}
              />
            </div>

            <label className="flex items-center gap-3 border-2 border-foreground bg-muted/10 px-4 py-4 text-sm font-bold cursor-pointer hover:bg-muted/30 transition-colors">
              <input
                type="checkbox"
                checked={form.enabled}
                onChange={(event) => onChange({ enabled: event.target.checked })}
                className="h-5 w-5 rounded-none border-2 border-foreground text-foreground focus:ring-foreground focus:ring-offset-0"
              />
              <span>创建后立即启用调度</span>
            </label>

            {formError && <div className="border-2 border-red-900 bg-red-100 px-4 py-3 text-sm font-bold text-red-900 shadow-hard">{formError}</div>}
          </div>

          <div className="space-y-4 min-w-0">
            <div>
              <label className="mb-2 block text-sm font-black tracking-widest">{form.jobType === 'scan' ? '扫描输入目录' : '选择目录'}</label>
              {form.jobType === 'workflow' ? (
                <>
                  <input
                    value={filterText}
                    onChange={(event) => setFilterText(event.target.value)}
                    className="w-full border-2 border-foreground bg-background px-4 py-3 text-sm font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-2 focus:ring-offset-background"
                    placeholder="搜索目录名称或路径"
                  />
                  <p className="mt-2 text-xs font-bold text-muted-foreground">已选择 <span className="text-foreground">{form.folderIds.length}</span> 个目录。</p>
                </>
              ) : (
                <div className="flex flex-col gap-3 border-2 border-foreground bg-card px-4 py-4 shadow-hard">
                  <div>
                    <p className="text-sm font-black">已选择 {form.sourceDirs.length} 个扫描输入目录</p>
                    <p className="text-xs font-medium text-muted-foreground mt-1">每个扫描任务维护自己的目录列表，不再依赖设置页。</p>
                  </div>
                  <button
                    type="button"
                    onClick={() => setDirPickerOpen(true)}
                    className="inline-flex items-center justify-center gap-2 border-2 border-foreground bg-background px-4 py-2 text-sm font-bold transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5"
                  >
                    <FolderSearch className="h-4 w-4" />
                    添加目录
                  </button>
                </div>
              )}
            </div>
            
            <div className="h-[320px] overflow-y-auto border-2 border-foreground bg-muted/10 p-3">
              <div className="space-y-2">
                {form.jobType === 'scan' ? form.sourceDirs.map((dir) => (
                  <div key={dir} className="flex items-center justify-between border-2 border-foreground bg-background px-3 py-3 text-sm transition-colors hover:bg-muted/30">
                    <p className="break-all font-mono text-xs font-bold text-foreground">{dir}</p>
                    <button
                      type="button"
                      onClick={() => onToggleFolder(dir)}
                      className="ml-3 shrink-0 border-2 border-red-900 bg-red-100 px-2 py-1 text-xs font-bold text-red-900 transition-all hover:bg-red-900 hover:text-red-100 hover:shadow-hard hover:-translate-y-0.5"
                    >
                      移除
                    </button>
                  </div>
                )) : filteredFolders.map((folder) => (
                  <label key={folder.id} className="flex cursor-pointer items-start gap-3 border-2 border-foreground bg-background px-3 py-3 text-sm transition-colors hover:bg-muted/30">
                    <input
                      type="checkbox"
                      checked={form.folderIds.includes(folder.id)}
                      onChange={() => onToggleFolder(folder.id)}
                      className="mt-0.5 h-4 w-4 rounded-none border-2 border-foreground text-foreground focus:ring-foreground focus:ring-offset-0"
                    />
                    <div className="min-w-0">
                      <p className="truncate font-black">{folder.name}</p>
                      <p className="truncate font-mono text-xs font-bold text-muted-foreground mt-0.5">{folder.path}</p>
                    </div>
                  </label>
                ))}
                {((form.jobType === 'scan' && form.sourceDirs.length === 0) || (form.jobType === 'workflow' && filteredFolders.length === 0)) && (
                  <div className="border-2 border-dashed border-foreground px-4 py-12 text-center text-sm font-bold text-muted-foreground">
                    {form.jobType === 'scan' ? '尚未为这个扫描任务添加输入目录。' : '没有匹配的目录。'}
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>

        <div className="flex items-center justify-end gap-3 border-t-2 border-foreground bg-muted/30 px-6 py-5">
          <button type="button" onClick={onClose} disabled={isSaving} className="border-2 border-foreground bg-background px-6 py-2.5 text-sm font-bold transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5 disabled:opacity-50">
            取消
          </button>
          <button type="button" onClick={() => void onSave()} disabled={isSaving} className="border-2 border-foreground bg-primary px-6 py-2.5 text-sm font-bold text-primary-foreground transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5 disabled:opacity-50 disabled:hover:bg-primary disabled:hover:text-primary-foreground disabled:hover:shadow-none disabled:hover:translate-y-0">
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

  // GSAP Stagger Animation for items
  useEffect(() => {
    if (!isLoading && jobs.length > 0 && activeTab === 'history') {
      gsap.fromTo(
        '.job-row',
        { opacity: 0, x: -20 },
        { opacity: 1, x: 0, duration: 0.4, stagger: 0.05, ease: 'power2.out', clearProps: 'all' }
      )
    }
  }, [jobs, isLoading, activeTab])

  useEffect(() => {
    if (!isScheduledLoading && scheduledWorkflows.length > 0 && activeTab === 'scheduled') {
      gsap.fromTo(
        '.scheduled-row',
        { opacity: 0, x: -20 },
        { opacity: 1, x: 0, duration: 0.4, stagger: 0.05, ease: 'power2.out', clearProps: 'all' }
      )
    }
  }, [scheduledWorkflows, isScheduledLoading, activeTab])

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
    <div className="flex flex-col gap-8 p-6">
      <div className="flex items-end justify-between border-b-2 border-foreground pb-4">
        <div>
          <h1 className="text-3xl font-black tracking-tight uppercase">任务管理</h1>
          <p className="mt-1 text-sm font-bold text-muted-foreground">在这里管理带 cron 的工作流作业，并查看执行历史。</p>
        </div>
        <button
          type="button"
          onClick={() => void handleRefresh()}
          disabled={isLoading || isScheduledLoading}
          className="flex items-center gap-2 border-2 border-foreground bg-background px-4 py-2 text-sm font-bold transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5 disabled:opacity-50 disabled:hover:bg-background disabled:hover:text-foreground disabled:hover:shadow-none disabled:hover:translate-y-0"
        >
          <RefreshCw className={cn('h-4 w-4', (isLoading || isScheduledLoading) && 'animate-spin')} />
          刷新
        </button>
      </div>

      <div className="inline-flex border-2 border-foreground bg-background shadow-hard self-start">
        <button
          type="button"
          onClick={() => setActiveTab('scheduled')}
          className={cn(
            'px-6 py-2.5 text-sm font-black transition-colors',
            activeTab === 'scheduled' ? 'bg-foreground text-background' : 'hover:bg-muted',
          )}
        >
          计划任务
        </button>
        <div className="w-0.5 bg-foreground" />
        <button
          type="button"
          onClick={() => setActiveTab('history')}
          className={cn(
            'px-6 py-2.5 text-sm font-black transition-colors',
            activeTab === 'history' ? 'bg-foreground text-background' : 'hover:bg-muted',
          )}
        >
          执行历史
        </button>
      </div>

      {scheduledError && activeTab === 'scheduled' && (
        <div className="border-2 border-red-900 bg-red-100 px-4 py-3 text-sm font-bold text-red-900 shadow-hard">{scheduledError}</div>
      )}
      {error && activeTab === 'history' && (
        <div className="border-2 border-red-900 bg-red-100 px-4 py-3 text-sm font-bold text-red-900 shadow-hard">{error}</div>
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
            <h2 className="text-xl font-black tracking-tight">执行历史</h2>
            <p className="mt-1 text-sm font-medium text-muted-foreground">共 <span className="text-foreground font-bold">{total}</span> 条任务记录，可展开查看工作流运行与节点细节。</p>
          </div>
          <div className="overflow-hidden border-2 border-foreground bg-card shadow-hard">
            <table className="w-full">
              <thead>
                <tr className="border-b-2 border-foreground bg-muted/50">
                  <th className="w-12 px-4 py-4" />
                  <th className="px-4 py-4 text-left text-xs font-black uppercase tracking-widest text-foreground">ID</th>
                  <th className="px-4 py-4 text-left text-xs font-black uppercase tracking-widest text-foreground">类型</th>
                  <th className="px-4 py-4 text-left text-xs font-black uppercase tracking-widest text-foreground">状态</th>
                  <th className="w-48 px-4 py-4 text-left text-xs font-black uppercase tracking-widest text-foreground">进度</th>
                  <th className="px-4 py-4 text-left text-xs font-black uppercase tracking-widest text-foreground">目录数</th>
                  <th className="px-4 py-4 text-left text-xs font-black uppercase tracking-widest text-foreground">创建时间</th>
                  <th className="px-4 py-4 text-left text-xs font-black uppercase tracking-widest text-foreground">耗时</th>
                </tr>
              </thead>
              <tbody>
                {isLoading && jobs.length === 0 ? (
                  <tr>
                    <td colSpan={8} className="px-4 py-16 text-center font-bold text-muted-foreground">正在加载任务...</td>
                  </tr>
                ) : jobs.length === 0 ? (
                  <tr>
                    <td colSpan={8} className="px-4 py-16 text-center font-bold text-muted-foreground border-2 border-dashed border-foreground m-4">暂无任务记录，执行计划任务后会显示在这里。</td>
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
