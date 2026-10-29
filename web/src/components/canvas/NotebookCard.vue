<script setup lang="ts">
import { computed, ref, onBeforeUnmount } from 'vue'
import Icon from '@/components/common/Icon.vue'
import NotebookHtmlPane from '@/components/canvas/NotebookHtmlPane.vue'
import type { NotebookMeta } from '@/types/notebook'
import type { NotebookCardLayout } from '@/types/canvas'
import { useCanvasStore, clampViewScale } from '@/stores/canvas'
import { useI18n } from '@/composables/useI18n'
import { useWorkspaceSessionStore } from '@/stores/workspaceSession'

const props = defineProps<{
  notebook: NotebookMeta
}>()

const { t } = useI18n()
const wsSession = useWorkspaceSessionStore()
const canvas = useCanvasStore()

const knowledgeCard = computed(() => Boolean(wsSession.nexusKbType))

function formatQuality(v: number | undefined): string | null {
  if (typeof v !== 'number' || !Number.isFinite(v)) return null
  return v.toFixed(2)
}

const isActive = computed(() => canvas.activeNotebookId === props.notebook.id)

const layout = computed(() => canvas.layoutFor(props.notebook.id))

/** 正文区铺满卡片布局尺寸；随画布 world 的 scale 与卡片一起缩放，不在此层做反缩放 */
const noteBodyCounterStyle = computed(() => {
  const { w, h } = layout.value
  return {
    position: 'absolute' as const,
    left: '0',
    top: '0',
    width: `${w}px`,
    height: `${h}px`,
    boxSizing: 'border-box' as const,
  }
})

const cardStyle = computed(() => {
  const L = layout.value
  /** 激活卡片略抬高，仍低于生成蒙层 (80) */
  const z = isActive.value ? 44 : 4
  const base = {
    left: `calc(50% + ${L.cx}px - ${L.w / 2}px)`,
    top: `calc(50% + ${L.cy}px - ${L.h / 2}px)`,
    width: `${L.w}px`,
    height: `${L.h}px`,
    zIndex: z,
  }
  if (!isActive.value) return base
  /** 与标题条 bar 相同思路：world 缩放 s 后，用 1/s 反算线宽，使屏幕上的选中环厚度大致恒定、缩小时仍醒目 */
  const s = clampViewScale(canvas.viewScale)
  const borderWorld = Math.max(1, 1.15 / s)
  const ringWorld = Math.max(1, 2.75 / s)
  const glowWorld = Math.max(6, 14 / s)
  return {
    ...base,
    borderWidth: `${borderWorld}px`,
    borderStyle: 'solid' as const,
    borderColor: 'rgba(124, 58, 237, 0.72)',
    boxShadow: `${'var(--shadow-float-md)'}, 0 0 0 ${ringWorld}px rgba(139, 92, 246, 0.26), 0 0 ${glowWorld}px rgba(124, 58, 237, 0.16)`,
  }
})

/** 标题条与画布缩放反向补偿：缩小画布时标题相对变大，放大画布时相对变小（与 world 的 scale 组合后接近恒定屏幕尺寸） */
const barWrapStyle = computed(() => {
  const s = clampViewScale(canvas.viewScale)
  const inv = 1 / s
  return {
    position: 'absolute' as const,
    left: '0',
    bottom: 'calc(100% + 6px)',
    zIndex: 55,
    width: `calc(100% * ${s})`,
    transform: `scale(${inv})`,
    transformOrigin: 'bottom left' as const,
    boxSizing: 'border-box' as const,
  }
})

type DragKind = 'move' | 'n' | 's' | 'e' | 'w' | 'ne' | 'nw' | 'se' | 'sw'

type DragSession = {
  kind: DragKind
  x0: number
  y0: number
  L0: NotebookCardLayout
  shift: boolean
}

const session = ref<DragSession | null>(null)

function clampSize(w: number, h: number) {
  const MIN_W = 240
  const MIN_H = 160
  const MAX_W = 2400
  const MAX_H = 1800
  return {
    w: Math.min(MAX_W, Math.max(MIN_W, w)),
    h: Math.min(MAX_H, Math.max(MIN_H, h)),
  }
}

function coupleProportional(dw: number, dh: number, w0: number, h0: number) {
  const r = w0 / h0
  if (Math.abs(dw) >= Math.abs(dh * r)) {
    return { dw, dh: dw / r }
  }
  return { dw: dh * r, dh }
}

function computeLayoutFromResize(
  kind: Exclude<DragKind, 'move'>,
  L0: NotebookCardLayout,
  dx: number,
  dy: number,
  shift: boolean,
): NotebookCardLayout {
  let cx = L0.cx
  let cy = L0.cy
  let w = L0.w
  let h = L0.h

  const corner = (dw: number, dh: number) => {
    let cdw = dw
    let cdh = dh
    if (shift) {
      const c = coupleProportional(cdw, cdh, L0.w, L0.h)
      cdw = c.dw
      cdh = c.dh
    }
    w = L0.w + cdw
    h = L0.h + cdh
    cx = L0.cx + cdw / 2
    cy = L0.cy + cdh / 2
  }

  switch (kind) {
    case 'e': {
      const dw = dx
      w = L0.w + dw
      cx = L0.cx + dw / 2
      break
    }
    case 'w': {
      const dw = -dx
      w = L0.w + dw
      cx = L0.cx - dw / 2
      break
    }
    case 's': {
      const dh = dy
      h = L0.h + dh
      cy = L0.cy + dh / 2
      break
    }
    case 'n': {
      const dh = -dy
      h = L0.h + dh
      cy = L0.cy - dh / 2
      break
    }
    case 'se':
      corner(dx, dy)
      break
    case 'nw':
      corner(-dx, -dy)
      break
    case 'ne':
      corner(dx, -dy)
      break
    case 'sw':
      corner(-dx, dy)
      break
    default:
      break
  }

  const cl = clampSize(w, h)
  w = cl.w
  h = cl.h
  return { cx, cy, w, h }
}

function onMove(e: PointerEvent) {
  const s = session.value
  if (!s) return
  const sc = canvas.viewScale
  const dx = (e.clientX - s.x0) / sc
  const dy = (e.clientY - s.y0) / sc

  if (s.kind === 'move') {
    canvas.patchNotebookLayout(props.notebook.id, {
      cx: s.L0.cx + dx,
      cy: s.L0.cy + dy,
    })
    return
  }

  const next = computeLayoutFromResize(s.kind, s.L0, dx, dy, e.shiftKey)
  canvas.patchNotebookLayout(props.notebook.id, next)
}

function onUp(_e: PointerEvent) {
  if (!session.value) return
  session.value = null
  window.removeEventListener('pointermove', onMove)
  window.removeEventListener('pointerup', onUp)
  window.removeEventListener('pointercancel', onUp)
}

function startMove(e: PointerEvent) {
  if (e.button !== 0) return
  e.preventDefault()
  e.stopPropagation()
  canvas.selectNotebook(props.notebook.id)
  session.value = {
    kind: 'move',
    x0: e.clientX,
    y0: e.clientY,
    L0: { ...layout.value },
    shift: e.shiftKey,
  }
  window.addEventListener('pointermove', onMove)
  window.addEventListener('pointerup', onUp)
  window.addEventListener('pointercancel', onUp)
}

function startResize(kind: Exclude<DragKind, 'move'>, e: PointerEvent) {
  if (e.button !== 0) return
  e.preventDefault()
  e.stopPropagation()
  canvas.selectNotebook(props.notebook.id)
  session.value = {
    kind,
    x0: e.clientX,
    y0: e.clientY,
    L0: { ...layout.value },
    shift: e.shiftKey,
  }
  window.addEventListener('pointermove', onMove)
  window.addEventListener('pointerup', onUp)
  window.addEventListener('pointercancel', onUp)
}

function onCardPointerDown(e: PointerEvent) {
  if (e.button === 0) canvas.selectNotebook(props.notebook.id)
}

const resizeHandles = ['n', 's', 'e', 'w', 'ne', 'nw', 'se', 'sw'] as const

onBeforeUnmount(() => {
  window.removeEventListener('pointermove', onMove)
  window.removeEventListener('pointerup', onUp)
  window.removeEventListener('pointercancel', onUp)
})
</script>

<template>
  <article
    class="notebook-card"
    :class="{ 'notebook-card--active': isActive }"
    :style="cardStyle"
    :data-notebook-id="notebook.id"
    @pointerdown="onCardPointerDown"
  >
    <div class="notebook-card__bar-scaler" :style="barWrapStyle">
      <header class="notebook-card__bar" @pointerdown="startMove">
        <span class="notebook-card__title">
          <Icon class="notebook-card__title-icon" name="file-description" :size="15" />
          <span class="notebook-card__title-text" :title="notebook.title">{{ notebook.title }}</span>
        </span>
        <span v-if="knowledgeCard && notebook.indexReady !== undefined" class="notebook-card__badges">
          <span
            class="notebook-card__badge"
            :class="notebook.indexReady ? 'notebook-card__badge--ok' : 'notebook-card__badge--pending'"
            :title="notebook.indexReady ? t('knowledge.statusReady') : t('knowledge.statusIndexing')"
          />
          <span
            v-if="formatQuality(notebook.qualityScore)"
            class="notebook-card__badge notebook-card__badge--score"
          >
            {{ formatQuality(notebook.qualityScore) }}
          </span>
          <span
            v-if="notebook.chunkCount != null && notebook.chunkCount > 0"
            class="notebook-card__badge notebook-card__badge--muted"
          >
            {{ notebook.chunkCount }}
          </span>
        </span>
      </header>
    </div>
    <div class="notebook-card__body">
      <div class="notebook-card__body-counter" :style="noteBodyCounterStyle">
        <NotebookHtmlPane :notebook-id="notebook.id" />
      </div>
    </div>

    <button
      v-for="k in resizeHandles"
      :key="k"
      type="button"
      class="notebook-card__handle"
      :class="`notebook-card__handle--${k}`"
      :aria-label="t('notebookCard.resizeAria', { handle: k })"
      @pointerdown="startResize(k, $event)"
    />
  </article>
</template>

<style scoped>
.notebook-card {
  position: absolute;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.92);
  border: 1px solid var(--color-border);
  box-shadow: var(--shadow-float-md);
  overflow: visible;
  user-select: none;
}

.notebook-card__bar-scaler {
  pointer-events: auto;
}

.notebook-card__bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  width: 100%;
  max-width: 100%;
  min-width: 0;
  height: 41px;
  box-sizing: border-box;
  padding: 0 12px;
  cursor: grab;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  text-shadow:
    0 0 10px rgba(255, 255, 255, 0.95),
    0 1px 2px rgba(255, 255, 255, 0.85);
}

.notebook-card__bar:active {
  cursor: grabbing;
}

.notebook-card__title {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  flex: 1;
  overflow: hidden;
}

.notebook-card__title-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.notebook-card__title-icon {
  flex-shrink: 0;
  opacity: 0.55;
}

.notebook-card__badges {
  display: flex;
  align-items: center;
  gap: 5px;
  flex-shrink: 0;
}

.notebook-card__badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 8px;
  height: 8px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 700;
  line-height: 1;
}

.notebook-card__badge--ok {
  background: #34c759;
  box-shadow: 0 0 0 2px rgba(52, 199, 89, 0.25);
}

.notebook-card__badge--pending {
  background: #ff9500;
  animation: nb-pulse 1.2s ease-in-out infinite;
}

@keyframes nb-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.45;
  }
}

.notebook-card__badge--score {
  min-width: auto;
  height: auto;
  padding: 2px 6px;
  border-radius: 6px;
  background: rgba(60, 60, 67, 0.1);
  color: var(--color-text-secondary);
}

.notebook-card__badge--muted {
  min-width: auto;
  height: auto;
  padding: 2px 6px;
  border-radius: 6px;
  background: rgba(60, 60, 67, 0.08);
  color: var(--color-text-secondary);
}

.notebook-card__hint {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-secondary);
  text-shadow:
    0 0 8px rgba(255, 255, 255, 0.92),
    0 1px 2px rgba(255, 255, 255, 0.8);
}
.notebook-card__hint-icon {
  flex-shrink: 0;
  opacity: 0.55;
}

.notebook-card__body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  position: relative;
  border-radius: 12px;
  overflow: hidden;
  background: #fff;
}

.notebook-card__body-counter {
  box-sizing: border-box;
}

.notebook-card__handle {
  position: absolute;
  padding: 0;
  margin: 0;
  border: none;
  background: transparent;
  z-index: 60;
}

.notebook-card__handle--n {
  top: -4px;
  left: 12px;
  right: 12px;
  height: 8px;
  cursor: ns-resize;
}

.notebook-card__handle--s {
  bottom: -4px;
  left: 12px;
  right: 12px;
  height: 8px;
  cursor: ns-resize;
}

.notebook-card__handle--e {
  right: -4px;
  top: 12px;
  bottom: 12px;
  width: 8px;
  cursor: ew-resize;
}

.notebook-card__handle--w {
  left: -4px;
  top: 12px;
  bottom: 12px;
  width: 8px;
  cursor: ew-resize;
}

.notebook-card__handle--ne {
  top: -5px;
  right: -5px;
  width: 12px;
  height: 12px;
  cursor: nesw-resize;
}

.notebook-card__handle--nw {
  top: -5px;
  left: -5px;
  width: 12px;
  height: 12px;
  cursor: nwse-resize;
}

.notebook-card__handle--se {
  bottom: -5px;
  right: -5px;
  width: 12px;
  height: 12px;
  cursor: nwse-resize;
}

.notebook-card__handle--sw {
  bottom: -5px;
  left: -5px;
  width: 12px;
  height: 12px;
  cursor: nesw-resize;
}

.notebook-card__handle:hover {
  background: rgba(124, 58, 237, 0.14);
  border-radius: 4px;
}
</style>
