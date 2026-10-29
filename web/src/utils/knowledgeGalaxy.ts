import { parseExtensions, extString, type EntityFilterKey } from '@/utils/knowledgeLibrary'
import { hasSemanticTags4D } from '@/utils/semantic4d'

export interface GalaxyInputNode {
  id: string
  title: string
  embedding?: number[]
  extensions?: Record<string, unknown>
  /** Relative content volume for star sizing (chunk count, text length, etc.). */
  contentWeight?: number
}

export interface GalaxyLayoutNode {
  id: string
  title: string
  x: number
  y: number
  group: string
  hasEmbedding: boolean
  hasSemantic4D: boolean
  radius: number
  contentWeight: number
  clusterId: number
}

export interface GalaxyEdge {
  source: string
  target: string
  similarity: number
}

export interface GalaxyLayout {
  nodes: GalaxyLayoutNode[]
  edges: GalaxyEdge[]
  bounds: { minX: number; maxX: number; minY: number; maxY: number }
  clusterCount: number
}

/** Min cosine similarity to draw an edge. Chained connections start from content-heaviest nodes. */
export const GALAXY_EDGE_THRESHOLD = 0.55

/**
 * 星系世界坐标相对屏幕的密度倍数。画布「100%」对应 viewScale={@link GALAXY_VIEW_BASE_SCALE}，
 * 布局尺寸按本系数放大，避免 CSS scale 放大超过 1 时 SVG 发糊。
 */
export const GALAXY_LAYOUT_DENSITY = 4

/** 星系模式 UI 100% 对应的内部 viewScale（= 1 / density）。 */
export const GALAXY_VIEW_BASE_SCALE = 1 / GALAXY_LAYOUT_DENSITY

const G = GALAXY_LAYOUT_DENSITY

/** Edge-to-edge minimum gap between any two stars (world px). */
export const GALAXY_MIN_STAR_GAP = 20 * G
/** Star radius bounds (world px). */
export const GALAXY_MIN_STAR_RADIUS = 4 * G
export const GALAXY_MAX_STAR_RADIUS = 11 * G
/** Center distance for linked stars at max / min similarity. */
export const GALAXY_MIN_LINK_DIST = 28 * G
export const GALAXY_MAX_LINK_DIST = 72 * G
/** Minimum gap between semantic cluster bounding boxes (world px). */
export const GALAXY_MIN_CLUSTER_GAP = 160 * G
export const GALAXY_CLUSTER_PADDING = 32 * G
/** Active node ring padding beyond dot radius (world px). */
export const GALAXY_ACTIVE_RING_PAD = 4 * G
/** Label offset above dot (world px). */
export const GALAXY_LABEL_OFFSET = 8 * G

/** Orbit ring radii (world px): ring 1 at base, each subsequent ring +step further. */
export const GALAXY_ORBIT_RING_BASE = 140 * G
export const GALAXY_ORBIT_RING_STEP = 110 * G

/** 将内部 viewScale 转为 UI 百分比（星系 100% = base）。 */
export function galaxyViewScaleToPercent(internal: number): number {
  return Math.round((internal / GALAXY_VIEW_BASE_SCALE) * 100)
}

/** 旧版以 viewScale≈1 为 UI 100% 的会话迁移到星系基准（已 ≤ base 的视为新格式）。 */
export function migrateGalaxyViewScale(internal: number): number {
  const s = Number.isFinite(internal) ? internal : GALAXY_VIEW_BASE_SCALE
  if (s > GALAXY_VIEW_BASE_SCALE * 1.2) return s * GALAXY_VIEW_BASE_SCALE
  return s
}

export const GALAXY_THEME = {
  node: '#AEAEB2',
  nodeActive: '#636366',
  edge: 'rgba(60, 60, 67, 0.45)',
} as const

export function cosineSimilarity(a: number[], b: number[]): number {
  if (a.length === 0 || b.length === 0 || a.length !== b.length) return 0
  let dot = 0
  let na = 0
  let nb = 0
  for (let i = 0; i < a.length; i++) {
    dot += a[i] * b[i]
    na += a[i] * a[i]
    nb += b[i] * b[i]
  }
  if (na === 0 || nb === 0) return 0
  return dot / (Math.sqrt(na) * Math.sqrt(nb))
}

export function buildGalaxyEdges(nodes: GalaxyInputNode[], threshold = GALAXY_EDGE_THRESHOLD): GalaxyEdge[] {
  const edges: GalaxyEdge[] = []
  for (let i = 0; i < nodes.length; i++) {
    const ai = nodes[i]?.embedding
    if (!ai?.length) continue
    for (let j = i + 1; j < nodes.length; j++) {
      const bj = nodes[j]?.embedding
      if (!bj?.length) continue
      const sim = cosineSimilarity(ai, bj)
      if (sim >= threshold) {
        edges.push({ source: nodes[i]!.id, target: nodes[j]!.id, similarity: sim })
      }
    }
  }
  return edges
}

/** Chain connections: sort by contentWeight desc, each node connects to the most similar
 *  already-chained node. Forms a tree rooted at the heaviest node, no redundant edges. */
export function buildChainEdges(edges: GalaxyEdge[], nodes: GalaxyInputNode[]): GalaxyEdge[] {
  const adj = new Map<string, { target: string; similarity: number }[]>()
  for (const e of edges) {
    if (!adj.has(e.source)) adj.set(e.source, [])
    if (!adj.has(e.target)) adj.set(e.target, [])
    adj.get(e.source)!.push({ target: e.target, similarity: e.similarity })
    adj.get(e.target)!.push({ target: e.source, similarity: e.similarity })
  }

  // Sort nodes by contentWeight descending; heaviest nodes anchor the chain
  const sorted = [...nodes].sort((a, b) => (b.contentWeight ?? 0) - (a.contentWeight ?? 0))
  const processed = new Set<string>()
  const chainEdges: GalaxyEdge[] = []
  const edgeSet = new Set<string>()

  for (const node of sorted) {
    if (processed.size === 0) {
      processed.add(node.id)
      continue
    }
    const neighbors = adj.get(node.id) ?? []
    let best: { target: string; similarity: number } | null = null
    for (const n of neighbors) {
      if (processed.has(n.target) && (!best || n.similarity > best.similarity)) {
        best = n
      }
    }
    if (best) {
      const key = node.id < best.target ? `${node.id}::${best.target}` : `${best.target}::${node.id}`
      if (!edgeSet.has(key)) {
        edgeSet.add(key)
        chainEdges.push({ source: node.id, target: best.target, similarity: best.similarity })
      }
    }
    processed.add(node.id)
  }
  return chainEdges
}


function groupLabel(extensions?: Record<string, unknown>): string {
  const ext = parseExtensions(extensions)
  return extString(ext, 'subject' as EntityFilterKey) || extString(ext, 'doc_type' as EntityFilterKey) || 'default'
}

export function starRadiusFromWeight(weight: number, minW: number, maxW: number): number {
  if (!Number.isFinite(weight) || weight <= 0) return GALAXY_MIN_STAR_RADIUS
  if (maxW <= minW) return (GALAXY_MIN_STAR_RADIUS + GALAXY_MAX_STAR_RADIUS) / 2
  const t = Math.sqrt((weight - minW) / (maxW - minW))
  return GALAXY_MIN_STAR_RADIUS + t * (GALAXY_MAX_STAR_RADIUS - GALAXY_MIN_STAR_RADIUS)
}

export function linkTargetDistance(similarity: number, rA: number, rB: number): number {
  const span = 1 - GALAXY_EDGE_THRESHOLD
  const t = span > 0 ? (similarity - GALAXY_EDGE_THRESHOLD) / span : 1
  const clamped = Math.max(0, Math.min(1, t))
  const dist = GALAXY_MAX_LINK_DIST - clamped * (GALAXY_MAX_LINK_DIST - GALAXY_MIN_LINK_DIST)
  const floor = rA + rB + GALAXY_MIN_STAR_GAP
  return Math.max(floor, Math.min(GALAXY_MAX_LINK_DIST, dist))
}

function minCenterDistance(rA: number, rB: number): number {
  return rA + rB + GALAXY_MIN_STAR_GAP
}

class UnionFind {
  private parent: number[]
  constructor(n: number) {
    this.parent = Array.from({ length: n }, (_, i) => i)
  }
  find(x: number): number {
    if (this.parent[x] !== x) this.parent[x] = this.find(this.parent[x]!)
    return this.parent[x]!
  }
  union(a: number, b: number) {
    const ra = this.find(a)
    const rb = this.find(b)
    if (ra !== rb) this.parent[rb] = ra
  }
}

function clusterByEdges(n: number, edges: GalaxyEdge[], idToIndex: Map<string, number>): number[] {
  const uf = new UnionFind(n)
  for (const e of edges) {
    const i = idToIndex.get(e.source)
    const j = idToIndex.get(e.target)
    if (i != null && j != null) uf.union(i, j)
  }
  const rootToCluster = new Map<number, number>()
  const clusterOf = Array.from({ length: n }, () => 0)
  let nextCluster = 0
  for (let i = 0; i < n; i++) {
    const root = uf.find(i)
    let cid = rootToCluster.get(root)
    if (cid == null) {
      cid = nextCluster++
      rootToCluster.set(root, cid)
    }
    clusterOf[i] = cid
  }
  return clusterOf
}

interface Vec2 {
  x: number
  y: number
}

function layoutCluster(
  memberIndices: number[],
  radii: number[],
  edges: GalaxyEdge[],
  idToIndex: Map<string, number>,
): Vec2[] {
  const m = memberIndices.length
  const local = memberIndices.map((_, localIdx) => {
    const angle = (localIdx / Math.max(m, 1)) * Math.PI * 2
    const ring = Math.min(72, 22 + m * 5)
    return {
      x: Math.cos(angle) * ring,
      y: Math.sin(angle) * ring,
    }
  })

  const localIndex = new Map<number, number>()
  memberIndices.forEach((g, li) => localIndex.set(g, li))

  const clusterEdges = edges.filter((e) => {
    const i = idToIndex.get(e.source)
    const j = idToIndex.get(e.target)
    return i != null && j != null && localIndex.has(i) && localIndex.has(j)
  })

  const iterations = Math.min(180, 80 + m * 8)
  for (let iter = 0; iter < iterations; iter++) {
    const damp = 0.82 - (iter / iterations) * 0.28

    for (let a = 0; a < m; a++) {
      for (let b = a + 1; b < m; b++) {
        const gi = memberIndices[a]!
        const gj = memberIndices[b]!
        const dx = local[b]!.x - local[a]!.x
        const dy = local[b]!.y - local[a]!.y
        const dist = Math.hypot(dx, dy) || 0.01
        const minDist = minCenterDistance(radii[gi]!, radii[gj]!)
        if (dist < minDist) {
          const push = ((minDist - dist) / dist) * 0.55
          const fx = dx * push
          const fy = dy * push
          local[a]!.x -= fx
          local[a]!.y -= fy
          local[b]!.x += fx
          local[b]!.y += fy
        }
        const repulse = 260 / (dist * dist)
        const fx = (dx / dist) * repulse
        const fy = (dy / dist) * repulse
        local[a]!.x -= fx
        local[a]!.y -= fy
        local[b]!.x += fx
        local[b]!.y += fy
      }
    }

    for (const e of clusterEdges) {
      const i = localIndex.get(idToIndex.get(e.source)!)
      const j = localIndex.get(idToIndex.get(e.target)!)
      if (i == null || j == null) continue
      const gi = memberIndices[i]!
      const gj = memberIndices[j]!
      const dx = local[j]!.x - local[i]!.x
      const dy = local[j]!.y - local[i]!.y
      const dist = Math.hypot(dx, dy) || 0.01
      const target = linkTargetDistance(e.similarity, radii[gi]!, radii[gj]!)
      const pull = (dist - target) * 0.07 * (0.55 + e.similarity * 0.45)
      const fx = (dx / dist) * pull
      const fy = (dy / dist) * pull
      local[i]!.x += fx
      local[i]!.y += fy
      local[j]!.x -= fx
      local[j]!.y -= fy
    }

    for (const p of local) {
      p.x *= damp
      p.y *= damp
    }
  }

  enforceMinGap(local, memberIndices, radii, 6)
  return local
}

function enforceMinGap(local: Vec2[], memberIndices: number[], radii: number[], passes: number) {
  const m = local.length
  for (let pass = 0; pass < passes; pass++) {
    for (let a = 0; a < m; a++) {
      for (let b = a + 1; b < m; b++) {
        const gi = memberIndices[a]!
        const gj = memberIndices[b]!
        const dx = local[b]!.x - local[a]!.x
        const dy = local[b]!.y - local[a]!.y
        const dist = Math.hypot(dx, dy) || 0.01
        const minDist = minCenterDistance(radii[gi]!, radii[gj]!)
        if (dist < minDist) {
          const push = (minDist - dist) / 2
          const ux = dx / dist
          const uy = dy / dist
          local[a]!.x -= ux * push
          local[a]!.y -= uy * push
          local[b]!.x += ux * push
          local[b]!.y += uy * push
        }
      }
    }
  }
}

function clusterBounds(local: Vec2[], memberIndices: number[], radii: number[]) {
  let minX = Infinity
  let maxX = -Infinity
  let minY = Infinity
  let maxY = -Infinity
  local.forEach((p, li) => {
    const r = radii[memberIndices[li]!]!
    minX = Math.min(minX, p.x - r)
    maxX = Math.max(maxX, p.x + r)
    minY = Math.min(minY, p.y - r)
    maxY = Math.max(maxY, p.y + r)
  })
  return { minX, maxX, minY, maxY, w: maxX - minX, h: maxY - minY }
}

/** Cluster-aware force layout: wide inter-cluster gaps, similarity-scaled intra-cluster links. */
export function layoutGalaxy(nodes: GalaxyInputNode[], edges: GalaxyEdge[]): GalaxyLayout {
  const n = nodes.length
  if (n === 0) {
    return {
      nodes: [],
      edges,
      bounds: { minX: 0, maxX: 900, minY: 0, maxY: 620 },
      clusterCount: 0,
    }
  }

  const weights = nodes.map((node) => Math.max(1, node.contentWeight ?? 1))
  const minW = Math.min(...weights)
  const maxW = Math.max(...weights)
  const radii = weights.map((w) => starRadiusFromWeight(w, minW, maxW))

  const idToIndex = new Map(nodes.map((node, i) => [node.id, i]))
  const clusterOf = clusterByEdges(n, edges, idToIndex)
  const clusterCount = clusterOf.length ? Math.max(...clusterOf) + 1 : 1

  const membersByCluster = Array.from({ length: clusterCount }, () => [] as number[])
  for (let i = 0; i < n; i++) membersByCluster[clusterOf[i]!]!.push(i)

  const clusterLayouts: { local: Vec2[]; members: number[]; bounds: ReturnType<typeof clusterBounds> }[] = []
  for (let cid = 0; cid < clusterCount; cid++) {
    const members = membersByCluster[cid]!
    if (members.length === 0) continue
    const local =
      members.length === 1
        ? [{ x: 0, y: 0 }]
        : layoutCluster(members, radii, edges, idToIndex)
    clusterLayouts.push({
      local,
      members,
      bounds: clusterBounds(local, members, radii),
    })
  }

  const cols = Math.max(1, Math.ceil(Math.sqrt(clusterLayouts.length)))
  const clusterPositions: Vec2[] = []

  for (let i = 0; i < clusterLayouts.length; i++) {
    const col = i % cols
    const row = Math.floor(i / cols)
    const { bounds } = clusterLayouts[i]!
    const w = bounds.w + GALAXY_CLUSTER_PADDING * 2
    const h = bounds.h + GALAXY_CLUSTER_PADDING * 2
    let cx = 0
    for (let c = 0; c < col; c++) {
      const idx = row * cols + c
      const prevBounds = clusterLayouts[idx]!.bounds
      cx += prevBounds.w + GALAXY_CLUSTER_PADDING * 2 + GALAXY_MIN_CLUSTER_GAP
    }
    let cy = 0
    for (let r = 0; r < row; r++) {
      let rowH = 0
      for (let c = 0; c < cols; c++) {
        const idx = r * cols + c
        if (idx >= clusterLayouts.length) break
        const b = clusterLayouts[idx]!.bounds
        rowH = Math.max(rowH, b.h + GALAXY_CLUSTER_PADDING * 2)
      }
      cy += rowH + GALAXY_MIN_CLUSTER_GAP
    }
    clusterPositions.push({ x: cx + w / 2, y: cy + h / 2 })
  }

  const layoutNodes: GalaxyLayoutNode[] = []
  for (let ci = 0; ci < clusterLayouts.length; ci++) {
    const { local, members } = clusterLayouts[ci]!
    const { bounds } = clusterLayouts[ci]!
    const anchor = clusterPositions[ci]!
    const offsetX = anchor.x - (bounds.minX + bounds.maxX) / 2
    const offsetY = anchor.y - (bounds.minY + bounds.maxY) / 2

    members.forEach((globalIdx, li) => {
      const node = nodes[globalIdx]!
      layoutNodes.push({
        id: node.id,
        title: node.title,
        x: local[li]!.x + offsetX,
        y: local[li]!.y + offsetY,
        group: groupLabel(node.extensions),
        hasEmbedding: Boolean(node.embedding?.length),
        hasSemantic4D: hasSemanticTags4D(parseExtensions(node.extensions)),
        radius: radii[globalIdx]!,
        contentWeight: weights[globalIdx]!,
        clusterId: clusterOf[globalIdx]!,
      })
    })
  }

  let minX = Infinity
  let maxX = -Infinity
  let minY = Infinity
  let maxY = -Infinity
  for (const p of layoutNodes) {
    minX = Math.min(minX, p.x - p.radius)
    maxX = Math.max(maxX, p.x + p.radius)
    minY = Math.min(minY, p.y - p.radius)
    maxY = Math.max(maxY, p.y + p.radius)
  }
  const pad = 28 * G
  return {
    nodes: layoutNodes,
    edges,
    bounds: {
      minX: minX - pad,
      maxX: maxX + pad,
      minY: minY - pad,
      maxY: maxY + pad,
    },
    clusterCount: clusterLayouts.length,
  }
}

/** Heliocentric orbit layout: heaviest node at center (level 0), others in concentric
 *  rings by BFS distance. Each ring distributes nodes evenly. */
export function layoutOrbit(nodes: GalaxyInputNode[], edges: GalaxyEdge[]): GalaxyLayout {
  const n = nodes.length
  if (n === 0) {
    return { nodes: [], edges, bounds: { minX: 0, maxX: 900, minY: 0, maxY: 620 }, clusterCount: 0 }
  }

  const sorted = nodes.map((node, i) => ({ node, origIdx: i }))
  sorted.sort((a, b) => (b.node.contentWeight ?? 0) - (a.node.contentWeight ?? 0))

  const weights = sorted.map(s => Math.max(1, s.node.contentWeight ?? 1))
  const minW = Math.min(...weights)
  const maxW = Math.max(...weights)
  const radii = weights.map(w => starRadiusFromWeight(w, minW, maxW))

  const adj = new Map<string, string[]>()
  for (const s of sorted) adj.set(s.node.id, [])
  for (const e of edges) {
    adj.get(e.source)?.push(e.target)
    adj.get(e.target)?.push(e.source)
  }

  const centerId = sorted[0]!.node.id
  const levelOf = new Map<string, number>()
  const queue = [centerId]
  levelOf.set(centerId, 0)
  while (queue.length > 0) {
    const cur = queue.shift()!
    const lvl = levelOf.get(cur)!
    for (const nb of adj.get(cur) ?? []) {
      if (!levelOf.has(nb)) { levelOf.set(nb, lvl + 1); queue.push(nb) }
    }
  }
  const maxLevel = levelOf.size > 0 ? Math.max(...levelOf.values()) : 0
  for (const s of sorted) {
    if (!levelOf.has(s.node.id)) levelOf.set(s.node.id, maxLevel + 1)
  }

  const byLevel = new Map<number, number[]>()
  for (let i = 0; i < sorted.length; i++) {
    const lvl = levelOf.get(sorted[i]!.node.id)!
    if (!byLevel.has(lvl)) byLevel.set(lvl, [])
    byLevel.get(lvl)!.push(i)
  }

  const layoutNodes: GalaxyLayoutNode[] = []
  const ringPhases = new Map<number, number>()
  for (const [level] of byLevel) ringPhases.set(level, Math.random() * Math.PI * 2)

  for (const [level, indices] of byLevel) {
    const ringR = level === 0 ? 0 : GALAXY_ORBIT_RING_BASE + (level - 1) * GALAXY_ORBIT_RING_STEP
    const count = indices.length
    const phase = ringPhases.get(level)!

    for (let j = 0; j < count; j++) {
      const si = indices[j]!
      const { node: nd } = sorted[si]!
      const angle = level === 0 ? 0 : (j / count) * Math.PI * 2 + phase
      layoutNodes.push({
        id: nd.id,
        title: nd.title,
        x: ringR * Math.cos(angle),
        y: ringR * Math.sin(angle),
        group: groupLabel(nd.extensions),
        hasEmbedding: Boolean(nd.embedding?.length),
        hasSemantic4D: hasSemanticTags4D(parseExtensions(nd.extensions)),
        radius: radii[si]!,
        contentWeight: weights[si]!,
        clusterId: level,
      })
    }
  }

  // 基于轨道环半径计算边界，确保所有节点的完整轨道路径都在视口内
  let maxExtent = 0
  for (const [level, indices] of byLevel) {
    const ringR = level === 0 ? 0 : GALAXY_ORBIT_RING_BASE + (level - 1) * GALAXY_ORBIT_RING_STEP
    let maxNodeRadius = 0
    for (const idx of indices) {
      maxNodeRadius = Math.max(maxNodeRadius, radii[idx]!)
    }
    maxExtent = Math.max(maxExtent, ringR + maxNodeRadius * 2)
  }
  const pad = 40 * G
  return {
    nodes: layoutNodes,
    edges,
    bounds: {
      minX: -maxExtent - pad,
      maxX: maxExtent + pad,
      minY: -maxExtent - pad,
      maxY: maxExtent + pad,
    },
    clusterCount: byLevel.size,
  }
}

const GROUP_COLORS = ['#8E8E93', '#AEAEB2', '#636366', '#98989D', '#48484A', '#C7C7CC']

export function groupColor(group: string): string {
  if (group === 'default') return GALAXY_THEME.node
  let h = 0
  for (let i = 0; i < group.length; i++) h = (h * 31 + group.charCodeAt(i)) >>> 0
  return GROUP_COLORS[h % GROUP_COLORS.length]!
}

/** Gray edge opacity scales with similarity; color stays neutral. */
export function edgeStrokeColor(_similarity: number): string {
  return GALAXY_THEME.edge
}

export function buildGalaxyLayout(nodes: GalaxyInputNode[]): GalaxyLayout {
  const edges = buildGalaxyEdges(nodes)
  const chained = buildChainEdges(edges, nodes)
  return layoutOrbit(nodes, chained)
}

