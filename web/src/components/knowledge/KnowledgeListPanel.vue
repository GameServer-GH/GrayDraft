<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import Icon from '@/components/common/Icon.vue'
import { useI18n } from '@/composables/useI18n'
import { useKnowledgeLibrary } from '@/composables/useKnowledgeLibrary'
import { useCanvasStore } from '@/stores/canvas'
import { useKnowledgeLibrarySSE } from '@/composables/useKnowledgeLibrarySync'
import { useKnowledgePanel } from '@/composables/useKnowledgePanel'
import type { KnowledgeEntryMeta } from '@/types/notebook'

const props = withDefaults(
  defineProps<{
    searchQuery?: string
  }>(),
  { searchQuery: '' },
)

const emit = defineEmits<{
  openDetail: [docId: string]
}>()

const { t } = useI18n()
const canvas = useCanvasStore()
const {
  listRows,
  isGroupExpanded,
  toggleGroup,
  selectEntry,
  segmentLabel,
  chatFilter,
  chattedDocIds,
  setChatFilter,
} = useKnowledgeLibrary()

type ChatFilterOption = { value: 'all' | 'chatted' | 'unchatted'; label: string }
const chatFilterOptions = computed<ChatFilterOption[]>(() => [
  { value: 'all', label: t('knowledge.sidebar.filterAll') },
  { value: 'chatted', label: t('knowledge.sidebar.filterChatted') },
  { value: 'unchatted', label: t('knowledge.sidebar.filterUnchatted') },
])

const rows = computed(() => {
  const q = props.searchQuery.trim().toLowerCase()
  if (!q) return listRows.value
  return listRows.value.filter((row) => {
    if (row.title.toLowerCase().includes(q)) return true
    if (row.kind === 'group' && row.group) {
      return row.group.segments.some((s) => s.title.toLowerCase().includes(q))
    }
    return false
  })
})

function formatDate(iso: string) {
  try {
    return new Date(iso).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' })
  } catch {
    return iso
  }
}

function metaLine(n: KnowledgeEntryMeta) {
  const date = formatDate(n.updatedAt)
  const status = n.indexReady ? t('knowledge.statusReady') : t('knowledge.statusIndexing')
  const parts = [status]
  if (typeof n.qualityScore === 'number' && Number.isFinite(n.qualityScore)) {
    parts.push(t('knowledge.list.quality', { score: n.qualityScore.toFixed(2) }))
  }
  if (n.chunkCount != null && n.chunkCount > 0) {
    parts.push(t('knowledge.list.chunks', { count: String(n.chunkCount) }))
  }
  parts.push(date)
  return parts.join(' · ')
}

function onSelect(id: string) {
  if (canvas.activeNotebookId === id) {
    emit('openDetail', id)
    return
  }
  selectEntry(id)
}

const knowledge = useKnowledgePanel()
const { start: startSSE, stop: stopSSE } = useKnowledgeLibrarySSE(() => knowledge.enabled.value)
onMounted(() => startSSE())
onUnmounted(stopSSE)

function importingStatusLabel(status: string) {
    if (status === 'completed') return '已导入'
    if (status === 'failed') return '导入失败'
    return '正在导入...'
}
</script>

<template>
  <div class="kb-list">
    <!-- 已对话 / 未对话筛选：帮助在大量知识点中定位聊过的条目 -->
    <div class="kb-list__chat-filter" role="group" :aria-label="t('knowledge.sidebar.filterAria')">
      <button
        v-for="opt in chatFilterOptions"
        :key="opt.value"
        type="button"
        class="kb-list__chat-filter-btn"
        :class="{ 'kb-list__chat-filter-btn--active': chatFilter === opt.value }"
        :aria-pressed="chatFilter === opt.value"
        @click="setChatFilter(opt.value)"
      >
        {{ opt.label }}
      </button>
    </div>

    <!-- Importing placeholder cards with masked overlay -->
    <div v-if="canvas.importingCards.length > 0" class="kb-list__importing">
        <div
            v-for="card in canvas.importingCards"
            :key="card.segment"
            class="kb-list__importing-card"
            :class="{ 'kb-list__importing-card--done': card.status === 'completed', 'kb-list__importing-card--fail': card.status === 'failed' }"
        >
            <div class="kb-list__importing-mask"></div>
            <div class="kb-list__importing-body">
                <div class="kb-list__importing-head">
                    <span class="kb-list__importing-name">{{ card.title }}</span>
                    <span class="kb-list__importing-seg">{{ card.segment }}/{{ card.total }}</span>
                </div>
                <div class="kb-list__importing-status">
                    <span v-if="card.status === 'queued'" class="kb-list__importing-spinner"></span>
                    <Icon v-else-if="card.status === 'completed'" class="kb-list__importing-check" name="circle-check" :size="14" />
                    <Icon v-else-if="card.status === 'failed'" class="kb-list__importing-cross" name="circle-x" :size="14" />
                    <span class="kb-list__importing-label">{{ importingStatusLabel(card.status) }}</span>
                </div>
            </div>
        </div>
    </div>

    <p v-if="canvas.notebookLoadError" class="kb-list__err" role="alert">{{ canvas.notebookLoadError }}</p>
    <p v-else-if="canvas.notebooks.length === 0 && canvas.importingCards.length === 0" class="kb-list__empty">{{ t('knowledge.sidebar.emptyServer') }}</p>
    <p v-else-if="rows.length === 0" class="kb-list__empty">{{ t('knowledge.sidebar.emptyFilter') }}</p>

    <ul v-else class="kb-list__ul" role="listbox" :aria-activedescendant="canvas.activeNotebookId ?? undefined">
      <template v-for="row in rows" :key="row.id">
        <li
          v-if="row.kind === 'standalone' && row.entry"
          :id="row.entry.id"
          class="kb-list__item"
          :class="{ 'kb-list__item--active': row.entry.id === canvas.activeNotebookId }"
          role="option"
          :aria-selected="row.entry.id === canvas.activeNotebookId"
          @click="onSelect(row.entry.id)"
        >
          <div class="kb-list__item-head">
            <span class="kb-list__name">{{ row.entry.title }}</span>
            <span
              v-if="chattedDocIds.has(row.entry.id)"
              class="kb-list__chatted-dot"
              :title="t('knowledge.sidebar.chattedTooltip')"
            ></span>
            <span v-if="segmentLabel(row.entry)" class="kb-list__seg-badge">{{ segmentLabel(row.entry) }}</span>
          </div>
          <p v-if="row.entry.preview?.trim()" class="kb-list__preview">{{ row.entry.preview }}</p>
          <span class="kb-list__meta">{{ metaLine(row.entry) }}</span>
        </li>

        <li v-else-if="row.kind === 'group' && row.group" class="kb-list__group" role="presentation">
          <button
            type="button"
            class="kb-list__group-head"
            :aria-expanded="isGroupExpanded(row.group.key)"
            @click="toggleGroup(row.group.key)"
          >
            <Icon
              class="kb-list__group-chevron"
              :class="{ 'kb-list__group-chevron--open': isGroupExpanded(row.group.key) }"
              name="chevron-right"
              :size="14"
            />
            <span class="kb-list__group-title">{{ row.group.parentTitle }}</span>
            <span class="kb-list__group-count">{{
              t('knowledge.library.segmentCount', { count: String(row.group.segments.length) })
            }}</span>
          </button>
          <ul v-show="isGroupExpanded(row.group.key)" class="kb-list__group-ul">
            <li
              v-for="seg in row.group.segments"
              :id="seg.id"
              :key="seg.id"
              class="kb-list__item kb-list__item--nested"
              :class="{ 'kb-list__item--active': seg.id === canvas.activeNotebookId }"
              role="option"
              :aria-selected="seg.id === canvas.activeNotebookId"
              @click="onSelect(seg.id)"
            >
              <div class="kb-list__item-head">
                <span class="kb-list__name">{{ seg.title }}</span>
                <span
                  v-if="chattedDocIds.has(seg.id)"
                  class="kb-list__chatted-dot"
                  :title="t('knowledge.sidebar.chattedTooltip')"
                ></span>
                <span v-if="segmentLabel(seg)" class="kb-list__seg-badge">{{ segmentLabel(seg) }}</span>
              </div>
              <p v-if="seg.preview?.trim()" class="kb-list__preview">{{ seg.preview }}</p>
              <span class="kb-list__meta">{{ metaLine(seg) }}</span>
            </li>
          </ul>
        </li>
      </template>
    </ul>
  </div>
</template>

<style scoped>
.kb-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.kb-list__err {
  margin: 0;
  padding: 10px 12px;
  font-size: 13px;
  color: #b42318;
  background: rgba(254, 228, 226, 0.65);
  border: 1px solid rgba(240, 68, 56, 0.25);
  border-radius: 10px;
}

.kb-list__empty {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-secondary);
}

.kb-list__ul {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.kb-list__item {
  border-radius: 10px;
  padding: 10px 12px;
  cursor: pointer;
  border: none;
  border-left: 3px solid transparent;
  transition:
    background 0.15s ease,
    border-color 0.15s ease,
    box-shadow 0.15s ease;
}

.kb-list__item:hover {
  background: rgba(255, 255, 255, 0.55);
  border-left-color: rgba(60, 60, 67, 0.18);
  box-shadow: 2px 0 6px rgba(60, 60, 67, 0.04);
}

/* 正在对话的知识点：黄色高亮，与普通选中/悬停区分 */
.kb-list__item--active {
  background: rgba(255, 193, 7, 0.18);
  border-left-color: #f0a500;
  box-shadow: none;
}

.kb-list__item--active .kb-list__name {
  color: #1a1a1e;
}

.kb-list__item--active:hover {
  background: rgba(255, 193, 7, 0.24);
  border-left-color: #f0a500;
}

.kb-list__chatted-dot {
  flex-shrink: 0;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #34c759;
  opacity: 0.85;
}

.kb-list__chat-filter {
  display: flex;
  gap: 6px;
}

.kb-list__chat-filter-btn {
  flex: 1;
  padding: 5px 8px;
  border: 1px solid rgba(60, 60, 67, 0.14);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.4);
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: all 0.15s ease;
}

.kb-list__chat-filter-btn:hover {
  background: rgba(255, 255, 255, 0.65);
  color: var(--color-text);
}

.kb-list__chat-filter-btn--active {
  background: rgba(0, 122, 255, 0.12);
  border-color: rgba(0, 122, 255, 0.35);
  color: var(--color-accent);
}

.kb-list__item--nested {
  margin-left: 8px;
  padding-left: 10px;
  border-left: 2px solid rgba(60, 60, 67, 0.14);
}

.kb-list__item--nested.kb-list__item--active {
  border-left: 3px solid #f0a500;
}

.kb-list__item-head {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.kb-list__name {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kb-list__seg-badge {
  flex-shrink: 0;
  padding: 2px 6px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 700;
  color: var(--color-accent);
  background: rgba(0, 122, 255, 0.1);
}

.kb-list__preview {
  margin: 4px 0 0;
  font-size: 11px;
  line-height: 1.4;
  color: var(--color-text-secondary);
  overflow: hidden;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
}

.kb-list__meta {
  display: block;
  margin-top: 4px;
  font-size: 10px;
  color: var(--color-text-secondary);
}

.kb-list__group {
  border-radius: 10px;
  border: 1px solid rgba(60, 60, 67, 0.1);
  background: rgba(255, 255, 255, 0.35);
  overflow: hidden;
}

.kb-list__group-head {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 10px 12px;
  border: none;
  background: transparent;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.kb-list__group-head:hover {
  background: rgba(255, 255, 255, 0.45);
}

.kb-list__group-chevron {
  flex-shrink: 0;
  transition: transform 0.15s ease;
  opacity: 0.6;
}

.kb-list__group-chevron--open {
  transform: rotate(90deg);
}

.kb-list__group-title {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kb-list__group-count {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.kb-list__group-ul {
  list-style: none;
  margin: 0;
  padding: 0 0 6px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.kb-list__importing {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 8px;
}

.kb-list__importing-card {
    position: relative;
    border-radius: 10px;
    overflow: hidden;
    border: 1px solid rgba(0, 122, 255, 0.18);
    background: rgba(255, 255, 255, 0.7);
    transition: all 0.25s ease;
}

.kb-list__importing-card--done {
    border-color: rgba(52, 199, 89, 0.3);
    background: rgba(240, 255, 244, 0.5);
}

.kb-list__importing-card--fail {
    border-color: rgba(255, 59, 48, 0.25);
    background: rgba(255, 240, 240, 0.5);
}

.kb-list__importing-mask {
    position: absolute;
    inset: 0;
    background: linear-gradient(110deg, transparent 30%, rgba(0, 122, 255, 0.06) 50%, transparent 70%);
    background-size: 200% 100%;
    animation: kb-importing-shimmer 1.8s ease-in-out infinite;
    pointer-events: none;
}

.kb-list__importing-card--done .kb-list__importing-mask {
    animation: none;
    background: transparent;
}

.kb-list__importing-card--fail .kb-list__importing-mask {
    animation: none;
    background: transparent;
}

@keyframes kb-importing-shimmer {
    0% { background-position: 200% 0; }
    100% { background-position: -200% 0; }
}

.kb-list__importing-body {
    position: relative;
    padding: 10px 12px;
    z-index: 1;
}

.kb-list__importing-head {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
}

.kb-list__importing-name {
    flex: 1;
    min-width: 0;
    font-size: 13px;
    font-weight: 600;
    line-height: 1.3;
    color: var(--color-text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.kb-list__importing-seg {
    flex-shrink: 0;
    padding: 2px 6px;
    border-radius: 999px;
    font-size: 10px;
    font-weight: 700;
    color: var(--color-accent);
    background: rgba(0, 122, 255, 0.1);
}

.kb-list__importing-status {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 6px;
    font-size: 11px;
    color: var(--color-text-secondary);
}

.kb-list__importing-spinner {
    width: 12px;
    height: 12px;
    border: 2px solid rgba(0, 122, 255, 0.2);
    border-top-color: var(--color-accent);
    border-radius: 50%;
    animation: kb-importing-spin 0.7s linear infinite;
    flex-shrink: 0;
}

@keyframes kb-importing-spin {
    to { transform: rotate(360deg); }
}

.kb-list__importing-check {
    font-size: 13px;
    color: #34c759;
    font-weight: 700;
}

.kb-list__importing-cross {
    font-size: 11px;
    color: #ff3b30;
    font-weight: 700;
}

.kb-list__importing-label {
    font-weight: 600;
}
</style>
