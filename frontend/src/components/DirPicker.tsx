import { useCallback, useEffect, useState } from 'react'
import { ChevronRight, Folder, FolderOpen, Loader2, X } from 'lucide-react'

import { listDirs, type FsDirEntry } from '@/api/fs'
import { cn } from '@/lib/utils'

export interface DirPickerProps {
  open: boolean
  initialPath?: string
  onConfirm: (path: string) => void
  onCancel: () => void
  title?: string
}

interface NavEntry {
  path: string
  entries: FsDirEntry[]
}

export function DirPicker({ open, initialPath = '/', onConfirm, onCancel, title }: DirPickerProps) {
  const [current, setCurrent] = useState<NavEntry | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [selected, setSelected] = useState<string | null>(null)
  const [pathInput, setPathInput] = useState(initialPath)

  const navigate = useCallback(async (path: string) => {
    setIsLoading(true)
    setError(null)
    try {
      const res = await listDirs(path)
      setCurrent({ path: res.path, entries: res.entries })
      setSelected(res.path)
      setPathInput(res.path)
    } catch (e) {
      setError(e instanceof Error ? e.message : '无法读取目录')
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    if (open) {
      void navigate(initialPath)
    }
  }, [open, initialPath, navigate])

  function handleInputKeyDown(e: { key: string }) {
    if (e.key === 'Enter') {
      void navigate(pathInput.trim())
    }
  }

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="flex w-full max-w-lg flex-col rounded-2xl border border-border bg-card shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border px-5 py-4">
          <h2 className="text-base font-semibold">{title ?? '选择目录'}</h2>
          <button
            type="button"
            onClick={onCancel}
            className="rounded-lg border border-border p-1.5 transition hover:bg-accent"
            aria-label="关闭"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* Current path input */}
        <div className="border-b border-border px-5 py-3">
          <div className="flex gap-2">
            <input
              type="text"
              value={pathInput}
              placeholder="/path/to/dir"
              className="flex-1 rounded-lg border border-border bg-background px-3 py-2 text-sm outline-none focus:border-primary"
              onChange={(e) => setPathInput(e.target.value)}
              onKeyDown={handleInputKeyDown}
            />
            <button
              type="button"
              onClick={() => { void navigate(pathInput.trim()) }}
              className="rounded-lg border border-border px-3 py-2 text-sm transition hover:bg-accent"
            >
              前往
            </button>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">按 Enter 或点击「前往」跳转到指定路径</p>
        </div>

        {/* Directory listing */}
        <div className="max-h-72 overflow-y-auto">
          {/* Parent dir navigation */}
          {current && current.path !== '/' && (
            <button
              type="button"
              onClick={() => {
                const parent = current.path.split('/').slice(0, -1).join('/') || '/'
                void navigate(parent)
              }}
              className="flex w-full items-center gap-2 border-b border-border px-5 py-3 text-sm text-muted-foreground transition hover:bg-accent"
            >
              <ChevronRight className="h-4 w-4 rotate-180" />
              <span>上级目录</span>
            </button>
          )}

          {isLoading && (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          )}

          {error && (
            <div className="px-5 py-4 text-sm text-red-600">{error}</div>
          )}

          {!isLoading && !error && current && (
            <>
              {/* Current dir selectable row */}
              <button
                type="button"
                onClick={() => setSelected(current.path)}
                className={cn(
                  'flex w-full items-center gap-2 px-5 py-3 text-sm transition',
                  selected === current.path ? 'bg-primary/10 text-primary' : 'hover:bg-accent',
                )}
              >
                <FolderOpen className="h-4 w-4 shrink-0" />
                <span className="break-all text-left">{current.path}</span>
              </button>

              {current.entries.length === 0 && (
                <p className="px-5 py-4 text-sm text-muted-foreground">此目录下没有子目录。</p>
              )}

              {current.entries.map((entry) => (
                <div key={entry.path} className="flex items-center">
                  <button
                    type="button"
                    onClick={() => setSelected(entry.path)}
                    className={cn(
                      'flex flex-1 items-center gap-2 px-5 py-3 text-sm transition',
                      selected === entry.path ? 'bg-primary/10 text-primary' : 'hover:bg-accent',
                    )}
                  >
                    <Folder className="h-4 w-4 shrink-0" />
                    <span className="break-all text-left">{entry.name}</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => void navigate(entry.path)}
                    title="进入此目录"
                    className="mr-2 rounded-md px-2 py-1.5 text-muted-foreground transition hover:bg-accent hover:text-foreground"
                  >
                    <ChevronRight className="h-4 w-4" />
                  </button>
                </div>
              ))}
            </>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between border-t border-border px-5 py-4">
          <p className="max-w-[60%] truncate text-xs text-muted-foreground">
            {selected ?? '未选择'}
          </p>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={onCancel}
              className="rounded-lg border border-border px-4 py-2 text-sm transition hover:bg-accent"
            >
              取消
            </button>
            <button
              type="button"
              disabled={selected === null}
              onClick={() => { if (selected) onConfirm(selected) }}
              className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
            >
              确认选择
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
