import { useEffect } from 'react'

import { useFolderStore } from '@/store/folderStore'
import { useJobStore } from '@/store/jobStore'

interface ScanProgressEvent {
  scanned: number
  total: number
}

interface JobProgressEvent {
  job_id: string
  done: number
  total: number
}

interface JobDoneEvent {
  job_id: string
}

interface JobErrorEvent {
  job_id: string
  error: string
}

export function useSSE() {
  useEffect(() => {
    let eventSource: EventSource | null = null
    let reconnectTimer: number | null = null

    const connect = () => {
      eventSource = new EventSource('/api/events')

      eventSource.addEventListener('scan.progress', (event) => {
        const payload = JSON.parse(event.data) as ScanProgressEvent
        useFolderStore.getState().handleScanProgress(payload)
      })

      eventSource.addEventListener('scan.done', () => {
        const store = useFolderStore.getState()
        store.handleScanDone()
        void store.fetchFolders()
      })

      eventSource.addEventListener('job.progress', (event) => {
        const payload = JSON.parse(event.data) as JobProgressEvent
        useJobStore.getState().handleJobProgress({
          job_id: payload.job_id,
          status: 'running',
          done: payload.done,
          total: payload.total,
          failed: 0,
          updated_at: new Date().toISOString(),
        })
      })

      eventSource.addEventListener('job.done', (event) => {
        const payload = JSON.parse(event.data) as JobDoneEvent
        useJobStore.getState().handleJobDone(payload.job_id)
        void useFolderStore.getState().fetchFolders()
      })

      eventSource.addEventListener('job.error', (event) => {
        const payload = JSON.parse(event.data) as JobErrorEvent
        useJobStore.getState().handleJobError(payload.job_id, payload.error)
      })

      eventSource.onerror = () => {
        eventSource?.close()
        reconnectTimer = window.setTimeout(connect, 3000)
      }
    }

    connect()

    return () => {
      if (reconnectTimer !== null) {
        window.clearTimeout(reconnectTimer)
      }

      eventSource?.close()
      useJobStore.getState().stopAllPolling()
    }
  }, [])
}
