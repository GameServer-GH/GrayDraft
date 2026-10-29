<script setup lang="ts">
import { watch, nextTick, ref, useId } from 'vue'

const props = withDefaults(
  defineProps<{
    title: string
    fieldLabel: string
    confirmLabel: string
    cancelLabel: string
    /** 与 TipsMenu 等高层 z-index 对齐，须高于 `10050` */
    zIndex?: number
    busy?: boolean
  }>(),
  {
    zIndex: 11000,
    busy: false,
  },
)

const emit = defineEmits<{
  confirm: [value: string]
  cancel: []
}>()

const open = defineModel<boolean>({ required: true })
const text = defineModel<string>('text', { required: true })

const titleId = useId()
const fieldId = useId()
const inputRef = ref<HTMLInputElement | null>(null)

watch(open, async (v) => {
  if (!v) return
  await nextTick()
  inputRef.value?.focus()
  inputRef.value?.select()
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
  emit('confirm', text.value.trim())
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
      class="form-text-dlg"
      :style="{ zIndex: String(zIndex) }"
      @keydown="onKeydown"
    >
      <div
        class="form-text-dlg__backdrop"
        aria-hidden="true"
        @pointerdown="onBackdropPointerDown"
      />
      <form
        role="dialog"
        aria-modal="true"
        class="form-text-dlg__panel glass-panel"
        :aria-labelledby="titleId"
        @submit.prevent="onConfirm"
      >
        <h2 :id="titleId" class="form-text-dlg__title">{{ title }}</h2>
        <div class="form-text-dlg__field">
          <label :for="fieldId" class="form-text-dlg__label">{{ fieldLabel }}</label>
          <input
            :id="fieldId"
            ref="inputRef"
            v-model="text"
            type="text"
            class="form-text-dlg__input"
            autocomplete="off"
            :disabled="busy"
          />
        </div>
        <div class="form-text-dlg__actions">
          <button
            type="button"
            class="form-text-dlg__btn form-text-dlg__btn--secondary"
            :disabled="busy"
            @click="onCancel"
          >
            {{ cancelLabel }}
          </button>
          <button type="submit" class="form-text-dlg__btn" :disabled="busy">
            {{ confirmLabel }}
          </button>
        </div>
      </form>
    </div>
  </Teleport>
</template>

<style scoped>
.form-text-dlg {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-4);
  box-sizing: border-box;
}

.form-text-dlg__backdrop {
  position: absolute;
  inset: 0;
  background: rgba(28, 28, 32, 0.38);
  backdrop-filter: blur(10px) saturate(1.1);
  -webkit-backdrop-filter: blur(10px) saturate(1.1);
}

.form-text-dlg__panel {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 400px;
  padding: var(--space-4) var(--space-5);
  border-radius: 14px;
  box-sizing: border-box;
  box-shadow: var(--shadow-float-lg);
  background: var(--glass-bg-deep);
}

.form-text-dlg__title {
  margin: 0 0 var(--space-3);
  font-size: 17px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--color-text);
}

.form-text-dlg__field {
  margin-bottom: var(--space-4);
}

.form-text-dlg__label {
  display: block;
  margin-bottom: var(--space-2);
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.form-text-dlg__input {
  box-sizing: border-box;
  width: 100%;
  min-height: 40px;
  padding: 8px 12px;
  border-radius: 10px;
  border: none;
  font: inherit;
  font-size: 14px;
  color: var(--color-text);
  background: rgba(255, 255, 255, 0.55);
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.08);
}

.form-text-dlg__input:focus {
  outline: none;
  box-shadow:
    inset 0 0 0 1px rgba(0, 122, 255, 0.35),
    0 0 0 2px rgba(0, 122, 255, 0.12);
}

.form-text-dlg__input:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.form-text-dlg__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}

.form-text-dlg__btn {
  min-height: 40px;
  padding: 0 16px;
  border-radius: 10px;
  border: none;
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition:
    filter 0.14s ease,
    opacity 0.16s ease,
    transform 0.12s ease;
}

.form-text-dlg__btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.form-text-dlg__btn--secondary {
  color: var(--color-text);
  background: rgba(255, 255, 255, 0.5);
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.06);
}

.form-text-dlg__btn--secondary:hover:not(:disabled) {
  filter: brightness(1.04);
}

.form-text-dlg__btn[type='submit'] {
  color: #fff;
  background: var(--color-accent);
  box-shadow: 0 4px 14px rgba(0, 122, 255, 0.28);
}

.form-text-dlg__btn[type='submit']:hover:not(:disabled) {
  filter: brightness(1.05);
}

.form-text-dlg__btn:active:not(:disabled) {
  transform: scale(0.98);
}

.form-text-dlg__btn:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px rgba(0, 122, 255, 0.25);
}
</style>
