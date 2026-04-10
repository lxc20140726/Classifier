import { useEffect, useMemo, useState } from 'react'
import { FolderOpen } from 'lucide-react'

import { DirPicker } from '@/components/DirPicker'
import { cn } from '@/lib/utils'
import { useConfigStore } from '@/store/configStore'

export type PathRefType = 'scan' | 'output' | 'custom'

export interface ConfiguredPathFieldValue {
  pathRefType: PathRefType
  pathRefKey: string
  pathSuffix: string
}

export interface ConfiguredPathFieldProps {
  value: ConfiguredPathFieldValue
  placeholder?: string
  pickerTitle?: string
  defaultOutputKey?: 'video' | 'manga' | 'photo' | 'other' | 'mixed'
  onChange: (next: ConfiguredPathFieldValue) => void
}

const OUTPUT_KEYS: Array<{ key: 'video' | 'manga' | 'photo' | 'other' | 'mixed'; label: string }> = [
  { key: 'video', label: '视频' },
  { key: 'manga', label: '漫画' },
  { key: 'photo', label: '写真' },
  { key: 'other', label: '其他' },
  { key: 'mixed', label: '混合' },
]

export function ConfiguredPathField({
  value,
  placeholder,
  pickerTitle,
  defaultOutputKey = 'mixed',
  onChange,
}: ConfiguredPathFieldProps) {
  const [open, setOpen] = useState(false)
  const { scanInputDirs, outputDirs, load } = useConfigStore()

  useEffect(() => {
    void load()
  }, [load])

  const resolvedOutputPath = useMemo(() => {
    const key = value.pathRefKey as keyof typeof outputDirs
    return outputDirs[key] ?? ''
  }, [outputDirs, value.pathRefKey])

  const resolvedScanPath = useMemo(() => {
    if (scanInputDirs.length === 0) return ''
    if (value.pathRefKey.trim() === '') return scanInputDirs[0]
    const idx = Number.parseInt(value.pathRefKey, 10)
    if (Number.isInteger(idx) && idx >= 0 && idx < scanInputDirs.length) {
      return scanInputDirs[idx]
    }
    return scanInputDirs.find((item) => item === value.pathRefKey) ?? ''
  }, [scanInputDirs, value.pathRefKey])

  return (
    <div className="space-y-2">
      <div className="grid grid-cols-3 gap-2">
        {(['scan', 'output', 'custom'] as const).map((mode) => (
          <button
            key={mode}
            type="button"
            onClick={() => {
              if (mode === 'scan') {
                onChange({ pathRefType: 'scan', pathRefKey: '0', pathSuffix: value.pathSuffix })
                return
              }
              if (mode === 'output') {
                onChange({ pathRefType: 'output', pathRefKey: defaultOutputKey, pathSuffix: value.pathSuffix })
                return
              }
              onChange({ pathRefType: 'custom', pathRefKey: value.pathRefKey, pathSuffix: value.pathSuffix })
            }}
            className={cn(
              'border-2 px-3 py-2 text-xs font-bold transition-all',
              value.pathRefType === mode
                ? 'border-foreground bg-foreground text-background'
                : 'border-foreground bg-background text-foreground',
            )}
          >
            {mode === 'scan' ? '扫描目录' : mode === 'output' ? '输出目录' : '自定义路径'}
          </button>
        ))}
      </div>

      {value.pathRefType === 'scan' && (
        <div className="space-y-2">
          <select
            value={value.pathRefKey}
            onChange={(event) => onChange({ ...value, pathRefKey: event.target.value })}
            className="w-full border-2 border-foreground bg-background px-3 py-2 text-sm font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-1"
          >
            {scanInputDirs.length === 0 && <option value="">暂无扫描目录，请先去系统配置填写</option>}
            {scanInputDirs.map((path, index) => (
              <option key={`${path}-${index}`} value={String(index)}>
                {`#${index + 1} ${path}`}
              </option>
            ))}
          </select>
          <input
            type="text"
            value={resolvedScanPath}
            readOnly
            className="w-full border-2 border-foreground bg-muted/20 px-3 py-2 text-xs font-mono font-bold"
          />
        </div>
      )}

      {value.pathRefType === 'output' && (
        <div className="space-y-2">
          <select
            value={value.pathRefKey}
            onChange={(event) => onChange({ ...value, pathRefKey: event.target.value })}
            className="w-full border-2 border-foreground bg-background px-3 py-2 text-sm font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-1"
          >
            {OUTPUT_KEYS.map((option) => (
              <option key={option.key} value={option.key}>{option.label}</option>
            ))}
          </select>
          <input
            type="text"
            value={resolvedOutputPath}
            readOnly
            className="w-full border-2 border-foreground bg-muted/20 px-3 py-2 text-xs font-mono font-bold"
          />
        </div>
      )}

      {value.pathRefType === 'custom' && (
        <div className="flex gap-2">
          <input
            type="text"
            value={value.pathRefKey}
            onChange={(event) => onChange({ ...value, pathRefKey: event.target.value })}
            placeholder={placeholder ?? '/data/path'}
            className="w-full border-2 border-foreground bg-background px-3 py-2 text-sm font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-1"
          />
          <button
            type="button"
            onClick={() => setOpen(true)}
            className="shrink-0 border-2 border-foreground bg-background px-3 py-2 text-foreground transition-all hover:bg-foreground hover:text-background hover:-translate-y-0.5"
          >
            <FolderOpen className="h-4 w-4" />
          </button>
          <DirPicker
            open={open}
            initialPath={value.pathRefKey || '/'}
            title={pickerTitle}
            onConfirm={(path) => {
              onChange({ ...value, pathRefKey: path })
              setOpen(false)
            }}
            onCancel={() => setOpen(false)}
          />
        </div>
      )}

      <input
        type="text"
        value={value.pathSuffix}
        onChange={(event) => onChange({ ...value, pathSuffix: event.target.value })}
        placeholder="可选：相对子目录（例如 .processed）"
        className="w-full border-2 border-foreground bg-background px-3 py-2 text-xs font-mono font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-1"
      />
    </div>
  )
}
