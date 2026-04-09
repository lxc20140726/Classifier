import { useEffect, useState } from 'react'
import { FolderSearch, Plus, Trash2 } from 'lucide-react'

import { getConfig, updateConfig } from '@/api/config'
import { DirPicker } from '@/components/DirPicker'
import { useConfigStore } from '@/store/configStore'
import type { AppConfig, ConfiguredPathCategory, ConfiguredPathOption } from '@/types'

interface FormState {
  scanInputDirs: string[]
  outputDirs: NonNullable<AppConfig['output_dirs']>
  pathOptions: ConfiguredPathOption[]
}

type PickerTarget =
  | { kind: 'scan'; index: number }
  | { kind: 'output'; key: keyof NonNullable<AppConfig['output_dirs']> }
  | { kind: 'path_option'; index: number }

const INITIAL_FORM: FormState = {
  scanInputDirs: [''],
  outputDirs: {
    video: '',
    manga: '',
    photo: '',
    other: '',
    mixed: '',
  },
  pathOptions: [],
}

const OUTPUT_DIR_LABELS: Record<keyof NonNullable<AppConfig['output_dirs']>, string> = {
  video: '视频输出目录',
  manga: '漫画输出目录',
  photo: '写真输出目录',
  other: '其他输出目录',
  mixed: '混合输出目录',
}

const PATH_OPTION_CATEGORY_LABELS: Record<ConfiguredPathCategory, string> = {
  scan: '扫描目录',
  target: '目标目录',
  output: '输出目录',
  general: '通用目录',
}

function buildPathOptionID() {
  return `path-${Date.now()}-${Math.floor(Math.random() * 100000)}`
}

export default function SettingsPage() {
  const [form, setForm] = useState<FormState>(INITIAL_FORM)
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)
  const [pickerOpen, setPickerOpen] = useState(false)
  const [pickerTarget, setPickerTarget] = useState<PickerTarget>({ kind: 'scan', index: 0 })
  const { sourceDir, load: loadConfigStore } = useConfigStore()

  useEffect(() => {
    let active = true

    async function loadConfig() {
      try {
        const response = await getConfig()
        if (!active) return

        setForm({
          scanInputDirs: response.data.scan_input_dirs?.length
            ? response.data.scan_input_dirs
            : response.data.source_dir
              ? [response.data.source_dir]
              : [''],
          outputDirs: {
            video: response.data.output_dirs?.video ?? '',
            manga: response.data.output_dirs?.manga ?? '',
            photo: response.data.output_dirs?.photo ?? '',
            other: response.data.output_dirs?.other ?? '',
            mixed: response.data.output_dirs?.mixed ?? '',
          },
          pathOptions: response.data.path_options ?? [],
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

  useEffect(() => {
    void loadConfigStore()
  }, [loadConfigStore])

  function addDir(path: string) {
    setForm((prev) => {
      if (pickerTarget.kind === 'scan') {
        const nextScanDirs = [...prev.scanInputDirs]
        nextScanDirs[pickerTarget.index] = path
        return {
          ...prev,
          scanInputDirs: nextScanDirs,
        }
      }

      if (pickerTarget.kind === 'output') {
        return {
          ...prev,
          outputDirs: {
            ...prev.outputDirs,
            [pickerTarget.key]: path,
          },
        }
      }

      const nextPathOptions = [...prev.pathOptions]
      const current = nextPathOptions[pickerTarget.index]
      nextPathOptions[pickerTarget.index] = {
        ...current,
        path,
      }
      return {
        ...prev,
        pathOptions: nextPathOptions,
      }
    })
    setPickerOpen(false)
  }

  function addScanInputDirRow() {
    setForm((prev) => ({ ...prev, scanInputDirs: [...prev.scanInputDirs, ''] }))
  }

  function removeScanInputDirRow(index: number) {
    setForm((prev) => {
      const next = prev.scanInputDirs.filter((_, i) => i !== index)
      return { ...prev, scanInputDirs: next.length > 0 ? next : [''] }
    })
  }

  function updateScanInputDirRow(index: number, value: string) {
    setForm((prev) => ({
      ...prev,
      scanInputDirs: prev.scanInputDirs.map((item, i) => (i === index ? value : item)),
    }))
  }

  function addPathOption() {
    setForm((prev) => ({
      ...prev,
      pathOptions: [
        ...prev.pathOptions,
        { id: buildPathOptionID(), name: '', path: '', category: 'general' },
      ],
    }))
  }

  function removePathOption(index: number) {
    setForm((prev) => ({
      ...prev,
      pathOptions: prev.pathOptions.filter((_, i) => i !== index),
    }))
  }

  function updatePathOption(index: number, patch: Partial<ConfiguredPathOption>) {
    setForm((prev) => ({
      ...prev,
      pathOptions: prev.pathOptions.map((item, i) => (i === index ? { ...item, ...patch } : item)),
    }))
  }

  async function handleSubmit(e: { preventDefault(): void }) {
    e.preventDefault()
    setIsSaving(true)
    setError(null)
    setSuccess(null)

    try {
      const cleanedScanInputDirs = form.scanInputDirs.map((item) => item.trim()).filter((item) => item.length > 0)
      const cleanedPathOptions = form.pathOptions
        .map((item) => ({
          ...item,
          id: item.id.trim(),
          name: item.name.trim(),
          path: item.path.trim(),
        }))
        .filter((item) => item.id !== '' || item.name !== '' || item.path !== '')

      await updateConfig({
        scan_input_dirs: cleanedScanInputDirs,
        output_dirs: form.outputDirs,
        path_options: cleanedPathOptions,
      })
      await loadConfigStore(true)
      setSuccess('配置已保存')
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : '保存失败')
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <section className="mx-auto max-w-3xl px-6 py-8">
      <h1 className="mb-8 text-3xl font-black tracking-tight uppercase">系统配置</h1>

      <form onSubmit={(e) => void handleSubmit(e)} className="space-y-8">
        <div className="space-y-4 border-2 border-foreground bg-card p-6 shadow-hard">
          <div className="flex items-center justify-between gap-4">
            <div>
              <label className="block text-sm font-black tracking-widest">扫描输入目录（可多个）</label>
              <p className="mt-1 text-xs font-bold text-muted-foreground">手动扫描和扫描计划任务围绕这组目录工作。</p>
            </div>
            <button
              type="button"
              onClick={addScanInputDirRow}
              className="flex items-center gap-2 border-2 border-foreground bg-background px-4 py-2 text-sm font-bold transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5"
            >
              <Plus className="h-4 w-4" />
              添加目录
            </button>
          </div>
          <div className="space-y-2">
            {form.scanInputDirs.map((scanInputDir, index) => (
              <div key={index} className="flex gap-2">
                <input
                  value={scanInputDir}
                  onChange={(event) => updateScanInputDirRow(index, event.target.value)}
                  className="w-full border-2 border-foreground bg-background px-4 py-3 text-sm font-mono font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-2 focus:ring-offset-background"
                  placeholder="/data/source"
                />
                <button
                  type="button"
                  onClick={() => {
                    setPickerTarget({ kind: 'scan', index })
                    setPickerOpen(true)
                  }}
                  className="border-2 border-foreground bg-background px-3 py-2 text-foreground transition-all hover:bg-foreground hover:text-background"
                >
                  <FolderSearch className="h-4 w-4" />
                </button>
                <button
                  type="button"
                  onClick={() => removeScanInputDirRow(index)}
                  className="border-2 border-red-900 bg-red-100 px-3 py-2 text-red-900 transition-all hover:bg-red-900 hover:text-red-100"
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            ))}
          </div>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-black tracking-widest">分类输出目录</label>
            <p className="mt-1 text-xs font-bold text-muted-foreground">保留用于现有分类输出场景。</p>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            {Object.entries(OUTPUT_DIR_LABELS).map(([key, label]) => (
              <div key={key} className="border-2 border-foreground bg-card p-5 shadow-hard transition-all hover:-translate-y-1 hover:shadow-hard-hover">
                <div className="mb-3 flex items-start justify-between gap-3">
                  <div>
                    <p className="text-sm font-black">{label}</p>
                    <p className="mt-1 text-[10px] font-bold text-muted-foreground">对应 `{key}` 分类的目标路径</p>
                  </div>
                  <button
                    type="button"
                    onClick={() => {
                      setPickerTarget({ kind: 'output', key: key as keyof NonNullable<AppConfig['output_dirs']> })
                      setPickerOpen(true)
                    }}
                    className="border-2 border-foreground bg-background px-3 py-1.5 text-xs font-bold transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5"
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
                  className="w-full border-2 border-foreground bg-background px-3 py-2 text-xs font-mono font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-1"
                  placeholder={`/data/target/${key}`}
                />
              </div>
            ))}
          </div>
        </div>

        <div className="space-y-4 border-2 border-foreground bg-card p-6 shadow-hard">
          <div className="flex items-center justify-between gap-4">
            <div>
              <label className="block text-sm font-black tracking-widest">路径选项库</label>
              <p className="mt-1 text-xs font-bold text-muted-foreground">
                可在扫描配置、节点配置中复用；使用处仍支持切换为自定义路径。
              </p>
            </div>
            <button
              type="button"
              onClick={addPathOption}
              className="flex items-center gap-2 border-2 border-foreground bg-background px-4 py-2 text-sm font-bold transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5"
            >
              <Plus className="h-4 w-4" />
              添加路径选项
            </button>
          </div>
          <div className="space-y-3">
            {form.pathOptions.length === 0 && (
              <p className="border-2 border-dashed border-foreground px-4 py-5 text-sm font-bold text-muted-foreground">暂无路径选项。</p>
            )}
            {form.pathOptions.map((item, index) => (
              <div key={item.id || index} className="space-y-3 border-2 border-foreground bg-muted/20 p-4">
                <div className="grid gap-3 md:grid-cols-[1fr,1.2fr,0.8fr,auto]">
                  <input
                    value={item.name}
                    onChange={(event) => updatePathOption(index, { name: event.target.value })}
                    className="w-full border-2 border-foreground bg-background px-3 py-2 text-sm font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-1"
                    placeholder="名称，例如：默认扫描目录"
                  />
                  <div className="flex gap-2">
                    <input
                      value={item.path}
                      onChange={(event) => updatePathOption(index, { path: event.target.value })}
                      className="w-full border-2 border-foreground bg-background px-3 py-2 text-xs font-mono font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-1"
                      placeholder="/data/path"
                    />
                    <button
                      type="button"
                      onClick={() => {
                        setPickerTarget({ kind: 'path_option', index })
                        setPickerOpen(true)
                      }}
                      className="shrink-0 border-2 border-foreground bg-background px-3 py-2 text-foreground transition-all hover:bg-foreground hover:text-background"
                    >
                      <FolderSearch className="h-4 w-4" />
                    </button>
                  </div>
                  <select
                    value={item.category}
                    onChange={(event) => updatePathOption(index, { category: event.target.value as ConfiguredPathCategory })}
                    className="w-full border-2 border-foreground bg-background px-3 py-2 text-sm font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-1"
                  >
                    {Object.entries(PATH_OPTION_CATEGORY_LABELS).map(([category, label]) => (
                      <option key={category} value={category}>{label}</option>
                    ))}
                  </select>
                  <button
                    type="button"
                    onClick={() => removePathOption(index)}
                    className="shrink-0 border-2 border-red-900 bg-red-100 px-3 py-2 text-red-900 transition-all hover:bg-red-900 hover:text-red-100"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
                <p className="text-[10px] font-bold text-muted-foreground">ID：{item.id}</p>
              </div>
            ))}
          </div>
        </div>

        {error && <p className="border-2 border-red-900 bg-red-100 px-4 py-3 text-sm font-bold text-red-900 shadow-hard">{error}</p>}
        {success && <p className="border-2 border-green-900 bg-green-100 px-4 py-3 text-sm font-bold text-green-900 shadow-hard">{success}</p>}

        <div className="flex justify-end pt-4">
          <button
            type="submit"
            disabled={isLoading || isSaving}
            className="border-2 border-foreground bg-primary px-8 py-3 text-sm font-black tracking-widest text-primary-foreground transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5 disabled:opacity-50 disabled:hover:bg-primary disabled:hover:text-primary-foreground disabled:hover:shadow-none disabled:hover:translate-y-0"
          >
            {isSaving ? '保存中…' : '保存配置'}
          </button>
        </div>
      </form>

      <DirPicker
        open={pickerOpen}
        initialPath={sourceDir}
        title={pickerTarget.kind === 'scan' ? '选择扫描输入目录' : pickerTarget.kind === 'output' ? '选择输出目录' : '选择路径'}
        onConfirm={addDir}
        onCancel={() => setPickerOpen(false)}
      />
    </section>
  )
}
