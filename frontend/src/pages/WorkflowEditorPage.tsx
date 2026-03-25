import { useEffect, useMemo, useState } from 'react'
import {
  addEdge,
  Background,
  Controls,
  MiniMap,
  ReactFlow,
  ReactFlowProvider,
  Handle,
  Position,
  useEdgesState,
  useNodesState,
  type Connection,
  type Edge,
  type Node,
  type NodeProps,
  type OnConnect,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { ArrowLeft, Plus, Save, Settings2, Trash2, Wand2 } from 'lucide-react'
import { Link, useNavigate, useParams } from 'react-router-dom'

import { ApiRequestError } from '@/api/client'
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

type EditorNodeData = {
  label: string
  type: string
  enabled: boolean
  schema?: NodeSchema
}

type EditorNode = Node<EditorNodeData>

const INITIAL_GRAPH: WorkflowGraph = { nodes: [], edges: [] }

const NODE_ACCENT_CLASSES = [
  'from-sky-500/20 to-cyan-500/10 border-sky-200',
  'from-emerald-500/20 to-teal-500/10 border-emerald-200',
  'from-amber-500/20 to-orange-500/10 border-amber-200',
  'from-rose-500/20 to-pink-500/10 border-rose-200',
  'from-slate-500/20 to-zinc-500/10 border-slate-200',
]

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
    if (edge.target !== nodeId) {
      continue
    }
    const targetPortIndex = parseHandleIndex(edge.targetHandle)
    const sourcePortIndex = parseHandleIndex(edge.sourceHandle)
    if (targetPortIndex == null || sourcePortIndex == null) {
      continue
    }
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
  if (!handle) {
    return null
  }
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
      ui_position: {
        x: Math.round(node.position.x),
        y: Math.round(node.position.y),
      },
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

function getConfigEntries(config: Record<string, unknown>) {
  return Object.entries(config).sort(([left], [right]) => left.localeCompare(right, 'zh-CN'))
}

function parseConfigValue(input: string): unknown {
  const trimmed = input.trim()
  if (trimmed === 'true') {
    return true
  }
  if (trimmed === 'false') {
    return false
  }
  if (trimmed !== '' && /^-?\d+(\.\d+)?$/.test(trimmed)) {
    return Number(trimmed)
  }
  if ((trimmed.startsWith('{') && trimmed.endsWith('}')) || (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
    try {
      return JSON.parse(trimmed) as unknown
    } catch {
      return input
    }
  }
  return input
}

function stringifyConfigValue(value: unknown) {
  if (typeof value === 'string') {
    return value
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }
  return JSON.stringify(value, null, 2)
}

type RenameConditionalRule = {
  condition: string
  template: string
}

type RenamePreviewResult = {
  strategy: string
  targetName: string
  warning: string | null
}

const RENAME_PREVIEW_SAMPLE = {
  name: 'Dune[2021]',
  category: 'video',
  parent: '电影合集',
  index: 1,
}

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
    title = title.replaceAll(`(${year})`, '')
    title = title.replaceAll(`[${year}]`, '')
    title = title.replaceAll(year, '')
  }
  title = renamePreviewTrimDecorators(title.trim())
  return title === '' ? name.trim() : title
}

function renamePreviewTrimDecorators(input: string) {
  const trimChars = '-_()[] '
  let start = 0
  let end = input.length

  for (; start < end; start += 1) {
    if (!trimChars.includes(input[start])) {
      break
    }
  }
  for (; end > start; end -= 1) {
    if (!trimChars.includes(input[end - 1])) {
      break
    }
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
  if (!Array.isArray(raw)) {
    return [] as RenameConditionalRule[]
  }

  const rules: RenameConditionalRule[] = []
  for (const item of raw) {
    if (!item || typeof item !== 'object') {
      continue
    }
    const itemMap = item as Record<string, unknown>
    const condition = renamePreviewString(itemMap.condition) || renamePreviewString(itemMap.if)
    const template = renamePreviewString(itemMap.template)
    if (template === '') {
      continue
    }
    rules.push({ condition, template })
  }
  return rules
}

function renamePreviewEvaluateCondition(condition: string, name: string, category: string) {
  const trimmed = condition.trim()
  if (trimmed === '') {
    return false
  }

  const containsMatch = trimmed.match(/^name\s+CONTAINS\s+"([^"]+)"$/i)
  if (containsMatch) {
    return name.toLowerCase().includes(containsMatch[1].toLowerCase())
  }

  const matchesMatch = trimmed.match(/^name\s+MATCHES\s+"([^"]+)"$/i)
  if (matchesMatch) {
    try {
      return new RegExp(matchesMatch[1]).test(name)
    } catch {
      return false
    }
  }

  const categoryMatch = trimmed.match(/^category\s*==\s*"([^"]+)"$/i)
  if (categoryMatch) {
    return category.trim().toLowerCase() === categoryMatch[1].trim().toLowerCase()
  }

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
    if (template !== '') {
      candidate = renamePreviewRenderTemplate(template, variables)
    }
  } else if (strategy === 'regex_extract') {
    if (regexPattern !== '') {
      try {
        const regex = new RegExp(regexPattern)
        const matches = regex.exec(currentName)
        if (matches && matches.length > 0) {
          const groups = matches.groups ? Object.entries(matches.groups) : []
          for (const [key, value] of groups) {
            if (typeof value === 'string') {
              variables[key] = value
            }
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

  if (candidate.trim() === '') {
    candidate = currentName
  }
  if (skipIfSame && candidate === currentName) {
    warning = warning ?? '当前命名结果与原名称一致，实际执行时会保持原名。'
  }

  return { strategy, targetName: candidate, warning }
}

function WorkflowNodeCard({ data, selected }: NodeProps<EditorNode>) {
  const inputPorts = data.schema?.input_ports ?? []
  const outputPorts = data.schema?.output_ports ?? []
  const accentClass = NODE_ACCENT_CLASSES[Math.abs(hashString(data.type)) % NODE_ACCENT_CLASSES.length]

  return (
    <div className={cn(
      'min-w-[220px] rounded-2xl border bg-gradient-to-br p-3 shadow-sm transition-all',
      accentClass,
      data.enabled ? 'opacity-100' : 'opacity-60 grayscale-[0.2]',
      selected && 'ring-2 ring-primary/60 shadow-lg',
    )}>
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

      <div className="mb-3 flex items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-medium uppercase tracking-[0.18em] text-muted-foreground">
            {data.type}
          </p>
          <h3 className="mt-1 text-sm font-semibold text-foreground">{data.label}</h3>
        </div>
        <span className={cn(
          'rounded-full px-2 py-0.5 text-[10px] font-medium',
          data.enabled ? 'bg-background/80 text-foreground' : 'bg-muted text-muted-foreground',
        )}>
          {data.enabled ? '启用' : '停用'}
        </span>
      </div>

      <div className="grid gap-2 text-[11px] text-muted-foreground">
        <div className="flex items-center justify-between gap-4">
          <span>输入 {inputPorts.length}</span>
          <span>输出 {outputPorts.length}</span>
        </div>
        <div className="flex flex-wrap gap-1.5">
          {inputPorts.slice(0, 3).map((port) => (
            <span key={port.name} className="rounded-full bg-background/80 px-2 py-0.5 text-[10px] text-foreground">
              {port.name}
            </span>
          ))}
          {inputPorts.length > 3 && <span>...</span>}
        </div>
      </div>
    </div>
  )
}

function WorkflowEditorScreen() {
  const navigate = useNavigate()
  const params = useParams<{ id: string }>()
  const workflowDefId = params.id ?? ''
  const [workflowDef, setWorkflowDef] = useState<WorkflowDefinition | null>(null)
  const [workflowNodes, setWorkflowNodes] = useState<Record<string, WorkflowGraphNode>>({})
  const [schemas, setSchemas] = useState<NodeSchema[]>([])
  const [nodes, setNodes, onNodesChange] = useNodesState<EditorNode>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const [selectedEdgeId, setSelectedEdgeId] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [newConfigKey, setNewConfigKey] = useState('')
  const [newConfigValue, setNewConfigValue] = useState('')

  const schemaMap = useMemo(() => new Map(schemas.map((schema) => [schema.type, schema])), [schemas])
  const selectedNode = useMemo(() => nodes.find((node) => node.id === selectedNodeId) ?? null, [nodes, selectedNodeId])
  const selectedWorkflowNode = selectedNode ? workflowNodes[selectedNode.id] : null
  const renamePreview = useMemo(() => {
    if (!selectedNode || !selectedWorkflowNode || selectedNode.data.type !== 'rename-node') {
      return null
    }
    return buildRenamePreview(selectedWorkflowNode.config)
  }, [selectedNode, selectedWorkflowNode])

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
        if (!active) {
          return
        }

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
        setSelectedNodeId(graph.nodes[0]?.id ?? null)
        setSelectedEdgeId(null)
      } catch (loadError) {
        if (!active) {
          return
        }
        setError(loadError instanceof Error ? loadError.message : '加载工作流编辑器失败')
      } finally {
        if (active) {
          setIsLoading(false)
        }
      }
    }

    void load()
    return () => {
      active = false
    }
  }, [workflowDefId, setEdges, setNodes])

  const onConnect = useMemo<OnConnect>(() => (connection: Connection) => {
    setEdges((currentEdges) => addEdge({ ...connection, animated: false }, currentEdges))
    setNotice(null)
  }, [setEdges])

  function addNode(schema: NodeSchema) {
    setNodes((currentNodes) => {
      const id = buildNodeId(schema.type, currentNodes)
      const nextNode: EditorNode = {
        id,
        type: 'workflowNode',
        position: {
          x: 160 + ((currentNodes.length % 3) * 280),
          y: 140 + (Math.floor(currentNodes.length / 3) * 180),
        },
        data: {
          label: schema.label,
          type: schema.type,
          enabled: true,
          schema,
        },
      }
      setWorkflowNodes((currentWorkflowNodes) => ({
        ...currentWorkflowNodes,
        [id]: {
          id,
          type: schema.type,
          label: schema.label,
          config: {},
          inputs: {},
          ui_position: nextNode.position,
          enabled: true,
        },
      }))
      setSelectedNodeId(id)
      return [...currentNodes, nextNode]
    })
  }

  function updateSelectedNode(patch: Partial<WorkflowGraphNode> & { schemaType?: string }) {
    if (!selectedNode) {
      return
    }

    setNodes((currentNodes) => currentNodes.map((node) => {
      if (node.id !== selectedNode.id) {
        return node
      }
      const nextType = patch.schemaType ?? node.data.type
      const nextSchema = schemaMap.get(nextType)
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
    }))

    setWorkflowNodes((currentWorkflowNodes) => {
      const previous = currentWorkflowNodes[selectedNode.id]
      if (!previous) {
        return currentWorkflowNodes
      }
      return {
        ...currentWorkflowNodes,
        [selectedNode.id]: {
          ...previous,
          ...patch,
          type: patch.schemaType ?? previous.type,
        },
      }
    })
  }

  function updateConfigValue(key: string, rawValue: string) {
    if (!selectedNode) {
      return
    }
    setWorkflowNodes((currentWorkflowNodes) => {
      const previous = currentWorkflowNodes[selectedNode.id]
      if (!previous) {
        return currentWorkflowNodes
      }
      return {
        ...currentWorkflowNodes,
        [selectedNode.id]: {
          ...previous,
          config: {
            ...previous.config,
            [key]: parseConfigValue(rawValue),
          },
        },
      }
    })
  }

  function removeConfigKey(key: string) {
    if (!selectedNode) {
      return
    }
    setWorkflowNodes((currentWorkflowNodes) => {
      const previous = currentWorkflowNodes[selectedNode.id]
      if (!previous) {
        return currentWorkflowNodes
      }
      const nextConfig = { ...previous.config }
      delete nextConfig[key]
      return {
        ...currentWorkflowNodes,
        [selectedNode.id]: {
          ...previous,
          config: nextConfig,
        },
      }
    })
  }

  function addConfigEntry() {
    if (!selectedNode || newConfigKey.trim() === '') {
      return
    }
    updateConfigValue(newConfigKey.trim(), newConfigValue)
    setNewConfigKey('')
    setNewConfigValue('')
  }

  function deleteSelectedNode() {
    if (!selectedNode) {
      return
    }
    setNodes((currentNodes) => currentNodes.filter((node) => node.id !== selectedNode.id))
    setEdges((currentEdges) => currentEdges.filter((edge) => edge.source !== selectedNode.id && edge.target !== selectedNode.id))
    setWorkflowNodes((currentWorkflowNodes) => {
      const next = { ...currentWorkflowNodes }
      delete next[selectedNode.id]
      return next
    })
    setSelectedNodeId(null)
  }

  function deleteSelectedEdge() {
    if (!selectedEdgeId) {
      return
    }
    setEdges((currentEdges) => currentEdges.filter((edge) => edge.id !== selectedEdgeId))
    setSelectedEdgeId(null)
  }

  async function handleSave() {
    if (!workflowDef) {
      return
    }

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
    return <div className="flex h-screen items-center justify-center text-sm text-muted-foreground">正在加载工作流编辑器...</div>
  }

  return (
    <div className="flex h-screen flex-col bg-[radial-gradient(circle_at_top_left,rgba(14,165,233,0.08),transparent_28%),radial-gradient(circle_at_bottom_right,rgba(251,191,36,0.08),transparent_26%)] bg-background text-foreground">
      <header className="border-b border-border/80 bg-background/90 backdrop-blur">
        <div className="flex items-center justify-between gap-4 px-5 py-4">
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
              <p className="text-xs uppercase tracking-[0.22em] text-muted-foreground">Workflow Editor</p>
              <h1 className="text-lg font-semibold">{workflowDef?.name ?? '工作流编辑器'}</h1>
            </div>
          </div>

          <div className="flex items-center gap-3">
            {error && <span className="text-sm text-red-600">{error}</span>}
            {notice && <span className="text-sm text-emerald-700">{notice}</span>}
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

      <div className="flex min-h-0 flex-1">
        <aside className="hidden w-72 shrink-0 border-r border-border/80 bg-background/85 lg:block">
          <div className="border-b border-border/70 px-5 py-4">
            <div className="flex items-center gap-2 text-sm font-semibold">
              <Wand2 className="h-4 w-4 text-sky-600" />
              节点面板
            </div>
            <p className="mt-1 text-xs text-muted-foreground">点击添加节点，随后在画布中拖动位置并连线。</p>
          </div>
          <div className="space-y-3 p-4">
            {schemas.map((schema) => (
              <button
                key={schema.type}
                type="button"
                onClick={() => addNode(schema)}
                className="w-full rounded-2xl border border-border bg-card px-4 py-3 text-left transition hover:-translate-y-0.5 hover:border-sky-300 hover:shadow-sm"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="font-medium">{schema.label}</span>
                  <Plus className="h-4 w-4 text-muted-foreground" />
                </div>
                <p className="mt-1 text-xs text-muted-foreground">{schema.description || schema.type}</p>
              </button>
            ))}
          </div>
        </aside>

        <div className="relative min-w-0 flex-1">
          <div className="absolute left-4 top-4 z-10 flex gap-2 lg:hidden">
            <Link
              to="/workflow-defs"
              className="rounded-xl border border-border bg-background px-3 py-2 text-sm shadow-sm"
            >
              返回
            </Link>
          </div>
          <ReactFlow
            nodes={nodes}
            edges={edges}
            nodeTypes={{ workflowNode: WorkflowNodeCard }}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            onNodeClick={(_, node) => {
              setSelectedNodeId(node.id)
              setSelectedEdgeId(null)
            }}
            onPaneClick={() => {
              setSelectedNodeId(null)
              setSelectedEdgeId(null)
            }}
            onEdgeClick={(_, edge) => {
              setSelectedEdgeId(edge.id)
              setSelectedNodeId(null)
            }}
            fitView
            className="bg-[linear-gradient(135deg,rgba(248,250,252,0.9),rgba(240,249,255,0.9))]"
            defaultEdgeOptions={{
              style: { strokeWidth: 2, stroke: '#0f766e' },
            }}
          >
            <Background gap={24} size={1} color="#cbd5e1" />
            <MiniMap className="!bg-background/90" pannable zoomable />
            <Controls />
          </ReactFlow>

          {selectedEdgeId && (
            <div className="absolute bottom-4 left-4 z-10 rounded-2xl border border-border bg-background/95 px-4 py-3 shadow-lg">
              <div className="flex items-center gap-3">
                <div>
                  <p className="text-sm font-medium">已选中连线</p>
                  <p className="text-xs text-muted-foreground">删除后会同时清除目标节点上的 link_source。</p>
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

        <aside className="w-[360px] shrink-0 border-l border-border/80 bg-background/90">
          <div className="border-b border-border/70 px-5 py-4">
            <div className="flex items-center gap-2 text-sm font-semibold">
              <Settings2 className="h-4 w-4 text-amber-600" />
              属性面板
            </div>
            <p className="mt-1 text-xs text-muted-foreground">
              {selectedNode ? '修改节点标签、启用状态和配置项。' : '请选择一个节点以编辑属性。'}
            </p>
          </div>

          {!selectedNode || !selectedWorkflowNode ? (
            <div className="flex h-[calc(100%-76px)] items-center justify-center px-8 text-center text-sm text-muted-foreground">
              选中一个节点后，这里会显示其配置表单。
            </div>
          ) : (
            <div className="space-y-6 overflow-auto p-5">
              <div className="space-y-2">
                <label className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">标签</label>
                <input
                  value={selectedNode.data.label}
                  onChange={(event) => updateSelectedNode({ label: event.target.value })}
                  className="w-full rounded-xl border border-border bg-background px-3 py-2 text-sm outline-none ring-primary focus:ring-2"
                />
              </div>

              <div className="space-y-2">
                <label className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">节点类型</label>
                <select
                  value={selectedNode.data.type}
                  onChange={(event) => updateSelectedNode({ schemaType: event.target.value })}
                  className="w-full rounded-xl border border-border bg-background px-3 py-2 text-sm outline-none ring-primary focus:ring-2"
                >
                  {schemas.map((schema) => (
                    <option key={schema.type} value={schema.type}>{schema.label}</option>
                  ))}
                </select>
              </div>

              <label className="flex items-center justify-between rounded-2xl border border-border bg-card px-4 py-3">
                <span className="text-sm font-medium">启用该节点</span>
                <input
                  type="checkbox"
                  checked={selectedNode.data.enabled}
                  onChange={(event) => updateSelectedNode({ enabled: event.target.checked })}
                  className="h-4 w-4 rounded border-border text-primary focus:ring-primary"
                />
              </label>

              {renamePreview && (
                <div className="space-y-3 rounded-2xl border border-emerald-200 bg-emerald-50/70 p-3">
                  <div className="flex items-center justify-between">
                    <h2 className="text-sm font-semibold text-emerald-800">重命名预览</h2>
                    <span className="text-[11px] text-emerald-700">策略：{renamePreview.strategy}</span>
                  </div>
                  <p className="text-xs text-emerald-800/90">
                    示例输入：名称 <span className="font-mono">{RENAME_PREVIEW_SAMPLE.name}</span>，分类 <span className="font-mono">{RENAME_PREVIEW_SAMPLE.category}</span>，父目录 <span className="font-mono">{RENAME_PREVIEW_SAMPLE.parent}</span>
                  </p>
                  <div className="rounded-xl border border-emerald-200 bg-background px-3 py-2">
                    <p className="text-[11px] text-muted-foreground">预览目标名称</p>
                    <p className="font-mono text-sm text-foreground">{renamePreview.targetName}</p>
                  </div>
                  {renamePreview.warning && (
                    <p className="text-xs text-amber-700">{renamePreview.warning}</p>
                  )}
                </div>
              )}

              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <div>
                    <h2 className="text-sm font-semibold">配置项</h2>
                    <p className="text-xs text-muted-foreground">支持文本、数字、布尔值与 JSON。</p>
                  </div>
                </div>

                {getConfigEntries(selectedWorkflowNode.config).length === 0 && (
                  <div className="rounded-2xl border border-dashed border-border px-4 py-5 text-sm text-muted-foreground">
                    当前没有配置项，添加后即可在保存时写入 `graph_json`。
                  </div>
                )}

                {getConfigEntries(selectedWorkflowNode.config).map(([key, value]) => (
                  <div key={key} className="rounded-2xl border border-border bg-card p-3">
                    <div className="mb-2 flex items-center justify-between gap-3">
                      <label className="text-sm font-medium">{key}</label>
                      <button
                        type="button"
                        onClick={() => removeConfigKey(key)}
                        className="rounded-lg p-1.5 text-muted-foreground transition hover:bg-red-50 hover:text-red-600"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                    <textarea
                      value={stringifyConfigValue(value)}
                      onChange={(event) => updateConfigValue(key, event.target.value)}
                      rows={typeof value === 'string' && value.length < 80 ? 2 : 4}
                      className="w-full rounded-xl border border-border bg-background px-3 py-2 font-mono text-xs outline-none ring-primary focus:ring-2"
                    />
                  </div>
                ))}

                <div className="rounded-2xl border border-border bg-muted/30 p-3">
                  <p className="mb-2 text-sm font-medium">添加配置项</p>
                  <div className="grid gap-2">
                    <input
                      value={newConfigKey}
                      onChange={(event) => setNewConfigKey(event.target.value)}
                      placeholder="配置键，例如 target_dir"
                      className="w-full rounded-xl border border-border bg-background px-3 py-2 text-sm outline-none ring-primary focus:ring-2"
                    />
                    <textarea
                      value={newConfigValue}
                      onChange={(event) => setNewConfigValue(event.target.value)}
                      placeholder="配置值，例如 /data/target/video 或 true"
                      rows={3}
                      className="w-full rounded-xl border border-border bg-background px-3 py-2 font-mono text-xs outline-none ring-primary focus:ring-2"
                    />
                    <button
                      type="button"
                      onClick={addConfigEntry}
                      className="inline-flex items-center justify-center gap-2 rounded-xl border border-border bg-background px-3 py-2 text-sm transition hover:bg-accent"
                    >
                      <Plus className="h-4 w-4" />
                      添加配置项
                    </button>
                  </div>
                </div>
              </div>

              <div className="rounded-2xl border border-red-200 bg-red-50/60 p-3">
                <button
                  type="button"
                  onClick={deleteSelectedNode}
                  className="inline-flex items-center gap-2 text-sm font-medium text-red-700"
                >
                  <Trash2 className="h-4 w-4" />
                  删除该节点
                </button>
              </div>
            </div>
          )}
        </aside>
      </div>
    </div>
  )
}

function hashString(input: string) {
  let hash = 0
  for (let index = 0; index < input.length; index += 1) {
    hash = ((hash << 5) - hash) + input.charCodeAt(index)
    hash |= 0
  }
  return hash
}

export default function WorkflowEditorPage() {
  return (
    <ReactFlowProvider>
      <WorkflowEditorScreen />
    </ReactFlowProvider>
  )
}
