import { useEffect, useState, type FormEvent } from 'react'

import { getConfig, updateConfig } from '@/api/config'

interface FormState {
  source_dir: string
  target_dir: string
}

const INITIAL_FORM: FormState = {
  source_dir: '',
  target_dir: '',
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

        setForm({
          source_dir: response.data.source_dir ?? '',
          target_dir: response.data.target_dir ?? '',
        })
        setError(null)
      } catch (loadError) {
        if (!active) return
        setError(loadError instanceof Error ? loadError.message : 'Failed to load config')
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

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setIsSaving(true)
    setError(null)
    setSuccess(null)

    try {
      await updateConfig(form)
      setSuccess('Settings saved successfully.')
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : 'Failed to save config')
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <section className="mx-auto max-w-3xl space-y-6">
      <header className="space-y-2">
        <p className="text-sm uppercase tracking-[0.24em] text-muted-foreground">Settings</p>
        <h2 className="text-3xl font-semibold tracking-tight">Configure source and target paths</h2>
        <p className="text-sm text-muted-foreground">
          These values are stored through the backend config API and used by scan and move flows.
        </p>
      </header>

      <form onSubmit={handleSubmit} className="space-y-5 rounded-2xl border border-border bg-white p-6 shadow-sm">
        <div className="space-y-2">
          <label htmlFor="source_dir" className="text-sm font-medium text-foreground">
            Source directory
          </label>
          <input
            id="source_dir"
            value={form.source_dir}
            onChange={(event) =>
              setForm((current) => ({ ...current, source_dir: event.target.value }))
            }
            className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm outline-none transition focus:border-primary"
            placeholder="/data/source"
            disabled={isLoading || isSaving}
          />
        </div>

        <div className="space-y-2">
          <label htmlFor="target_dir" className="text-sm font-medium text-foreground">
            Target directory
          </label>
          <input
            id="target_dir"
            value={form.target_dir}
            onChange={(event) =>
              setForm((current) => ({ ...current, target_dir: event.target.value }))
            }
            className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm outline-none transition focus:border-primary"
            placeholder="/data/target"
            disabled={isLoading || isSaving}
          />
        </div>

        {isLoading && <p className="text-sm text-muted-foreground">Loading current settings...</p>}
        {error && <p className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>}
        {success && <p className="rounded-lg bg-green-50 px-3 py-2 text-sm text-green-700">{success}</p>}

        <div className="flex justify-end">
          <button
            type="submit"
            disabled={isLoading || isSaving}
            className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {isSaving ? 'Saving...' : 'Save settings'}
          </button>
        </div>
      </form>
    </section>
  )
}
