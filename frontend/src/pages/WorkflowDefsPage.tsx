import { useEffect, useState } from 'react'
import { Check, Pencil, Plus, Trash2 } from 'lucide-react'
import { Link } from 'react-router-dom'

import { cn } from '@/lib/utils'
import { useWorkflowDefStore } from '@/store/workflowDefStore'
import type { WorkflowDefinition } from '@/types'

export type WorkflowDefsPageProps = Record<string, never>

interface FormState {
  name: string
  graphJson: string
}

const EMPTY_FORM: FormState = { name: '', graphJson: '{}' }

type ModalMode = { kind: 'create' } | { kind: 'edit'; def: WorkflowDefinition }

export default function WorkflowDefsPage(_props: WorkflowDefsPageProps) {
  const { defs, isLoading, error, fetchDefs, createDef, updateDef, deleteDef, setActive } =
    useWorkflowDefStore()

  const [modal, setModal] = useState<ModalMode | null>(null)
  const [form, setForm] = useState<FormState>(EMPTY_FORM)
  const [formError, setFormError] = useState<string | null>(null)
  const [isSaving, setIsSaving] = useState(false)

  useEffect(() => {
    void fetchDefs()
  }, [fetchDefs])

  function openCreate() {
    setForm(EMPTY_FORM)
    setFormError(null)
    setModal({ kind: 'create' })
  }

  function openEdit(def: WorkflowDefinition) {
    setForm({ name: def.name, graphJson: def.graph_json })
    setFormError(null)
    setModal({ kind: 'edit', def })
  }

  function closeModal() {
    setModal(null)
    setFormError(null)
  }

  async function handleSave() {
    if (!form.name.trim()) {
      setFormError('名称不能为空')
      return
    }
    try {
      JSON.parse(form.graphJson)
    } catch {
      setFormError('Graph JSON 格式不正确')
      return
    }

    setIsSaving(true)
    setFormError(null)
    try {
      if (modal?.kind === 'create') {
        await createDef(form.name.trim(), form.graphJson)
      } else if (modal?.kind === 'edit') {
        await updateDef(modal.def.id, { name: form.name.trim(), graph_json: form.graphJson })
      }
      closeModal()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : '保存失败')
    } finally {
      setIsSaving(false)
    }
  }

  async function handleDelete(def: WorkflowDefinition) {
    if (!window.confirm('确认删除此工作流定义？')) return
    await deleteDef(def.id)
  }

  async function handleSetActive(def: WorkflowDefinition) {
    await setActive(def.id)
  }

  return (
    <section className="mx-auto max-w-5xl px-4 py-8">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-xl font-semibold">工作流定义</h1>
        <button
          type="button"
          onClick={openCreate}
          className="flex items-center gap-1.5 rounded-lg bg-primary px-3 py-2 text-sm font-medium text-primary-foreground transition hover:opacity-90"
        >
          <Plus className="h-4 w-4" />
          新建
        </button>
      </div>

      {isLoading && <p className="text-sm text-muted-foreground">加载中…</p>}

      {error && <p className="text-sm text-red-600">{error}</p>}

      {!isLoading && !error && defs.length === 0 && (
        <div className="rounded-xl border border-dashed border-border py-16 text-center">
          <p className="text-sm text-muted-foreground">暂无工作流定义，点击「新建」创建第一个。</p>
        </div>
      )}

      {defs.length > 0 && (
        <div className="overflow-hidden rounded-xl border border-border">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-muted/40 text-left">
                <th className="px-4 py-3 font-medium">名称</th>
                <th className="px-4 py-3 font-medium">版本</th>
                <th className="px-4 py-3 font-medium">状态</th>
                <th className="px-4 py-3 font-medium">创建时间</th>
                <th className="px-4 py-3 font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              {defs.map((def, idx) => (
                <tr
                  key={def.id}
                  className={cn(
                    'border-b border-border last:border-0',
                    idx % 2 === 0 ? 'bg-background' : 'bg-muted/20',
                  )}
                >
                  <td className="px-4 py-3 font-medium">{def.name}</td>
                  <td className="px-4 py-3 text-muted-foreground">v{def.version}</td>
                  <td className="px-4 py-3">
                    {def.is_active ? (
                      <span className="inline-flex items-center gap-1 rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-medium text-green-700">
                        <Check className="h-3 w-3" />
                        已激活
                      </span>
                    ) : (
                      <button
                        type="button"
                        onClick={() => void handleSetActive(def)}
                        className="rounded-lg border border-border px-2.5 py-1 text-xs transition hover:bg-accent"
                      >
                        设为激活
                      </button>
                    )}
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {new Date(def.created_at).toLocaleString('zh-CN')}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => openEdit(def)}
                        className="flex items-center gap-1 rounded-lg border border-border px-2.5 py-1 text-xs transition hover:bg-accent"
                      >
                        <Pencil className="h-3 w-3" />
                        编辑
                      </button>
                      <Link
                        to={`/workflow-defs/${def.id}/editor`}
                        className="flex items-center gap-1 rounded-lg border border-sky-200 px-2.5 py-1 text-xs text-sky-700 transition hover:bg-sky-50"
                      >
                        可视化编辑
                      </Link>
                      <button
                        type="button"
                        onClick={() => void handleDelete(def)}
                        className="flex items-center gap-1 rounded-lg border border-border px-2.5 py-1 text-xs text-red-600 transition hover:bg-red-50"
                      >
                        <Trash2 className="h-3 w-3" />
                        删除
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {modal !== null && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
          <div className="w-full max-w-lg rounded-2xl border border-border bg-background p-6 shadow-xl">
            <h2 className="mb-5 text-base font-semibold">
              {modal.kind === 'create' ? '新建工作流定义' : '编辑工作流定义'}
            </h2>

            <div className="space-y-4">
              <div>
                <label className="mb-1.5 block text-sm font-medium">名称</label>
                <input
                  type="text"
                  value={form.name}
                  onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))}
                  placeholder="工作流名称"
                  className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm outline-none ring-primary focus:ring-2"
                />
              </div>

              <div>
                <label className="mb-1.5 block text-sm font-medium">Graph JSON</label>
                <textarea
                  value={form.graphJson}
                  onChange={(e) => setForm((prev) => ({ ...prev, graphJson: e.target.value }))}
                  rows={10}
                  spellCheck={false}
                  className="w-full rounded-lg border border-border bg-background px-3 py-2 font-mono text-xs outline-none ring-primary focus:ring-2"
                />
              </div>

              {formError && <p className="text-sm text-red-600">{formError}</p>}
            </div>

            <div className="mt-6 flex justify-end gap-2">
              <button
                type="button"
                onClick={closeModal}
                disabled={isSaving}
                className="rounded-lg border border-border px-4 py-2 text-sm transition hover:bg-accent disabled:cursor-not-allowed disabled:opacity-60"
              >
                取消
              </button>
              <button
                type="button"
                onClick={() => void handleSave()}
                disabled={isSaving}
                className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isSaving ? '保存中…' : '保存'}
              </button>
            </div>
          </div>
        </div>
      )}
    </section>
  )
}
