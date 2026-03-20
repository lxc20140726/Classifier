import { useEffect, useMemo, useState, type FormEvent } from 'react'

import { getConfig, updateConfig } from '@/api/config'

interface FormState {
  scanDirsText: string
}

const INITIAL_FORM: FormState = {
  scanDirsText: '',
}

function normalizeLines(value: string): string[] {
  return value
    .split('\n')
    .map((item) => item.trim())
    .filter(Boolean)
}

export default function SettingsPage() {
  const [form, setForm] = useState<FormState>(INITIAL_FORM)
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  useEffect(() => {
    let active = true

    async function loadConfig() {
      try {
        const response = await getConfig()
        if (!active) return

        const scanDirs = response.data.scan_input_dirs ?? []
        setForm({
          scanDirsText: scanDirs.join('\n'),
        })
        setError(null)
      } catch (loadError) {
        if (!active) return
        setError(loadError instanceof Error ? loadError.message : '加载配置失败')
      } finally {
        if (active) {
          setIsLoading(false)
        }
      }
    }

    void loadConfig()

    return () => {
      active = false
    }
  }, [])

  const scanDirs = useMemo(() => normalizeLines(form.scanDirsText), [form.scanDirsText])

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setIsSaving(true)
    setError(null)
    setSuccess(null)

    try {
      await updateConfig({
        scan_input_dirs: JSON.stringify(scanDirs),
        source_dir: scanDirs[0] ?? '',
      })
      setSuccess('扫描目录已保存。')
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : '保存配置失败')
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <section className="mx-auto max-w-4xl space-y-6">
      <header className="space-y-2">
        <p className="text-sm uppercase tracking-[0.24em] text-muted-foreground">设置</p>
        <h2 className="text-3xl font-semibold tracking-tight">扫描目录配置</h2>
        <p className="text-sm text-muted-foreground">
          在这里维护需要扫描的一个或多个输入目录。扫描时会按目录逐个发现文件夹，并立即记录分类结果。
        </p>
      </header>

      <form onSubmit={handleSubmit} className="space-y-6 rounded-2xl border border-border bg-card p-6 shadow-sm">
        <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_280px]">
          <div className="space-y-3">
            <label htmlFor="scan_dirs" className="text-sm font-medium text-foreground">
              扫描输入目录
            </label>
            <textarea
              id="scan_dirs"
              value={form.scanDirsText}
              onChange={(event) =>
                setForm((current) => ({ ...current, scanDirsText: event.target.value }))
              }
              className="min-h-56 w-full rounded-xl border border-border bg-background px-4 py-3 text-sm outline-none transition focus:border-primary"
              placeholder={'每行填写一个目录，例如：\n/data/source\n/data/source-2'}
              disabled={isLoading || isSaving}
            />
            <p className="text-xs text-muted-foreground">
              每行一个绝对路径。保存后会写入后端配置，并作为扫描入口目录。
            </p>
          </div>

          <div className="space-y-4 rounded-2xl border border-border bg-muted/30 p-5">
            <div>
              <h3 className="text-sm font-semibold">当前预览</h3>
              <p className="mt-1 text-xs text-muted-foreground">共 {scanDirs.length} 个扫描目录</p>
            </div>

            <ul className="space-y-2 text-sm">
              {scanDirs.length === 0 ? (
                <li className="rounded-xl border border-dashed border-border px-3 py-4 text-xs text-muted-foreground">
                  还没有配置扫描目录。
                </li>
              ) : (
                scanDirs.map((dir) => (
                  <li key={dir} className="rounded-xl border border-border bg-background px-3 py-3 break-all">
                    {dir}
                  </li>
                ))
              )}
            </ul>

            <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-900">
              输出目录不再在这里统一配置。后续处理工作流的输出目录应由节点单独配置，同一来源目录下的不同文件夹可以走不同的输出位置。
            </div>
          </div>
        </div>

        {isLoading && <p className="text-sm text-muted-foreground">正在读取当前配置...</p>}
        {error && <p className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>}
        {success && <p className="rounded-lg bg-green-50 px-3 py-2 text-sm text-green-700">{success}</p>}

        <div className="flex justify-end">
          <button
            type="submit"
            disabled={isLoading || isSaving}
            className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {isSaving ? '保存中...' : '保存扫描目录'}
          </button>
        </div>
      </form>
    </section>
  )
}
