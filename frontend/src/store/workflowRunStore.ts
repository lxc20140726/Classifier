import { create } from 'zustand'

import { getWorkflowRunDetail, listWorkflowRunsByJob, provideWorkflowRunInput, resumeWorkflowRun, rollbackWorkflowRun } from '@/api/workflowRuns'
import type { NodeRun, NodeRunStatus, NodeType, ProvideInputBody, WorkflowNodeEvent, WorkflowRun, WorkflowRunStatus } from '@/types'

interface WorkflowRunStore {
  runsByJobId: Record<string, WorkflowRun[]>
  nodesByRunId: Record<string, NodeRun[]>
  fetchingJobIds: Set<string>
  fetchingRunIds: Set<string>
  fetchRunsForJob: (jobId: string) => Promise<void>
  fetchRunDetail: (runId: string) => Promise<void>
  resumeRun: (runId: string) => Promise<void>
  rollbackRun: (runId: string) => Promise<void>
  provideInput: (runId: string, category: ProvideInputBody['category']) => Promise<void>
  handleNodeEvent: (event: WorkflowNodeEvent) => void
  updateRunStatus: (workflowRunId: string, status: WorkflowRunStatus) => void
}

export const useWorkflowRunStore = create<WorkflowRunStore>((set, get) => ({
  runsByJobId: {},
  nodesByRunId: {},
  fetchingJobIds: new Set(),
  fetchingRunIds: new Set(),

  async fetchRunsForJob(jobId) {
    if (get().fetchingJobIds.has(jobId)) return
    set((state) => ({ fetchingJobIds: new Set([...state.fetchingJobIds, jobId]) }))
    try {
      const response = await listWorkflowRunsByJob(jobId, { limit: 100 })
      set((state) => ({
        runsByJobId: { ...state.runsByJobId, [jobId]: response.data },
        fetchingJobIds: new Set([...state.fetchingJobIds].filter((id) => id !== jobId)),
      }))
    } catch (error) {
      console.error(`fetchRunsForJob ${jobId}:`, error)
      set((state) => ({
        fetchingJobIds: new Set([...state.fetchingJobIds].filter((id) => id !== jobId)),
      }))
    }
  },

  async fetchRunDetail(runId) {
    if (get().fetchingRunIds.has(runId)) return
    set((state) => ({ fetchingRunIds: new Set([...state.fetchingRunIds, runId]) }))
    try {
      const response = await getWorkflowRunDetail(runId)
      const jobId = response.data.job_id
      set((state) => {
        const existingRuns = state.runsByJobId[jobId] ?? []
        const idx = existingRuns.findIndex((r) => r.id === runId)
        const updatedRuns =
          idx !== -1
            ? existingRuns.map((r, i) => (i === idx ? response.data : r))
            : [...existingRuns, response.data]
        return {
          runsByJobId: { ...state.runsByJobId, [jobId]: updatedRuns },
          nodesByRunId: { ...state.nodesByRunId, [runId]: response.node_runs },
          fetchingRunIds: new Set([...state.fetchingRunIds].filter((id) => id !== runId)),
        }
      })
    } catch (error) {
      console.error(`fetchRunDetail ${runId}:`, error)
      set((state) => ({
        fetchingRunIds: new Set([...state.fetchingRunIds].filter((id) => id !== runId)),
      }))
    }
  },

  async resumeRun(runId) {
    await resumeWorkflowRun(runId)
  },

  async rollbackRun(runId) {
    await rollbackWorkflowRun(runId)
  },

  async provideInput(runId, category) {
    await provideWorkflowRunInput(runId, { category })
    await get().fetchRunDetail(runId)
  },

  handleNodeEvent(event) {
    const { workflow_run_id, node_id, node_type, error } = event
    const status: NodeRunStatus = error ? 'failed' : (event.status ?? 'running')

    set((state) => {
      const existing = state.nodesByRunId[workflow_run_id] ?? []
      const idx = existing.findIndex((n) => n.node_id === node_id)
      let updated: NodeRun[]
      if (idx !== -1) {
        updated = existing.map((n, i) =>
          i === idx ? { ...n, status, error: error ?? n.error } : n,
        )
      } else {
        const placeholder: NodeRun = {
          id: '',
          workflow_run_id,
          node_id,
          node_type: node_type as NodeType,
          sequence: 0,
          status,
          input_json: '',
          output_json: '',
          error: error ?? '',
          started_at: status === 'running' ? new Date().toISOString() : null,
          finished_at: status !== 'running' ? new Date().toISOString() : null,
          created_at: new Date().toISOString(),
        }
        updated = [...existing, placeholder]
      }
      return { nodesByRunId: { ...state.nodesByRunId, [workflow_run_id]: updated } }
    })
  },

  updateRunStatus(workflowRunId, status) {
    set((state) => {
      const updatedRunsByJobId = { ...state.runsByJobId }
      for (const jobId of Object.keys(updatedRunsByJobId)) {
        const runs = updatedRunsByJobId[jobId]
        const idx = runs.findIndex((r) => r.id === workflowRunId)
        if (idx !== -1) {
          updatedRunsByJobId[jobId] = runs.map((r, i) =>
            i === idx ? { ...r, status } : r,
          )
          break
        }
      }
      return { runsByJobId: updatedRunsByJobId }
    })
  },
}))
