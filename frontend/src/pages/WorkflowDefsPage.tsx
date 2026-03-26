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
    <section className="mx-auto max-w-5xl px-6 py-8">
      <div className="mb-8 flex items-end justify-between border-b-2 border-foreground pb-4">
        <h1 className="text-3xl font-black tracking-tight uppercase">工作流定义</h1>
        <button
          type="button"
          onClick={openCreate}
          className="flex items-center gap-2 border-2 border-foreground bg-primary px-5 py-2.5 text-sm font-bold text-primary-foreground transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5"
        >
          <Plus className="h-4 w-4" />
          新建
        </button>
      </div>

      {isLoading && <p className="text-sm font-bold text-muted-foreground">加载中…</p>}

      {error && <p className="border-2 border-red-900 bg-red-100 px-4 py-3 text-sm font-bold text-red-900 shadow-hard">{error}</p>}

      {!isLoading && !error && defs.length === 0 && (
        <div className="border-2 border-dashed border-foreground py-20 text-center">
          <p className="text-sm font-bold text-muted-foreground">暂无工作流定义，点击「新建」创建第一个。</p>
        </div>
      )}

      {defs.length > 0 && (
        <div className="overflow-hidden border-2 border-foreground bg-card shadow-hard">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b-2 border-foreground bg-muted/50 text-left">
                <th className="px-5 py-4 font-black tracking-widest">名称</th>
                <th className="px-5 py-4 font-black tracking-widest">版本</th>
                <th className="px-5 py-4 font-black tracking-widest">状态</th>
                <th className="px-5 py-4 font-black tracking-widest">创建时间</th>
                <th className="px-5 py-4 font-black tracking-widest">操作</th>
              </tr>
            </thead>
            <tbody>
              {defs.map((def, idx) => (
                <tr
                  key={def.id}
                  className={cn(
                    'border-b-2 border-foreground last:border-0 transition-colors hover:bg-muted/30',
                    idx % 2 === 0 ? 'bg-background' : 'bg-muted/10',
                  )}
                >
                  <td className="px-5 py-4 font-black">{def.name}</td>
                  <td className="px-5 py-4 font-mono font-bold text-muted-foreground">v{def.version}</td>
                  <td className="px-5 py-4">
                    {def.is_active ? (
                      <span className="inline-flex items-center gap-1.5 border-2 border-foreground bg-green-300 px-3 py-1 text-xs font-black text-green-900">
                        <Check className="h-3 w-3" />
                        已激活
                      </span>
                    ) : (
                      <button
                        type="button"
                        onClick={() => void handleSetActive(def)}
                        className="border-2 border-foreground bg-background px-3 py-1 text-xs font-bold transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5"
                      >
                        设为激活
                      </button>
                    )}
                  </td>
                  <td className="px-5 py-4 font-mono text-xs font-bold text-muted-foreground">
                    {new Date(def.created_at).toLocaleString('zh-CN')}
                  </td>
                  <td className="px-5 py-4">
                    <div className="flex items-center gap-3">
                      <button
                        type="button"
                        onClick={() => openEdit(def)}
                        className="flex items-center gap-1.5 border-2 border-foreground bg-background px-3 py-1.5 text-xs font-bold transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5"
                      >
                        <Pencil className="h-3 w-3" />
                        编辑
                      </button>
                      <Link
                        to={`/workflow-defs/${def.id}/editor`}
                        className="flex items-center gap-1.5 border-2 border-foreground bg-primary px-3 py-1.5 text-xs font-bold text-primary-foreground transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5"
                      >
                        可视化编辑
                      </Link>
                      <button
                        type="button"
                        onClick={() => void handleDelete(def)}
                        className="flex items-center gap-1.5 border-2 border-red-900 bg-red-100 px-3 py-1.5 text-xs font-bold text-red-900 transition-all hover:bg-red-900 hover:text-red-100 hover:shadow-hard hover:-translate-y-0.5"
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
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="w-full max-w-lg border-2 border-foreground bg-background p-6 shadow-hard-lg">
            <h2 className="mb-6 text-xl font-black tracking-tight">
              {modal.kind === 'create' ? '新建工作流定义' : '编辑工作流定义'}
            </h2>

            <div className="space-y-5">
              <div>
                <label className="mb-2 block text-sm font-black tracking-widest">名称</label>
                <input
                  type="text"
                  value={form.name}
                  onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))}
                  placeholder="工作流名称"
                  className="w-full border-2 border-foreground bg-background px-4 py-3 text-sm font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-2 focus:ring-offset-background"
                />
              </div>

              <div>
                <label className="mb-2 block text-sm font-black tracking-widest">GRAPH JSON</label>
                <textarea
                  value={form.graphJson}
                  onChange={(e) => setForm((prev) => ({ ...prev, graphJson: e.target.value }))}
                  rows={10}
                  spellCheck={false}
                  className="w-full border-2 border-foreground bg-muted/30 px-4 py-3 font-mono text-xs font-bold outline-none focus:ring-2 focus:ring-foreground focus:ring-offset-2 focus:ring-offset-background"
                />
              </div>

              {formError && <p className="border-2 border-red-900 bg-red-100 px-4 py-3 text-sm font-bold text-red-900 shadow-hard">{formError}</p>}
            </div>

            <div className="mt-8 flex justify-end gap-3">
              <button
                type="button"
                onClick={closeModal}
                disabled={isSaving}
                className="border-2 border-foreground bg-background px-6 py-2.5 text-sm font-bold transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5 disabled:opacity-50 disabled:hover:bg-background disabled:hover:text-foreground disabled:hover:shadow-none disabled:hover:translate-y-0"
              >
                取消
              </button>
              <button
                type="button"
                onClick={() => void handleSave()}
                disabled={isSaving}
                className="border-2 border-foreground bg-primary px-6 py-2.5 text-sm font-bold text-primary-foreground transition-all hover:bg-foreground hover:text-background hover:shadow-hard hover:-translate-y-0.5 disabled:opacity-50 disabled:hover:bg-primary disabled:hover:text-primary-foreground disabled:hover:shadow-none disabled:hover:translate-y-0"
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
