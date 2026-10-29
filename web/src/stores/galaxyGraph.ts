import { ref } from 'vue'
import { defineStore } from 'pinia'
import { getKnowledgeGraph } from '@/services/knowledge'
import {
  addGalaxyNodes,
  applyFullLayout,
  contentWeightFromMeta,
  graphNodeToInput,
  inputFromNotebookMeta,
  notebookGalaxySignature,
  patchGalaxyMetadata,
  planListSync,
  removeGalaxyNodes,
  updateGalaxyNode,
  type GalaxySyncOp,
} from '@/utils/galaxySync'
import type { GalaxyInputNode, GalaxyLayout } from '@/utils/knowledgeGalaxy'
import { formatNexusError } from '@/utils/nexusError'
import { useCanvasStore } from '@/stores/canvas'

const GRAPH_LIMIT = 120

export const useGalaxyGraphStore = defineStore('galaxyGraph', () => {
  const active = ref(false)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const layout = ref<GalaxyLayout | null>(null)
  const inputNodes = ref<GalaxyInputNode[]>([])
  const nodeCount = ref(0)
  const edgeCount = ref(0)
  const metaSnapshots = ref<Record<string, string>>({})
  const pendingSync = ref<GalaxySyncOp | null>(null)
  let loadFullInflight: Promise<void> | null = null
  /** 工作区切换时递增，丢弃过期的 loadFull 结果 */
  let loadGeneration = 0

  /** 切换/离开工作区时清空星系图，避免展示上一工作区数据 */
  function resetForWorkspaceChange() {
    loadGeneration++
    loadFullInflight = null
    loading.value = false
    layout.value = null
    inputNodes.value = []
    nodeCount.value = 0
    edgeCount.value = 0
    metaSnapshots.value = {}
    error.value = null
    pendingSync.value = null
  }

  function setActive(on: boolean) {
    active.value = on
    if (!on) {
      layout.value = null
      inputNodes.value = []
      nodeCount.value = 0
      edgeCount.value = 0
      metaSnapshots.value = {}
      error.value = null
      pendingSync.value = null
      return
    }
    void flushPendingSync()
  }

  async function flushPendingSync() {
    if (!active.value) return
    const op = pendingSync.value
    if (op) {
      pendingSync.value = null
      await sync(op)
      return
    }
    if (!layout.value && !loading.value) {
      await loadFull()
    }
  }

  /** 列表已更新但画布尚未挂载时排队；激活后由 {@link flushPendingSync} 执行。 */
  async function syncWhenReady(op: GalaxySyncOp) {
    if (!active.value) {
      const prev = pendingSync.value
      if (op.type === 'full' || prev?.type !== 'full') {
        pendingSync.value = op
      }
      return
    }
    await sync(op)
  }

  function weightByIdFromCanvas(): Map<string, number> {
    const canvas = useCanvasStore()
    return new Map(canvas.notebooks.map((n) => [n.id, contentWeightFromMeta(n)]))
  }

  function refreshSnapshots() {
    const canvas = useCanvasStore()
    const next: Record<string, string> = {}
    for (const n of canvas.notebooks) {
      next[n.id] = notebookGalaxySignature(n)
    }
    metaSnapshots.value = next
  }

  function applyCounts() {
    nodeCount.value = inputNodes.value.length
    edgeCount.value = layout.value?.edges.length ?? 0
  }

  async function fetchGraphInputs(ids?: string[]): Promise<GalaxyInputNode[]> {
    const res = await getKnowledgeGraph({ limit: GRAPH_LIMIT })
    const want = ids ? new Set(ids) : null
    const weights = weightByIdFromCanvas()
    return (res.nodes ?? [])
      .filter((n) => !want || want.has(n.id))
      .map((n) => graphNodeToInput(n, weights))
  }

  async function loadFull() {
    const gen = loadGeneration
    if (loadFullInflight) {
      await loadFullInflight
      return
    }
    loadFullInflight = (async () => {
      loading.value = true
      error.value = null
      try {
        const nodes = await fetchGraphInputs()
        if (gen !== loadGeneration) return
        inputNodes.value = nodes
        layout.value = applyFullLayout(nodes)
        applyCounts()
        refreshSnapshots()
      } catch (e) {
        if (gen !== loadGeneration) return
        error.value = formatNexusError(e)
        layout.value = null
        inputNodes.value = []
        nodeCount.value = 0
        edgeCount.value = 0
      } finally {
        if (gen === loadGeneration) loading.value = false
      }
    })()
    try {
      await loadFullInflight
    } finally {
      if (gen === loadGeneration) loadFullInflight = null
    }
  }

  async function applyAdd(ids: string[]) {
    if (!active.value || !ids.length) return
    const unique = [...new Set(ids)]
    if (!layout.value) {
      await loadFull()
      return
    }
    const missing = unique.filter((id) => !inputNodes.value.some((n) => n.id === id))
    if (!missing.length) return
    const fetched = await fetchGraphInputs(missing)
    const canvas = useCanvasStore()
    for (const id of missing) {
      if (!fetched.some((n) => n.id === id)) {
        const meta = canvas.notebooks.find((n) => n.id === id)
        if (meta) fetched.push(inputFromNotebookMeta(meta))
      }
    }
    const result = addGalaxyNodes(layout.value, inputNodes.value, fetched)
    inputNodes.value = result.inputNodes
    layout.value = result.layout
    applyCounts()
    refreshSnapshots()
  }

  function applyRemove(ids: string[]) {
    if (!layout.value || !ids.length) return
    const result = removeGalaxyNodes(layout.value, inputNodes.value, ids)
    inputNodes.value = result.inputNodes
    layout.value = result.layout
    applyCounts()
    refreshSnapshots()
  }

  async function applyUpdate(id: string, refetchEmbedding: boolean) {
    if (!active.value) return
    if (!layout.value) {
      await loadFull()
      return
    }
    let next: GalaxyInputNode | undefined
    if (refetchEmbedding) {
      const fetched = await fetchGraphInputs([id])
      next = fetched[0]
    }
    if (!next) {
      const meta = useCanvasStore().notebooks.find((n) => n.id === id)
      if (meta) next = inputFromNotebookMeta(meta)
    }
    if (!next) return
    const cur = inputNodes.value.find((n) => n.id === id)
    if (cur?.embedding?.length && !refetchEmbedding) {
      next.embedding = cur.embedding
    }
    const result = updateGalaxyNode(layout.value, inputNodes.value, id, next, refetchEmbedding)
    inputNodes.value = result.inputNodes
    layout.value = result.layout
    applyCounts()
    refreshSnapshots()
  }

  function applyMetadata(ids?: string[]) {
    if (!layout.value) return
    const canvas = useCanvasStore()
    const metas = ids?.length
      ? canvas.notebooks.filter((n) => ids.includes(n.id))
      : canvas.notebooks
    const result = patchGalaxyMetadata(layout.value, inputNodes.value, metas)
    inputNodes.value = result.inputNodes
    layout.value = result.layout
    applyCounts()
    refreshSnapshots()
  }

  async function applyReindex(ids: string[]) {
    for (const id of ids) {
      await applyUpdate(id, true)
    }
  }

  async function sync(op: GalaxySyncOp) {
    if (!active.value) {
      pendingSync.value = op
      return
    }

    switch (op.type) {
      case 'full':
        await loadFull()
        return
      case 'batch':
        for (const child of op.ops) await sync(child)
        return
      case 'list-sync': {
        const canvas = useCanvasStore()
        const inputIds = new Set(inputNodes.value.map((n) => n.id))
        const ops = planListSync(canvas.notebooks, inputIds, metaSnapshots.value)
        if (!layout.value && !loading.value) {
          await loadFull()
          if (!ops.length) return
        }
        if (!ops.length) {
          refreshSnapshots()
          return
        }
        await sync({ type: 'batch', ops })
        return
      }
      case 'add':
        loading.value = true
        error.value = null
        try {
          await applyAdd(op.ids)
        } catch (e) {
          error.value = formatNexusError(e)
        } finally {
          loading.value = false
        }
        return
      case 'remove':
        applyRemove(op.ids)
        return
      case 'update':
        loading.value = true
        error.value = null
        try {
          await applyUpdate(op.id, Boolean(op.refetchEmbedding))
        } catch (e) {
          error.value = formatNexusError(e)
        } finally {
          loading.value = false
        }
        return
      case 'metadata':
        applyMetadata(op.ids)
        return
      case 'reindex':
        loading.value = true
        error.value = null
        try {
          await applyReindex(op.ids)
        } catch (e) {
          error.value = formatNexusError(e)
        } finally {
          loading.value = false
        }
        return
      default:
        return
    }
  }

  return {
    active,
    loading,
    error,
    layout,
    inputNodes,
    nodeCount,
    edgeCount,
    setActive,
    resetForWorkspaceChange,
    loadFull,
    sync,
    syncWhenReady,
    reload: () => sync({ type: 'full' }),
  }
})
