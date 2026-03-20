import { create } from 'zustand'

import { getJobProgress, listJobs, type JobQueryParams } from '@/api/jobs'
import type { Job, JobProgress } from '@/types'

interface JobStore {
  jobs: Job[]
  total: number
  page: number
  limit: number
  isLoading: boolean
  error: string | null
  pollingJobIds: Set<string>
  pollingTimers: Map<string, number>
  fetchJobs: (params?: JobQueryParams) => Promise<void>
  addJob: (job: Job) => void
  updateJob: (jobId: string, updates: Partial<Job>) => void
  handleJobProgress: (progress: JobProgress) => void
  handleJobDone: (jobId: string) => void
  handleJobError: (jobId: string, error: string) => void
  startPolling: (jobId: string) => void
  stopPolling: (jobId: string) => void
  stopAllPolling: () => void
}

export const useJobStore = create<JobStore>((set, get) => ({
  jobs: [],
  total: 0,
  page: 1,
  limit: 20,
  isLoading: false,
  error: null,
  pollingJobIds: new Set(),
  pollingTimers: new Map(),

  async fetchJobs(params = {}) {
    set({ isLoading: true, error: null })

    try {
      const response = await listJobs({ page: get().page, limit: get().limit, ...params })
      set({
        jobs: response.data,
        total: response.total,
        page: response.page,
        limit: response.limit,
    isLoading: false,
      })

      response.data.forEach((job) => {
        if (job.status === 'running') {
          get().startPolling(job.id)
        }
      })
    } catch (error) {
      set({
        isLoading: false,
        error: error instanceof Error ? error.message : 'Failed to load jobs',
      })
    }
  },

  addJob(job) {
    set((state) => ({
      jobs: [job, ...state.jobs],
      total: state.total + 1,
    }))

    if (job.status === 'running') {
    get().startPolling(job.id)
    }
  },

  updateJob(jobId, updates) {
    set((state) => ({
   jobs: state.jobs.map((job) => (job.id === jobId ? { ...job, ...updates } : job)),
    }))

    const job = get().jobs.find((j) => j.id === jobId)
    if (job && ['succeeded', 'failed', 'partial', 'cancelled'].includes(job.status)) {
      get().stopPolling(jobId)
    }
  },

  handleJobProgress(progress) {
    get().updateJob(progress.job_id, {
      status: progress.status,
      done: progress.done,
      total: progress.total,
      failed: progress.failed,
      updated_at: progress.updated_at,
    })
  },

  handleJobDone(jobId) {
    get().updateJob(jobId, { status: 'succeeded' })
    get().stopPolling(jobId)
  },

  handleJobError(jobId, error) {
    get().updateJob(jobId, { status: 'failed', error })
    get().stopPolling(jobId)
  },

  startPolling(jobId) {
    const { pollingJobIds, pollingTimers } = get()

    if (pollingJobIds.has(jobId)) {
      return
    }

    pollingJobIds.add(jobId)

    const poll = async () => {
      try {
        const progress = await getJobProgress(jobId)
        get().handleJobProgress(progress)

        if (['succeeded', 'failed', 'partial', 'cancelled'].includes(progress.status)) {
          get().stopPolling(jobId)
        } else {
          const timer = window.setTimeout(poll, 2000)
          pollingTimers.set(jobId, timer)
        }
    } catch (error) {
        console.error(`Failed to poll job ${jobId}:`, error)
        get().stopPolling(jobId)
      }
    }

    void poll()
  },

  stopPolling(jobId) {
    const { pollingJobIds, pollingTimers } = get()

    pollingJobIds.delete(jobId)

    const timer = pollingTimers.get(jobId)
    if (timer !== undefined) {
      window.clearTimeout(timer)
      pollingTimers.delete(jobId)
    }
  },

  stopAllPolling() {
    const { pollingTimers } = get()

    pollingTimers.forEach((timer) => {
      window.clearTimeout(timer)
    })

    set({ pollingJobIds: new Set(), pollingTimers: new Map() })
  },
}))
