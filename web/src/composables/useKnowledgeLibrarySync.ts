import { useCanvasStore } from '@/stores/canvas'
import { useGalaxyGraphStore } from '@/stores/galaxyGraph'
import { useWorkspaceSessionStore } from '@/stores/workspaceSession'
import { listKnowledgeEntries, subscribeKnowledgeLibrary } from '@/services/knowledge'
import { listNotebooks } from '@/services/notebook'
import { formatNexusError } from '@/utils/nexusError'
import type { GalaxySyncOp } from '@/utils/galaxySync'
import type { NotebookMeta } from '@/types/notebook'

export type { GalaxySyncOp } from '@/utils/galaxySync'

/** `false` 表示只刷新左侧列表，不同步星系。 */
export type KnowledgeLibraryGalaxyOp = GalaxySyncOp | false

function isKnowledgeWorkspace(): boolean {
  return Boolean(useWorkspaceSessionStore().nexusKbType?.trim())
}

/**
 * 从服务端刷新左侧知识/笔记列表，并在知识库模式下同步星系图。
 * 所有 mutation 接口成功后应调用本函数（或下方语义化封装）。
 */
export type RefreshKnowledgeLibraryOpts = {
  /** 为 true 时只同步星系图，不重复请求 ListKnowledgeEntries（工作区刚打开时列表已在 openWithKey 拉过） */
  skipList?: boolean
}

export async function refreshKnowledgeLibrary(
  galaxyOp: KnowledgeLibraryGalaxyOp = { type: 'list-sync' },
  opts?: RefreshKnowledgeLibraryOpts,
): Promise<NotebookMeta[]> {
  const canvas = useCanvasStore()
  const galaxy = useGalaxyGraphStore()
  const knowledgeMode = isKnowledgeWorkspace()

  try {
    let list: NotebookMeta[]
    if (opts?.skipList && canvas.notebooks.length > 0) {
      list = canvas.notebooks
    } else {
      list = knowledgeMode ? await listKnowledgeEntries() : await listNotebooks()
      canvas.setNotebooks(list)
    }
    canvas.setNotebookLoadError(null)
    if (knowledgeMode && galaxyOp !== false) {
      await galaxy.syncWhenReady(galaxyOp)
    }
    return list
  } catch (e) {
    canvas.setNotebooks([])
    canvas.setNotebookLoadError(formatNexusError(e, { nexusConfigured: knowledgeMode }))
    throw e
  }
}

/** 新建条目入库成功后 */
export async function afterKnowledgeAdded(docId?: string | null) {
  return refreshKnowledgeLibrary(
    docId ? { type: 'add', ids: [docId] } : { type: 'list-sync' },
  )
}

/** 删除条目成功后 */
export async function afterKnowledgeRemoved(id: string) {
  return refreshKnowledgeLibrary({ type: 'remove', ids: [id] })
}

/** 更新标题/内容成功后 */
export async function afterKnowledgeUpdated(id: string, refetchEmbedding = false) {
  return refreshKnowledgeLibrary({ type: 'update', id, refetchEmbedding })
}

/** 批量变更（智能导入、多文档）：全量重拉星系图以反映新节点与边 */
export async function afterKnowledgeBulkChange() {
  return refreshKnowledgeLibrary({ type: 'full' })
}

/** 清空知识库数据后 */
export async function afterKnowledgeDataCleared() {
  return refreshKnowledgeLibrary({ type: 'full' })
}

/** 启动 SSE 订阅，自动在知识库变更时刷新列表和星系图 */
export function useKnowledgeLibrarySSE(enabled: () => boolean) {
    let cleanup: (() => void) | null = null

    function start() {
        if (!enabled()) return
        stop()
        cleanup = subscribeKnowledgeLibrary(async (ev) => {
            if (ev.type === 'library-changed') {
                try {
                    await refreshKnowledgeLibrary({ type: 'full' })
                } catch {
                    /* refresh handles error display */
                }
            }
        })
    }

    function stop() {
        if (cleanup) {
            cleanup()
            cleanup = null
        }
    }

    return { start, stop }
}
