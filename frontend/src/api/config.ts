import { request } from '@/api/client'
import type { AppConfig } from '@/types'

type RawConfig = Record<string, string>

function parseScanInputDirs(value?: string): string[] {
  if (value == null || value.trim() === '') {
    return []
  }

  try {
    const parsed = JSON.parse(value) as unknown
    if (Array.isArray(parsed)) {
      return parsed.filter((item): item is string => typeof item === 'string' && item.trim() !== '')
    }
  } catch {
    return value
      .split('\n')
      .map((item) => item.trim())
      .filter(Boolean)
  }

  return []
}

export async function getConfig(): Promise<{ data: AppConfig }> {
  const response = await request<{ data: RawConfig }>('/config')
  return {
    data: {
      source_dir: response.data.source_dir,
      target_dir: response.data.target_dir,
      scan_input_dirs: parseScanInputDirs(response.data.scan_input_dirs),
    },
  }
}

export function updateConfig(values: Record<string, string>) {
  return request<{ saved: boolean }>('/config', {
    method: 'PUT',
    body: JSON.stringify(values),
  })
}
