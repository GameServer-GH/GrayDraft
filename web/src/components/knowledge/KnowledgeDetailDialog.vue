<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/common/Icon.vue'
import KnowledgeDetailPane from '@/components/canvas/KnowledgeDetailPane.vue'
import { useI18n } from '@/composables/useI18n'
import { useCanvasStore } from '@/stores/canvas'

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    documentId: string | null
    chunkId?: string | null
    mediaUrl?: string | null
    zIndex?: number
  }>(),
  { zIndex: 10_200, chunkId: null, mediaUrl: null },
)

const emit = defineEmits<{
  'update:modelValue': [open: boolean]
  navigate: [docId: string]
}>()

const { t } = useI18n()
const canvas = useCanvasStore()

const headingId = 'knowledge-detail-dlg-heading'

const title = computed(() => {
  if (!props.documentId) return t('knowledge.detailModal.title')
  return canvas.notebooks.find((n) => n.id === props.documentId)?.title ?? t('knowledge.detailModal.title')
})

function close() {
  emit('update:modelValue', false)
}

function onBackdropPointerDown() {
  close()
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.preventDefault()
    close()
  }
}

function onNavigate(docId: string) {
  emit('navigate', docId)
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="modelValue && documentId"
      class="knowledge-detail-dlg"
      :style="{ zIndex: String(zIndex) }"
      @keydown="onKeydown"
    >
      <div class="knowledge-detail-dlg__backdrop" aria-hidden="true" @pointerdown="onBackdropPointerDown" />
      <div
        role="dialog"
        aria-modal="true"
        class="knowledge-detail-dlg__panel glass-panel"
        :aria-labelledby="headingId"
      >
        <header class="knowledge-detail-dlg__header">
          <h2 :id="headingId" class="knowledge-detail-dlg__heading">{{ title }}</h2>
          <button
            type="button"
            class="knowledge-detail-dlg__close"
            :aria-label="t('knowledge.detailModal.closeAria')"
            @click="close"
          >
            <Icon name="x" :size="18" />
          </button>
        </header>
        <div class="knowledge-detail-dlg__body">
          <KnowledgeDetailPane
            :notebook-id="documentId"
            :focus-chunk-id="chunkId"
            :media-url="mediaUrl"
            @navigate="onNavigate"
          />
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.knowledge-detail-dlg {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-4);
}

.knowledge-detail-dlg__backdrop {
  position: absolute;
  inset: 0;
  background: rgba(28, 28, 32, 0.38);
  backdrop-filter: blur(10px);
}

.knowledge-detail-dlg__panel {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  width: 100%;
  max-width: min(720px, 92vw);
  max-height: min(88vh, 760px);
  overflow: hidden;
  border-radius: 16px;
  box-shadow: var(--shadow-float-lg);
  background: var(--glass-bg-deep);
}

.knowledge-detail-dlg__header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  flex-shrink: 0;
  padding: var(--space-4) var(--space-5) var(--space-2);
}

.knowledge-detail-dlg__heading {
  flex: 1;
  min-width: 0;
  margin: 0;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.35;
}

.knowledge-detail-dlg__close {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  padding: 0;
  border: none;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.55);
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: background 0.14s ease;
}

.knowledge-detail-dlg__close:hover {
  background: rgba(255, 255, 255, 0.85);
}

.knowledge-detail-dlg__body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: var(--space-3) var(--space-5) var(--space-5);
}
</style>
