<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Icon from '@/components/common/Icon.vue'
import { useI18n } from '@/composables/useI18n'
import { useKnowledgeDetailStore } from '@/stores/knowledgeDetail'
import { useCanvasStore } from '@/stores/canvas'
import type { KnowledgeChunk } from '@/types/knowledge'
import {
  buildBreadcrumbs,
  parseExtensions,
  splitContentWithQuickLinks,
} from '@/utils/knowledgeLibrary'
import { formatSemanticTags4D, parseSemanticTags4D } from '@/utils/semantic4d'
import { renderKnowledgeMarkdown } from '@/utils/renderMarkdown'
import { callBackend } from '@/services/bridge'

const props = withDefaults(
  defineProps<{
    notebookId: string
    focusChunkId?: string | null
    mediaUrl?: string | null
  }>(),
  { focusChunkId: null, mediaUrl: null },
)

const emit = defineEmits<{
  navigate: [docId: string]
}>()

const { t } = useI18n()
const detailStore = useKnowledgeDetailStore()
const canvas = useCanvasStore()

const activeChunkIndex = ref(0)
const mediaPreview = ref('')
const mediaLoading = ref(false)

function md5FromMediaUrl(url: string | null | undefined): string | null {
  if (!url) return null
  const matched = url.match(/([a-fA-F0-9]{32})(?:[^a-fA-F0-9]|$)/)
  return matched?.[1] ?? null
}

function applyFocusChunk() {
  const id = props.focusChunkId?.trim()
  if (!id) return
  const found = chunks.value.find((c) => c.id === id)
  if (found) activeChunkIndex.value = found.chunk_index
}

const meta = computed(() => canvas.notebooks.find((n) => n.id === props.notebookId))

const slot = computed(() => detailStore.slotFor(props.notebookId))
const detail = computed(() => slot.value.detail)
const loading = computed(() => slot.value.loading)
const loadError = computed(() => slot.value.error)

const extensions = computed(() => parseExtensions(detail.value?.extensions ?? meta.value?.extensions))

const semantic4dLine = computed(() => {
  const tags = parseSemanticTags4D(extensions.value)
  return tags ? formatSemanticTags4D(tags) : ''
})

const globalSummary = computed(() => {
  const s = extensions.value.global_summary
  return typeof s === 'string' ? s.trim() : ''
})

const breadcrumbs = computed(() => {
  if (!detail.value) return []
  return buildBreadcrumbs(
    detail.value.title,
    extensions.value,
    (index, total) => t('knowledge.library.segmentPart', { index, total }),
    canvas.notebooks,
  )
})

const chunks = computed(() => detail.value?.chunks ?? [])

const activeChunk = computed((): KnowledgeChunk | null => {
  if (chunks.value.length === 0) return null
  const idx = Math.min(activeChunkIndex.value, chunks.value.length - 1)
  return chunks.value[idx] ?? null
})

const displayContent = computed(() => {
  if (activeChunk.value) return activeChunk.value.content
  return detail.value?.content ?? meta.value?.preview ?? ''
})

const quickLinkTitles = computed(() =>
  canvas.notebooks.map((n) => ({ id: n.id, title: n.title })),
)

const contentSegments = computed(() =>
  splitContentWithQuickLinks(displayContent.value, quickLinkTitles.value, props.notebookId),
)

watch(
  () => [props.notebookId, slot.value.revision] as const,
  ([id]) => {
    activeChunkIndex.value = 0
    if (id) void detailStore.load(id)
    applyFocusChunk()
  },
  { immediate: true },
)

watch(
  () => [props.focusChunkId, chunks.value.map((c) => c.id).join('|')] as const,
  () => applyFocusChunk(),
)

watch(
  () => props.mediaUrl,
  async (url) => {
    mediaPreview.value = ''
    const md5 = md5FromMediaUrl(url)
    if (!md5) return
    mediaLoading.value = true
    try {
      const meta = await callBackend<{ preview?: string }>('GetMultimodalMedia', { md5 })
      mediaPreview.value = (meta.preview ?? '').trim()
    } catch {
      mediaPreview.value = ''
    } finally {
      mediaLoading.value = false
    }
  },
  { immediate: true },
)

function formatScore(v: number | undefined): string {
  if (typeof v !== 'number' || !Number.isFinite(v)) return '—'
  return v.toFixed(2)
}

function chunkStatusLabel(c: KnowledgeChunk): string {
  return c.index_ready ? t('knowledge.detail.chunkReady') : t('knowledge.detail.chunkIndexing')
}

function selectChunk(index: number) {
  activeChunkIndex.value = index
}

function onQuickLink(docId: string) {
  emit('navigate', docId)
}

function renderMarkdownSegment(text: string): string {
  return renderKnowledgeMarkdown(text)
}

function renderQaField(text: string): string {
  return renderKnowledgeMarkdown(text)
}

const emptyPreviewHtml = computed(() => {
  const preview = meta.value?.preview?.trim() ?? ''
  if (!preview) return ''
  return renderKnowledgeMarkdown(preview)
})
</script>

<template>
  <div class="kb-detail">
    <div v-if="loading && !detail" class="kb-detail__loading" role="status">
      <Icon name="refresh" :size="18" class="kb-detail__spin" />
      {{ t('knowledge.detail.loading') }}
    </div>
    <p v-else-if="loadError" class="kb-detail__err" role="alert">{{ loadError }}</p>
    <template v-else-if="detail">
      <nav v-if="breadcrumbs.length" class="kb-detail__crumbs" aria-label="Breadcrumb">
        <template v-for="(crumb, i) in breadcrumbs" :key="i">
          <span v-if="i > 0" class="kb-detail__crumb-sep" aria-hidden="true">›</span>
          <span class="kb-detail__crumb" :class="{ 'kb-detail__crumb--muted': crumb.muted }">{{
            crumb.label
          }}</span>
        </template>
      </nav>

      <div class="kb-detail__meta" role="status">
        <span
          class="kb-detail__badge"
          :class="detail.index_ready ? 'kb-detail__badge--ok' : 'kb-detail__badge--pending'"
        >
          {{ detail.index_ready ? t('knowledge.statusReady') : t('knowledge.statusIndexing') }}
        </span>
        <span class="kb-detail__badge kb-detail__badge--muted">
          {{ t('knowledge.detail.quality') }} {{ formatScore(detail.quality_score) }}
        </span>
        <span v-if="detail.kb_version != null" class="kb-detail__badge kb-detail__badge--muted">
          v{{ detail.kb_version }}
        </span>
        <span class="kb-detail__badge kb-detail__badge--muted">
          {{
            t('knowledge.detail.chunks', {
              count: String(detail.chunks?.length ?? meta?.chunkCount ?? 0),
            })
          }}
        </span>
        <span v-if="detail.qa_pairs?.length" class="kb-detail__badge kb-detail__badge--muted">
          {{ t('knowledge.detail.qaCount', { count: String(detail.qa_pairs.length) }) }}
        </span>
      </div>

      <section
        v-if="semantic4dLine || globalSummary"
        class="kb-detail__section kb-detail__section--muted"
        :aria-label="t('knowledge.detail.semantic4dAria')"
      >
        <h3 class="kb-detail__section-title">{{ t('knowledge.detail.semantic4dTitle') }}</h3>
        <p v-if="semantic4dLine" class="kb-detail__semantic-line">{{ semantic4dLine }}</p>
        <p v-if="globalSummary" class="kb-detail__semantic-summary">{{ globalSummary }}</p>
      </section>

      <section
        v-if="mediaUrl"
        class="kb-detail__section kb-detail__section--muted"
        :aria-label="t('knowledge.detail.mediaAria')"
      >
        <h3 class="kb-detail__section-title">{{ t('knowledge.detail.mediaTitle') }}</h3>
        <p v-if="mediaPreview" class="kb-detail__semantic-summary">{{ mediaPreview }}</p>
        <p v-else-if="!mediaLoading" class="kb-detail__semantic-summary">{{ t('knowledge.detail.mediaEmpty') }}</p>
      </section>

      <section v-if="chunks.length > 1" class="kb-detail__section" :aria-label="t('knowledge.detail.chunksAria')">
        <h3 class="kb-detail__section-title">{{ t('knowledge.detail.chunksTitle') }}</h3>
        <div class="kb-detail__chunk-tabs" role="tablist">
          <button
            v-for="c in chunks"
            :key="c.id"
            type="button"
            class="kb-detail__chunk-tab"
            :class="{ 'kb-detail__chunk-tab--active': c.chunk_index === activeChunkIndex }"
            role="tab"
            :aria-selected="c.chunk_index === activeChunkIndex"
            @click="selectChunk(c.chunk_index)"
          >
            #{{ c.chunk_index + 1 }}
          </button>
        </div>
      </section>

      <section class="kb-detail__section" :aria-label="t('knowledge.detail.contentAria')">
        <h3 class="kb-detail__section-title">{{ t('knowledge.detail.contentTitle') }}</h3>
        <div class="kb-detail__content kb-detail__md">
          <template v-for="(seg, i) in contentSegments" :key="i">
            <button
              v-if="seg.docId"
              type="button"
              class="kb-detail__quick-link"
              @click="onQuickLink(seg.docId)"
            >
              {{ seg.text }}
            </button>
            <div
              v-else-if="seg.text.trim()"
              class="kb-detail__md-seg"
              v-html="renderMarkdownSegment(seg.text)"
            />
          </template>
        </div>
        <a
          v-if="detail.source_url?.trim()"
          class="kb-detail__source"
          :href="detail.source_url"
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ detail.source_url }}
        </a>
      </section>

      <section
        v-if="activeChunk"
        class="kb-detail__section kb-detail__section--muted"
        :aria-label="t('knowledge.detail.chunkMetaAria')"
      >
        <div class="kb-detail__chunk-head">
          <span class="kb-detail__chunk-idx">#{{ activeChunk.chunk_index + 1 }}</span>
          <span
            class="kb-detail__chunk-status"
            :class="activeChunk.index_ready ? 'kb-detail__chunk-status--ok' : ''"
          >
            {{ chunkStatusLabel(activeChunk) }}
          </span>
          <span class="kb-detail__chunk-score">{{ formatScore(activeChunk.quality_score) }}</span>
        </div>
      </section>

      <section
        v-if="detail.qa_pairs?.length"
        class="kb-detail__section"
        :aria-label="t('knowledge.detail.qaAria')"
      >
        <h3 class="kb-detail__section-title">{{ t('knowledge.detail.qaTitle') }}</h3>
        <ul class="kb-detail__qa">
          <li v-for="(qa, i) in detail.qa_pairs" :key="i" class="kb-detail__qa-item">
            <div class="kb-detail__qa-q kb-detail__md" v-html="renderQaField(qa.question)" />
            <div class="kb-detail__qa-a kb-detail__md" v-html="renderQaField(qa.answer)" />
          </li>
        </ul>
      </section>
    </template>
    <div
      v-else-if="emptyPreviewHtml"
      class="kb-detail__empty kb-detail__md"
      v-html="emptyPreviewHtml"
    />
    <p v-else class="kb-detail__empty">{{ t('knowledge.detail.noContent') }}</p>
  </div>
</template>

<style scoped>
.kb-detail {
  height: 100%;
  overflow: auto;
  padding: 18px 20px 24px;
  box-sizing: border-box;
  font-size: 13px;
  line-height: 1.5;
  color: var(--color-text);
}

.kb-detail__loading {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--color-text-secondary);
  font-weight: 600;
}

.kb-detail__spin {
  animation: kb-spin 0.9s linear infinite;
}

@keyframes kb-spin {
  to {
    transform: rotate(360deg);
  }
}

.kb-detail__err {
  margin: 0;
  color: #b42318;
  font-size: 13px;
}

.kb-detail__empty {
  margin: 0;
  color: var(--color-text-secondary);
  white-space: pre-wrap;
  word-break: break-word;
}

.kb-detail__crumbs {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin-bottom: 12px;
  font-size: 12px;
  line-height: 1.4;
}

.kb-detail__crumb {
  font-weight: 600;
  color: var(--color-text);
}

.kb-detail__crumb--muted {
  font-weight: 600;
  color: var(--color-text-secondary);
}

.kb-detail__crumb-sep {
  color: var(--color-text-secondary);
  opacity: 0.6;
}

.kb-detail__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 14px;
}

.kb-detail__badge {
  display: inline-flex;
  align-items: center;
  padding: 3px 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.01em;
}

.kb-detail__badge--ok {
  background: rgba(52, 199, 89, 0.14);
  color: #1a7f37;
}

.kb-detail__badge--pending {
  background: rgba(255, 149, 0, 0.14);
  color: #9a5b00;
}

.kb-detail__badge--muted {
  background: rgba(60, 60, 67, 0.08);
  color: var(--color-text-secondary);
}

.kb-detail__section {
  margin-bottom: 16px;
}

.kb-detail__section--muted {
  opacity: 0.85;
}

.kb-detail__semantic-line,
.kb-detail__semantic-summary {
  margin: 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--color-text-secondary);
}

.kb-detail__semantic-summary {
  margin-top: 6px;
  opacity: 0.9;
}

.kb-detail__section-title {
  margin: 0 0 8px;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--color-text-secondary);
}

.kb-detail__chunk-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.kb-detail__chunk-tab {
  padding: 5px 10px;
  border: 1px solid rgba(60, 60, 67, 0.12);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.7);
  font: inherit;
  font-size: 11px;
  font-weight: 700;
  color: var(--color-text-secondary);
  cursor: pointer;
}

.kb-detail__chunk-tab--active {
  border-color: var(--color-selected-line);
  background: var(--color-selected-soft);
  color: var(--color-text);
}

.kb-detail__content {
  margin: 0;
  font-size: 14px;
  line-height: 1.6;
}

.kb-detail__md-seg {
  display: contents;
}

.kb-detail__md :is(h1, h2, h3, h4) {
  margin: 0.65em 0 0.35em;
  font-weight: 600;
  line-height: 1.35;
}

.kb-detail__md h1 {
  font-size: 1.25em;
}

.kb-detail__md h2 {
  font-size: 1.12em;
}

.kb-detail__md h3 {
  font-size: 1.05em;
}

.kb-detail__md :is(p, ul, ol, pre, blockquote, table) {
  margin: 0.4em 0;
}

.kb-detail__md > :first-child,
.kb-detail__md-seg > :first-child {
  margin-top: 0;
}

.kb-detail__md > :last-child,
.kb-detail__md-seg > :last-child {
  margin-bottom: 0;
}

.kb-detail__md ul,
.kb-detail__md ol {
  padding-left: 1.25em;
}

.kb-detail__md blockquote {
  margin-left: 0;
  padding: 0.35em 0 0.35em 0.75em;
  border-left: 3px solid rgba(60, 60, 67, 0.2);
  color: var(--color-text-secondary);
}

.kb-detail__md pre {
  overflow-x: auto;
  padding: 8px 10px;
  border-radius: 8px;
  background: rgba(60, 60, 67, 0.08);
  font-size: 12px;
  line-height: 1.45;
}

.kb-detail__md code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.92em;
}

.kb-detail__md pre code {
  padding: 0;
  background: transparent;
}

.kb-detail__md :not(pre) > code {
  padding: 0.1em 0.35em;
  border-radius: 4px;
  background: rgba(60, 60, 67, 0.1);
}

.kb-detail__md a {
  color: var(--color-accent);
  text-decoration: underline;
  text-underline-offset: 2px;
}

.kb-detail__md strong {
  font-weight: 700;
}

.kb-detail__md table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.kb-detail__md th,
.kb-detail__md td {
  padding: 6px 10px;
  border: 1px solid rgba(60, 60, 67, 0.14);
  text-align: left;
  vertical-align: top;
  word-break: break-word;
}

.kb-detail__md th {
  font-weight: 600;
  background: rgba(60, 60, 67, 0.06);
}

.kb-detail__quick-link {
  display: inline;
  padding: 0;
  border: none;
  background: none;
  font: inherit;
  font-size: inherit;
  line-height: inherit;
  color: var(--color-accent);
  text-decoration: underline;
  text-underline-offset: 2px;
  cursor: pointer;
}

.kb-detail__quick-link:hover {
  filter: brightness(0.92);
}

.kb-detail__source {
  display: inline-block;
  margin-top: 10px;
  font-size: 12px;
  color: var(--color-accent);
  word-break: break-all;
}

.kb-detail__chunk-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.kb-detail__chunk-idx {
  font-size: 11px;
  font-weight: 700;
  color: var(--color-text-secondary);
}

.kb-detail__chunk-status {
  font-size: 10px;
  font-weight: 600;
  color: #9a5b00;
}

.kb-detail__chunk-status--ok {
  color: #1a7f37;
}

.kb-detail__chunk-score {
  margin-left: auto;
  font-size: 10px;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.kb-detail__qa {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.kb-detail__qa-item {
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(0, 122, 255, 0.06);
  border: 1px solid rgba(0, 122, 255, 0.12);
}

.kb-detail__qa-q {
  margin: 0 0 6px;
  font-size: 12px;
  font-weight: 600;
}

.kb-detail__qa-a {
  margin: 0;
  font-size: 12px;
  color: var(--color-text-secondary);
}

.kb-detail__qa-a.kb-detail__md :is(p, ul, ol) {
  margin: 0.25em 0;
}
</style>
