import { request } from '@/api/client'
import type { AppConfig } from '@/types'

export function getConfig() {
  return request<{ data: AppConfig }>('/config')
}

export function updateConfig(values: AppConfig) {
  return request<{ saved: boolean }>('/config', {
    method: 'PUT',
    body: JSON.stringify(values),
  })
}
