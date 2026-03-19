import { useEffect } from 'react'

import { useFolderStore } from '@/store/folderStore'

interface ScanProgressEvent {
  scanned: number
  total: number
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

      eventSource.addEventListener('job.done', () => {
        void useFolderStore.getState().fetchFolders()
      })

      eventSource.addEventListener('job.error', () => {})

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
    }
  }, [])
}
