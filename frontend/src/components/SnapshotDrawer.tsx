import { useEffect, useState } from 'react'
import { RotateCcw, X } from 'lucide-react'

import { revertSnapshot } from '@/api/snapshots'
import { cn } from '@/lib/utils'
import { useSnapshotStore } from '@/store/snapshotStore'
import type { Snapshot } from '@/types'

export interface SnapshotDrawerProps {
  open: boolean
  folderId: string | null
  onClose: () => void
}

interface DrawerState {
  revertingId: string | null
  localError: string | null
}

const OP_LABEL: Record<Snapshot['operation_type'], string> = {
  rename: 'Rename',
  move: 'Move',
}

const STATUS_CLASSES: Record<Snapshot['status'], string> = {
  pending: 'bg-yellow-100 text-yellow-800',
  committed: 'bg-green-100 text-green-800',
  reverted: 'bg-gray-100 text-gray-500',
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString()
}

export function SnapshotDrawer({ open, folderId, onClose }: SnapshotDrawerProps) {
  const snapshots = useSnapshotStore((store) => store.snapshots)
  const isLoading = useSnapshotStore((store) => store.isLoading)
  const storeError = useSnapshotStore((store) => store.error)
  const fetchSnapshots = useSnapshotStore((store) => store.fetchSnapshots)
  const handleRevertDone = useSnapshotStore((store) => store.handleRevertDone)
  const [state, setState] = useState<DrawerState>({ revertingId: null, localError: null })

  useEffect(() => {
    if (open && folderId) {
      void fetchSnapshots(folderId)
    }
  }, [fetchSnapshots, folderId, open])

  async function handleRevert(snapshotId: string) {
    if (!folderId) return

    setState({ revertingId: snapshotId, localError: null })

    try {
      await revertSnapshot(snapshotId)
      handleRevertDone(snapshotId)
      await fetchSnapshots(folderId)
      setState({ revertingId: null, localError: null })
    } catch (error) {
      setState({
        revertingId: null,
        localError: error instanceof Error ? error.message : 'Revert failed',
      })
    }
  }

  const error = state.localError ?? storeError

  return (
    <>
      <div
        className={cn(
          'fixed inset-0 z-40 bg-black/40 transition-opacity',
          open ? 'pointer-events-auto opacity-100' : 'pointer-events-none opacity-0',
        )}
        onClick={onClose}
        aria-hidden="true"
      />

      <aside
        className={cn(
          'fixed right-0 top-0 z-50 flex h-full w-full max-w-md flex-col bg-white shadow-xl transition-transform duration-200',
          open ? 'translate-x-0' : 'translate-x-full',
        )}
        aria-label="Snapshot history"
      >
        <div className="flex items-center justify-between border-b px-4 py-3">
          <h2 className="text-base font-semibold">Snapshot History</h2>
          <button
            type="button"
            onClick={onClose}
            className="rounded p-1 hover:bg-gray-100"
            aria-label="Close snapshot drawer"
          >
            <X size={18} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-4 py-3 text-sm">
          {error && <div className="mb-3 rounded bg-red-50 px-3 py-2 text-red-700">{error}</div>}

          {isLoading && <p className="text-gray-500">Loading snapshots...</p>}

          {!isLoading && !error && snapshots.length === 0 && (
            <p className="text-gray-500">No snapshots found for this folder.</p>
          )}

          {snapshots.length > 0 && (
            <ul className="space-y-3">
              {snapshots.map((snapshot) => (
                <li key={snapshot.id} className="rounded-lg border p-3">
                  <div className="flex items-start justify-between gap-2">
                    <div className="space-y-1">
                      <div className="flex items-center gap-2">
                        <span className="font-medium">{OP_LABEL[snapshot.operation_type]}</span>
                        <span
                          className={cn(
                            'rounded px-1.5 py-0.5 text-xs font-medium',
                            STATUS_CLASSES[snapshot.status],
                          )}
                        >
                          {snapshot.status}
                        </span>
                      </div>
                      <p className="text-xs text-gray-400">{formatDate(snapshot.created_at)}</p>
                      <p className="text-xs text-gray-500">
                        {snapshot.before.length} file{snapshot.before.length !== 1 ? 's' : ''}
                      </p>
                    </div>

                    {snapshot.status !== 'reverted' && (
                      <button
                        type="button"
                        disabled={state.revertingId !== null}
                        onClick={() => {
                          void handleRevert(snapshot.id)
                        }}
                        className="flex shrink-0 items-center gap-1 rounded border px-2 py-1 text-xs hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
                        aria-label="Revert snapshot"
                      >
                        <RotateCcw size={12} />
                        {state.revertingId === snapshot.id ? 'Reverting...' : 'Revert'}
                      </button>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      </aside>
    </>
  )
}
