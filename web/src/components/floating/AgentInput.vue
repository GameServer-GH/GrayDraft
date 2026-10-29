<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Select from '@/components/form/Select.vue'
import Icon from '@/components/common/Icon.vue'
import IconCircleButton from '@/components/form/IconCircleButton.vue'
import type { FormSelectOption } from '@/components/form/Select.vue'
import { useI18n } from '@/composables/useI18n'
import { useAgentModelConfigStore, type VendorKindId } from '@/stores/agentModelConfig'
import { useUiStore } from '@/stores/ui'
import type { PendingImage } from '@/stores/agent'

const model = defineModel<string>({ default: '' })

const props = withDefaults(
  defineProps<{
    disabled?: boolean
    placeholder?: string
    notebookDisplayTitle?: string | null
    knowledgeMode?: boolean
    knowledgeDockMode?: 'import' | 'chat' | 'search'
    showAttach?: boolean
    hasAttachments?: boolean
    showImageUpload?: boolean
    voiceRecording?: boolean
    pendingImages?: PendingImage[]
    /** Draft media URL typed by user */
    mediaUrlDraft?: string
  }>(),
  {
    disabled: false,
    notebookDisplayTitle: null,
    knowledgeMode: false,
    showAttach: true,
    hasAttachments: false,
    showImageUpload: false,
    voiceRecording: false,
    pendingImages: () => [],
    mediaUrlDraft: '',
  },
)

const ui = useUiStore()
const agentModelConfig = useAgentModelConfigStore()
const { t } = useI18n()

const notebookSelected = computed(() => {
  const v = props.notebookDisplayTitle
  return typeof v === 'string' && v.trim().length > 0
})

const notebookChipText = computed(() => {
  if (props.knowledgeMode && !notebookSelected.value) {
    return t('agentInput.knowledgeWorkspaceContext')
  }
  if (!notebookSelected.value) return t('agentInput.noNotebookSelected')
  return props.notebookDisplayTitle!.trim()
})

const resolvedPlaceholder = computed(() => {
  if (props.placeholder) return props.placeholder
  if (props.knowledgeMode && props.knowledgeDockMode === 'search') {
    return t('agentInput.knowledgeSearchPlaceholder')
  }
  if (props.knowledgeMode && props.knowledgeDockMode === 'chat') {
    return t('agentInput.knowledgeChatPlaceholder')
  }
  if (props.knowledgeMode) return t('agentInput.knowledgePlaceholder')
  return t('agentInput.placeholder')
})

function vendorKindLabel(kind: VendorKindId): string {
  if (kind === 'anthropic') return t('settings.vendorAnthropic')
  if (kind === 'openai') return t('settings.vendorOpenai')
  if (kind === 'google') return t('settings.vendorGoogle')
  if (kind === 'zhipu') return t('settings.vendorZhipu')
  return t('settings.vendorDeepseek')
}

const agentModelSelectOptions = computed<FormSelectOption[]>(() => {
  const opts: FormSelectOption[] = [
    {
      value: 'auto',
      label: t('agentModel.auto'),
    },
  ]
  for (const e of agentModelConfig.enabledPickerEntries) {
    opts.push({
      value: e.value,
      label: `${vendorKindLabel(e.kindId)} · ${e.modelName}`,
    })
  }
  return opts
})

watch(
  [
    () => ui.selectedAgentModel,
    () => agentModelConfig.enabledPickerEntries.map((e) => e.value).join('|'),
  ],
  () => {
    if (!agentModelConfig.selectionIsValid(ui.selectedAgentModel)) {
      ui.setSelectedAgentModel('auto')
    }
  },
)

const emit = defineEmits<{
  submit: []
  cancel: []
  keydown: [KeyboardEvent]
  attachFiles: [files: File[]]
  attachImages: [files: File[]]
  attachVideos: [files: File[]]
  removeImage: [imageId: string]
  startVoice: []
  stopVoice: []
  cancelVoice: []
  'update:mediaUrlDraft': [url: string]
  pasteMediaUrl: [url: string]
}>()

const hasImages = computed(() => props.pendingImages.length > 0)
const hasUrlText = computed(() => (props.mediaUrlDraft ?? '').trim().length > 0)
const hasContent = computed(
  () => model.value.trim().length > 0 || props.hasAttachments || hasImages.value || hasUrlText.value,
)

const sendAriaLabel = computed(() => {
  if (props.disabled) return t('agentInput.stop')
  if (!props.knowledgeMode && !notebookSelected.value) return t('agentInput.sendNeedsNotebook')
  if (!hasContent.value) return t('agentInput.sendNeedsContent')
  if (props.knowledgeMode && props.knowledgeDockMode === 'search') return t('agentInput.sendSearch')
  if (props.knowledgeMode && props.knowledgeDockMode === 'chat') return t('agentInput.sendChat')
  if (props.knowledgeMode) return t('agentInput.sendImport')
  return t('agentInput.send')
})

const sendButtonDisabled = computed(() => {
  if (props.disabled) return false
  if (hasContent.value) {
    if (props.knowledgeMode) return false
    return !notebookSelected.value
  }
  return true
})

function onSendOrStopClick() {
  if (props.disabled) emit('cancel')
  else emit('submit')
}

const fileInputRef = ref<HTMLInputElement | null>(null)
const imageInputRef = ref<HTMLInputElement | null>(null)
const videoInputRef = ref<HTMLInputElement | null>(null)

function openAttachPicker() { fileInputRef.value?.click() }
function onFileInputChange(ev: Event) {
  const input = ev.target as HTMLInputElement
  const list = input.files
  if (!list?.length) return
  emit('attachFiles', [...list])
  input.value = ''
}

function openImagePicker() { imageInputRef.value?.click() }
function onImageInputChange(ev: Event) {
  const input = ev.target as HTMLInputElement
  const list = input.files
  if (!list?.length) return
  emit('attachImages', [...list])
  input.value = ''
}

function openVideoPicker() { videoInputRef.value?.click() }
function onVideoInputChange(ev: Event) {
  const input = ev.target as HTMLInputElement
  const list = input.files
  if (!list?.length) return
  emit('attachVideos', [...list])
  input.value = ''
}

function onPasteMediaUrl() {
  const url = (props.mediaUrlDraft ?? '').trim()
  if (!url) return
  emit('pasteMediaUrl', url)
}

function imageStatusLabel(img: PendingImage): string {
  if (img.status === 'recognizing') return t('agentInput.imageRecognizing')
  if (img.status === 'error') return img.error ?? t('agentInput.imageError')
  return ''
}

function imageTypeLabel(img: PendingImage): string {
  if (img.mediaType === 'video') return t('agentInput.mediaTypeVideo')
  if (img.mediaUrl) return t('agentInput.mediaTypeLink')
  return ''
}

function imagePreviewClass(img: PendingImage): string {
  if (img.mediaType === 'video') return 'agent-input__image-thumb--video'
  if (img.mediaUrl) return 'agent-input__image-thumb--url'
  return ''
}


</script>

<template>
  <div class="agent-input glass-panel">
    <div
      class="agent-input__notebook"
      :class="{ 'agent-input__notebook--empty': !notebookSelected }"
      role="status"
      :aria-label="t('agentInput.currentNotebook')"
    >
      <Icon class="agent-input__notebook-icon" name="file-description" :size="16" />
      <span class="agent-input__notebook-text">{{ notebookChipText }}</span>
    </div>

    <!-- Image / video thumbnail previews -->
    <div v-if="pendingImages.length > 0" class="agent-input__images">
      <div
        v-for="img in pendingImages"
        :key="img.id"
        class="agent-input__image-thumb"
        :class="[imagePreviewClass(img), { 'agent-input__image-thumb--error': img.status === 'error' }]"
      >
        <img
          v-if="img.mediaType !== 'video' || img.thumbnail"
          :src="img.thumbnail || img.mediaUrl"
          :alt="img.filename"
          class="agent-input__image-preview"
        />
        <div v-else class="agent-input__image-video-placeholder">
          <Icon name="video" :size="24" />
        </div>
        <span v-if="img.status === 'recognizing'" class="agent-input__image-status">
          <span class="agent-input__image-spinner" />
          {{ imageStatusLabel(img) }}
        </span>
        <span v-if="imageTypeLabel(img)" class="agent-input__image-type-badge">{{ imageTypeLabel(img) }}</span>
        <button
          v-if="img.status !== 'recognizing'"
          type="button"
          class="agent-input__image-remove"
          :aria-label="t('agentInput.removeImageAria', { name: img.filename })"
          @click="emit('removeImage', img.id)"
        >
          <Icon name="x" :size="12" />
        </button>
      </div>
    </div>

    <!-- Media URL input (compact, below thumbnails) -->
    <div v-if="showImageUpload" class="agent-input__url-row">
      <input
        :value="mediaUrlDraft"
        type="url"
        class="agent-input__url-field"
        :placeholder="t('agentInput.mediaUrlPlaceholder')"
        :disabled="disabled"
        @input="emit('update:mediaUrlDraft', ($event.target as HTMLInputElement).value)"
        @keydown.enter.prevent="onPasteMediaUrl()"
      />
      <button
        v-if="(mediaUrlDraft ?? '').trim()"
        type="button"
        class="agent-input__url-add"
        :disabled="disabled"
        @click="onPasteMediaUrl()"
      >
        {{ t('agentInput.mediaUrlAdd') }}
      </button>
    </div>

    <textarea
      v-model="model"
      class="agent-input__field"
      rows="3"
      :disabled="disabled"
      :placeholder="resolvedPlaceholder"
      :aria-label="t('agentInput.aria')"
      @keydown="$emit('keydown', $event)"
    />
    <div class="agent-input__actions">
      <div class="agent-input__model">
        <Select
          id="agent-model"
          :model-value="ui.selectedAgentModel"
          :options="agentModelSelectOptions"
          :aria-label="t('agentInput.modelSelectAria')"
          @update:model-value="ui.setSelectedAgentModel"
        />
      </div>
      <div class="agent-input__right">
        <!-- Hidden file inputs -->
        <input ref="fileInputRef" type="file" class="agent-input__file-input" multiple
          accept="image/*,.pdf,.txt,.md,.doc,.docx,.json,.csv,text/plain,application/pdf"
          tabindex="-1" aria-hidden="true" @change="onFileInputChange" />
        <input ref="imageInputRef" type="file" class="agent-input__file-input" multiple
          accept="image/*" tabindex="-1" aria-hidden="true" @change="onImageInputChange" />
        <input ref="videoInputRef" type="file" class="agent-input__file-input"
          accept="video/mp4,video/webm,video/quicktime" tabindex="-1" aria-hidden="true"
          @change="onVideoInputChange" />

        <!-- Voice -->
        <IconCircleButton
          :pending="voiceRecording" :disabled="disabled"
          :aria-label="voiceRecording ? t('agentInput.recordingAria') : t('agentInput.voiceAria')"
          :icon="voiceRecording ? 'microphone-filled' : 'microphone'" :icon-size="20"
          @click="voiceRecording ? emit('stopVoice') : emit('startVoice')"
        />

        <!-- Video upload -->
        <IconCircleButton
          v-if="showImageUpload" :disabled="disabled"
          :aria-label="t('agentInput.videoUploadAria')" icon="video" :icon-size="20"
          @click="openVideoPicker"
        />

        <!-- Image upload -->
        <IconCircleButton
          v-if="showImageUpload" :disabled="disabled"
          :aria-label="t('agentInput.imageUploadAria')" icon="photo" :icon-size="20"
          @click="openImagePicker"
        />

        <!-- File attach (import mode) -->
        <IconCircleButton
          v-if="showAttach" :disabled="disabled"
          :aria-label="t('agentInput.uploadAria')" icon="cloud-upload" :icon-size="20"
          @click="openAttachPicker"
        />

        <!-- Send / Stop -->
        <IconCircleButton
          :pending="disabled" :disabled="sendButtonDisabled"
          :aria-label="sendAriaLabel"
          :icon="disabled ? 'player-stop' : 'send'" :icon-size="20"
          @click="onSendOrStopClick"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.agent-input { border-radius: 14px; padding: var(--space-3); box-shadow: var(--shadow-float-lg); }
.agent-input__notebook {
  display: flex; align-items: flex-start; gap: 8px; margin: 0 0 8px; padding: 8px 10px;
  border-radius: 10px; background: rgba(60,60,67,0.06); border: 1px solid rgba(60,60,67,0.08);
  font-size: 12px; font-weight: 600; line-height: 1.4; color: var(--color-text);
}
.agent-input__notebook--empty { color: var(--color-text-secondary); font-weight: 600; }
.agent-input__notebook-icon { flex-shrink: 0; margin-top: 1px; opacity: 0.72; }
.agent-input__notebook-text { min-width: 0; flex: 1; }

/* ── Image / Video thumbnails ── */
.agent-input__images { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 8px; }
.agent-input__image-thumb {
  position: relative; width: 64px; height: 64px; border-radius: 8px; overflow: hidden;
  border: 1px solid rgba(60,60,67,0.12); background: rgba(60,60,67,0.04); flex-shrink: 0;
}
.agent-input__image-thumb--error { border-color: rgba(196,30,30,0.35); background: rgba(196,30,30,0.04); }
.agent-input__image-thumb--video { border-color: rgba(99,102,241,0.25); }
.agent-input__image-thumb--url { border-color: rgba(52,199,89,0.25); }
.agent-input__image-preview { width: 100%; height: 100%; object-fit: cover; }
.agent-input__image-video-placeholder {
  width: 100%; height: 100%; display: flex; align-items: center; justify-content: center;
  background: rgba(99,102,241,0.08); color: rgba(99,102,241,0.6);
}
.agent-input__image-status {
  position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center;
  justify-content: center; gap: 2px; background: rgba(0,0,0,0.35); color: #fff;
  font-size: 10px; font-weight: 600;
}
.agent-input__image-type-badge {
  position: absolute; bottom: 2px; left: 2px; padding: 1px 5px; border-radius: 4px;
  background: rgba(0,0,0,0.55); color: #fff; font-size: 9px; font-weight: 600; line-height: 1.4;
}
.agent-input__image-spinner {
  width: 14px; height: 14px; border: 2px solid rgba(255,255,255,0.35); border-top-color: #fff;
  border-radius: 50%; animation: agent-input-spin 0.7s linear infinite;
}
@keyframes agent-input-spin { to { transform: rotate(360deg); } }
.agent-input__image-remove {
  position: absolute; top: 2px; right: 2px; width: 18px; height: 18px; display: flex;
  align-items: center; justify-content: center; border: none; border-radius: 50%;
  background: rgba(0,0,0,0.5); color: #fff; font-size: 12px; line-height: 1; cursor: pointer; padding: 0;
}
.agent-input__image-remove:hover { background: rgba(196,30,30,0.75); }

/* ── URL row ── */
.agent-input__url-row { display: flex; gap: 6px; margin-bottom: 8px; }
.agent-input__url-field {
  flex: 1; min-width: 0; padding: 6px 10px; border: 1px solid rgba(60,60,67,0.14); border-radius: 8px;
  font-size: 12px; line-height: 1.4; color: var(--color-text); background: rgba(255,255,255,0.6);
  outline: none;
}
.agent-input__url-field::placeholder { color: var(--color-text-secondary); opacity: 0.65; }
.agent-input__url-field:disabled { opacity: 0.45; }
.agent-input__url-add {
  flex-shrink: 0; padding: 6px 10px; border: 1px solid rgba(0,122,255,0.22); border-radius: 8px;
  background: rgba(0,122,255,0.08); color: var(--color-accent); font-size: 12px; font-weight: 600;
  cursor: pointer;
}
.agent-input__url-add:disabled { opacity: 0.35; cursor: not-allowed; }
.agent-input__url-add:hover:not(:disabled) { background: rgba(0,122,255,0.14); }

.agent-input__field {
  display: block; width: 100%; box-sizing: border-box; margin: 0; padding: 8px;
  border: none; border-radius: 0; background: transparent; resize: vertical;
  min-height: 72px; max-height: 200px; font: inherit; font-size: 14px; line-height: 1.45;
  color: var(--color-text); outline: none;
}
.agent-input__field::placeholder { color: var(--color-text-secondary); opacity: 0.85; }
.agent-input__field:disabled { opacity: 0.55; cursor: not-allowed; }
.agent-input__actions { display: flex; align-items: flex-end; justify-content: space-between; gap: var(--space-3); margin-top: var(--space-2); flex-wrap: wrap; }
.agent-input__model { display: flex; align-items: center; gap: 8px; min-width: 0; }
.agent-input__right { position: relative; display: flex; align-items: center; gap: var(--space-2); margin-left: auto; }
.agent-input__file-input { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); white-space: nowrap; border: 0; }
</style>
