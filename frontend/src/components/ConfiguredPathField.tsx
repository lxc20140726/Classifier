import { useEffect, useMemo, useState } from 'react'
import { FolderOpen } from 'lucide-react'

import { DirPicker } from '@/components/DirPicker'
import { cn } from '@/lib/utils'
import { useConfigStore } from '@/store/configStore'
import type { ConfiguredPathCategory, ConfiguredPathOption } from '@/types'

type PathSourceMode = 'preset' | 'custom'

export interface ConfiguredPathFieldValue {
  path: string
  source: PathSourceMode
  optionId: string
}

export interface ConfiguredPathFieldProps {
  value: ConfiguredPathFieldValue
  allowedCategories: ConfiguredPathCategory[]
  placeholder?: string
  pickerTitle?: string
  onChange: (next: ConfiguredPathFieldValue) => void
}

function categoryLabel(category: ConfiguredPathOption['category']) {
  if (category === 'scan') return '扫描'
  if (category === 'target') return '目标'
  if (category === 'output') return '输出'
  return '通用'
}

export function ConfiguredPathField({
  value,
  allowedCategories,
  placeholder,
  pickerTitle,
  onChange,
}: ConfiguredPathFieldProps) {
  const [open, setOpen] = useState(false)
  const { sourceDir, pathOptions, load } = useConfigStore()

  useEffect(() => {
    void load()
  }, [load])

  const filteredOptions = useMemo(
    () => pathOptions.filter((item) => allowedCategories.includes(item.category)),
    [allowedCategories, pathOptions],
  )

  const selectedOption = useMemo(
    () => filteredOptions.find((item) => item.id === value.optionId),
    [filteredOptions, value.optionId],
  )

  const hasInvalidPreset = value.source === 'preset' && value.optionId !== '' && selectedOption == null

  return (
    <div className="space-y-2">
      <div className="grid grid-cols-2 gap-2">
        <button
          type="button"
          onClick={() => {
            onChange({
              path: selectedOption?.path ?? value.path,
              source: 'preset',
              optionId: selectedOption?.id ?? value.optionId,
            })
          }}
          className={cn(
            'border-2 px-3 py-2 text-xs font-bold transition-all',
            value.source === 'preset'
              ? 'border-foreground bg-foreground text-background'
              : 'border-foreground bg-background text-foreground',
          )}
        >
          配置路径
        </button>
        <button
          type="button"
          onClick={() => onChange({ path: value.path, source: 'custom', optionId: '' })}
          className={cn(
            'border-2 px-3 py-2 text-xs font-bold transition-all',
            value.source === 'custom'
              ? 'border-foreground bg-foreground text-background'
              : 'border-foreground bg-background text-foreground',
          )}
        >
          自定义路径
        </button>
      </div>

      {value.source === 'preset' ? (
        <div className="space-y-2">
          <select
            value={selectedOption?.id ?? ''}
            onChange={(event) => {
              const option = filteredOptions.find((item) => item.id === event.target.value)
              if (!option) {
                onChange({ path: value.path, source: 'preset', optionId: '' })
                return
              }
              onChange({ path: option.path, source: 'preset', optionId: option.id })
            }}
            className="w-full border-2 border-foreground bg-background px-3 py-2 text-sm font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-1"
          >
            <option value="">请选择配置路径</option>
            {filteredOptions.map((option) => (
              <option key={option.id} value={option.id}>
                {option.name}（{categoryLabel(option.category)}）- {option.path}
              </option>
            ))}
          </select>
          <input
            type="text"
            value={selectedOption?.path ?? value.path}
            readOnly
            className="w-full border-2 border-foreground bg-muted/20 px-3 py-2 text-xs font-mono font-bold"
          />
          {hasInvalidPreset && (
            <p className="border-2 border-amber-700 bg-amber-100 px-3 py-2 text-xs font-bold text-amber-900">
              原路径选项已失效，当前保留落盘路径：{value.path || '（空）'}
            </p>
          )}
        </div>
      ) : (
        <>
          <div className="flex gap-2">
            <input
              type="text"
              value={value.path}
              onChange={(event) => onChange({ path: event.target.value, source: 'custom', optionId: '' })}
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
          </div>
          <DirPicker
            open={open}
            initialPath={value.path || sourceDir}
            title={pickerTitle}
            onConfirm={(path) => {
              onChange({ path, source: 'custom', optionId: '' })
              setOpen(false)
            }}
            onCancel={() => setOpen(false)}
          />
        </>
      )}
    </div>
  )
}
