<script setup lang="ts">
import { ref, computed, watch, onUnmounted, nextTick } from 'vue'
import Icon from '@/components/common/Icon.vue'

export interface FormSelectOption {
  value: string
  label: string
  /** 卡片副标题，可选 */
  description?: string
}

const GAP = 6
const VIEW_MARGIN = 8
/** 单行选项大致高度 + gap，用于打开前估算上下翻转 */
const EST_ROW_H = 56

const props = withDefaults(
  defineProps<{
    modelValue: string
    options: FormSelectOption[]
    disabled?: boolean
    ariaLabel?: string
    id?: string
  }>(),
  { disabled: false },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const open = ref(false)
const rootRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const panelRef = ref<HTMLElement | null>(null)

/** 下拉相对触发器：下方展开 vs 上方展开（影响进入动画与阶梯方向） */
const placementDir = ref<'below' | 'above'>('below')

const panelPositionStyle = ref<Record<string, string>>({})

const selected = computed(() => props.options.find((o) => o.value === props.modelValue))

const selectedLabel = computed(() => selected.value?.label ?? props.modelValue)

const panelTransitionName = computed(() =>
  placementDir.value === 'below' ? 'form-select-panel-down' : 'form-select-panel-up',
)

function staggerIndex(i: number) {
  const n = props.options.length
  if (n <= 1) return 0
  return placementDir.value === 'above' ? n - 1 - i : i
}

function viewportSize() {
  const vv = window.visualViewport
  return {
    vw: vv?.width ?? window.innerWidth,
    vh: vv?.height ?? window.innerHeight,
  }
}

/** 打开前估算上下，让首帧进入动画方向大致正确 */
function estimatePlacementDir() {
  const trigger = triggerRef.value
  if (!trigger || props.options.length === 0) {
    placementDir.value = 'below'
    return
  }
  const tr = trigger.getBoundingClientRect()
  const { vh } = viewportSize()
  const estH = Math.min(360, props.options.length * EST_ROW_H + 28)
  const spaceBelow = vh - tr.bottom - GAP - VIEW_MARGIN
  const spaceAbove = tr.top - GAP - VIEW_MARGIN
  if (estH <= spaceBelow) placementDir.value = 'below'
  else if (estH <= spaceAbove) placementDir.value = 'above'
  else placementDir.value = spaceAbove > spaceBelow ? 'above' : 'below'
}

function updatePlacement() {
  const trigger = triggerRef.value
  const panel = panelRef.value
  if (!trigger || !panel || !open.value) return

  const tr = trigger.getBoundingClientRect()
  const { vw, vh } = viewportSize()
  const m = VIEW_MARGIN
  const gap = GAP

  panel.style.maxHeight = ''

  let naturalH = panel.getBoundingClientRect().height
  const spaceBelow = vh - tr.bottom - gap - m
  const spaceAbove = tr.top - gap - m

  let placeAbove: boolean
  if (naturalH <= spaceBelow) placeAbove = false
  else if (naturalH <= spaceAbove) placeAbove = true
  else placeAbove = spaceAbove > spaceBelow

  const maxH = Math.max(96, placeAbove ? spaceAbove : spaceBelow)
  if (naturalH > maxH) {
    panel.style.maxHeight = `${maxH}px`
  } else {
    panel.style.maxHeight = ''
  }

  const ph = panel.getBoundingClientRect().height
  let top = placeAbove ? tr.top - gap - ph : tr.bottom + gap
  const minTop = m
  const maxTop = vh - m - ph
  if (top < minTop) top = minTop
  if (top > maxTop) top = maxTop

  let width = Math.max(tr.width, 168)
  width = Math.min(width, vw - 2 * m)

  let left = tr.left
  if (left + width > vw - m) left = vw - m - width
  if (left < m) left = m

  placementDir.value = placeAbove ? 'above' : 'below'

  panelPositionStyle.value = {
    top: `${Math.round(top)}px`,
    left: `${Math.round(left)}px`,
    width: `${Math.round(width)}px`,
    zIndex: '10050',
  }
}

function schedulePlacement() {
  nextTick(() => {
    updatePlacement()
    requestAnimationFrame(() => updatePlacement())
  })
}

function toggle() {
  if (props.disabled) return
  if (!open.value) estimatePlacementDir()
  open.value = !open.value
}

function choose(value: string) {
  emit('update:modelValue', value)
  open.value = false
}

function onDocPointerDown(e: PointerEvent) {
  if (!open.value) return
  const t = e.target as Node
  if (rootRef.value?.contains(t) || panelRef.value?.contains(t)) return
  open.value = false
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') open.value = false
}

function bindViewportListeners() {
  window.addEventListener('scroll', updatePlacement, true)
  window.addEventListener('resize', updatePlacement)
  window.visualViewport?.addEventListener('resize', updatePlacement)
  window.visualViewport?.addEventListener('scroll', updatePlacement)
}

function unbindViewportListeners() {
  window.removeEventListener('scroll', updatePlacement, true)
  window.removeEventListener('resize', updatePlacement)
  window.visualViewport?.removeEventListener('resize', updatePlacement)
  window.visualViewport?.removeEventListener('scroll', updatePlacement)
}

watch(open, (v) => {
  if (v) {
    document.addEventListener('pointerdown', onDocPointerDown, true)
    document.addEventListener('keydown', onKeydown)
    bindViewportListeners()
    schedulePlacement()
  } else {
    document.removeEventListener('pointerdown', onDocPointerDown, true)
    document.removeEventListener('keydown', onKeydown)
    unbindViewportListeners()
    panelPositionStyle.value = {}
  }
})

watch(
  () => props.options.length,
  () => {
    if (open.value) schedulePlacement()
  },
)

onUnmounted(() => {
  document.removeEventListener('pointerdown', onDocPointerDown, true)
  document.removeEventListener('keydown', onKeydown)
  unbindViewportListeners()
})
</script>

<template>
  <div
    ref="rootRef"
    class="form-select"
    :class="{ 'form-select--open': open, 'form-select--disabled': disabled }"
  >
    <button
      ref="triggerRef"
      :id="id"
      type="button"
      class="form-select__trigger"
      :class="{ 'form-select__trigger--pressed': open }"
      :disabled="disabled"
      :aria-label="ariaLabel"
      aria-haspopup="listbox"
      :aria-expanded="open"
      @click="toggle"
    >
      <span class="form-select__trigger-text" :title="selectedLabel">{{ selectedLabel }}</span>
      <Icon
        class="form-select__chevron"
        :class="{ 'form-select__chevron--up': open }"
        name="chevron-down"
        :size="16"
      />
    </button>

    <!-- Teleport 到 body：避免祖先 transform（如 #gd-compose-root）使 fixed 定位参考系与 getBoundingClientRect 不一致 -->
    <Teleport to="body">
      <Transition :name="panelTransitionName">
        <div
          v-if="open"
          ref="panelRef"
          class="form-select__panel"
          :class="{ 'form-select__panel--flip-up': placementDir === 'above' }"
          role="listbox"
          :aria-label="ariaLabel"
          :style="panelPositionStyle"
        >
          <button
            v-for="(opt, i) in options"
            :key="opt.value"
            type="button"
            class="form-select__option"
            :class="{ 'form-select__option--active': opt.value === modelValue }"
            :style="{ '--i': staggerIndex(i) }"
            role="option"
            :aria-selected="opt.value === modelValue"
            @click="choose(opt.value)"
          >
            <span class="form-select__option-main">
              <span class="form-select__option-label">{{ opt.label }}</span>
            </span>
            <Icon
              v-if="opt.value === modelValue"
              class="form-select__option-check"
              name="check"
              :size="16"
            />
          </button>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.form-select {
  position: relative;
  display: inline-flex;
  min-width: 128px;
  vertical-align: middle;
}

.form-select--disabled {
  opacity: 0.55;
  pointer-events: none;
}

.form-select__trigger {
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  min-height: 34px;
  padding: 6px 10px 6px 12px;
  font-size: 13px;
  font-weight: 600;
  font-family: inherit;
  line-height: 1.2;
  color: var(--color-text);
  cursor: pointer;
  border-radius: 30px;
  border: 1px solid var(--color-border);
  background: rgba(255, 255, 255, 0.85);
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.6) inset;
  transition:
    border-color 0.18s ease,
    box-shadow 0.18s ease,
    transform 0.12s ease,
    background 0.18s ease;
}

.form-select__trigger:hover:not(:disabled) {
  border-color: var(--color-border-strong);
  background: rgba(255, 255, 255, 0.95);
}

.form-select__trigger:active:not(:disabled) {
  transform: scale(0.97);
}

.form-select__trigger:focus-visible {
  outline: none;
  border-color: var(--color-selected-line);
  box-shadow: 0 0 0 2px var(--color-selected-soft);
}

.form-select__trigger--pressed:not(:disabled) {
  border-color: var(--color-selected-line);
  box-shadow: 0 0 0 2px var(--color-selected-soft);
}

.form-select__trigger-text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: left;
}

.form-select__chevron {
  flex-shrink: 0;
  opacity: 0.55;
  transition: transform 0.22s cubic-bezier(0.22, 1, 0.36, 1), opacity 0.18s ease;
}

.form-select__chevron--up {
  transform: rotate(-180deg);
  opacity: 0.85;
}

.form-select__panel {
  position: fixed;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px;
  border-radius: 14px;
  border: 1px solid var(--color-border);
  background: var(--glass-bg-deep);
  backdrop-filter: blur(var(--glass-blur)) saturate(var(--glass-saturation));
  box-shadow: var(--shadow-float-md);
  overflow-x: hidden;
  overflow-y: auto;
  max-width: calc(100vw - 16px);
}

.form-select__option {
  --i: 0;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  width: 100%;
  margin: 0;
  padding: 10px 12px;
  text-align: left;
  font: inherit;
  color: var(--color-text);
  cursor: pointer;
  border: 1px solid var(--color-border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.72);
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.5) inset;
  transition:
    border-color 0.15s ease,
    background 0.15s ease,
    transform 0.12s ease;
  animation: form-select-card-in-down 0.38s cubic-bezier(0.22, 1, 0.36, 1) both;
  animation-delay: calc(var(--i) * 55ms);
}

.form-select__option:hover {
  border-color: rgba(28, 28, 30, 0.16);
  background: #fff;
}

.form-select__option:active {
  transform: scale(0.98);
}

.form-select__option--active {
  border-color: var(--color-selected-line);
  background: #f4f4f5;
  box-shadow: inset 3px 0 0 var(--color-selected);
}

.form-select__option-main {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.form-select__option-label {
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.01em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.form-select__option-desc {
  font-size: 11px;
  line-height: 1.35;
  color: var(--color-text-secondary);
}

.form-select__option-check {
  flex-shrink: 0;
  margin-top: 2px;
  opacity: 0.9;
}

@keyframes form-select-card-in-down {
  from {
    opacity: 0;
    transform: translateY(-10px) scale(0.96);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@keyframes form-select-card-in-up {
  from {
    opacity: 0;
    transform: translateY(10px) scale(0.96);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.form-select__panel--flip-up .form-select__option {
  animation-name: form-select-card-in-up;
}

/* 向下展开：面板从略上方淡入 */
.form-select-panel-down-enter-active {
  transition:
    opacity 0.22s ease,
    transform 0.28s cubic-bezier(0.22, 1, 0.36, 1);
}

.form-select-panel-down-enter-from {
  opacity: 0;
  transform: translateY(-8px);
}

.form-select-panel-down-leave-active {
  transition: opacity 0.14s ease;
}

.form-select-panel-down-leave-to {
  opacity: 0;
}

/* 向上展开：面板从略下方淡入 */
.form-select-panel-up-enter-active {
  transition:
    opacity 0.22s ease,
    transform 0.28s cubic-bezier(0.22, 1, 0.36, 1);
}

.form-select-panel-up-enter-from {
  opacity: 0;
  transform: translateY(8px);
}

.form-select-panel-up-leave-active {
  transition: opacity 0.14s ease;
}

.form-select-panel-up-leave-to {
  opacity: 0;
}

@media (prefers-reduced-motion: reduce) {
  .form-select__trigger:active:not(:disabled) {
    transform: none;
  }

  .form-select__chevron {
    transition: none;
  }

  .form-select__option {
    animation: none;
    animation-delay: 0ms;
  }

  .form-select-panel-down-enter-active,
  .form-select-panel-down-leave-active,
  .form-select-panel-up-enter-active,
  .form-select-panel-up-leave-active {
    transition: none;
  }

  .form-select-panel-down-enter-from,
  .form-select-panel-up-enter-from {
    opacity: 1;
    transform: none;
  }
}
</style>
