<script setup lang="ts">
import { watch, nextTick, ref, useId } from 'vue'
import ExtensionsFieldsEditor from '@/components/form/ExtensionsFieldsEditor.vue'
import type { ExtensionFieldRow } from '@/utils/extensionsEditor'
import type { KnowledgeExtensions } from '@/types/knowledge'
import { extensionsFromRows } from '@/utils/extensionsEditor'

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
    statusHint?: string
    zIndex?: number
    busy?: boolean
  }>(),
  {
    statusHint: '',
    zIndex: 11000,
    busy: false,
  },
)

const emit = defineEmits<{
  confirm: [payload: { title: string; content: string; source_url: string; extensions?: KnowledgeExtensions }]
  cancel: []
}>()

const open = defineModel<boolean>({ required: true })
const titleText = defineModel<string>('titleText', { required: true })
const contentText = defineModel<string>('contentText', { required: true })
const sourceUrl = defineModel<string>('sourceUrl', { required: true })
const extensionRows = defineModel<ExtensionFieldRow[]>('extensionRows', { required: true })

const titleId = useId()
const contentId = useId()
const sourceId = useId()
const titleRef = ref<HTMLInputElement | null>(null)

watch(open, async (v) => {
  if (!v) return
  await nextTick()
  titleRef.value?.focus()
})

function onBackdropPointerDown(e: PointerEvent) {
  if (e.target === e.currentTarget) onCancel()
}

function onCancel() {
  if (props.busy) return
  open.value = false
  emit('cancel')
}

function onConfirm() {
  if (props.busy) return
  const title = titleText.value.trim()
  const content = contentText.value.trim()
  if (!title || !content) return
  emit('confirm', {
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
      class="knowledge-add-dlg"
      :style="{ zIndex: String(zIndex) }"
      @keydown="onKeydown"
    >
      <div
        class="knowledge-add-dlg__backdrop"
        aria-hidden="true"
        @pointerdown="onBackdropPointerDown"
      />
      <form
        role="dialog"
        aria-modal="true"
        class="knowledge-add-dlg__panel glass-panel"
        :aria-labelledby="titleId"
        @submit.prevent="onConfirm"
      >
        <h2 :id="titleId" class="knowledge-add-dlg__heading">{{ title }}</h2>
        <p v-if="statusHint" class="knowledge-add-dlg__status" role="status">{{ statusHint }}</p>

        <div class="knowledge-add-dlg__field">
          <label :for="titleId + '-input'" class="knowledge-add-dlg__label">{{ titleLabel }}</label>
          <input
            :id="titleId + '-input'"
            ref="titleRef"
            v-model="titleText"
            type="text"
            class="knowledge-add-dlg__input"
            autocomplete="off"
            :disabled="busy"
          />
        </div>

        <div class="knowledge-add-dlg__field">
          <label :for="contentId" class="knowledge-add-dlg__label">{{ contentLabel }}</label>
          <textarea
            :id="contentId"
            v-model="contentText"
            class="knowledge-add-dlg__textarea"
            rows="8"
            :disabled="busy"
          />
        </div>

        <div class="knowledge-add-dlg__field">
          <label :for="sourceId" class="knowledge-add-dlg__label">{{ sourceUrlLabel }}</label>
          <input
            :id="sourceId"
            v-model="sourceUrl"
            type="url"
            class="knowledge-add-dlg__input"
            autocomplete="off"
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

        <div class="knowledge-add-dlg__actions">
          <button
            type="button"
            class="knowledge-add-dlg__btn knowledge-add-dlg__btn--secondary"
            :disabled="busy"
            @click="onCancel"
          >
            {{ cancelLabel }}
          </button>
          <button type="submit" class="knowledge-add-dlg__btn" :disabled="busy">
            {{ confirmLabel }}
          </button>
        </div>
      </form>
    </div>
  </Teleport>
</template>

<style scoped>
.knowledge-add-dlg {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-4);
  box-sizing: border-box;
}

.knowledge-add-dlg__backdrop {
  position: absolute;
  inset: 0;
  background: rgba(28, 28, 32, 0.38);
  backdrop-filter: blur(10px) saturate(1.1);
  -webkit-backdrop-filter: blur(10px) saturate(1.1);
}

.knowledge-add-dlg__panel {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 480px;
  max-height: min(90vh, 640px);
  overflow: auto;
  padding: var(--space-4) var(--space-5);
  border-radius: 14px;
  box-sizing: border-box;
  box-shadow: var(--shadow-float-lg);
  background: var(--glass-bg-deep);
}

.knowledge-add-dlg__heading {
  margin: 0 0 var(--space-2);
  font-size: 17px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--color-text);
}

.knowledge-add-dlg__status {
  margin: 0 0 var(--space-3);
  font-size: 13px;
  line-height: 1.45;
  color: var(--color-text-secondary);
}

.knowledge-add-dlg__field {
  margin-bottom: var(--space-3);
}

.knowledge-add-dlg__label {
  display: block;
  margin-bottom: var(--space-2);
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.knowledge-add-dlg__input,
.knowledge-add-dlg__textarea {
  box-sizing: border-box;
  width: 100%;
  padding: 8px 12px;
  border-radius: 10px;
  border: none;
  font: inherit;
  font-size: 14px;
  color: var(--color-text);
  background: rgba(255, 255, 255, 0.55);
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.08);
}

.knowledge-add-dlg__textarea {
  resize: vertical;
  min-height: 120px;
  line-height: 1.5;
}

.knowledge-add-dlg__input:focus,
.knowledge-add-dlg__textarea:focus {
  outline: none;
  box-shadow:
    inset 0 0 0 1px rgba(0, 122, 255, 0.35),
    0 0 0 2px rgba(0, 122, 255, 0.12);
}

.knowledge-add-dlg__input:disabled,
.knowledge-add-dlg__textarea:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.knowledge-add-dlg__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
  margin-top: var(--space-2);
}

.knowledge-add-dlg__btn {
  min-height: 40px;
  padding: 0 16px;
  border-radius: 10px;
  border: none;
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
}

.knowledge-add-dlg__btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.knowledge-add-dlg__btn--secondary {
  color: var(--color-text);
  background: rgba(255, 255, 255, 0.5);
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.06);
}

.knowledge-add-dlg__btn[type='submit'] {
  color: #fff;
  background: var(--color-accent);
  box-shadow: 0 4px 14px rgba(0, 122, 255, 0.28);
}
</style>
