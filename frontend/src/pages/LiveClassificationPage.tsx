import { useEffect, useMemo } from 'react'
import { Link } from 'react-router-dom'
import { AlertTriangle, CheckCircle2, CircleDashed, Clock3, Loader2, Workflow } from 'lucide-react'

import { cn } from '@/lib/utils'
import { useLiveClassificationStore } from '@/store/liveClassificationStore'
import type { Category, LiveClassificationItem, LiveClassificationStatus } from '@/types'

const CATEGORY_LABEL: Record<Category, string> = {
  photo: '写真',
  video: '视频',
  mixed: '混合',
  manga: '漫画',
  other: '其他',
}

const CATEGORY_SOURCE_LABEL: Record<LiveClassificationItem['category_source'], string> = {
  auto: '自动',
  manual: '手动',
  workflow: '工作流',
}

const STATUS_LABEL: Record<LiveClassificationStatus, string> = {
  scanning: '扫描中',
  classifying: '分类中',
  waiting_input: '待确认',
  completed: '已完成',
  failed: '失败',
}

const STATUS_TONE: Record<LiveClassificationStatus, string> = {
  scanning: 'border-blue-900 bg-blue-100 text-blue-900',
  classifying: 'border-indigo-900 bg-indigo-100 text-indigo-900',
  waiting_input: 'border-amber-900 bg-amber-100 text-amber-900',
  completed: 'border-green-900 bg-green-100 text-green-900',
  failed: 'border-red-900 bg-red-100 text-red-900',
}

const STATUS_ORDER: LiveClassificationStatus[] = ['scanning', 'classifying', 'waiting_input', 'completed', 'failed']

function formatTime(value: string): string {
  const timestamp = Date.parse(value)
  if (Number.isNaN(timestamp)) return '-'
  return new Date(timestamp).toLocaleString('zh-CN')
}

function StatusIcon({ status }: { status: LiveClassificationStatus }) {
  if (status === 'completed') return <CheckCircle2 className="h-4 w-4" />
  if (status === 'failed') return <AlertTriangle className="h-4 w-4" />
  if (status === 'waiting_input') return <Clock3 className="h-4 w-4" />
  if (status === 'classifying') return <Workflow className="h-4 w-4" />
  return <CircleDashed className="h-4 w-4" />
}

function LiveCard({ item }: { item: LiveClassificationItem }) {
  return (
    <article className="border-2 border-foreground bg-card p-4 shadow-hard">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="truncate text-sm font-black tracking-tight" title={item.folder_name}>
            {item.folder_name}
          </h3>
          <p className="mt-1 truncate font-mono text-[11px] text-muted-foreground" title={item.folder_path}>
            {item.folder_path}
          </p>
        </div>
        <span className={cn('inline-flex items-center gap-1 border-2 px-2 py-0.5 text-[11px] font-black', STATUS_TONE[item.classification_status])}>
          <StatusIcon status={item.classification_status} />
          {STATUS_LABEL[item.classification_status]}
        </span>
      </div>

      <dl className="mt-3 space-y-1 text-xs">
        <div className="flex items-center justify-between gap-2">
          <dt className="text-muted-foreground">当前分类</dt>
          <dd className="font-bold">{CATEGORY_LABEL[item.category]} / {CATEGORY_SOURCE_LABEL[item.category_source]}</dd>
        </div>
        <div className="flex items-center justify-between gap-2">
          <dt className="text-muted-foreground">当前节点</dt>
          <dd className="truncate font-mono font-bold">{item.node_id || '-'} {item.node_type ? `(${item.node_type})` : ''}</dd>
        </div>
        <div className="flex items-center justify-between gap-2">
          <dt className="text-muted-foreground">最近更新时间</dt>
          <dd className="font-bold">{formatTime(item.last_event_at)}</dd>
        </div>
        <div className="flex items-center justify-between gap-2">
          <dt className="text-muted-foreground">错误/原因</dt>
          <dd className="truncate font-bold text-red-800">{item.error || '-'}</dd>
        </div>
      </dl>

      <div className="mt-3 flex items-center gap-2 border-t-2 border-foreground pt-3">
        <Link
          to="/"
          className="border-2 border-foreground bg-background px-2 py-1 text-[11px] font-bold transition-all hover:-translate-y-0.5 hover:bg-foreground hover:text-background hover:shadow-hard"
        >
          目录列表
        </Link>
        {item.job_id && (
          <Link
            to={`/job-history?job_id=${encodeURIComponent(item.job_id)}${item.workflow_run_id ? `&workflow_run_id=${encodeURIComponent(item.workflow_run_id)}` : ''}`}
            className="border-2 border-foreground bg-background px-2 py-1 text-[11px] font-bold transition-all hover:-translate-y-0.5 hover:bg-foreground hover:text-background hover:shadow-hard"
          >
            作业历史
          </Link>
        )}
      </div>
    </article>
  )
}

export default function LiveClassificationPage() {
  const orderedIds = useLiveClassificationStore((s) => s.orderedIds)
  const itemsById = useLiveClassificationStore((s) => s.itemsById)
  const currentScanJobId = useLiveClassificationStore((s) => s.currentScanJobId)
  const isLoading = useLiveClassificationStore((s) => s.isLoading)
  const error = useLiveClassificationStore((s) => s.error)
  const loadInitial = useLiveClassificationStore((s) => s.loadInitial)

  useEffect(() => {
    if (orderedIds.length === 0) {
      void loadInitial()
    }
  }, [loadInitial, orderedIds.length])

  const grouped = useMemo(() => {
    const map: Record<LiveClassificationStatus, LiveClassificationItem[]> = {
      scanning: [],
      classifying: [],
      waiting_input: [],
      completed: [],
      failed: [],
    }
    for (const id of orderedIds) {
      const item = itemsById[id]
      if (!item) continue
      map[item.classification_status].push(item)
    }
    return map
  }, [itemsById, orderedIds])

  return (
    <section className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-4 border-b-2 border-foreground pb-4">
        <div>
          <h1 className="text-3xl font-black tracking-tight">实时分类</h1>
          <p className="mt-1 text-sm font-bold text-muted-foreground">
            按目录聚合实时状态，最新变化始终置顶
          </p>
        </div>
        {currentScanJobId && (
          <div className="inline-flex items-center gap-2 border-2 border-foreground bg-blue-100 px-3 py-2 text-xs font-black text-blue-900">
            <Loader2 className="h-4 w-4 animate-spin" />
            扫描任务进行中：{currentScanJobId}
          </div>
        )}
      </div>

      {error && (
        <div className="border-2 border-red-900 bg-red-100 px-4 py-3 text-sm font-bold text-red-900">
          {error}
        </div>
      )}

      {isLoading && orderedIds.length === 0 ? (
        <div className="flex items-center justify-center border-2 border-foreground bg-card py-16">
          <Loader2 className="h-6 w-6 animate-spin" />
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-5">
          {STATUS_ORDER.map((status) => (
            <section key={status} className="border-2 border-foreground bg-muted/20 p-3">
              <header className="mb-3 flex items-center justify-between border-b-2 border-foreground pb-2">
                <span className="text-sm font-black">{STATUS_LABEL[status]}</span>
                <span className="border-2 border-foreground bg-background px-1.5 py-0.5 text-[11px] font-black">
                  {grouped[status].length}
                </span>
              </header>
              <div className="space-y-3">
                {grouped[status].length === 0 ? (
                  <p className="py-6 text-center text-xs font-bold text-muted-foreground">暂无目录</p>
                ) : (
                  grouped[status].map((item) => <LiveCard key={item.folder_id} item={item} />)
                )}
              </div>
            </section>
          ))}
        </div>
      )}
    </section>
  )
}
