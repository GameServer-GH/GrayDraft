import type { NotebookMeta } from '@/types/notebook'
import type { KnowledgeGraphNode } from '@/types/knowledge'
import {
  buildGalaxyEdges,
  buildGalaxyLayout,
  GALAXY_MIN_CLUSTER_GAP,
  GALAXY_LAYOUT_DENSITY,
  layoutGalaxy,
  linkTargetDistance,
  starRadiusFromWeight,
  type GalaxyEdge,
  type GalaxyInputNode,
  type GalaxyLayout,
  type GalaxyLayoutNode,
} from '@/utils/knowledgeGalaxy'
import { parseExtensions, extString, type EntityFilterKey } from '@/utils/knowledgeLibrary'
import { hasSemanticTags4D } from '@/utils/semantic4d'

const G = GALAXY_LAYOUT_DENSITY

export type GalaxySyncOp =
  | { type: 'full' }
  | { type: 'add'; ids: string[] }
  | { type: 'remove'; ids: string[] }
  /** 更新单节点元数据（标题、分组、权重）；可选重拉 embedding 并重算关联边 */
  | { type: 'update'; id: string; refetchEmbedding?: boolean }
  /** 仅同步列表元数据，不请求图谱 API */
  | { type: 'metadata'; ids?: string[] }
  /** 索引完成后刷新 embedding 与关联 */
  | { type: 'reindex'; ids: string[] }
  /** 批量执行 */
  | { type: 'batch'; ops: GalaxySyncOp[] }
  /** 对比左侧列表与当前图谱缓存，自动选择增量策略 */
  | { type: 'list-sync' }

function galaxyGroupLabel(extensions?: Record<string, unknown>): string {
  const ext = parseExtensions(extensions)
  return extString(ext, 'subject' as EntityFilterKey) || extString(ext, 'doc_type' as EntityFilterKey) || 'default'
}

export function contentWeightFromMeta(meta: NotebookMeta): number {
  return Math.max(1, (meta.chunkCount ?? 1) + Math.floor((meta.preview?.length ?? 0) / 400))
}

export function notebookGalaxySignature(meta: NotebookMeta): string {
  const ext = meta.extensions ? JSON.stringify(meta.extensions) : ''
  return `${meta.title}\0${meta.chunkCount ?? 0}\0${meta.indexReady ? 1 : 0}\0${ext}`
}

export function graphNodeToInput(
  n: KnowledgeGraphNode,
  weightById: Map<string, number>,
): GalaxyInputNode {
  return {
    id: n.id,
    title: n.title,
    embedding: n.embedding,
    extensions: n.extensions,
    contentWeight: weightById.get(n.id) ?? Math.max(1, Math.ceil(n.title.length / 12)),
  }
}

export function inputFromNotebookMeta(meta: NotebookMeta): GalaxyInputNode {
  return {
    id: meta.id,
    title: meta.title,
    extensions: meta.extensions,
    contentWeight: contentWeightFromMeta(meta),
  }
}

function edgeKey(e: GalaxyEdge): string {
  const a = e.source < e.target ? e.source : e.target
  const b = e.source < e.target ? e.target : e.source
  return `${a}|${b}`
}

export function mergeGalaxyEdges(existing: GalaxyEdge[], incoming: GalaxyEdge[]): GalaxyEdge[] {
  const map = new Map<string, GalaxyEdge>()
  for (const e of existing) map.set(edgeKey(e), e)
  for (const e of incoming) {
    const k = edgeKey(e)
    const prev = map.get(k)
    if (!prev || e.similarity > prev.similarity) map.set(k, e)
  }
  return [...map.values()]
}

export function buildEdgesForNode(node: GalaxyInputNode, all: GalaxyInputNode[]): GalaxyEdge[] {
  return buildGalaxyEdges([node, ...all.filter((n) => n.id !== node.id)]).filter(
    (e) => e.source === node.id || e.target === node.id,
  )
}

export function recomputeGalaxyBounds(nodes: GalaxyLayoutNode[]): GalaxyLayout['bounds'] {
  if (!nodes.length) {
    return { minX: 0, maxX: 900 * G, minY: 0, maxY: 620 * G }
  }
  let minX = Infinity
  let maxX = -Infinity
  let minY = Infinity
  let maxY = -Infinity
  for (const p of nodes) {
    minX = Math.min(minX, p.x - p.radius)
    maxX = Math.max(maxX, p.x + p.radius)
    minY = Math.min(minY, p.y - p.radius)
    maxY = Math.max(maxY, p.y + p.radius)
  }
  const pad = 28 * G
  return { minX: minX - pad, maxX: maxX + pad, minY: minY - pad, maxY: maxY + pad }
}

function weightRange(nodes: GalaxyLayoutNode[]) {
  const weights = nodes.map((n) => n.contentWeight)
  return { minW: Math.min(...weights), maxW: Math.max(...weights) }
}

function patchLayoutNodeFromInput(
  layoutNode: GalaxyLayoutNode,
  input: GalaxyInputNode,
  minW: number,
  maxW: number,
) {
  layoutNode.title = input.title
  layoutNode.group = galaxyGroupLabel(input.extensions)
  layoutNode.contentWeight = Math.max(1, input.contentWeight ?? 1)
  layoutNode.radius = starRadiusFromWeight(layoutNode.contentWeight, minW, maxW)
  layoutNode.hasEmbedding = Boolean(input.embedding?.length)
  layoutNode.hasSemantic4D = hasSemanticTags4D(parseExtensions(input.extensions))
}

function hashAngle(id: string): number {
  let h = 0
  for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) >>> 0
  return ((h % 360) * Math.PI) / 180
}

function placeOneNode(
  input: GalaxyInputNode,
  layout: GalaxyLayout,
  edges: GalaxyEdge[],
  minW: number,
  maxW: number,
): GalaxyLayoutNode {
  const w = Math.max(1, input.contentWeight ?? 1)
  const r = starRadiusFromWeight(w, minW, maxW)
  const byId = new Map(layout.nodes.map((n) => [n.id, n]))
  const related = edges.filter((e) => e.source === input.id || e.target === input.id)

  if (related.length) {
    const best = [...related].sort((a, b) => b.similarity - a.similarity)[0]!
    const neighborId = best.source === input.id ? best.target : best.source
    const neighbor = byId.get(neighborId)
    if (neighbor) {
      const dist = linkTargetDistance(best.similarity, r, neighbor.radius)
      const ang = hashAngle(input.id)
      return {
        id: input.id,
        title: input.title,
        x: neighbor.x + Math.cos(ang) * dist,
        y: neighbor.y + Math.sin(ang) * dist,
        group: galaxyGroupLabel(input.extensions),
        hasEmbedding: Boolean(input.embedding?.length),
        hasSemantic4D: hasSemanticTags4D(parseExtensions(input.extensions)),
        radius: r,
        contentWeight: w,
        clusterId: neighbor.clusterId,
      }
    }
  }

  const cx = (layout.bounds.minX + layout.bounds.maxX) / 2
  const y = layout.bounds.maxY + GALAXY_MIN_CLUSTER_GAP
  return {
    id: input.id,
    title: input.title,
    x: cx,
    y,
    group: galaxyGroupLabel(input.extensions),
    hasEmbedding: Boolean(input.embedding?.length),
    hasSemantic4D: hasSemanticTags4D(parseExtensions(input.extensions)),
    radius: r,
    contentWeight: w,
    clusterId: layout.clusterCount,
  }
}

export function removeGalaxyNodes(
  layout: GalaxyLayout,
  inputNodes: GalaxyInputNode[],
  ids: string[],
): { layout: GalaxyLayout; inputNodes: GalaxyInputNode[] } {
  const drop = new Set(ids)
  const nextInputs = inputNodes.filter((n) => !drop.has(n.id))
  const nextNodes = layout.nodes.filter((n) => !drop.has(n.id))
  const nextEdges = layout.edges.filter((e) => !drop.has(e.source) && !drop.has(e.target))
  return {
    inputNodes: nextInputs,
    layout: {
      ...layout,
      nodes: nextNodes,
      edges: nextEdges,
      bounds: recomputeGalaxyBounds(nextNodes),
      clusterCount: layout.clusterCount,
    },
  }
}

export function addGalaxyNodes(
  layout: GalaxyLayout,
  inputNodes: GalaxyInputNode[],
  added: GalaxyInputNode[],
): { layout: GalaxyLayout; inputNodes: GalaxyInputNode[] } {
  const mergedInputs = [...inputNodes]
  const known = new Set(inputNodes.map((n) => n.id))
  for (const n of added) {
    if (!known.has(n.id)) {
      mergedInputs.push(n)
      known.add(n.id)
    }
  }
  const newEdges = added.flatMap((n) => buildEdgesForNode(n, mergedInputs))
  const edges = mergeGalaxyEdges(layout.edges, newEdges)
  const { minW, maxW } = weightRange([
    ...layout.nodes,
    ...added.map((n) => ({
      id: n.id,
      title: n.title,
      x: 0,
      y: 0,
      group: 'default',
      hasEmbedding: false,
      hasSemantic4D: hasSemanticTags4D(parseExtensions(n.extensions)),
      radius: 1,
      contentWeight: n.contentWeight ?? 1,
      clusterId: 0,
    })),
  ])
  const placed = added.map((n) => placeOneNode(n, layout, edges, minW, maxW))
  const nodes = [...layout.nodes, ...placed]
  return {
    inputNodes: mergedInputs,
    layout: {
      ...layout,
      nodes,
      edges,
      bounds: recomputeGalaxyBounds(nodes),
    },
  }
}

export function updateGalaxyNode(
  layout: GalaxyLayout,
  inputNodes: GalaxyInputNode[],
  id: string,
  nextInput: GalaxyInputNode,
  reposition: boolean,
): { layout: GalaxyLayout; inputNodes: GalaxyInputNode[] } {
  const mergedInputs = inputNodes.map((n) => (n.id === id ? nextInput : n))
  const edgesWithout = layout.edges.filter((e) => e.source !== id && e.target !== id)
  const nodeEdges = buildEdgesForNode(nextInput, mergedInputs)
  const edges = mergeGalaxyEdges(edgesWithout, nodeEdges)
  const nodes = layout.nodes.map((n) => {
    if (n.id !== id) return n
    const { minW, maxW } = weightRange(layout.nodes)
    const copy = { ...n }
    patchLayoutNodeFromInput(copy, nextInput, minW, maxW)
    return copy
  })
  let finalNodes = nodes
  if (reposition) {
    const { minW, maxW } = weightRange(nodes)
    const placed = placeOneNode(nextInput, { ...layout, nodes, edges }, edges, minW, maxW)
    finalNodes = nodes.map((n) => (n.id === id ? placed : n))
  }
  return {
    inputNodes: mergedInputs,
    layout: {
      ...layout,
      nodes: finalNodes,
      edges,
      bounds: recomputeGalaxyBounds(finalNodes),
    },
  }
}

export function patchGalaxyMetadata(
  layout: GalaxyLayout,
  inputNodes: GalaxyInputNode[],
  metas: NotebookMeta[],
): { layout: GalaxyLayout; inputNodes: GalaxyInputNode[] } {
  const metaById = new Map(metas.map((m) => [m.id, m]))
  const mergedInputs = inputNodes.map((n) => {
    const m = metaById.get(n.id)
    return m ? inputFromNotebookMeta(m) : n
  })
  const { minW, maxW } = weightRange(layout.nodes)
  const nodes = layout.nodes.map((ln) => {
    const input = mergedInputs.find((n) => n.id === ln.id)
    if (!input) return ln
    const copy = { ...ln }
    patchLayoutNodeFromInput(copy, input, minW, maxW)
    return copy
  })
  return { layout: { ...layout, nodes }, inputNodes: mergedInputs }
}

export function planListSync(
  notebooks: NotebookMeta[],
  inputIds: Set<string>,
  snapshots: Record<string, string>,
): GalaxySyncOp[] {
  const nbIds = new Set(notebooks.map((n) => n.id))
  const ops: GalaxySyncOp[] = []

  const removed = [...inputIds].filter((id) => !nbIds.has(id))
  if (removed.length) ops.push({ type: 'remove', ids: removed })

  const added = notebooks.filter((n) => !inputIds.has(n.id)).map((n) => n.id)
  if (added.length) ops.push({ type: 'add', ids: added })

  const reindexIds: string[] = []
  const metadataIds: string[] = []

  for (const n of notebooks) {
    if (!inputIds.has(n.id)) continue
    const sig = notebookGalaxySignature(n)
    const prev = snapshots[n.id]
    if (prev === sig) continue
    if (n.indexReady) reindexIds.push(n.id)
    else metadataIds.push(n.id)
  }

  if (reindexIds.length) ops.push({ type: 'reindex', ids: reindexIds })
  if (metadataIds.length) ops.push({ type: 'metadata', ids: metadataIds })

  return ops
}

export function applyFullLayout(inputs: GalaxyInputNode[]): GalaxyLayout {
  return buildGalaxyLayout(inputs)
}

/** 仅重算全局布局（节点集不变，结构大改时可用） */
export function relayoutGalaxy(inputs: GalaxyInputNode[], edges: GalaxyEdge[]): GalaxyLayout {
  return layoutGalaxy(inputs, edges)
}
