import { useEffect, useState } from 'react'

import { ChevronDown, ChevronRight, RefreshCw } from 'lucide-react'

import { cn } from '@/lib/utils'
import { useJobStore } from '@/store/jobStore'
import type { Job, JobStatus } from '@/types'

function formatDate(dateStr: string | null) {
  if (!dateStr) return '—'
  return new Date(dateStr).toLocaleString()
}

function formatDuration(startedAt: string | null, finishedAt: string | null) {
  if (!startedAt) return '—'
  const end = finishedAt ? new Date(finishedAt) : new Date()
  const start = new Date(startedAt)
  const secs = Math.floor((end.getTime() - start.getTime()) / 1000)
  if (secs < 60) return `${secs}s`
  if (secs < 3600) return `${Math.floor(secs / 60)}m ${secs % 60}s`
  return `${Math.floor(secs / 3600)}h ${Math.floor((secs % 3600) / 60)}m`
}

const STATUS_STYLES: Record<JobStatus, string> = {
  pending: 'bg-muted text-muted-foreground',
  running: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300',
  succeeded: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300',
  failed: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300',
  partial: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-300',
  cancelled: 'bg-muted text-muted-foreground',
}

function StatusBadge({ status }: { status: JobStatus }) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
        STATUS_STYLES[status],
      )}
    >
      {status}
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
            aria-label={expanded ? 'Collapse' : 'Expand'}
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
          <StatusBadge status={job.status} />
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
            <div className="grid gap-3 text-sm">
              {job.error && (
                <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2">
                  <p className="font-medium text-destructive">Error</p>
                  <p className="mt-0.5 text-destructive/80">{job.error}</p>
                </div>
              )}
              <div className="grid grid-cols-2 gap-x-8 gap-y-1.5 sm:grid-cols-3">
                <div>
                  <p className="text-xs text-muted-foreground">Job ID</p>
                  <p className="font-mono text-xs">{job.id}</p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">Type</p>
                  <p>{job.type}</p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">Status</p>
                  <StatusBadge status={job.status} />
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">Folders</p>
                  <p>{job.folder_ids.length} selected</p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">Done / Total</p>
                  <p>
                    {job.done} / {job.total}
                    {job.failed > 0 && (
                      <span className="ml-1 text-destructive">({job.failed} failed)</span>
                    )}
                  </p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">Started</p>
                  <p>{formatDate(job.started_at)}</p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">Finished</p>
                  <p>{formatDate(job.finished_at)}</p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">Created</p>
                  <p>{formatDate(job.created_at)}</p>
                </div>
              </div>
              {job.folder_ids.length > 0 && (
                <div>
                  <p className="text-xs text-muted-foreground">Folder IDs</p>
                  <ul className="mt-1 space-y-0.5">
                    {job.folder_ids.map((fid) => (
                      <li key={fid} className="font-mono text-xs">
                        {fid}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
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
          <h1 className="text-2xl font-semibold tracking-tight">Jobs</h1>
          <p className="mt-0.5 text-sm text-muted-foreground">
            {total} job{total !== 1 ? 's' : ''} total
          </p>
        </div>
        <button
          type="button"
          onClick={() => void fetchJobs()}
          disabled={isLoading}
          className="flex items-center gap-2 rounded-md border border-input bg-background px-3 py-1.5 text-sm font-medium shadow-sm transition-colors hover:bg-accent disabled:opacity-50"
        >
          <RefreshCw className={cn('h-3.5 w-3.5', isLoading && 'animate-spin')} />
          Refresh
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
                Type
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Status
              </th>
              <th className="w-48 px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Progress
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Folders
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Created
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Duration
              </th>
            </tr>
          </thead>
          <tbody>
            {isLoading && jobs.length === 0 ? (
              <tr>
                <td colSpan={8} className="px-4 py-12 text-center text-sm text-muted-foreground">
                  Loading jobs…
                </td>
              </tr>
            ) : jobs.length === 0 ? (
              <tr>
                <td colSpan={8} className="px-4 py-12 text-center text-sm text-muted-foreground">
                  No jobs yet. Move operations will appear here.
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
