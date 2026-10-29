<script setup lang="ts">
import { watch, nextTick, ref, useId } from 'vue'
import ExtensionsFieldsEditor from '@/components/form/ExtensionsFieldsEditor.vue'
import { getKnowledgeEntry } from '@/services/knowledge'
import { formatNexusError } from '@/utils/nexusError'
import type { ExtensionFieldRow } from '@/utils/extensionsEditor'
import { extensionsFromRows, rowsFromExtensions } from '@/utils/extensionsEditor'
import type { KnowledgeExtensions } from '@/types/knowledge'

const props = withDefaults(
  defineProps<{
    title: string
    titleLabel: string
    contentLabel: string
    sourceUrlLabel: string
    extensionsLabel: string
    extensionsHint?: string
    extensionKeyPlaceholder: string
    extensionValuePlaceholder: string
    addExtensionLabel: string
    removeExtensionAria: string
    confirmLabel: string
    cancelLabel: string
    loadingLabel: string
    zIndex?: number
    busy?: boolean
    /** 要编辑的文档 id；打开时拉取详情 */
    documentId: string | null
  }>(),
  { zIndex: 11000, busy: false, documentId: null },
)

const emit = defineEmits<{
  confirm: [payload: { id: string; title: string; content: string; source_url: string; extensions?: KnowledgeExtensions }]
  cancel: []
}>()

const open = defineModel<boolean>({ required: true })
const titleText = defineModel<string>('titleText', { required: true })
const contentText = defineModel<string>('contentText', { required: true })
const sourceUrl = defineModel<string>('sourceUrl', { required: true })
const extensionRows = defineModel<ExtensionFieldRow[]>('extensionRows', { required: true })

const headingId = useId()
const titleRef = ref<HTMLInputElement | null>(null)
const loadError = ref('')
const loading = ref(false)

watch([open, () => props.documentId], async ([isOpen, docId]) => {
  if (!isOpen || !docId) return
  loadError.value = ''
  loading.value = true
  titleText.value = ''
  contentText.value = ''
  sourceUrl.value = ''
  extensionRows.value = []
  try {
    const doc = await getKnowledgeEntry(docId)
    titleText.value = doc.title ?? ''
    contentText.value = doc.content ?? ''
    sourceUrl.value = doc.source_url ?? ''
    extensionRows.value = rowsFromExtensions(doc.extensions)
  } catch (e) {
    loadError.value = formatNexusError(e)
  } finally {
    loading.value = false
    await nextTick()
    if (!loadError.value) titleRef.value?.focus()
  }
})

function onBackdropPointerDown(e: PointerEvent) {
  if (e.target === e.currentTarget) onCancel()
}

function onCancel() {
  if (props.busy || loading.value) return
  open.value = false
  emit('cancel')
}

function onConfirm() {
  if (props.busy || loading.value || loadError.value) return
  const id = props.documentId?.trim()
  const title = titleText.value.trim()
  const content = contentText.value.trim()
  if (!id || !title || !content) return
  emit('confirm', {
    id,
    title,
    content,
    source_url: sourceUrl.value.trim(),
    extensions: extensionsFromRows(extensionRows.value),
  })
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.preventDefault()
    onCancel()
  }
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="knowledge-edit-dlg"
      :style="{ zIndex: String(zIndex) }"
      @keydown="onKeydown"
    >
      <div class="knowledge-edit-dlg__backdrop" aria-hidden="true" @pointerdown="onBackdropPointerDown" />
      <form
        role="dialog"
        aria-modal="true"
        class="knowledge-edit-dlg__panel glass-panel"
        :aria-labelledby="headingId"
        @submit.prevent="onConfirm"
      >
        <h2 :id="headingId" class="knowledge-edit-dlg__heading">{{ title }}</h2>
        <p v-if="loading" class="knowledge-edit-dlg__hint">{{ loadingLabel }}</p>
        <p v-else-if="loadError" class="knowledge-edit-dlg__err" role="alert">{{ loadError }}</p>

        <template v-if="!loading && !loadError">
          <div class="knowledge-edit-dlg__field">
            <label :for="headingId + '-t'" class="knowledge-edit-dlg__label">{{ titleLabel }}</label>
            <input
              :id="headingId + '-t'"
              ref="titleRef"
              v-model="titleText"
              type="text"
              class="knowledge-edit-dlg__input"
              :disabled="busy"
            />
          </div>
          <div class="knowledge-edit-dlg__field">
            <label :for="headingId + '-c'" class="knowledge-edit-dlg__label">{{ contentLabel }}</label>
            <textarea
              :id="headingId + '-c'"
              v-model="contentText"
              class="knowledge-edit-dlg__textarea"
              rows="10"
              :disabled="busy"
            />
          </div>
          <div class="knowledge-edit-dlg__field">
            <label :for="headingId + '-u'" class="knowledge-edit-dlg__label">{{ sourceUrlLabel }}</label>
            <input
              :id="headingId + '-u'"
              v-model="sourceUrl"
              type="url"
              class="knowledge-edit-dlg__input"
              :disabled="busy"
            />
          </div>
          <ExtensionsFieldsEditor
            v-model:rows="extensionRows"
            :label="extensionsLabel"
            :hint="extensionsHint"
            :key-placeholder="extensionKeyPlaceholder"
            :value-placeholder="extensionValuePlaceholder"
            :add-label="addExtensionLabel"
            :remove-aria="removeExtensionAria"
            :disabled="busy"
          />
        </template>

        <div class="knowledge-edit-dlg__actions">
          <button
            type="button"
            class="knowledge-edit-dlg__btn knowledge-edit-dlg__btn--secondary"
            :disabled="busy || loading"
            @click="onCancel"
          >
            {{ cancelLabel }}
          </button>
          <button
            type="submit"
            class="knowledge-edit-dlg__btn"
            :disabled="busy || loading || Boolean(loadError)"
          >
            {{ confirmLabel }}
          </button>
        </div>
      </form>
    </div>
  </Teleport>
</template>

<style scoped>
.knowledge-edit-dlg {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-4);
}
.knowledge-edit-dlg__backdrop {
  position: absolute;
  inset: 0;
  background: rgba(28, 28, 32, 0.38);
  backdrop-filter: blur(10px);
}
.knowledge-edit-dlg__panel {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 480px;
  max-height: min(90vh, 680px);
  overflow: auto;
  padding: var(--space-4) var(--space-5);
  border-radius: 14px;
  box-shadow: var(--shadow-float-lg);
  background: var(--glass-bg-deep);
}
.knowledge-edit-dlg__heading {
  margin: 0 0 var(--space-2);
  font-size: 17px;
  font-weight: 700;
}
.knowledge-edit-dlg__hint,
.knowledge-edit-dlg__err {
  margin: 0 0 var(--space-3);
  font-size: 13px;
  line-height: 1.45;
}
.knowledge-edit-dlg__err {
  color: #b42318;
}
.knowledge-edit-dlg__field {
  margin-bottom: var(--space-3);
}
.knowledge-edit-dlg__label {
  display: block;
  margin-bottom: var(--space-2);
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-secondary);
}
.knowledge-edit-dlg__input,
.knowledge-edit-dlg__textarea {
  box-sizing: border-box;
  width: 100%;
  padding: 8px 12px;
  border-radius: 10px;
  border: none;
  font: inherit;
  font-size: 14px;
  background: rgba(255, 255, 255, 0.55);
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.08);
}
.knowledge-edit-dlg__textarea {
  resize: vertical;
  min-height: 140px;
  line-height: 1.5;
}
.knowledge-edit-dlg__actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: var(--space-2);
}
.knowledge-edit-dlg__btn {
  min-height: 40px;
  padding: 0 16px;
  border-radius: 10px;
  border: none;
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
}
.knowledge-edit-dlg__btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
.knowledge-edit-dlg__btn--secondary {
  background: rgba(255, 255, 255, 0.5);
}
.knowledge-edit-dlg__btn[type='submit'] {
  color: #fff;
  background: var(--color-accent);
}
</style>
