import { useEffect, useState } from 'react'
import { Plus, Trash2, FolderSearch } from 'lucide-react'

import { getConfig, updateConfig } from '@/api/config'
import { DirPicker } from '@/components/DirPicker'
import type { AppConfig } from '@/types'

interface FormState {
  scanInputDirs: string[]
  targetDir: string
  outputDirs: NonNullable<AppConfig['output_dirs']>
}

const INITIAL_FORM: FormState = {
  scanInputDirs: [],
  targetDir: '',
  outputDirs: {
    video: '',
    manga: '',
    photo: '',
    other: '',
    mixed: '',
  },
}

const OUTPUT_DIR_LABELS: Record<keyof NonNullable<AppConfig['output_dirs']>, string> = {
  video: '视频输出目录',
  manga: '漫画输出目录',
  photo: '写真输出目录',
  other: '其他输出目录',
  mixed: '混合输出目录',
}

export default function SettingsPage() {
  const [form, setForm] = useState<FormState>(INITIAL_FORM)
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)
  const [pickerOpen, setPickerOpen] = useState(false)
  const [pickerTarget, setPickerTarget] = useState<'scan' | 'target' | keyof NonNullable<AppConfig['output_dirs']>>('scan')

  useEffect(() => {
    let active = true

    async function loadConfig() {
      try {
        const response = await getConfig()
        if (!active) return

        setForm({
          scanInputDirs: response.data.scan_input_dirs ?? [],
          targetDir: response.data.target_dir ?? '',
          outputDirs: {
            video: response.data.output_dirs?.video ?? '',
            manga: response.data.output_dirs?.manga ?? '',
            photo: response.data.output_dirs?.photo ?? '',
            other: response.data.output_dirs?.other ?? '',
            mixed: response.data.output_dirs?.mixed ?? '',
          },
        })
        setError(null)
      } catch (loadError) {
        if (!active) return
        setError(loadError instanceof Error ? loadError.message : '加载配置失败')
      } finally {
        if (active) setIsLoading(false)
      }
    }

    void loadConfig()
    return () => { active = false }
  }, [])

  function removeDir(index: number) {
    setForm((prev) => ({
      ...prev,
      scanInputDirs: prev.scanInputDirs.filter((_, i) => i !== index),
    }))
  }

  function addDir(path: string) {
    setForm((prev) => {
      if (pickerTarget === 'scan') {
        return {
          ...prev,
          scanInputDirs: prev.scanInputDirs.includes(path)
            ? prev.scanInputDirs
            : [...prev.scanInputDirs, path],
        }
      }

      if (pickerTarget === 'target') {
        return {
          ...prev,
          targetDir: path,
        }
      }

      return {
        ...prev,
        outputDirs: {
          ...prev.outputDirs,
          [pickerTarget]: path,
        },
      }
    })
    setPickerOpen(false)
  }

  async function handleSubmit(e: { preventDefault(): void }) {
    e.preventDefault()
    setIsSaving(true)
    setError(null)
    setSuccess(null)

    try {
      await updateConfig({
        scan_input_dirs: form.scanInputDirs,
        source_dir: form.scanInputDirs[0] ?? '',
        target_dir: form.targetDir,
        output_dirs: form.outputDirs,
      })
      setSuccess('配置已保存')
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : '保存失败')
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <section className="mx-auto max-w-2xl px-4 py-8">
      <h1 className="mb-6 text-xl font-semibold">系统配置</h1>

      <form onSubmit={(e) => void handleSubmit(e)} className="space-y-8">
        {/* Scan input dirs */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <label className="block text-sm font-medium">扫描输入目录</label>
              <p className="text-xs text-muted-foreground">每次扫描会遍历以下所有目录的直接子文件夹。</p>
            </div>
            <button
              type="button"
              onClick={() => setPickerOpen(true)}
              className="flex items-center gap-1.5 rounded-lg border border-border px-3 py-2 text-sm transition hover:bg-accent"
            >
              <FolderSearch className="h-4 w-4" />
              添加目录
            </button>
          </div>

          {isLoading && <p className="text-sm text-muted-foreground">加载中…</p>}

          {!isLoading && form.scanInputDirs.length === 0 && (
            <div className="rounded-xl border border-dashed border-border px-5 py-8 text-center">
              <FolderSearch className="mx-auto mb-2 h-8 w-8 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">尚未配置扫描目录，点击「添加目录」开始。</p>
            </div>
          )}

          <ul className="space-y-2">
            {form.scanInputDirs.map((dir, idx) => (
              <li
                key={dir}
                className="flex items-center justify-between rounded-xl border border-border bg-card px-4 py-3"
              >
                <span className="break-all text-sm font-mono text-foreground">{dir}</span>
                <button
                  type="button"
                  onClick={() => removeDir(idx)}
                  className="ml-3 shrink-0 rounded-lg p-1.5 text-muted-foreground transition hover:bg-red-50 hover:text-red-600"
                  aria-label="删除"
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              </li>
            ))}
          </ul>

          {form.scanInputDirs.length > 0 && (
            <button
              type="button"
              onClick={() => setPickerOpen(true)}
              className="flex items-center gap-1.5 text-sm text-muted-foreground transition hover:text-foreground"
            >
              <Plus className="h-4 w-4" />
              添加更多目录
            </button>
          )}
        </div>

        <div className="space-y-3 rounded-2xl border border-border bg-card p-4">
          <div className="flex items-center justify-between gap-3">
            <div>
              <label className="block text-sm font-medium">默认目标根目录</label>
              <p className="text-xs text-muted-foreground">用于自动补全各分类输出路径，可单独覆盖。</p>
            </div>
            <button
              type="button"
              onClick={() => {
                setPickerTarget('target')
                setPickerOpen(true)
              }}
              className="flex items-center gap-1.5 rounded-lg border border-border px-3 py-2 text-sm transition hover:bg-accent"
            >
              <FolderSearch className="h-4 w-4" />
              选择目录
            </button>
          </div>
          <input
            value={form.targetDir}
            onChange={(event) => setForm((prev) => ({ ...prev, targetDir: event.target.value }))}
            className="w-full rounded-xl border border-border bg-background px-3 py-2 text-sm font-mono outline-none ring-primary focus:ring-2"
            placeholder="/data/target"
          />
        </div>

        <div className="space-y-3">
          <div>
            <label className="block text-sm font-medium">分类输出目录</label>
            <p className="text-xs text-muted-foreground">这些路径会写入结构化 app_config，供处理工作流中的 move-node 使用。</p>
          </div>
          <div className="grid gap-3 md:grid-cols-2">
            {Object.entries(OUTPUT_DIR_LABELS).map(([key, label]) => (
              <div key={key} className="rounded-2xl border border-border bg-card p-4">
                <div className="mb-2 flex items-center justify-between gap-3">
                  <div>
                    <p className="text-sm font-medium">{label}</p>
                    <p className="text-xs text-muted-foreground">对应 `{key}` 分类的 move-node 目标路径。</p>
                  </div>
                  <button
                    type="button"
                    onClick={() => {
                      setPickerTarget(key as keyof NonNullable<AppConfig['output_dirs']>)
                      setPickerOpen(true)
                    }}
                    className="rounded-lg border border-border px-2.5 py-1.5 text-xs transition hover:bg-accent"
                  >
                    选择
                  </button>
                </div>
                <input
                  value={form.outputDirs[key as keyof NonNullable<AppConfig['output_dirs']>] ?? ''}
                  onChange={(event) => setForm((prev) => ({
                    ...prev,
                    outputDirs: {
                      ...prev.outputDirs,
                      [key]: event.target.value,
                    },
                  }))}
                  className="w-full rounded-xl border border-border bg-background px-3 py-2 text-sm font-mono outline-none ring-primary focus:ring-2"
                  placeholder={`/data/target/${key}`}
                />
              </div>
            ))}
          </div>
        </div>

        {error && <p className="text-sm text-red-600">{error}</p>}
        {success && <p className="text-sm text-green-700">{success}</p>}

        <div className="flex justify-end">
          <button
            type="submit"
            disabled={isLoading || isSaving}
            className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {isSaving ? '保存中…' : '保存配置'}
          </button>
        </div>
      </form>

        <DirPicker
          open={pickerOpen}
          initialPath="/"
          title={pickerTarget === 'scan' ? '选择扫描输入目录' : '选择输出目录'}
          onConfirm={addDir}
          onCancel={() => setPickerOpen(false)}
        />
    </section>
  )
}
