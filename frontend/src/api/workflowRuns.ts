import { request } from '@/api/client'
import type { Job, NodeRun, PaginatedResponse, WorkflowRun, WorkflowRunDetail } from '@/types'

export interface WorkflowRunQueryParams {
  page?: number
  limit?: number
}

export interface StartWorkflowJobBody {
  workflow_def_id: string
  folder_ids: string[]
}

export function startWorkflowJob(body: StartWorkflowJobBody) {
  return request<{ data: Job }>('/jobs', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function listWorkflowRunsByJob(
  jobId: string,
  params: WorkflowRunQueryParams = {},
) {
  const search = new URLSearchParams()
  if (params.page) search.set('page', String(params.page))
  if (params.limit) search.set('limit', String(params.limit))
  const suffix = search.toString() ? `?${search.toString()}` : ''
  return request<PaginatedResponse<WorkflowRun>>(`/jobs/${jobId}/workflow-runs${suffix}`)
}

export function getWorkflowRunDetail(id: string) {
  return request<WorkflowRunDetail>(`/workflow-runs/${id}`)
}

export function listNodeRunsByWorkflowRun(workflowRunId: string) {
  return request<{ data: NodeRun[] }>(`/workflow-runs/${workflowRunId}/nodes`)
}

export function resumeWorkflowRun(id: string) {
  return request<{ resumed: boolean }>(`/workflow-runs/${id}/resume`, { method: 'POST' })
}

export function rollbackWorkflowRun(id: string) {
  return request<{ rolled_back: boolean }>(`/workflow-runs/${id}/rollback`, { method: 'POST' })
}
