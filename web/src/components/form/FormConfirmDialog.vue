<script setup lang="ts">
import { watch, nextTick, ref, useId } from 'vue'

const props = withDefaults(
  defineProps<{
    title: string
    message: string
    confirmLabel: string
    cancelLabel: string
    /** 主按钮为危险操作样式（如删除） */
    confirmDanger?: boolean
    /** 与 TipsMenu 等高层 z-index 对齐，须高于 `10050` */
    zIndex?: number
    busy?: boolean
  }>(),
  {
    confirmDanger: false,
    zIndex: 11000,
    busy: false,
  },
)

const emit = defineEmits<{
  confirm: []
  cancel: []
}>()

const open = defineModel<boolean>({ required: true })

const titleId = useId()
const cancelRef = ref<HTMLButtonElement | null>(null)

watch(open, async (v) => {
  if (!v) return
  await nextTick()
  cancelRef.value?.focus()
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
  emit('confirm')
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
      class="form-confirm-dlg"
      :style="{ zIndex: String(zIndex) }"
      @keydown="onKeydown"
    >
      <div
        class="form-confirm-dlg__backdrop"
        aria-hidden="true"
        @pointerdown="onBackdropPointerDown"
      />
      <div
        role="dialog"
        aria-modal="true"
        class="form-confirm-dlg__panel glass-panel"
        :aria-labelledby="titleId"
      >
        <h2 :id="titleId" class="form-confirm-dlg__title">{{ title }}</h2>
        <p class="form-confirm-dlg__message">{{ message }}</p>
        <div class="form-confirm-dlg__actions">
          <button
            ref="cancelRef"
            type="button"
            class="form-confirm-dlg__btn form-confirm-dlg__btn--secondary"
            :disabled="busy"
            @click="onCancel"
          >
            {{ cancelLabel }}
          </button>
          <button
            type="button"
            class="form-confirm-dlg__btn"
            :class="{ 'form-confirm-dlg__btn--danger': confirmDanger }"
            :disabled="busy"
            @click="onConfirm"
          >
            {{ confirmLabel }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.form-confirm-dlg {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-4);
  box-sizing: border-box;
}

.form-confirm-dlg__backdrop {
  position: absolute;
  inset: 0;
  background: rgba(28, 28, 32, 0.38);
  backdrop-filter: blur(10px) saturate(1.1);
  -webkit-backdrop-filter: blur(10px) saturate(1.1);
}

.form-confirm-dlg__panel {
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

.form-confirm-dlg__title {
  margin: 0 0 var(--space-2);
  font-size: 17px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--color-text);
}

.form-confirm-dlg__message {
  margin: 0 0 var(--space-4);
  font-size: 14px;
  line-height: 1.45;
  color: var(--color-text-secondary);
}

.form-confirm-dlg__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}

.form-confirm-dlg__btn {
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

.form-confirm-dlg__btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.form-confirm-dlg__btn--secondary {
  color: var(--color-text);
  background: rgba(255, 255, 255, 0.5);
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.06);
}

.form-confirm-dlg__btn--secondary:hover:not(:disabled) {
  filter: brightness(1.04);
}

.form-confirm-dlg__btn:not(.form-confirm-dlg__btn--secondary) {
  color: #fff;
  background: var(--color-accent);
  box-shadow: 0 4px 14px rgba(0, 122, 255, 0.28);
}

.form-confirm-dlg__btn:not(.form-confirm-dlg__btn--secondary):hover:not(:disabled) {
  filter: brightness(1.05);
}

.form-confirm-dlg__btn--danger {
  background: #d92d20 !important;
  box-shadow: 0 4px 14px rgba(217, 45, 32, 0.35) !important;
}

.form-confirm-dlg__btn:active:not(:disabled) {
  transform: scale(0.98);
}

.form-confirm-dlg__btn:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px rgba(0, 122, 255, 0.25);
}

.form-confirm-dlg__btn--danger:focus-visible {
  box-shadow: 0 0 0 3px rgba(217, 45, 32, 0.35);
}
</style>
