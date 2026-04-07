import type { WorkflowGraph, WorkflowGraphNode } from '@/types'

interface FolderPickerConfig {
  source_mode?: unknown
  saved_folder_ids?: unknown
  folder_ids?: unknown
  paths?: unknown
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function isGraphNodeCandidate(value: unknown): value is Record<string, unknown> {
  if (!isRecord(value)) return false
  return typeof value.id === 'string'
    && typeof value.type === 'string'
}

function normalizeGraphNode(value: unknown): WorkflowGraphNode | null {
  if (!isGraphNodeCandidate(value)) return null
  return {
    ...value,
    id: value.id as string,
    type: value.type as string,
    config: isRecord(value.config) ? value.config : {},
    enabled: typeof value.enabled === 'boolean' ? value.enabled : true,
  }
}

function parseGraphJson(graphJson: string): WorkflowGraph {
  let parsed: unknown
  try {
    parsed = JSON.parse(graphJson)
  } catch {
    throw new Error('工作流图 JSON 解析失败')
  }

  if (!isRecord(parsed)) {
    throw new Error('工作流图格式无效')
  }

  const { nodes, edges } = parsed
  if (!Array.isArray(nodes) || !Array.isArray(edges)) {
    throw new Error('工作流图格式无效')
  }

  const normalizedNodes = nodes
    .map((node) => normalizeGraphNode(node))
    .filter((node): node is WorkflowGraphNode => node !== null)

  return { nodes: normalizedNodes, edges }
}

function normalizeFolderIds(raw: unknown): string[] {
  if (!Array.isArray(raw)) return []
  return raw.filter((item): item is string => typeof item === 'string')
}

function isEnabledFolderPicker(node: WorkflowGraphNode): boolean {
  return node.type === 'folder-picker' && node.enabled
}

export interface FolderPickerLaunchCheckResult {
  enabledPickerCount: number
  initialSelectedFolderIds: string[]
}

export function checkLaunchableFolderPickers(graphJson: string): FolderPickerLaunchCheckResult {
  const graph = parseGraphJson(graphJson)
  const enabledPickers = graph.nodes.filter(isEnabledFolderPicker)

  const initialSelectedFolderIds = enabledPickers.length > 0
    ? normalizeFolderIds((enabledPickers[0].config as FolderPickerConfig | undefined)?.saved_folder_ids)
    : []

  return {
    enabledPickerCount: enabledPickers.length,
    initialSelectedFolderIds,
  }
}

export function applyFolderSelectionToEnabledPickers(graphJson: string, selectedFolderIds: string[]): string {
  const graph = parseGraphJson(graphJson)
  const enabledPickers = graph.nodes.filter(isEnabledFolderPicker)
  if (enabledPickers.length === 0) {
    throw new Error('该工作流缺少文件夹选择器节点，无法直接启动')
  }

  const normalizedFolderIds = normalizeFolderIds(selectedFolderIds)
  if (normalizedFolderIds.length === 0) {
    throw new Error('请至少选择一条文件夹记录')
  }

  const nextNodes = graph.nodes.map((node) => {
    if (!isEnabledFolderPicker(node)) return node
    const currentConfig = isRecord(node.config) ? node.config : {}
    return {
      ...node,
      config: {
        ...currentConfig,
        source_mode: 'folders',
        saved_folder_ids: normalizedFolderIds,
        folder_ids: normalizedFolderIds,
        paths: [],
      },
    }
  })

  return JSON.stringify({
    nodes: nextNodes,
    edges: graph.edges,
  })
}
