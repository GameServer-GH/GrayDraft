import { computed, ref, watch } from 'vue'
import { useCanvasStore } from '@/stores/canvas'
import {
  buildListRows,
  collectEntityOptions,
  matchesEntityFilter,
  type EntityFilterKey,
  type KnowledgeListMode,
} from '@/utils/knowledgeLibrary'
import type { KnowledgeEntryMeta } from '@/types/notebook'

const LIST_MODE_KEY = 'graydraft.knowledge.listMode'

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
  const listMode = ref<KnowledgeListMode>(loadListMode())
  const entityFilters = ref<Partial<Record<EntityFilterKey, string>>>({})
  const expandedGroups = ref<Record<string, boolean>>({})

  watch(listMode, (mode) => {
    try {
      localStorage.setItem(LIST_MODE_KEY, mode)
    } catch {
      /* ignore */
    }
  })

  const filteredEntries = computed(() => {
    const filters = entityFilters.value
    const hasFilter = Object.values(filters).some((v) => v?.trim())
    if (!hasFilter) return canvas.notebooks
    return canvas.notebooks.filter((e) => matchesEntityFilter(e, filters))
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
    filteredEntries,
    listRows,
    entityOptions,
    toggleListMode,
    setEntityFilter,
    clearEntityFilters,
    isGroupExpanded,
    toggleGroup,
    selectEntry,
    segmentLabel,
  }
}
