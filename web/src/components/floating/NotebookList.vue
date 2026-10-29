<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from '@/composables/useI18n'
import { useCanvasStore } from '@/stores/canvas'
import { useWorkspaceSessionStore } from '@/stores/workspaceSession'
import type { NotebookMeta } from '@/types/notebook'

const { t } = useI18n()

const props = withDefaults(
  defineProps<{
    /** 按标题子串过滤（不区分大小写） */
    searchQuery?: string
  }>(),
  {
    searchQuery: '',
  },
)

const canvas = useCanvasStore()
const session = useWorkspaceSessionStore()
const knowledgeList = computed(() => Boolean(session.nexusKbType))

const allNotebooks = computed(() => canvas.notebooks)

const items = computed(() => {
  const q = props.searchQuery.trim().toLowerCase()
  if (!q) return allNotebooks.value
  return allNotebooks.value.filter((n) => n.title.toLowerCase().includes(q))
})

/** 缩略图加载失败时回退到占位 */
const thumbLoadFailed = ref<Record<string, boolean>>({})

function select(id: string) {
  canvas.selectNotebook(id)
  canvas.recenterViewToActiveNotebookOrOrigin()
}

function formatDate(iso: string) {
  try {
    return new Date(iso).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' })
  } catch {
    return iso
  }
}

function metaLine(n: NotebookMeta) {
  const date = formatDate(n.updatedAt)
  if (!knowledgeList.value || n.indexReady === undefined) return date
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

function showThumbImage(n: NotebookMeta) {
  const url = n.thumbnailUrl?.trim()
  return Boolean(url) && !thumbLoadFailed.value[n.id]
}

function onThumbError(id: string) {
  thumbLoadFailed.value = { ...thumbLoadFailed.value, [id]: true }
}
</script>

<template>
  <div class="notebook-list">
    <p v-if="canvas.notebookLoadError" class="notebook-list__err" role="alert">
      {{ canvas.notebookLoadError }}
    </p>
    <p v-else-if="allNotebooks.length === 0" class="notebook-list__empty">{{ t('notebookList.emptyServer') }}</p>
    <p v-else-if="items.length === 0" class="notebook-list__empty">{{ t('notebookList.emptyFilter') }}</p>
    <ul v-else class="notebook-list__ul" role="listbox" :aria-activedescendant="canvas.activeNotebookId ?? undefined">
      <li
        v-for="n in items"
        :id="n.id"
        :key="n.id"
        class="notebook-list__item"
        :class="{ 'notebook-list__item--active': n.id === canvas.activeNotebookId }"
        role="option"
        :aria-selected="n.id === canvas.activeNotebookId"
        @click="select(n.id)"
      >
        <div class="notebook-list__thumb" aria-hidden="true">
          <img
            v-if="showThumbImage(n)"
            class="notebook-list__thumb-img"
            :src="n.thumbnailUrl"
            alt=""
            loading="lazy"
            @error="onThumbError(n.id)"
          />
          <p v-else-if="n.preview?.trim()" class="notebook-list__thumb-preview">{{ n.preview }}</p>
          <div v-else class="notebook-list__thumb-placeholder" />
        </div>
        <div class="notebook-list__body">
          <span class="notebook-list__name">{{ n.title }}</span>
          <span class="notebook-list__meta">{{ metaLine(n) }}</span>
        </div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.notebook-list__err {
  margin: 0;
  padding: 10px 12px;
  font-size: 13px;
  line-height: 1.45;
  color: #b42318;
  background: rgba(254, 228, 226, 0.65);
  border: 1px solid rgba(240, 68, 56, 0.25);
  border-radius: 10px;
}
.notebook-list__empty {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-secondary);
  line-height: 1.5;
}
.notebook-list__ul {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.notebook-list__item {
  display: flex;
  align-items: stretch;
  gap: 10px;
  min-height: 56px;
  border-radius: 12px;
  padding: 8px 10px;
  cursor: pointer;
  border: 1px solid transparent;
  transition:
    background 0.15s ease,
    border-color 0.15s ease,
    box-shadow 0.15s ease;
}
.notebook-list__item:hover {
  background: rgba(255, 255, 255, 0.55);
  border-color: var(--color-border);
  box-shadow: var(--shadow-float-sm);
}
.notebook-list__item--active {
  background: var(--color-selected-soft);
  border-color: rgba(28, 28, 30, 0.16);
  box-shadow: inset 3px 0 0 var(--color-selected);
}
.notebook-list__thumb {
  flex-shrink: 0;
  width: 52px;
  height: 52px;
  border-radius: 8px;
  overflow: hidden;
  align-self: center;
  background: rgba(60, 60, 67, 0.06);
  border: 1px solid rgba(60, 60, 67, 0.08);
}
.notebook-list__thumb-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.notebook-list__thumb-preview {
  margin: 0;
  padding: 6px 7px;
  font-size: 9px;
  line-height: 1.35;
  color: var(--color-text-secondary);
  overflow: hidden;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 5;
  line-clamp: 5;
  word-break: break-word;
}
.notebook-list__thumb-placeholder {
  width: 100%;
  height: 100%;
  background:
    linear-gradient(rgba(0, 0, 0, 0.07), rgba(0, 0, 0, 0.07)) 7px 11px / calc(100% - 14px) 5px no-repeat,
    linear-gradient(rgba(0, 0, 0, 0.055), rgba(0, 0, 0, 0.055)) 7px 21px / 62% 4px no-repeat,
    linear-gradient(rgba(0, 0, 0, 0.055), rgba(0, 0, 0, 0.055)) 7px 29px / 44% 4px no-repeat,
    linear-gradient(155deg, #ececee 0%, #e0e1e6 100%);
}
.notebook-list__body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 4px;
  padding: 2px 0;
}
.notebook-list__name {
  font-size: 14px;
  font-weight: 600;
  letter-spacing: -0.01em;
  line-height: 1.25;
  color: var(--color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.notebook-list__meta {
  font-size: 11px;
  line-height: 1.2;
  color: var(--color-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
