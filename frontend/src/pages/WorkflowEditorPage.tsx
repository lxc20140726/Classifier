import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode, type ReactElement } from 'react'
import {
  addEdge,
  Background,
  Controls,
  MiniMap,
  PanOnScrollMode,
  ReactFlow,
  ReactFlowProvider,
  Handle,
  Position,
  SelectionMode,
  useEdgesState,
  useNodesState,
  type Connection,
  type Edge,
  type Node,
  type NodeProps,
  type OnConnect,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { ArrowLeft, CheckCircle2, ChevronLeft, ChevronRight, FolderOpen, Loader2, MousePointer, Play, Plus, RotateCcw, Save, Trash2, TriangleAlert, Wand2 } from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'

import { ApiRequestError } from '@/api/client'
import { DirPicker } from '@/components/DirPicker'
import { useFolderStore } from '@/store/folderStore'
import { useWorkflowRunStore } from '@/store/workflowRunStore'
import { startWorkflowJob } from '@/api/workflowRuns'
import type { NodeRun, NodeRunStatus } from '@/types'
import { listNodeTypes } from '@/api/nodeTypes'
import { getWorkflowDef, updateWorkflowDef } from '@/api/workflowDefs'
import { cn } from '@/lib/utils'
import type {
  NodeInputSpec,
  NodeSchema,
  WorkflowDefinition,
  WorkflowGraph,
  WorkflowGraphEdge,
  WorkflowGraphNode,
} from '@/types'

// ─── Types ────────────────────────────────────────────────────────────────────

type EditorNodeData = {
  label: string
  type: string
  enabled: boolean
  schema: NodeSchema | undefined
}

type EditorNode = Node<EditorNodeData>

interface EditorContextValue {
  workflowNodes: Record<string, WorkflowGraphNode>
  schemas: NodeSchema[]
  updateNode: (nodeId: string, patch: Partial<WorkflowGraphNode> & { schemaType?: string }) => void
  updateNodeConfig: (nodeId: string, key: string, rawValue: string) => void
  removeNodeConfig: (nodeId: string, key: string) => void
  addNodeConfig: (nodeId: string, key: string, rawValue: string) => void
  deleteNode: (nodeId: string) => void
  nodeRunByNodeId: Record<string, NodeRun>
  onRollbackRun: (runId: string) => Promise<void>
}

const EditorContext = createContext<EditorContextValue | null>(null)

function useEditorContext(): EditorContextValue {
  const ctx = useContext(EditorContext)
  if (!ctx) throw new Error('useEditorContext must be used within EditorContext.Provider')
  return ctx
}

// ─── Constants ────────────────────────────────────────────────────────────────

const INITIAL_GRAPH: WorkflowGraph = { nodes: [], edges: [] }

/**
 * 节点分类定义：按业务语义划分颜色主题，保证同类节点颜色一致。
 *
 * 颜色语义：
 *   紫色 → 触发器（工作流入口）
 *   蓝色 → 扫描 & 读取（数据来源）
 *   青色 → 分类器（判断决策）
 *   琥珀 → 逻辑控制（流程分支）
 *   绿色 → 执行操作（文件变更）
 *   石板 → 审计日志（记录追踪）
 */
interface NodeCategory {
  label: string
  iconColor: string
  accentClass: string
  borderHoverClass: string
  types: ReadonlySet<string>
}

const NODE_CATEGORIES: NodeCategory[] = [
  {
    label: '触发器',
    iconColor: 'text-violet-600',
    accentClass: 'from-violet-500/20 to-purple-500/10 border-violet-200',
    borderHoverClass: 'hover:border-violet-300',
    types: new Set(['trigger']),
  },
  {
    label: '扫描 & 读取',
    iconColor: 'text-blue-600',
    accentClass: 'from-blue-500/20 to-indigo-500/10 border-blue-200',
    borderHoverClass: 'hover:border-blue-300',
    types: new Set(['folder-tree-scanner', 'classification-reader']),
  },
  {
    label: '分类器',
    iconColor: 'text-cyan-600',
    accentClass: 'from-cyan-500/20 to-teal-500/10 border-cyan-200',
    borderHoverClass: 'hover:border-cyan-300',
    types: new Set(['ext-ratio-classifier', 'name-keyword-classifier', 'file-tree-classifier', 'manual-classifier']),
  },
  {
    label: '逻辑控制',
    iconColor: 'text-amber-600',
    accentClass: 'from-amber-500/20 to-orange-500/10 border-amber-200',
    borderHoverClass: 'hover:border-amber-300',
    types: new Set(['confidence-check', 'folder-splitter', 'category-router', 'subtree-aggregator']),
  },
  {
    label: '执行操作',
    iconColor: 'text-emerald-600',
    accentClass: 'from-emerald-500/20 to-green-500/10 border-emerald-200',
    borderHoverClass: 'hover:border-emerald-300',
    types: new Set(['move', 'move-node', 'rename-node', 'compress-node', 'thumbnail-node']),
  },
  {
    label: '审计日志',
    iconColor: 'text-slate-500',
    accentClass: 'from-slate-400/20 to-zinc-400/10 border-slate-200',
    borderHoverClass: 'hover:border-slate-300',
    types: new Set(['audit-log']),
  },
]

const FALLBACK_CATEGORY: NodeCategory = {
  label: '其他',
  iconColor: 'text-gray-500',
  accentClass: 'from-gray-400/20 to-gray-300/10 border-gray-200',
  borderHoverClass: 'hover:border-gray-300',
  types: new Set(),
}

function getNodeCategory(nodeType: string): NodeCategory {
  return NODE_CATEGORIES.find((category) => category.types.has(nodeType)) ?? FALLBACK_CATEGORY
}

// ─── Utility functions ────────────────────────────────────────────────────────

function safeParseGraph(graphJson: string): WorkflowGraph {
  try {
    const parsed = JSON.parse(graphJson) as Partial<WorkflowGraph>
    return {
      nodes: Array.isArray(parsed.nodes) ? parsed.nodes : [],
      edges: Array.isArray(parsed.edges) ? parsed.edges : [],
    }
  } catch {
    return INITIAL_GRAPH
  }
}

function buildNodeId(nodeType: string, existing: Node<EditorNodeData>[]) {
  const normalized = nodeType.replace(/[^a-z0-9]+/gi, '-').replace(/(^-|-$)/g, '').toLowerCase() || 'node'
  let index = existing.length + 1
  let candidate = `${normalized}-${index}`
  const ids = new Set(existing.map((node) => node.id))
  for (; ids.has(candidate); index += 1) {
    candidate = `${normalized}-${index}`
  }
  return candidate
}

function graphToNodes(graph: WorkflowGraph, schemas: Map<string, NodeSchema>): EditorNode[] {
  return graph.nodes.map((node, index) => ({
    id: node.id,
    type: 'workflowNode',
    position: node.ui_position ?? { x: 120 + ((index % 4) * 240), y: 120 + (Math.floor(index / 4) * 160) },
    data: {
      label: node.label || schemas.get(node.type)?.label || node.type,
      type: node.type,
      enabled: node.enabled,
      schema: schemas.get(node.type),
    },
  }))
}

function graphToEdges(graph: WorkflowGraph): Edge[] {
  return graph.edges.map((edge, index) => ({
    id: edge.id || `edge-${index}`,
    source: edge.source,
    target: edge.target,
    sourceHandle: `out-${edge.source_port}`,
    targetHandle: `in-${edge.target_port}`,
    animated: false,
  }))
}

function buildInputMap(nodeId: string, edges: Edge[], schema?: NodeSchema, previous?: Record<string, NodeInputSpec>) {
  const nextInputs: Record<string, NodeInputSpec> = {}

  if (previous) {
    for (const [key, value] of Object.entries(previous)) {
      if (value.const_value !== undefined) {
        nextInputs[key] = { const_value: value.const_value }
      }
    }
  }

  for (const edge of edges) {
    if (edge.target !== nodeId) continue
    const targetPortIndex = parseHandleIndex(edge.targetHandle)
    const sourcePortIndex = parseHandleIndex(edge.sourceHandle)
    if (targetPortIndex == null || sourcePortIndex == null) continue
    const portName = schema?.input_ports?.[targetPortIndex]?.name ?? `input_${targetPortIndex}`
    nextInputs[portName] = {
      link_source: {
        source_node_id: edge.source,
        output_port_index: sourcePortIndex,
      },
    }
  }

  return nextInputs
}

function parseHandleIndex(handle?: string | null) {
  if (!handle) return null
  const raw = handle.split('-')[1]
  const parsed = Number.parseInt(raw, 10)
  return Number.isNaN(parsed) ? null : parsed
}

function nodesToGraph(
  rfNodes: EditorNode[],
  rfEdges: Edge[],
  workflowNodes: Record<string, WorkflowGraphNode>,
  schemaMap: Map<string, NodeSchema>,
): WorkflowGraph {
  const nodes: WorkflowGraphNode[] = rfNodes.map((node) => {
    const previous = workflowNodes[node.id]
    const nextType = node.data.type
    const schema = schemaMap.get(nextType)
    return {
      id: node.id,
      type: nextType,
      label: node.data.label,
      config: previous?.config ?? {},
      inputs: buildInputMap(node.id, rfEdges, schema, previous?.inputs),
      ui_position: { x: Math.round(node.position.x), y: Math.round(node.position.y) },
      enabled: node.data.enabled,
    }
  })

  const edges: WorkflowGraphEdge[] = rfEdges.map((edge) => ({
    id: edge.id,
    source: edge.source,
    source_port: parseHandleIndex(edge.sourceHandle) ?? 0,
    target: edge.target,
    target_port: parseHandleIndex(edge.targetHandle) ?? 0,
  }))

  return { nodes, edges }
}

function parseConfigValue(input: string): unknown {
  const trimmed = input.trim()
  if (trimmed === 'true') return true
  if (trimmed === 'false') return false
  if (trimmed !== '' && /^-?\d+(\.\d+)?$/.test(trimmed)) return Number(trimmed)
  if ((trimmed.startsWith('{') && trimmed.endsWith('}')) || (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
    try {
      return JSON.parse(trimmed) as unknown
    } catch {
      return input
    }
  }
  return input
}


// ─── Rename preview logic ─────────────────────────────────────────────────────

type RenameConditionalRule = { condition: string; template: string }
type RenamePreviewResult = { strategy: string; targetName: string; warning: string | null }

const RENAME_PREVIEW_SAMPLE = { name: 'Dune[2021]', category: 'video', parent: '电影合集', index: 1 }

function renamePreviewString(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

function renamePreviewBool(value: unknown, fallback = false) {
  return typeof value === 'boolean' ? value : fallback
}

function renamePreviewExtractYear(name: string) {
  const matched = name.match(/(19|20)\d{2}/)
  return matched?.[0]?.trim() ?? ''
}

function renamePreviewExtractTitle(name: string, year: string) {
  let title = name.trim()
  if (year !== '') {
    title = title.replaceAll(`(${year})`, '').replaceAll(`[${year}]`, '').replaceAll(year, '')
  }
  title = renamePreviewTrimDecorators(title.trim())
  return title === '' ? name.trim() : title
}

function renamePreviewTrimDecorators(input: string) {
  const trimChars = '-_()[] '
  let start = 0
  let end = input.length
  for (; start < end; start += 1) {
    if (!trimChars.includes(input[start])) break
  }
  for (; end > start; end -= 1) {
    if (!trimChars.includes(input[end - 1])) break
  }
  return input.slice(start, end)
}

function renamePreviewRenderTemplate(template: string, variables: Record<string, string>) {
  let result = template
  for (const [key, value] of Object.entries(variables)) {
    result = result.replaceAll(`{${key}}`, value)
  }
  return result.trim()
}

function renamePreviewParseRules(config: Record<string, unknown>) {
  const raw = config.rules
  if (!Array.isArray(raw)) return [] as RenameConditionalRule[]
  const rules: RenameConditionalRule[] = []
  for (const item of raw) {
    if (!item || typeof item !== 'object') continue
    const itemMap = item as Record<string, unknown>
    const condition = renamePreviewString(itemMap.condition) || renamePreviewString(itemMap.if)
    const template = renamePreviewString(itemMap.template)
    if (template === '') continue
    rules.push({ condition, template })
  }
  return rules
}

function renamePreviewEvaluateCondition(condition: string, name: string, category: string) {
  const trimmed = condition.trim()
  if (trimmed === '') return false
  const containsMatch = trimmed.match(/^name\s+CONTAINS\s+"([^"]+)"$/i)
  if (containsMatch) return name.toLowerCase().includes(containsMatch[1].toLowerCase())
  const matchesMatch = trimmed.match(/^name\s+MATCHES\s+"([^"]+)"$/i)
  if (matchesMatch) {
    try { return new RegExp(matchesMatch[1]).test(name) } catch { return false }
  }
  const categoryMatch = trimmed.match(/^category\s*==\s*"([^"]+)"$/i)
  if (categoryMatch) return category.trim().toLowerCase() === categoryMatch[1].trim().toLowerCase()
  return false
}

function buildRenamePreview(config: Record<string, unknown>): RenamePreviewResult {
  const strategy = renamePreviewString(config.strategy).toLowerCase() || 'template'
  const template = renamePreviewString(config.template)
  const regexPattern = renamePreviewString(config.regex) || renamePreviewString(config.pattern)
  const skipIfSame = renamePreviewBool(config.skip_if_same, false)
  const currentName = RENAME_PREVIEW_SAMPLE.name
  const year = renamePreviewExtractYear(currentName)
  const variables: Record<string, string> = {
    name: currentName,
    title: renamePreviewExtractTitle(currentName, year),
    category: RENAME_PREVIEW_SAMPLE.category,
    year,
    index: String(RENAME_PREVIEW_SAMPLE.index),
    parent: RENAME_PREVIEW_SAMPLE.parent,
  }

  let candidate = currentName
  let warning: string | null = null

  if (strategy === 'template') {
    if (template !== '') candidate = renamePreviewRenderTemplate(template, variables)
  } else if (strategy === 'regex_extract') {
    if (regexPattern !== '') {
      try {
        const regex = new RegExp(regexPattern)
        const matches = regex.exec(currentName)
        if (matches && matches.length > 0) {
          const groups = matches.groups ? Object.entries(matches.groups) : []
          for (const [key, value] of groups) {
            if (typeof value === 'string') variables[key] = value
          }
          if (template !== '') {
            candidate = renamePreviewRenderTemplate(template, variables)
          } else if (typeof matches.groups?.title === 'string' && matches.groups.title.trim() !== '') {
            candidate = matches.groups.title.trim()
          } else {
            const firstNamed = groups.find(([, value]) => typeof value === 'string' && value.trim() !== '')
            if (firstNamed && typeof firstNamed[1] === 'string') {
              candidate = firstNamed[1].trim()
            } else if (matches.length > 1 && typeof matches[1] === 'string' && matches[1].trim() !== '') {
              candidate = matches[1].trim()
            }
          }
        }
      } catch {
        warning = '正则表达式无效，预览已回退到原名称。'
      }
    }
  } else if (strategy === 'conditional') {
    const rules = renamePreviewParseRules(config)
    let defaultTemplate = ''
    for (const rule of rules) {
      if (rule.condition.trim().toUpperCase() === 'DEFAULT') {
        defaultTemplate = rule.template
        continue
      }
      if (renamePreviewEvaluateCondition(rule.condition, currentName, RENAME_PREVIEW_SAMPLE.category)) {
        candidate = renamePreviewRenderTemplate(rule.template, variables)
        break
      }
    }
    if (candidate === currentName && defaultTemplate !== '') {
      candidate = renamePreviewRenderTemplate(defaultTemplate, variables)
    }
  }

  if (candidate.trim() === '') candidate = currentName
  if (skipIfSame && candidate === currentName) {
    warning = warning ?? '当前命名结果与原名称一致，实际执行时会保持原名。'
  }

  return { strategy, targetName: candidate, warning }
}

// ─── Node config panels ─────────────────────────────────────────────────────

interface NodeConfigPanelProps {
  nodeId: string
  nodeType: string
  config: Record<string, unknown>
  updateNodeConfig: (nodeId: string, key: string, rawValue: string) => void
}

function cfgStr(config: Record<string, unknown>, key: string): string {
  const v = config[key]
  return typeof v === 'string' ? v : ''
}

function cfgNum(config: Record<string, unknown>, key: string, fallback: number): number {
  const v = config[key]
  return typeof v === 'number' ? v : fallback
}

function cfgJson(config: Record<string, unknown>, key: string): string {
  const v = config[key]
  if (v === undefined || v === null) return ''
  if (typeof v === 'string') return v
  return JSON.stringify(v, null, 2)
}

const FIELD_CLS =
  'w-full rounded-lg border border-border bg-background/80 px-2 py-1.5 text-sm outline-none ring-primary focus:ring-1'
const TEXTAREA_FIELD_CLS =
  'w-full rounded-lg border border-border bg-background/80 px-2 py-1.5 font-mono text-xs outline-none ring-primary focus:ring-1'

interface ConfigFieldProps {
  label: string
  hint?: string
  children: ReactNode
}

function ConfigField({ label, hint, children }: ConfigFieldProps) {
  return (
    <div>
      <label className="mb-0.5 block text-[10px] font-medium uppercase tracking-[0.15em] text-muted-foreground">
        {label}
      </label>
      {hint && <p className="mb-1 text-[10px] leading-relaxed text-muted-foreground">{hint}</p>}
      {children}
    </div>
  )
}

function NodeUsageHint({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-xl border border-dashed border-border bg-muted/20 p-2.5">
      <p className="text-[11px] leading-relaxed text-muted-foreground">{children}</p>
    </div>
  )
}

interface DirPickerFieldProps {
  value: string
  onChange: (val: string) => void
  placeholder?: string
  title?: string
}

function DirPickerField({ value, onChange, placeholder, title }: DirPickerFieldProps) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <div className="flex gap-1.5">
        <input
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder ?? '/data/path'}
          className={FIELD_CLS}
        />
        <button
          type="button"
          onClick={() => setOpen(true)}
          className="shrink-0 rounded-lg border border-border bg-background/80 px-2 py-1.5 text-muted-foreground transition hover:bg-accent hover:text-foreground"
        >
          <FolderOpen className="h-4 w-4" />
        </button>
      </div>
      <DirPicker
        open={open}
        initialPath={value || '/'}
        title={title}
        onConfirm={(path) => { onChange(path); setOpen(false) }}
        onCancel={() => setOpen(false)}
      />
    </>
  )
}

function NodeConfigPanel({ nodeId, nodeType, config, updateNodeConfig }: NodeConfigPanelProps) {
  const set = (key: string, val: string) => updateNodeConfig(nodeId, key, val)
  const strategy = cfgStr(config, 'strategy') || 'simple'

  switch (nodeType) {
    case 'trigger':
      return (
        <NodeUsageHint>
          工作流入口节点，自动触发并将当前文件夹传递给下游。无需配置。
        </NodeUsageHint>
      )

    case 'ext-ratio-classifier':
      return (
        <NodeUsageHint>
          通过文件扩展名比例自动判断类别（photo / video / manga / other）。无需配置，输出分类信号到 Confidence Check 或 Subtree Aggregator。
        </NodeUsageHint>
      )

    case 'file-tree-classifier':
      return (
        <NodeUsageHint>
          通过文件树结构（子目录层级与文件分布）判断文件夹类别。无需配置。
        </NodeUsageHint>
      )

    case 'manual-classifier':
      return (
        <NodeUsageHint>
          人工介入分类节点。工作流到达此节点时暂停（pending），需通过 API Resume 接口提交分类结果后继续执行。无需配置。
        </NodeUsageHint>
      )

    case 'classification-reader':
      return (
        <NodeUsageHint>
          读取当前工作流文件夹的已有分类结果，或从上游端口接收分类数据并透传。无需配置。
        </NodeUsageHint>
      )

    case 'folder-splitter':
      return (
        <NodeUsageHint>
          将文件夹树拆分为独立的叶子节点，逐一向下游传递处理项。无需配置。
        </NodeUsageHint>
      )

    case 'category-router':
      return (
        <NodeUsageHint>
          按分类类别将处理项路由至对应输出端口（video / manga / photo / other / mixed_leaf）。将各端口连线到对应的处理节点即可，无需配置。
        </NodeUsageHint>
      )

    case 'subtree-aggregator':
      return (
        <NodeUsageHint>
          汇总多个分类器的置信度信号，取最高置信度结果，将最终分类持久化到数据库。无需配置。
        </NodeUsageHint>
      )

    case 'audit-log':
      return (
        <NodeUsageHint>
          将处理结果写入审计日志，同时透传输入数据到下游。接在需要审计的节点之后连线，无需配置。
        </NodeUsageHint>
      )

    case 'folder-tree-scanner':
      return (
        <div className="space-y-3">
          <ConfigField label="扫描根目录" hint="留空则使用环境变量 SOURCE_DIR">
            <DirPickerField
              value={cfgStr(config, 'source_dir')}
              onChange={(v) => set('source_dir', v)}
              placeholder="/data/source"
              title="选择扫描根目录"
            />
          </ConfigField>
          <ConfigField label="最大扫描深度" hint="向下递归的最大层级数（默认 5）">
            <input
              type="number"
              min={1}
              max={20}
              value={cfgNum(config, 'max_depth', 5)}
              onChange={(e) => set('max_depth', e.target.value)}
              className={FIELD_CLS}
            />
          </ConfigField>
          <ConfigField label="最少文件数" hint="文件数低于此值的文件夹将被跳过（默认 0，不过滤）">
            <input
              type="number"
              min={0}
              value={cfgNum(config, 'min_file_count', 0)}
              onChange={(e) => set('min_file_count', e.target.value)}
              className={FIELD_CLS}
            />
          </ConfigField>
        </div>
      )

    case 'confidence-check':
      return (
        <div className="space-y-3">
          <ConfigField
            label="置信度阈值"
            hint="信号置信度 ≥ 阈值走 high 端口，否则走 low 端口（默认 0.75，范围 0–1）"
          >
            <input
              type="number"
              min={0}
              max={1}
              step={0.05}
              value={cfgNum(config, 'threshold', 0.75)}
              onChange={(e) => set('threshold', e.target.value)}
              className={FIELD_CLS}
            />
          </ConfigField>
        </div>
      )

    case 'name-keyword-classifier':
      return (
        <div className="space-y-3">
          <ConfigField
            label="关键词规则（JSON）"
            hint={'格式：[{"keywords":["关键词"],"category":"video"}]，按顺序匹配，第一条命中规则生效'}
          >
            <textarea
              rows={5}
              value={cfgJson(config, 'rules')}
              onChange={(e) => set('rules', e.target.value)}
              placeholder={'[\n  {"keywords": ["电影"], "category": "video"}\n]'}
              className={TEXTAREA_FIELD_CLS}
            />
          </ConfigField>
        </div>
      )

    case 'rename-node':
      return (
        <div className="space-y-3">
          <ConfigField label="重命名策略">
            <select value={strategy} onChange={(e) => set('strategy', e.target.value)} className={FIELD_CLS}>
              <option value="simple">simple — 保持原名</option>
              <option value="template">template — 模板替换</option>
              <option value="regex">regex — 正则提取</option>
              <option value="conditional">conditional — 条件规则</option>
            </select>
          </ConfigField>
          {strategy === 'template' && (
            <ConfigField label="模板" hint="可用变量：{name} {title} {year} {category} {parent} {index}">
              <input
                type="text"
                value={cfgStr(config, 'template')}
                onChange={(e) => set('template', e.target.value)}
                placeholder="{title} ({year})"
                className={FIELD_CLS}
              />
            </ConfigField>
          )}
          {strategy === 'regex' && (
            <ConfigField
              label="正则表达式"
              hint="使用命名捕获组 (?P<title>...) 提取标题，命中后输出捕获内容"
            >
              <input
                type="text"
                value={cfgStr(config, 'regex') || cfgStr(config, 'pattern')}
                onChange={(e) => set('regex', e.target.value)}
                placeholder="(?P<title>.+?)[\\s_]*[\\(\\[](?P<year>\\d{4})[\\)\\]]"
                className={FIELD_CLS}
              />
            </ConfigField>
          )}
          {strategy === 'conditional' && (
            <ConfigField
              label="条件规则（JSON）"
              hint={'格式：[{"condition":"category==video","template":"{title}"}]，condition 为 DEFAULT 时作为默认规则'}
            >
              <textarea
                rows={5}
                value={cfgJson(config, 'rules')}
                onChange={(e) => set('rules', e.target.value)}
                placeholder={'[\n  {"condition": "category==video", "template": "{title} ({year})"}\n]'}
                className={TEXTAREA_FIELD_CLS}
              />
            </ConfigField>
          )}
        </div>
      )

    case 'move':
    case 'move-node':
      return (
        <div className="space-y-3">
          <ConfigField label="目标目录" hint="移动后文件夹的存放路径">
            <DirPickerField
              value={cfgStr(config, 'target_dir')}
              onChange={(v) => set('target_dir', v)}
              placeholder="/data/target"
              title="选择目标目录"
            />
          </ConfigField>
          <ConfigField label="移动单元" hint="folder：整个文件夹；file：逐文件移动（默认 folder）">
            <select
              value={cfgStr(config, 'move_unit') || 'folder'}
              onChange={(e) => set('move_unit', e.target.value)}
              className={FIELD_CLS}
            >
              <option value="folder">folder — 整个文件夹</option>
              <option value="file">file — 逐文件</option>
            </select>
          </ConfigField>
          <ConfigField label="冲突策略" hint="目标路径已存在时的处理方式（默认 skip）">
            <select
              value={cfgStr(config, 'conflict_policy') || 'skip'}
              onChange={(e) => set('conflict_policy', e.target.value)}
              className={FIELD_CLS}
            >
              <option value="skip">skip — 跳过</option>
              <option value="overwrite">overwrite — 覆盖</option>
              <option value="rename">rename — 自动重命名</option>
            </select>
          </ConfigField>
        </div>
      )

    case 'compress-node':
      return (
        <div className="space-y-3">
          <ConfigField label="压缩范围" hint="all：所有文件夹；leaf：仅叶子节点（默认 all）">
            <select
              value={cfgStr(config, 'scope') || 'all'}
              onChange={(e) => set('scope', e.target.value)}
              className={FIELD_CLS}
            >
              <option value="all">all — 所有</option>
              <option value="leaf">leaf — 仅叶子</option>
            </select>
          </ConfigField>
          <ConfigField label="压缩格式" hint="cbz：漫画专用格式；zip：通用压缩（默认 cbz）">
            <select
              value={cfgStr(config, 'format') || 'cbz'}
              onChange={(e) => set('format', e.target.value)}
              className={FIELD_CLS}
            >
              <option value="cbz">cbz — 漫画格式</option>
              <option value="zip">zip — 通用压缩</option>
            </select>
          </ConfigField>
          <ConfigField label="输出目录" hint="压缩文件的存放路径，留空则放在原文件夹旁">
            <DirPickerField
              value={cfgStr(config, 'target_dir')}
              onChange={(v) => set('target_dir', v)}
              placeholder="/data/archive"
              title="选择输出目录"
            />
          </ConfigField>
        </div>
      )

    case 'thumbnail-node':
      return (
        <div className="space-y-3">
          <ConfigField label="输出目录" hint="缩略图的存放路径，留空则与视频文件同目录">
            <DirPickerField
              value={cfgStr(config, 'output_dir')}
              onChange={(v) => set('output_dir', v)}
              placeholder="/data/thumbnails"
              title="选择缩略图输出目录"
            />
          </ConfigField>
          <ConfigField label="截图偏移（秒）" hint="从视频第几秒截取缩略图（默认 8）">
            <input
              type="number"
              min={0}
              value={cfgNum(config, 'offset_seconds', 8)}
              onChange={(e) => set('offset_seconds', e.target.value)}
              className={FIELD_CLS}
            />
          </ConfigField>
          <ConfigField label="缩略图宽度（像素）" hint="输出图片宽度，高度等比缩放（默认 640）">
            <input
              type="number"
              min={64}
              max={1920}
              value={cfgNum(config, 'width', 640)}
              onChange={(e) => set('width', e.target.value)}
              className={FIELD_CLS}
            />
          </ConfigField>
        </div>
      )

    default:
      return (
        <NodeUsageHint>
          该节点暂无可配置项。
        </NodeUsageHint>
      )
  }
}

// ─── WorkflowNodeCard ─────────────────────────────────────────────────────────

const NODE_STATUS_CFG: Record<NodeRunStatus, { label: string; cls: string; icon: ReactElement | null }> = {
  running: { label: '执行中', cls: 'text-amber-700 bg-amber-50 border-amber-200', icon: <Loader2 className="h-3 w-3 animate-spin" /> },
  succeeded: { label: '完成', cls: 'text-emerald-700 bg-emerald-50 border-emerald-200', icon: <CheckCircle2 className="h-3 w-3" /> },
  failed: { label: '失败', cls: 'text-red-700 bg-red-50 border-red-200', icon: <TriangleAlert className="h-3 w-3" /> },
  pending: { label: '等待中', cls: 'text-muted-foreground bg-muted/40 border-border', icon: null },
  skipped: { label: '已跳过', cls: 'text-slate-500 bg-slate-50 border-slate-200', icon: null },
  waiting_input: { label: '等待输入', cls: 'text-blue-700 bg-blue-50 border-blue-200', icon: <Loader2 className="h-3 w-3 animate-pulse" /> },
}

function WorkflowNodeCard({ id, data, selected }: NodeProps<EditorNode>) {
  const { workflowNodes, updateNode, updateNodeConfig, deleteNode, nodeRunByNodeId, onRollbackRun } =
    useEditorContext()
  const workflowNode = workflowNodes[id] ?? null
  const nodeRun = nodeRunByNodeId[id] ?? null

  const inputPorts = data.schema?.input_ports ?? []
  const outputPorts = data.schema?.output_ports ?? []
  const category = getNodeCategory(data.type)
  const accentClass = category.accentClass

  const renamePreview = useMemo(() => {
    if (!workflowNode || data.type !== 'rename-node') return null
    return buildRenamePreview(workflowNode.config)
  }, [workflowNode, data.type])

  return (
    <div
      className={cn(
        'rounded-2xl border bg-gradient-to-br shadow-sm transition-shadow',
        accentClass,
        data.enabled ? 'opacity-100' : 'opacity-60 grayscale-[0.2]',
        selected ? 'w-[300px] shadow-lg ring-2 ring-primary/60' : 'min-w-[200px]',
      )}
    >
      {inputPorts.map((port, index) => (
        <Handle
          key={`in-${port.name}`}
          id={`in-${index}`}
          type="target"
          position={Position.Left}
          style={{ top: `${((index + 1) / (inputPorts.length + 1)) * 100}%` }}
        />
      ))}
      {outputPorts.map((port, index) => (
        <Handle
          key={`out-${port.name}`}
          id={`out-${index}`}
          type="source"
          position={Position.Right}
          style={{ top: `${((index + 1) / (outputPorts.length + 1)) * 100}%` }}
        />
      ))}

      {/* Collapsed header — always visible */}
      <div className="px-3 py-2.5">
        <div className="flex items-center justify-between gap-2">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-1.5">
              <span
                className={cn('h-1.5 w-1.5 shrink-0 rounded-full', category.iconColor.replace('text-', 'bg-'))}
              />
              <p className="truncate text-sm font-semibold leading-tight text-foreground">{data.label}</p>
            </div>
            <p className="ml-3 mt-0.5 truncate font-mono text-[10px] text-muted-foreground">{data.type}</p>
          </div>
          <span
            className={cn(
              'shrink-0 rounded-full px-2 py-0.5 text-[10px] font-medium',
              data.enabled ? 'bg-background/80 text-foreground' : 'bg-muted text-muted-foreground',
            )}
          >
            {data.enabled ? '启用' : '停用'}
          </span>
        </div>
        {(inputPorts.length > 0 || outputPorts.length > 0) && (
          <div className="ml-3 mt-1.5 flex gap-2 text-[10px] text-muted-foreground">
            {inputPorts.length > 0 && <span>↓ 输入 {inputPorts.length}</span>}
            {outputPorts.length > 0 && <span>↑ 输出 {outputPorts.length}</span>}
          </div>
        )}
        {nodeRun && (() => {
          const cfg = NODE_STATUS_CFG[nodeRun.status]
          return (
            <div className={cn('ml-3 mt-1.5 inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-medium', cfg.cls)}>
              {cfg.icon}
              {cfg.label}
              {nodeRun.error && <span className="ml-0.5 opacity-70">— {nodeRun.error}</span>}
            </div>
          )
        })()}
      </div>

      {/* Expanded config form — only when selected */}
      {selected && (
        <div className="nodrag nowheel nopan max-h-[55vh] overflow-y-auto border-t border-border/50 px-3 pb-3 pt-2.5">
          <div className="space-y-3">
            {/* Enable toggle */}
            <label className="flex cursor-pointer items-center justify-between rounded-xl border border-border bg-background/50 px-3 py-2">
              <span className="text-sm font-medium">启用该节点</span>
              <input
                type="checkbox"
                checked={data.enabled}
                onChange={(e) => updateNode(id, { enabled: e.target.checked })}
                className="h-4 w-4 rounded border-border text-primary focus:ring-primary"
              />
            </label>

            {/* Rename preview */}
            {renamePreview && (
              <div className="rounded-xl border border-emerald-200 bg-emerald-50/70 p-2.5">
                <div className="flex items-center justify-between">
                  <p className="text-xs font-semibold text-emerald-800">重命名预览</p>
                  <span className="text-[10px] text-emerald-700">策略：{renamePreview.strategy}</span>
                </div>
                <p className="mt-1 text-[10px] text-emerald-700/80">
                  示例：
                  <span className="font-mono">{RENAME_PREVIEW_SAMPLE.name}</span>
                </p>
                <div className="mt-1.5 rounded-lg border border-emerald-200 bg-white/80 px-2 py-1.5">
                  <p className="text-[10px] text-muted-foreground">预览目标名称</p>
                  <p className="font-mono text-sm text-foreground">{renamePreview.targetName}</p>
                </div>
                {renamePreview.warning && (
                  <p className="mt-1 text-[10px] text-amber-700">{renamePreview.warning}</p>
                )}
              </div>
            )}

            {workflowNode && (
              <NodeConfigPanel
                nodeId={id}
                nodeType={data.type}
                config={workflowNode.config}
                updateNodeConfig={updateNodeConfig}
              />
            )}

            {nodeRun && nodeRun.status === 'succeeded' && (
              <button
                type="button"
                onClick={() => void onRollbackRun(nodeRun.workflow_run_id)}
                className="inline-flex w-full items-center justify-center gap-2 rounded-xl border border-amber-200 bg-amber-50/60 px-3 py-2 text-sm font-medium text-amber-800 transition hover:bg-amber-100"
              >
                <RotateCcw className="h-4 w-4" />
                回退此节点的工作流运行
              </button>
            )}
            <button
              type="button"
              onClick={() => deleteNode(id)}
              className="inline-flex w-full items-center justify-center gap-2 rounded-xl border border-red-200 bg-red-50/60 px-3 py-2 text-sm font-medium text-red-700 transition hover:bg-red-100"
            >
              <Trash2 className="h-4 w-4" />
              删除该节点
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

const NODE_TYPES = { workflowNode: WorkflowNodeCard }

// ─── RunWorkflowModal ────────────────────────────────────────────────────────

interface RunWorkflowModalProps {
  open: boolean
  workflowDefId: string
  onClose: () => void
  onStarted: (jobId: string) => void
}

function RunWorkflowModal({ open, workflowDefId, onClose, onStarted }: RunWorkflowModalProps) {
  const folders = useFolderStore((s) => s.folders)
  const fetchFolders = useFolderStore((s) => s.fetchFolders)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (open) {
      void fetchFolders()
      setSelected(new Set())
      setError(null)
    }
  }, [open, fetchFolders])

  function toggleFolder(id: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function toggleAll() {
    if (selected.size === folders.length) setSelected(new Set())
    else setSelected(new Set(folders.map((f) => f.id)))
  }

  async function handleRun() {
    if (selected.size === 0) return
    setIsSubmitting(true)
    setError(null)
    try {
      const res = await startWorkflowJob({ workflow_def_id: workflowDefId, folder_ids: [...selected] })
      onStarted(res.job_id)
      onClose()
    } catch (e) {
      setError(e instanceof Error ? e.message : '启动失败')
    } finally {
      setIsSubmitting(false)
    }
  }

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="flex w-full max-w-md flex-col rounded-2xl border border-border bg-card shadow-2xl">
        <div className="flex items-center justify-between border-b border-border px-5 py-4">
          <h2 className="text-base font-semibold">运行工作流</h2>
          <button type="button" onClick={onClose} className="rounded-lg border border-border p-1.5 transition hover:bg-accent">
            <Trash2 className="h-4 w-4" />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto px-5 py-4">
          <div className="mb-3 flex items-center justify-between">
            <p className="text-sm text-muted-foreground">选择要处理的文件夹（{selected.size}/{folders.length}）</p>
            <button type="button" onClick={toggleAll} className="text-xs text-primary hover:underline">
              {selected.size === folders.length ? '取消全选' : '全选'}
            </button>
          </div>
          {folders.length === 0 && (
            <p className="py-4 text-center text-sm text-muted-foreground">暂无文件夹，请先扫描</p>
          )}
          <div className="max-h-72 space-y-1.5 overflow-y-auto">
            {folders.map((folder) => (
              <label
                key={folder.id}
                className={cn(
                  'flex cursor-pointer items-center gap-3 rounded-xl border px-3 py-2.5 transition',
                  selected.has(folder.id) ? 'border-primary/40 bg-primary/5' : 'border-border hover:bg-accent/50',
                )}
              >
                <input
                  type="checkbox"
                  checked={selected.has(folder.id)}
                  onChange={() => toggleFolder(folder.id)}
                  className="h-4 w-4 rounded border-border text-primary focus:ring-primary"
                />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">{folder.name}</p>
                  <p className="truncate text-[10px] text-muted-foreground">{folder.path}</p>
                </div>
              </label>
            ))}
          </div>
          {error && <p className="mt-3 text-sm text-red-600">{error}</p>}
        </div>
        <div className="flex justify-end gap-2 border-t border-border px-5 py-4">
          <button type="button" onClick={onClose} className="rounded-lg border border-border px-4 py-2 text-sm transition hover:bg-accent">取消</button>
          <button
            type="button"
            disabled={selected.size === 0 || isSubmitting}
            onClick={() => void handleRun()}
            className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition hover:opacity-90 disabled:opacity-50"
          >
            {isSubmitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
            {isSubmitting ? '启动中...' : '开始运行'}
          </button>
        </div>
      </div>
    </div>
  )
}

// ─── WorkflowEditorScreen ─────────────────────────────────────────────────────

function WorkflowEditorScreen() {
  const navigate = useNavigate()
  const params = useParams<{ id: string }>()
  const workflowDefId = params.id ?? ''

  const [workflowDef, setWorkflowDef] = useState<WorkflowDefinition | null>(null)
  const [workflowNodes, setWorkflowNodes] = useState<Record<string, WorkflowGraphNode>>({})
  const [schemas, setSchemas] = useState<NodeSchema[]>([])
  const [nodes, setNodes, onNodesChange] = useNodesState<EditorNode>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])
  const [selectedEdgeId, setSelectedEdgeId] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [isNodePanelOpen, setIsNodePanelOpen] = useState(false)
  const [isRunModalOpen, setIsRunModalOpen] = useState(false)
  const [selectionModeOn, setSelectionModeOn] = useState(false)
  const [activeJobId, setActiveJobId] = useState<string | null>(null)

  const nodesByRunId = useWorkflowRunStore((s) => s.nodesByRunId)
  const runsByJobId = useWorkflowRunStore((s) => s.runsByJobId)
  const fetchRunsForJob = useWorkflowRunStore((s) => s.fetchRunsForJob)
  const rollbackRun = useWorkflowRunStore((s) => s.rollbackRun)

  const nodeRunByNodeId = useMemo<Record<string, NodeRun>>(() => {
    if (!activeJobId) return {}
    const runs = runsByJobId[activeJobId] ?? []
    const result: Record<string, NodeRun> = {}
    for (const run of runs) {
      const nodeRuns = nodesByRunId[run.id] ?? []
      for (const nr of nodeRuns) {
        const prev = result[nr.node_id]
        if (!prev || nr.sequence > prev.sequence) result[nr.node_id] = nr
      }
    }
    return result
  }, [activeJobId, runsByJobId, nodesByRunId])

  useEffect(() => {
    if (!activeJobId) return
    void fetchRunsForJob(activeJobId)
  }, [activeJobId, fetchRunsForJob])

  const handleRollbackRun = useCallback(async (runId: string) => {
    await rollbackRun(runId)
    if (activeJobId) void fetchRunsForJob(activeJobId)
  }, [rollbackRun, activeJobId, fetchRunsForJob])

  const schemaMap = useMemo(() => new Map(schemas.map((schema) => [schema.type, schema])), [schemas])
  const schemaMapRef = useRef(schemaMap)
  schemaMapRef.current = schemaMap

  useEffect(() => {
    let active = true

    async function load() {
      if (!workflowDefId) {
        setError('缺少工作流 ID')
        setIsLoading(false)
        return
      }

      setIsLoading(true)
      setError(null)
      try {
        const [workflowResponse, nodeTypeResponse] = await Promise.all([
          getWorkflowDef(workflowDefId),
          listNodeTypes(),
        ])
        if (!active) return

        const nextSchemas = nodeTypeResponse.data ?? []
        const nextSchemaMap = new Map(nextSchemas.map((schema) => [schema.type, schema]))
        const nextWorkflow = workflowResponse.data
        const graph = safeParseGraph(nextWorkflow.graph_json)
        const nextWorkflowNodes = Object.fromEntries(graph.nodes.map((node) => [node.id, node]))

        setSchemas(nextSchemas)
        setWorkflowDef(nextWorkflow)
        setWorkflowNodes(nextWorkflowNodes)
        setNodes(graphToNodes(graph, nextSchemaMap))
        setEdges(graphToEdges(graph))
        setSelectedEdgeId(null)
      } catch (loadError) {
        if (!active) return
        setError(loadError instanceof Error ? loadError.message : '加载工作流编辑器失败')
      } finally {
        if (active) setIsLoading(false)
      }
    }

    void load()
    return () => { active = false }
  }, [workflowDefId, setEdges, setNodes])

  const onConnect = useMemo<OnConnect>(
    () => (connection: Connection) => {
      setEdges((currentEdges) => addEdge({ ...connection, animated: false }, currentEdges))
      setNotice(null)
    },
    [setEdges],
  )

  // ─── Context callbacks (stable via refs) ────────────────────────────────────

  const updateNode = useCallback(
    (nodeId: string, patch: Partial<WorkflowGraphNode> & { schemaType?: string }) => {
      setNodes((currentNodes) =>
        currentNodes.map((node) => {
          if (node.id !== nodeId) return node
          const nextType = patch.schemaType ?? node.data.type
          const nextSchema = schemaMapRef.current.get(nextType)
          return {
            ...node,
            data: {
              ...node.data,
              type: nextType,
              schema: nextSchema,
              label: patch.label ?? node.data.label,
              enabled: patch.enabled ?? node.data.enabled,
            },
          }
        }),
      )
      setWorkflowNodes((currentNodes) => {
        const previous = currentNodes[nodeId]
        if (!previous) return currentNodes
        return {
          ...currentNodes,
          [nodeId]: {
            ...previous,
            ...patch,
            type: patch.schemaType ?? previous.type,
          },
        }
      })
    },
    [setNodes, setWorkflowNodes],
  )

  const updateNodeConfig = useCallback(
    (nodeId: string, key: string, rawValue: string) => {
      setWorkflowNodes((currentNodes) => {
        const previous = currentNodes[nodeId]
        if (!previous) return currentNodes
        return {
          ...currentNodes,
          [nodeId]: {
            ...previous,
            config: { ...previous.config, [key]: parseConfigValue(rawValue) },
          },
        }
      })
    },
    [setWorkflowNodes],
  )

  const removeNodeConfig = useCallback(
    (nodeId: string, key: string) => {
      setWorkflowNodes((currentNodes) => {
        const previous = currentNodes[nodeId]
        if (!previous) return currentNodes
        const nextConfig = { ...previous.config }
        delete nextConfig[key]
        return { ...currentNodes, [nodeId]: { ...previous, config: nextConfig } }
      })
    },
    [setWorkflowNodes],
  )

  const addNodeConfig = useCallback(
    (nodeId: string, key: string, rawValue: string) => {
      setWorkflowNodes((currentNodes) => {
        const previous = currentNodes[nodeId]
        if (!previous) return currentNodes
        return {
          ...currentNodes,
          [nodeId]: {
            ...previous,
            config: { ...previous.config, [key]: parseConfigValue(rawValue) },
          },
        }
      })
    },
    [setWorkflowNodes],
  )

  const deleteNode = useCallback(
    (nodeId: string) => {
      setNodes((currentNodes) => currentNodes.filter((n) => n.id !== nodeId))
      setEdges((currentEdges) =>
        currentEdges.filter((e) => e.source !== nodeId && e.target !== nodeId),
      )
      setWorkflowNodes((currentNodes) => {
        const next = { ...currentNodes }
        delete next[nodeId]
        return next
      })
    },
    [setNodes, setEdges, setWorkflowNodes],
  )

  const editorContextValue: EditorContextValue = useMemo(
    () => ({ workflowNodes, schemas, updateNode, updateNodeConfig, removeNodeConfig, addNodeConfig, deleteNode, nodeRunByNodeId, onRollbackRun: handleRollbackRun }),
    [workflowNodes, schemas, updateNode, updateNodeConfig, removeNodeConfig, addNodeConfig, deleteNode, nodeRunByNodeId, handleRollbackRun],
  )

  // ─── Add node ────────────────────────────────────────────────────────────────

  function addNode(schema: NodeSchema) {
    setNodes((currentNodes) => {
      const nodeId = buildNodeId(schema.type, currentNodes)
      const nextNode: EditorNode = {
        id: nodeId,
        type: 'workflowNode',
        position: {
          x: 160 + ((currentNodes.length % 3) * 280),
          y: 140 + (Math.floor(currentNodes.length / 3) * 180),
        },
        data: { label: schema.label, type: schema.type, enabled: true, schema },
      }
      setWorkflowNodes((currentWorkflowNodes) => ({
        ...currentWorkflowNodes,
        [nodeId]: {
          id: nodeId,
          type: schema.type,
          label: schema.label,
          config: {},
          inputs: {},
          ui_position: nextNode.position,
          enabled: true,
        },
      }))
      return [...currentNodes, nextNode]
    })
  }

  function deleteSelectedEdge() {
    if (!selectedEdgeId) return
    setEdges((currentEdges) => currentEdges.filter((edge) => edge.id !== selectedEdgeId))
    setSelectedEdgeId(null)
  }

  async function handleSave() {
    if (!workflowDef) return
    setIsSaving(true)
    setError(null)
    setNotice(null)
    try {
      const graph = nodesToGraph(nodes, edges, workflowNodes, schemaMap)
      const graphJson = JSON.stringify(graph, null, 2)
      await updateWorkflowDef(workflowDef.id, { graph_json: graphJson })
      setWorkflowDef({ ...workflowDef, graph_json: graphJson })
      setWorkflowNodes(Object.fromEntries(graph.nodes.map((node) => [node.id, node])))
      setNotice('工作流已保存')
    } catch (saveError) {
      if (saveError instanceof ApiRequestError) {
        setError(saveError.message)
      } else {
        setError(saveError instanceof Error ? saveError.message : '保存失败')
      }
    } finally {
      setIsSaving(false)
    }
  }

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center text-sm text-muted-foreground">
        正在加载工作流编辑器...
      </div>
    )
  }

  return (
    <EditorContext.Provider value={editorContextValue}>
      {/* overflow-hidden ensures the page itself never scrolls */}
      <div className="flex h-screen overflow-hidden flex-col bg-background text-foreground">
        {/* ── Header ─────────────────────────────────────────────────────── */}
        <header className="shrink-0 border-b border-border/80 bg-background/90 backdrop-blur">
          <div className="flex items-center justify-between gap-4 px-5 py-3">
            <div className="flex items-center gap-3">
              <button
                type="button"
                onClick={() => navigate('/workflow-defs')}
                className="inline-flex items-center gap-2 rounded-xl border border-border bg-background px-3 py-2 text-sm transition hover:bg-accent"
              >
                <ArrowLeft className="h-4 w-4" />
                返回列表
              </button>
              <div>
                <p className="text-xs uppercase tracking-[0.22em] text-muted-foreground">
                  工作流编辑器
                </p>
                <h1 className="text-base font-semibold leading-tight">
                  {workflowDef?.name ?? '未命名工作流'}
                </h1>
              </div>
            </div>

            <div className="flex items-center gap-3">
              {error && <span className="text-sm text-red-600">{error}</span>}
              {notice && <span className="text-sm text-emerald-700">{notice}</span>}
              <button
                type="button"
                onClick={() => setIsRunModalOpen(true)}
                className="inline-flex items-center gap-2 rounded-xl border border-emerald-300 bg-emerald-50 px-4 py-2 text-sm font-medium text-emerald-800 transition hover:bg-emerald-100"
              >
                <Play className="h-4 w-4" />
                运行
              </button>
              <button
                type="button"
                onClick={() => void handleSave()}
                disabled={isSaving}
                className="inline-flex items-center gap-2 rounded-xl bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition hover:opacity-90 disabled:opacity-60"
              >
                <Save className="h-4 w-4" />
                {isSaving ? '保存中...' : '保存工作流'}
              </button>
            </div>
          </div>
        </header>

        {/* ── Body ───────────────────────────────────────────────────────── */}
        <div className="flex min-h-0 flex-1 overflow-hidden">
          {/* ── Node panel sidebar (collapsible) ─────────────────────────── */}
          <aside
            className={cn(
              'relative shrink-0 border-r border-border/80 bg-background/85 transition-[width] duration-200',
              isNodePanelOpen ? 'w-64' : 'w-10',
            )}
          >
            {/* Toggle button */}
            <button
              type="button"
              onClick={() => setIsNodePanelOpen((open) => !open)}
              title={isNodePanelOpen ? '收起节点面板' : '展开节点面板'}
              className={cn(
                'absolute top-3 z-10 flex h-7 w-7 items-center justify-center rounded-lg border border-border bg-background text-muted-foreground transition hover:bg-accent hover:text-foreground',
                isNodePanelOpen ? 'right-3' : 'left-1.5',
              )}
            >
              {isNodePanelOpen ? <ChevronLeft className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
            </button>

            {/* Panel content — only rendered when open */}
            {isNodePanelOpen && (
              <div className="flex h-full flex-col pt-12">
                <div className="border-b border-border/70 px-4 pb-3">
                  <div className="flex items-center gap-2 text-sm font-semibold">
                    <Wand2 className="h-4 w-4 text-sky-600" />
                    节点面板
                  </div>
                  <p className="mt-0.5 text-xs text-muted-foreground">点击节点将其添加到画布。</p>
                </div>
                <div className="flex-1 overflow-y-auto p-3">
                  {NODE_CATEGORIES.map((category) => {
                    const categorySchemas = schemas.filter((schema) => category.types.has(schema.type))
                    if (categorySchemas.length === 0) return null
                    return (
                      <div key={category.label} className="mb-4">
                        <div className="mb-1.5 flex items-center gap-1.5">
                          <span className={cn('h-2 w-2 rounded-full', category.iconColor.replace('text-', 'bg-'))} />
                          <p className={cn('text-[11px] font-semibold', category.iconColor)}>{category.label}</p>
                        </div>
                        <div className="space-y-1.5">
                          {categorySchemas.map((schema) => (
                            <button
                              key={schema.type}
                              type="button"
                              onClick={() => addNode(schema)}
                              className={cn(
                                'w-full rounded-xl border border-border bg-card px-3 py-2 text-left transition hover:-translate-y-0.5 hover:shadow-sm',
                                category.borderHoverClass,
                              )}
                            >
                              <div className="flex items-center justify-between gap-2">
                                <span className="text-sm font-medium">{schema.label}</span>
                                <Plus className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                              </div>
                              {schema.description && (
                                <p className="mt-0.5 text-[10px] text-muted-foreground line-clamp-1">
                                  {schema.description}
                                </p>
                              )}
                            </button>
                          ))}
                        </div>
                      </div>
                    )
                  })}
                  {/* Schemas not matched by any category */}
                  {(() => {
                    const knownTypes = new Set(NODE_CATEGORIES.flatMap((c) => [...c.types]))
                    const unknownSchemas = schemas.filter((schema) => !knownTypes.has(schema.type))
                    if (unknownSchemas.length === 0) return null
                    return (
                      <div className="mb-4">
                        <div className="mb-1.5 flex items-center gap-1.5">
                          <span className="h-2 w-2 rounded-full bg-gray-400" />
                          <p className="text-[11px] font-semibold text-gray-500">其他</p>
                        </div>
                        <div className="space-y-1.5">
                          {unknownSchemas.map((schema) => (
                            <button
                              key={schema.type}
                              type="button"
                              onClick={() => addNode(schema)}
                              className="w-full rounded-xl border border-border bg-card px-3 py-2 text-left transition hover:-translate-y-0.5 hover:border-gray-300 hover:shadow-sm"
                            >
                              <div className="flex items-center justify-between gap-2">
                                <span className="text-sm font-medium">{schema.label}</span>
                                <Plus className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                              </div>
                            </button>
                          ))}
                        </div>
                      </div>
                    )
                  })()}
                </div>
              </div>
            )}
          </aside>

          {/* ── Canvas ───────────────────────────────────────────────────── */}
          <div className="relative min-w-0 flex-1">
            <button
              type="button"
              onClick={() => setSelectionModeOn((v) => !v)}
              title={selectionModeOn ? '切换为拖拽模式' : '切换为框选模式'}
              className={cn(
                'absolute right-4 top-4 z-10 flex items-center gap-1.5 rounded-xl border px-3 py-2 text-xs font-medium shadow-sm transition',
                selectionModeOn
                  ? 'border-primary/40 bg-primary/10 text-primary'
                  : 'border-border bg-background/90 text-muted-foreground hover:bg-accent',
              )}
            >
              <MousePointer className="h-3.5 w-3.5" />
              {selectionModeOn ? '框选模式' : '拖拽模式'}
            </button>
            <ReactFlow
              nodes={nodes}
              edges={edges}
              nodeTypes={NODE_TYPES}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              onConnect={onConnect}
              onNodeClick={() => setSelectedEdgeId(null)}
              onPaneClick={() => setSelectedEdgeId(null)}
              onEdgeClick={(_, edge) => setSelectedEdgeId(edge.id)}
              fitView
              panOnScroll={!selectionModeOn}
              panOnScrollMode={PanOnScrollMode.Free}
              panOnDrag={!selectionModeOn}
              selectionOnDrag={selectionModeOn}
              selectionMode={SelectionMode.Partial}
              zoomOnScroll={false}
              zoomOnPinch
              preventScrolling
              className="bg-[linear-gradient(135deg,rgba(248,250,252,0.9),rgba(240,249,255,0.9))]"
              defaultEdgeOptions={{ style: { strokeWidth: 2, stroke: '#0f766e' } }}
            >
              <Background gap={24} size={1} color="#cbd5e1" />
              <MiniMap className="!bg-background/90" pannable zoomable />
              <Controls />
            </ReactFlow>

            {/* Edge selected — delete bar */}
            {selectedEdgeId && (
              <div className="absolute bottom-4 left-4 z-10 rounded-2xl border border-border bg-background/95 px-4 py-3 shadow-lg">
                <div className="flex items-center gap-4">
                  <div>
                    <p className="text-sm font-medium">已选中连线</p>
                    <p className="text-xs text-muted-foreground">
                      删除后会同时清除目标节点上的 link_source。
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={deleteSelectedEdge}
                    className="rounded-xl border border-red-200 px-3 py-2 text-sm text-red-700 transition hover:bg-red-50"
                  >
                    删除连线
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
      <RunWorkflowModal
        open={isRunModalOpen}
        workflowDefId={workflowDefId}
        onClose={() => setIsRunModalOpen(false)}
        onStarted={(jobId) => setActiveJobId(jobId)}
      />
    </EditorContext.Provider>
  )
}

// ─── Page entry ───────────────────────────────────────────────────────────────

export default function WorkflowEditorPage() {
  return (
    <ReactFlowProvider>
      <WorkflowEditorScreen />
    </ReactFlowProvider>
  )
}
