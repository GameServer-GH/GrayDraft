import { computed, ref, watch } from 'vue'
import { useCanvasStore } from '@/stores/canvas'
import { useAgentStore } from '@/stores/agent'
import type { KnowledgeDockMode } from '@/stores/agent'
import { buildAgentThreadKey } from '@/utils/agentThreadKey'
import { loadPersistedAgentThread } from '@/services/agentHistoryStorage'
import { getWorkspaceContextKey } from '@/services/workspaceContext'
import {
  buildListRows,
  collectEntityOptions,
  matchesEntityFilter,
  type EntityFilterKey,
  type KnowledgeListMode,
} from '@/utils/knowledgeLibrary'
import type { KnowledgeEntryMeta } from '@/types/notebook'

const LIST_MODE_KEY = 'graydraft.knowledge.listMode'

const CHAT_MODES: KnowledgeDockMode[] = ['import', 'chat', 'search']

export type KnowledgeChatFilter = 'all' | 'chatted' | 'unchatted'

function loadListMode(): KnowledgeListMode {
  try {
    const v = localStorage.getItem(LIST_MODE_KEY)
    if (v === 'flat' || v === 'grouped') return v
  } catch {
    /* ignore */
  }
  return 'grouped'
}

export function useKnowledgeLibrary() {
  const canvas = useCanvasStore()
  const agent = useAgentStore()
  const listMode = ref<KnowledgeListMode>(loadListMode())
  const entityFilters = ref<Partial<Record<EntityFilterKey, string>>>({})
  const expandedGroups = ref<Record<string, boolean>>({})
  /** 已对话 / 未对话筛选：依据本地历史与内存线程任一模式下存在消息 */
  const chatFilter = ref<KnowledgeChatFilter>('all')

  watch(listMode, (mode) => {
    try {
      localStorage.setItem(LIST_MODE_KEY, mode)
    } catch {
      /* ignore */
    }
  })

  /** 任一 dock 模式下存在对话消息的知识点 id 集合 */
  const chattedDocIds = computed(() => {
    const ws = getWorkspaceContextKey()
    const out = new Set<string>()
    if (!ws) return out
    for (const e of canvas.notebooks) {
      for (const mode of CHAT_MODES) {
        const key = buildAgentThreadKey(ws, e.id, mode)
        if (
          (loadPersistedAgentThread(key)?.messages.length ?? 0) > 0 ||
          (agent.threads[key]?.messages.length ?? 0) > 0
        ) {
          out.add(e.id)
          break
        }
      }
    }
    return out
  })

  const filteredEntries = computed(() => {
    let list = canvas.notebooks
    const filters = entityFilters.value
    if (Object.values(filters).some((v) => v?.trim())) {
      list = list.filter((e) => matchesEntityFilter(e, filters))
    }
    if (chatFilter.value === 'chatted') {
      list = list.filter((e) => chattedDocIds.value.has(e.id))
    } else if (chatFilter.value === 'unchatted') {
      list = list.filter((e) => !chattedDocIds.value.has(e.id))
    }
    return list
  })

  const listRows = computed(() => buildListRows(filteredEntries.value, listMode.value))

  const entityOptions = computed(() => collectEntityOptions(canvas.notebooks))

  function toggleListMode() {
    listMode.value = listMode.value === 'grouped' ? 'flat' : 'grouped'
  }

  function setEntityFilter(key: EntityFilterKey, value: string) {
    const next = { ...entityFilters.value }
    if (!value.trim()) delete next[key]
    else next[key] = value
    entityFilters.value = next
  }

  function clearEntityFilters() {
    entityFilters.value = {}
  }

  function setChatFilter(value: KnowledgeChatFilter) {
    chatFilter.value = value
  }

  function isGroupExpanded(key: string): boolean {
    return expandedGroups.value[key] ?? false
  }

  function toggleGroup(key: string) {
    expandedGroups.value = {
      ...expandedGroups.value,
      [key]: !isGroupExpanded(key),
    }
  }

  function selectEntry(id: string) {
    canvas.selectNotebook(id)
  }

  function segmentLabel(entry: KnowledgeEntryMeta): string | null {
    const ext = entry.extensions
    const idx = ext?.segment_index
    const total = ext?.segment_total
    if (typeof idx === 'number' && typeof total === 'number' && total > 1) {
      return `${idx}/${total}`
    }
    return null
  }

  return {
    listMode,
    entityFilters,
    chatFilter,
    chattedDocIds,
    filteredEntries,
    listRows,
    entityOptions,
    toggleListMode,
    setEntityFilter,
    clearEntityFilters,
    setChatFilter,
    isGroupExpanded,
    toggleGroup,
    selectEntry,
    segmentLabel,
  }
}
