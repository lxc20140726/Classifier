import { request } from '@/api/client'
import type { NodeRun, PaginatedResponse, ProvideInputBody, WorkflowRun, WorkflowRunDetail } from '@/types'

export interface WorkflowRunQueryParams {
  page?: number
  limit?: number
}

export interface StartWorkflowJobBody {
  workflow_def_id: string
  folder_ids: string[]
}

export function startWorkflowJob(body: StartWorkflowJobBody) {
  return request<{ job_id: string }>('/jobs', {
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

export function provideWorkflowRunInput(id: string, body: ProvideInputBody) {
  return request<undefined>(`/workflow-runs/${id}/provide-input`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function provideWorkflowRunRawInput(id: string, body: Record<string, unknown>) {
  return request<undefined>(`/workflow-runs/${id}/provide-input`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}
