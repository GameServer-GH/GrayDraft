<script lang="ts">
export interface TipsMenuAction {
  id: string
  label: string
  description?: string
  /** 危险操作（如删除），菜单项使用警示样式 */
  danger?: boolean
}
</script>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted, nextTick } from 'vue'
import Icon from '@/components/common/Icon.vue'

const GAP = 6
const VIEW_MARGIN = 8
const EST_ROW_H = 48
const CLOSE_DELAY_MS = 140

const props = withDefaults(
  defineProps<{
    /** 触发器上可见文案，同时作为 aria-label（与侧栏 span 二合一） */
    ariaLabel: string
    /** 下拉面板完整 aria-label；不传时回退为「{ariaLabel}菜单」 */
    panelAriaLabel?: string
    actions: TipsMenuAction[]
    disabled?: boolean
    id?: string
  }>(),
  { disabled: false },
)

const emit = defineEmits<{
  action: [id: string]
}>()

const open = ref(false)
const rootRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const panelRef = ref<HTMLElement | null>(null)
const placementDir = ref<'below' | 'above'>('below')
const panelPositionStyle = ref<Record<string, string>>({})

let closeTimer: ReturnType<typeof setTimeout> | null = null

const panelTransitionName = computed(() =>
  placementDir.value === 'below' ? 'form-tips-menu-panel-down' : 'form-tips-menu-panel-up',
)

function clearCloseTimer() {
  if (closeTimer) {
    clearTimeout(closeTimer)
    closeTimer = null
  }
}

function armClose() {
  clearCloseTimer()
  closeTimer = setTimeout(() => {
    closeTimer = null
    open.value = false
  }, CLOSE_DELAY_MS)
}

function viewportSize() {
  const vv = window.visualViewport
  return {
    vw: vv?.width ?? window.innerWidth,
    vh: vv?.height ?? window.innerHeight,
  }
}

function estimatePlacementDir() {
  const trigger = triggerRef.value
  if (!trigger || props.actions.length === 0) {
    placementDir.value = 'below'
    return
  }
  const tr = trigger.getBoundingClientRect()
  const { vh } = viewportSize()
  const estH = Math.min(360, props.actions.length * EST_ROW_H + 24)
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

  let width = Math.max(tr.width, 160)
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

function onTriggerEnter() {
  if (props.disabled) return
  clearCloseTimer()
  if (!open.value) estimatePlacementDir()
  open.value = true
}

function onTriggerLeave() {
  armClose()
}

function onPanelEnter() {
  clearCloseTimer()
}

function onPanelLeave() {
  armClose()
}

function onTriggerClick() {
  if (props.disabled) return
  clearCloseTimer()
  if (!open.value) {
    estimatePlacementDir()
    open.value = true
  } else {
    open.value = false
  }
}

function onAction(id: string) {
  emit('action', id)
  clearCloseTimer()
  open.value = false
}

function onDocPointerDown(e: PointerEvent) {
  if (!open.value) return
  const t = e.target as Node
  if (rootRef.value?.contains(t) || panelRef.value?.contains(t)) return
  clearCloseTimer()
  open.value = false
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    clearCloseTimer()
    open.value = false
  }
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
  () => props.actions.length,
  () => {
    if (open.value) schedulePlacement()
  },
)

onUnmounted(() => {
  clearCloseTimer()
  document.removeEventListener('pointerdown', onDocPointerDown, true)
  document.removeEventListener('keydown', onKeydown)
  unbindViewportListeners()
})
</script>

<template>
  <div
    ref="rootRef"
    class="form-tips-menu"
    :class="{ 'form-tips-menu--open': open, 'form-tips-menu--disabled': disabled }"
  >
    <button
      ref="triggerRef"
      :id="id"
      type="button"
      class="form-tips-menu__trigger"
      :class="{ 'form-tips-menu__trigger--open': open }"
      :disabled="disabled"
      :aria-label="ariaLabel"
      aria-haspopup="menu"
      :aria-expanded="open"
      @mouseenter="onTriggerEnter"
      @mouseleave="onTriggerLeave"
      @click="onTriggerClick"
    >
      <span class="form-tips-menu__trigger-text">{{ ariaLabel }}</span>
      <Icon
        class="form-tips-menu__chevron"
        name="chevron-down"
        :size="14"
        aria-hidden="true"
      />
    </button>

    <Teleport to="body">
      <Transition :name="panelTransitionName">
        <div
          v-if="open"
          ref="panelRef"
          class="form-tips-menu__panel"
          :class="{ 'form-tips-menu__panel--flip-up': placementDir === 'above' }"
          role="menu"
          :aria-label="panelAriaLabel ?? `${ariaLabel}菜单`"
          :style="panelPositionStyle"
          @mouseenter="onPanelEnter"
          @mouseleave="onPanelLeave"
        >
          <button
            v-for="(act, i) in actions"
            :key="act.id"
            type="button"
            class="form-tips-menu__action"
            :class="{ 'form-tips-menu__action--danger': act.danger }"
            :style="{ '--i': i }"
            role="menuitem"
            @click="onAction(act.id)"
          >
            <span class="form-tips-menu__action-main">
              <span class="form-tips-menu__action-label">{{ act.label }}</span>
              <span v-if="act.description" class="form-tips-menu__action-desc">{{ act.description }}</span>
            </span>
          </button>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.form-tips-menu {
  position: relative;
  display: inline-flex;
  min-width: 0;
  vertical-align: middle;
}

.form-tips-menu--disabled {
  opacity: 0.55;
  pointer-events: none;
}

.form-tips-menu__trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  min-height: 32px;
  padding: 6px 10px;
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  font-family: inherit;
  line-height: 1.25;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--color-text-secondary);
  cursor: pointer;
  border-radius: 10px;
  border: none;
  background: transparent;
  box-shadow: none;
  transition:
    color 0.15s ease,
    background 0.15s ease;
}

.form-tips-menu__trigger:hover:not(:disabled),
.form-tips-menu__trigger--open:not(:disabled) {
  color: var(--color-text);
  background: rgba(255, 255, 255, 0.42);
}

.form-tips-menu__trigger:focus-visible {
  outline: none;
  color: var(--color-text);
  background: rgba(255, 255, 255, 0.42);
  box-shadow: 0 0 0 2px rgba(0, 122, 255, 0.18);
}

.form-tips-menu__trigger-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.form-tips-menu__chevron {
  flex-shrink: 0;
  opacity: 0.45;
  transition: opacity 0.15s ease;
}

.form-tips-menu__trigger:hover:not(:disabled) .form-tips-menu__chevron,
.form-tips-menu__trigger--open:not(:disabled) .form-tips-menu__chevron {
  opacity: 0.72;
}

.form-tips-menu__panel {
  position: fixed;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 6px;
  border-radius: 12px;
  border: none;
  background: rgba(255, 255, 255, 0.55);
  backdrop-filter: blur(var(--glass-blur)) saturate(var(--glass-saturation));
  box-shadow: var(--shadow-float-md);
  overflow-x: hidden;
  overflow-y: auto;
  max-width: calc(100vw - 16px);
}

.form-tips-menu__action {
  --i: 0;
  display: flex;
  align-items: center;
  width: 100%;
  margin: 0;
  padding: 9px 11px;
  text-align: left;
  font: inherit;
  color: var(--color-text);
  cursor: pointer;
  border: none;
  border-radius: 8px;
  background: transparent;
  box-shadow: none;
  transition:
    color 0.12s ease,
    background 0.12s ease,
    transform 0.1s ease;
  animation: form-tips-menu-item-in-down 0.26s cubic-bezier(0.22, 1, 0.36, 1) both;
  animation-delay: calc(var(--i) * 40ms);
}

.form-tips-menu__action:hover {
  background: rgba(255, 255, 255, 0.72);
}

.form-tips-menu__action--danger .form-tips-menu__action-label {
  color: #c41e1e;
}

.form-tips-menu__action--danger .form-tips-menu__action-desc {
  color: rgba(196, 30, 30, 0.75);
}

.form-tips-menu__action--danger:hover {
  background: rgba(217, 45, 32, 0.1);
}

.form-tips-menu__action:active {
  transform: scale(0.98);
}

.form-tips-menu__action-main {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.form-tips-menu__action-label {
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.01em;
}

.form-tips-menu__action-desc {
  font-size: 11px;
  line-height: 1.35;
  color: var(--color-text-secondary);
}

@keyframes form-tips-menu-item-in-down {
  from {
    opacity: 0;
    transform: translateY(-6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes form-tips-menu-item-in-up {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.form-tips-menu__panel--flip-up .form-tips-menu__action {
  animation-name: form-tips-menu-item-in-up;
}

.form-tips-menu-panel-down-enter-active {
  transition:
    opacity 0.18s ease,
    transform 0.22s cubic-bezier(0.22, 1, 0.36, 1);
}

.form-tips-menu-panel-down-enter-from {
  opacity: 0;
  transform: translateY(-4px);
}

.form-tips-menu-panel-down-leave-active {
  transition: opacity 0.1s ease;
}

.form-tips-menu-panel-down-leave-to {
  opacity: 0;
}

.form-tips-menu-panel-up-enter-active {
  transition:
    opacity 0.18s ease,
    transform 0.22s cubic-bezier(0.22, 1, 0.36, 1);
}

.form-tips-menu-panel-up-enter-from {
  opacity: 0;
  transform: translateY(4px);
}

.form-tips-menu-panel-up-leave-active {
  transition: opacity 0.1s ease;
}

.form-tips-menu-panel-up-leave-to {
  opacity: 0;
}

@media (prefers-reduced-motion: reduce) {
  .form-tips-menu__action {
    animation: none;
    animation-delay: 0ms;
  }

  .form-tips-menu-panel-down-enter-active,
  .form-tips-menu-panel-down-leave-active,
  .form-tips-menu-panel-up-enter-active,
  .form-tips-menu-panel-up-leave-active {
    transition: none;
  }

  .form-tips-menu-panel-down-enter-from,
  .form-tips-menu-panel-up-enter-from {
    opacity: 1;
    transform: none;
  }
}
</style>
