<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount, watch } from 'vue'
import Icon from '@/components/common/Icon.vue'
import { useCanvasRenderer } from '@/composables/useCanvasRenderer'
import { useCanvasStore, WHEEL_ZOOM_STEP } from '@/stores/canvas'
import { useAgentStore } from '@/stores/agent'
import NotebookCard from '@/components/canvas/NotebookCard.vue'
import KnowledgeGalaxyCanvas from '@/components/knowledge/KnowledgeGalaxyCanvas.vue'
import { useKnowledgePanel } from '@/composables/useKnowledgePanel'
import { useKnowledgeGalaxy } from '@/composables/useKnowledgeGalaxy'
import { useI18n } from '@/composables/useI18n'
import { GALAXY_VIEW_BASE_SCALE, groupColor } from '@/utils/knowledgeGalaxy'

const emit = defineEmits<{
  openKnowledgeDetail: [docId: string]
}>()

const { t } = useI18n()
const canvas = useCanvasStore()
const agent = useAgentStore()
const knowledge = useKnowledgePanel()
const galaxy = useKnowledgeGalaxy(() => knowledge.enabled.value)
const viewportEl = ref<HTMLElement | null>(null)

const legendGroups = computed(() =>
  [...new Set((galaxy.layout.value?.nodes ?? []).map((n) => n.group))].slice(0, 8),
)

function onGalaxyNodeDetail(docId: string) {
  emit('openKnowledgeDetail', docId)
}

defineExpose({
  reloadGalaxy: galaxy.reload,
  syncGalaxy: galaxy.sync,
  /** 星系模式下将视口对准当前选中节点 */
  focusActiveGalaxyNode: () => { focusGalaxyNodeToActive() },
})
/** Agent 生成中：叠在正在生成的笔记卡片上（含未选中），仅视觉；z 高于任意卡片的选中态 */
const generatingOverlayStyle = computed(() => {
  if (!agent.streaming) return null
  const id = agent.streamingNotebookId ?? canvas.activeNotebookId
  if (!id || !canvas.notebooks.some((n) => n.id === id)) return null
  const L = canvas.layoutFor(id)
  return {
    position: 'absolute' as const,
    left: `calc(50% + ${L.cx}px - ${L.w / 2}px)`,
    top: `calc(50% + ${L.cy}px - ${L.h / 2}px)`,
    width: `${L.w}px`,
    height: `${L.h}px`,
    zIndex: 80,
    pointerEvents: 'none' as const,
    borderRadius: '12px',
  }
})

// ── Layer refs for unified renderer ──
const layerBgRef = ref<HTMLElement | null>(null)
const layerContentRef = ref<HTMLElement | null>(null)

// Static checkerboard — never recomputed during zoom

// Unified render loop: damped interpolation + direct DOM transform writes.
// Completely bypasses Vue reactivity for all zoom/pan rendering.
const galaxyCanvasRef = ref<InstanceType<typeof KnowledgeGalaxyCanvas> | null>(null)
const renderer = useCanvasRenderer(canvas, layerBgRef, layerContentRef, {
  lerpFactor: 0.28,
  velocityDecay: 0.88,
})

/**
 * 聚光蒙版：仅鼠标周围约 200px（屏幕坐标）内完全清晰，外围略雾化
 */
const SPOTLIGHT_CLEAR_PX = 200
const SPOTLIGHT_FEATHER_PX = 72
const spotlight = ref({ x: 0, y: 0 })
let spotlightRaf = 0

function updateSpotlight(clientX: number, clientY: number) {
  const el = viewportEl.value
  if (!el) return
  const r = el.getBoundingClientRect()
  spotlight.value = {
    x: clientX - r.left,
    y: clientY - r.top,
  }
}

function onViewportMouseMove(e: MouseEvent) {
  if (spotlightRaf) cancelAnimationFrame(spotlightRaf)
  spotlightRaf = requestAnimationFrame(() => {
    spotlightRaf = 0
    updateSpotlight(e.clientX, e.clientY)
  })
}

function onViewportMouseLeave() {
  if (spotlightRaf) cancelAnimationFrame(spotlightRaf)
  spotlightRaf = 0
  const el = viewportEl.value
  if (!el) return
  spotlight.value = {
    x: el.clientWidth / 2,
    y: el.clientHeight / 2,
  }
}

const spotlightMaskStyle = computed(() => {
  const { x, y } = spotlight.value
  return {
    '--spot-x': `${x}px`,
    '--spot-y': `${y}px`,
    '--spot-clear': `${SPOTLIGHT_CLEAR_PX}px`,
    '--spot-feather-end': `${SPOTLIGHT_CLEAR_PX + SPOTLIGHT_FEATHER_PX}px`,
  }
})

const SCROLL_EDGE_EPS = 1

/**
 * 滚轮目标常在笔记 Shadow 内，Light DOM 的 Element.contains 无法命中，会误判为「不在视口」从而
 * 漏掉 preventDefault，浏览器默认捏合/缩放会作用在笔记正文上。
 */
function isWheelEventInsideViewport(root: HTMLElement, ev: WheelEvent): boolean {
  const t = ev.target
  if (!(t instanceof Node)) return false
  let n: Node | null = t
  while (n) {
    if (n instanceof Element && root.contains(n)) return true
    if (n instanceof ShadowRoot) {
      n = n.host
      continue
    }
    n = n.parentNode
  }
  return false
}

function wheelEventTargetElement(ev: WheelEvent): HTMLElement | null {
  const t = ev.target
  if (t instanceof HTMLElement) return t
  if (t instanceof Node) {
    let n: Node | null = t
    while (n && !(n instanceof HTMLElement)) n = n.parentNode
    return n instanceof HTMLElement ? n : null
  }
  return null
}

/** 从 node 向视口方向爬一层；穿过 ShadowRoot.host，遇 root 则结束 */
function wheelParentElement(node: HTMLElement, root: HTMLElement): HTMLElement | null {
  const p = node.parentNode
  if (!p) return null
  if (p instanceof ShadowRoot) {
    const h = p.host
    return h instanceof HTMLElement && h !== root ? h : null
  }
  if (p instanceof HTMLElement) {
    if (p === root) return null
    return p
  }
  return null
}

function overflowAllowsScroll(overflow: string) {
  return overflow === 'auto' || overflow === 'scroll' || overflow === 'overlay'
}

/** 自底向上：是否存在仍可在该滚轮方向上滚动的祖先（含预览区、编辑器等），用于把「顶/底」溢出交给画布平移 */
function wheelStillScrollsAncestor(
  root: HTMLElement,
  start: HTMLElement | null,
  deltaX: number,
  deltaY: number,
): boolean {
  let node: HTMLElement | null = start
  while (node && node !== root) {
    const st = getComputedStyle(node)
    const el = node

    if (Math.abs(deltaY) >= Math.abs(deltaX) && deltaY !== 0 && overflowAllowsScroll(st.overflowY)) {
      const maxY = el.scrollHeight - el.clientHeight
      if (maxY > SCROLL_EDGE_EPS) {
        if (deltaY < 0 && el.scrollTop > SCROLL_EDGE_EPS) return true
        if (deltaY > 0 && el.scrollTop < maxY - SCROLL_EDGE_EPS) return true
      }
    }

    if (Math.abs(deltaX) > Math.abs(deltaY) && deltaX !== 0 && overflowAllowsScroll(st.overflowX)) {
      const maxX = el.scrollWidth - el.clientWidth
      if (maxX > SCROLL_EDGE_EPS) {
        if (deltaX < 0 && el.scrollLeft > SCROLL_EDGE_EPS) return true
        if (deltaX > 0 && el.scrollLeft < maxX - SCROLL_EDGE_EPS) return true
      }
    }

    node = wheelParentElement(el, root)
  }
  return false
}

/** 从事件路径定位当前激活笔记内的可滚动宿主（Shadow 内滚轮时 target 可能在 shadow 内） */
function findActiveNotebookScrollHost(ev: WheelEvent): HTMLElement | null {
  const id = canvas.activeNotebookId
  if (!id) return null
  for (const n of ev.composedPath()) {
    if (!(n instanceof HTMLElement)) continue
    const card = n.closest?.('[data-notebook-id].notebook-card--active')
    if (
      card &&
      card.classList.contains('notebook-card--active') &&
      card.getAttribute('data-notebook-id') === id
    ) {
      return card.querySelector('.notebook-html-pane')
    }
  }
  return null
}

function onWheelCapture(e: WheelEvent) {
  const root = viewportEl.value
  if (!root || !isWheelEventInsideViewport(root, e)) return

  if (e.ctrlKey || e.metaKey) {
    e.preventDefault()
    const r = root.getBoundingClientRect()
    const vx = e.clientX - r.left
    const vy = e.clientY - r.top
    const wheelStep = knowledge.enabled.value
      ? WHEEL_ZOOM_STEP * GALAXY_VIEW_BASE_SCALE
      : WHEEL_ZOOM_STEP
    canvas.zoomViewByWheelAt(e.deltaY, vx, vy, root.clientWidth, root.clientHeight, wheelStep)
    return
  }

  /* Option/Alt + 滚轮：在可滚动内容上也强制平移画布 */
  if (e.altKey) {
    e.preventDefault()
    canvas.panBy(-e.deltaX, -e.deltaY)
    return
  }

  const target = wheelEventTargetElement(e)
  const shell = findActiveNotebookScrollHost(e)
  const overActiveNotebook = !!shell

  /* 当前选中笔记上：只滚笔记正文区域，不拖动画布视角 */
  if (overActiveNotebook) {
    if (wheelStillScrollsAncestor(root, target, e.deltaX, e.deltaY)) return
    e.preventDefault()
    if (shell) {
      let dy = e.deltaY
      let dx = e.deltaX
      if (e.deltaMode === WheelEvent.DOM_DELTA_LINE) {
        dy *= 16
        dx *= 16
      } else if (e.deltaMode === WheelEvent.DOM_DELTA_PAGE) {
        dy *= shell.clientHeight
        dx *= shell.clientWidth
      }
      shell.scrollTop += dy
      shell.scrollLeft += dx
    }
    return
  }

  if (wheelStillScrollsAncestor(root, target, e.deltaX, e.deltaY)) return

  e.preventDefault()
  canvas.panBy(-e.deltaX, -e.deltaY)
}

type PanSession = { x0: number; y0: number; panX0: number; panY0: number; pointerId?: number }
const panSession = ref<PanSession | null>(null)

const isCanvasPanning = computed(() => panSession.value !== null)

// ── galaxy auto-pan ───────────────────────────────────────────────

/** 星系模式下选中左侧列表项时，自动平移视口对准节点 */
function focusGalaxyNodeToActive() {
  const id = canvas.activeNotebookId
  const layout = galaxy.layout.value
  if (!id || !layout?.nodes.length || !knowledge.enabled.value) return
  // 优先使用动画后的节点位置，回退到布局中的静态位置
  const animPos = galaxyCanvasRef.value?.getNodeAnimPos(id)
  const node = layout.nodes.find((n) => n.id === id)
  const nx = animPos?.x ?? node?.x
  const ny = animPos?.y ?? node?.y
  if (nx == null || ny == null) return
  const fb = layout.bounds
  canvas.focusGalaxyNode(nx, ny, fb.maxX - fb.minX, fb.maxY - fb.minY, fb.minX, fb.minY)
}

watch(
  [() => canvas.activeNotebookId, () => galaxy.layout.value],
  () => { focusGalaxyNodeToActive() },
)

function isCanvasPanBlockedTarget(el: EventTarget | null): boolean {
  const t = el as HTMLElement | null
  return !!(t?.closest('.notebook-card') || t?.closest('.kg-node'))
}

function onPanMove(e: PointerEvent) {
  const p = panSession.value
  if (!p) return
  const dx = e.clientX - p.x0
  const dy = e.clientY - p.y0
  canvas.panX = p.panX0 + dx
  canvas.panY = p.panY0 + dy
}

function endPan() {
  const p = panSession.value
  if (!p) return
  const pid = p.pointerId
  panSession.value = null
  if (pid != null && viewportEl.value) {
    try {
      viewportEl.value.releasePointerCapture(pid)
    } catch {
      /* 已释放或非本元素 capture */
    }
  }
  canvas.persistViewNow()
  window.removeEventListener('pointermove', onPanMove)
  window.removeEventListener('pointerup', endPan)
  window.removeEventListener('pointercancel', endPan)
}

function startPanSession(e: PointerEvent) {
  panSession.value = {
    x0: e.clientX,
    y0: e.clientY,
    panX0: canvas.panX,
    panY0: canvas.panY,
    pointerId: e.button === 0 ? e.pointerId : undefined,
  }
  if (e.button === 0 && viewportEl.value) {
    try {
      viewportEl.value.setPointerCapture(e.pointerId)
    } catch {
      /* */
    }
  }
  window.addEventListener('pointermove', onPanMove)
  window.addEventListener('pointerup', endPan)
  window.addEventListener('pointercancel', endPan)
}

function onViewportPointerDown(e: PointerEvent) {
  if (e.button === 1) {
    e.preventDefault()
    startPanSession(e)
    return
  }
  if (e.button === 0 && !isCanvasPanBlockedTarget(e.target)) {
    if (canvas.activeNotebookId != null) {
      canvas.selectNotebook(null)
    }
    e.preventDefault()
    startPanSession(e)
  }
}

/** 在捕获阶段处理：⌥/Alt + 左键在笔记卡片上拖动画布，避免只能中键拖移 */
function onViewportPointerDownCapture(e: PointerEvent) {
  if (e.button !== 0 || !e.altKey) return
  const t = e.target as HTMLElement | null
  if (t?.closest('.notebook-card__handle')) return
  const root = viewportEl.value
  if (!root || !t || !root.contains(t)) return
  if (!isCanvasPanBlockedTarget(e.target)) return
  e.preventDefault()
  e.stopPropagation()
  startPanSession(e)
}

onMounted(() => {
  const el = viewportEl.value
  if (!el) return
  el.addEventListener('wheel', onWheelCapture, { passive: false, capture: true })
  el.addEventListener('pointerdown', onViewportPointerDownCapture, { capture: true })
  spotlight.value = {
    x: el.clientWidth / 2,
    y: el.clientHeight / 2,
  }
})

onBeforeUnmount(() => {
  if (spotlightRaf) cancelAnimationFrame(spotlightRaf)
  viewportEl.value?.removeEventListener('wheel', onWheelCapture, { capture: true } as AddEventListenerOptions)
  viewportEl.value?.removeEventListener('pointerdown', onViewportPointerDownCapture, {
    capture: true,
  } as AddEventListenerOptions)
  window.removeEventListener('pointermove', onPanMove)
  window.removeEventListener('pointerup', endPan)
  window.removeEventListener('pointercancel', endPan)
})
</script>

<template>
  <div class="canvas-root">
    <div
      ref="viewportEl"
      class="canvas-root__viewport"
      :class="{ 'canvas-root__viewport--panning': isCanvasPanning }"
      @pointerdown="onViewportPointerDown"
      @mousemove="onViewportMouseMove"
      @mouseleave="onViewportMouseLeave"
    >
      <div
        ref="layerBgRef"
        class="canvas-root__layer canvas-root__layer--bg"
        aria-hidden="true"
      />
      <div class="canvas-root__spotlight-mask" :style="spotlightMaskStyle" aria-hidden="true" />
      <div ref="layerContentRef" class="canvas-root__layer canvas-root__layer--content">
        <KnowledgeGalaxyCanvas
          ref="galaxyCanvasRef"
          v-if="knowledge.enabled.value"
          :layout="galaxy.layout.value"
          :render-state="renderer.renderStateRef"
          @open-detail="onGalaxyNodeDetail"
        />
        <template v-else>
          <NotebookCard v-for="n in canvas.notebooks" :key="n.id" :notebook="n" />
          <div
            v-if="generatingOverlayStyle"
            class="canvas-root__nb-generating"
            :style="generatingOverlayStyle"
            aria-hidden="true"
          >
            <div class="canvas-root__nb-generating-veil" />
          </div>
        </template>
      </div>

      <div v-if="knowledge.enabled.value" class="canvas-root__galaxy-overlays">
        <div v-if="galaxy.loading.value && !galaxy.layout.value" class="canvas-root__galaxy-state" role="status">
          <Icon name="refresh" :size="20" class="canvas-root__galaxy-spin" />
          {{ t('knowledge.galaxy.loading') }}
        </div>
        <p v-else-if="galaxy.error.value" class="canvas-root__galaxy-err" role="alert">{{ galaxy.error.value }}</p>
        <p v-else-if="!galaxy.layout.value?.nodes.length" class="canvas-root__galaxy-state">
          {{ t('knowledge.galaxy.empty') }}
        </p>

        <div v-if="galaxy.layout.value?.nodes.length" class="canvas-root__galaxy-hud glass-panel">
          <span class="canvas-root__galaxy-stat">
            {{
              t('knowledge.galaxy.stats', {
                nodes: String(galaxy.nodeCount.value),
                edges: String(galaxy.edgeCount.value),
              })
            }}
          </span>
          <button
            type="button"
            class="canvas-root__galaxy-btn"
            :disabled="galaxy.loading.value"
            @click="galaxy.reload()"
          >
            <Icon name="refresh" :size="14" />
            {{ t('knowledge.galaxy.refresh') }}
          </button>
        </div>

        <aside
          v-if="galaxy.layout.value?.nodes.length"
          class="canvas-root__galaxy-legend glass-panel"
          :aria-label="t('knowledge.galaxy.legendAria')"
        >
          <h4 class="canvas-root__galaxy-legend-title">{{ t('knowledge.galaxy.legendTitle') }}</h4>
          <ul class="canvas-root__galaxy-legend-ul">
            <li v-for="g in legendGroups" :key="g" class="canvas-root__galaxy-legend-item">
              <span class="canvas-root__galaxy-legend-dot" :style="{ background: groupColor(g) }" />
              <span>{{ g === 'default' ? t('knowledge.galaxy.groupDefault') : g }}</span>
            </li>
          </ul>
        </aside>
      </div>
    </div>
  </div>
</template>

<style scoped>
.canvas-root {
  flex: 1;
  min-height: 0;
  width: 100%;
  height: 100%;
  position: relative;
  overflow: hidden;
  /* 与背景层一致，缩放平移时四角不露异色 */
  background: var(--color-bg-elevated);
}

.canvas-root__viewport {
  position: absolute;
  inset: 0;
  overflow: hidden;
  touch-action: none;
  cursor: default;
  background: var(--color-bg-elevated);
}

.canvas-root__viewport--panning {
  cursor: grabbing;
}

.canvas-root__layer {
  position: absolute;
  inset: 0;
  overflow: visible;
  will-change: transform;
}

.canvas-root__world {
  position: absolute;
  inset: 0;
  will-change: transform;
}

.canvas-root__world--bg {
  z-index: 0;
  background-color: var(--color-bg-elevated);
}

.canvas-root__world--surface {
  z-index: 35;
  pointer-events: none;
}

.canvas-root__world--surface > * {
  pointer-events: auto;
}

.canvas-root__nb-generating {
  position: absolute;
  box-sizing: border-box;
  overflow: visible;
}

.canvas-root__nb-generating-veil {
  position: absolute;
  inset: 0;
  z-index: 0;
  border-radius: 12px;
  pointer-events: none;
  overflow: hidden;
  background: linear-gradient(
    118deg,
    rgba(0, 80, 195, 0.26) 0%,
    rgba(85, 175, 255, 0.34) 38%,
    rgba(130, 215, 255, 0.3) 55%,
    rgba(35, 125, 240, 0.32) 100%
  );
  background-size: 180% 180%;
  box-shadow: inset 0 0 42px rgba(110, 200, 255, 0.16);
  backdrop-filter: saturate(1.18) brightness(1.04);
  -webkit-backdrop-filter: saturate(1.18) brightness(1.04);
  animation: canvas-root-nb-veil-base 9.7s ease-in-out infinite;
}

.canvas-root__nb-generating-veil::before {
  content: '';
  position: absolute;
  inset: -32%;
  border-radius: 42%;
  opacity: 0.62;
  background:
    radial-gradient(circle at 28% 38%, rgba(190, 240, 255, 0.55) 0%, transparent 46%),
    radial-gradient(circle at 78% 62%, rgba(70, 160, 255, 0.42) 0%, transparent 50%),
    conic-gradient(
      from 95deg at 52% 48%,
      transparent 0deg,
      rgba(0, 120, 230, 0.28) 55deg,
      rgba(160, 220, 255, 0.35) 110deg,
      transparent 175deg
    );
  mix-blend-mode: overlay;
  animation: canvas-root-nb-veil-drift-a 6.4s cubic-bezier(0.42, 0.01, 0.58, 1) infinite;
}

.canvas-root__nb-generating-veil::after {
  content: '';
  position: absolute;
  inset: -24%;
  border-radius: 38%;
  opacity: 0.48;
  background:
    radial-gradient(ellipse 95% 72% at 58% 28%, rgba(255, 255, 255, 0.38), transparent 58%),
    radial-gradient(ellipse 70% 95% at 22% 72%, rgba(100, 185, 255, 0.32), transparent 52%);
  mix-blend-mode: soft-light;
  animation: canvas-root-nb-veil-drift-b 8.1s cubic-bezier(0.36, 0.08, 0.64, 0.92) infinite reverse;
}

@keyframes canvas-root-nb-veil-base {
  0%,
  100% {
    background-position: 0% 50%;
    filter: hue-rotate(0deg);
  }
  27% {
    background-position: 88% 22%;
    filter: hue-rotate(14deg);
  }
  52% {
    background-position: 35% 88%;
    filter: hue-rotate(-10deg);
  }
  76% {
    background-position: 72% 65%;
    filter: hue-rotate(8deg);
  }
}

@keyframes canvas-root-nb-veil-drift-a {
  0%,
  100% {
    transform: translate(-3%, 2%) rotate(0deg) scale(1);
  }
  23% {
    transform: translate(11%, -9%) rotate(26deg) scale(1.14);
  }
  48% {
    transform: translate(-9%, 11%) rotate(-18deg) scale(0.9);
  }
  71% {
    transform: translate(7%, 6%) rotate(20deg) scale(1.08);
  }
  91% {
    transform: translate(-5%, -4%) rotate(-8deg) scale(0.98);
  }
}

@keyframes canvas-root-nb-veil-drift-b {
  0%,
  100% {
    transform: translate(4%, -2%) scale(1) rotate(0deg);
  }
  31% {
    transform: translate(-14%, 10%) scale(1.18) rotate(-12deg);
  }
  59% {
    transform: translate(9%, -11%) scale(0.86) rotate(16deg);
  }
  84% {
    transform: translate(-6%, 5%) scale(1.06) rotate(-5deg);
  }
}

@media (prefers-reduced-transparency: reduce) {
  .canvas-root__nb-generating-veil {
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .canvas-root__nb-generating-veil {
    animation-duration: 18s;
  }

  .canvas-root__nb-generating-veil::before {
    animation-duration: 18s;
  }

  .canvas-root__nb-generating-veil::after {
    animation-duration: 18s;
  }
}

.canvas-root__spotlight-mask {
  position: absolute;
  inset: 0;
  z-index: 25;
  pointer-events: none;
  backdrop-filter: blur(8px) saturate(0.94);
  -webkit-backdrop-filter: blur(8px) saturate(0.94);
  background: rgba(240, 240, 244, 0.22);
  mask-image: radial-gradient(
    circle at var(--spot-x, 50%) var(--spot-y, 50%),
    transparent 0,
    transparent var(--spot-clear, 200px),
    rgba(0, 0, 0, 0.5) var(--spot-feather-end, 272px),
    rgba(0, 0, 0, 0.92) 100%
  );
  -webkit-mask-image: radial-gradient(
    circle at var(--spot-x, 50%) var(--spot-y, 50%),
    transparent 0,
    transparent var(--spot-clear, 200px),
    rgba(0, 0, 0, 0.5) var(--spot-feather-end, 272px),
    rgba(0, 0, 0, 0.92) 100%
  );
  mask-mode: alpha;
}

@media (prefers-reduced-transparency: reduce) {
  .canvas-root__spotlight-mask {
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
    background: rgba(240, 240, 244, 0.35);
  }
}

.canvas-root__grid {
  position: absolute;
  inset: 0;
  /* 无线格；圆点大小/间距随 --stone-step 与 viewScale 变化 */
  background-color: var(--color-bg-elevated);
  background-image: radial-gradient(
    circle at 50% 54%,
    rgba(52, 52, 58, 0.28) 0 calc(var(--stone-step) * var(--stone-dot-core, 0.125)),
    rgba(32, 32, 38, 0.12) calc(var(--stone-step) * var(--stone-dot-core, 0.125)),
    transparent calc(var(--stone-step) * var(--stone-dot-fade, 0.19))
  );
  background-size: var(--stone-step) var(--stone-step);
  background-position: 0 0;
  pointer-events: none;
}

.canvas-root__grid--galaxy {
  background-image: radial-gradient(
    circle at 50% 54%,
    rgba(52, 52, 58, 0.38) 0 calc(var(--stone-step) * var(--stone-dot-core, 0.17)),
    rgba(32, 32, 38, 0.16) calc(var(--stone-step) * var(--stone-dot-core, 0.17)),
    transparent calc(var(--stone-step) * var(--stone-dot-fade, 0.26))
  );
}

.canvas-root__galaxy-overlays {
  position: absolute;
  inset: 0;
  z-index: 40;
  pointer-events: none;
}

.canvas-root__galaxy-overlays > * {
  pointer-events: auto;
}

.canvas-root__galaxy-state {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 18px;
  border-radius: 12px;
  font-size: 13px;
  color: var(--color-text-secondary);
  background: rgba(255, 255, 255, 0.82);
  border: 1px solid var(--color-border);
  box-shadow: var(--shadow-float-sm);
}

.canvas-root__galaxy-err {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  max-width: min(420px, 90vw);
  margin: 0;
  padding: 12px 18px;
  border-radius: 12px;
  font-size: 13px;
  color: #b42318;
  background: rgba(255, 255, 255, 0.88);
  border: 1px solid rgba(180, 35, 24, 0.2);
  box-shadow: var(--shadow-float-sm);
}

.canvas-root__galaxy-spin {
  animation: canvas-root-galaxy-spin 0.9s linear infinite;
}

@keyframes canvas-root-galaxy-spin {
  to {
    transform: rotate(360deg);
  }
}

.canvas-root__galaxy-hud {
  position: absolute;
  top: calc(var(--topbar-h, 56px) + var(--float-gap, 12px) + 8px);
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
  max-width: min(640px, calc(100vw - 32px));
  padding: 8px 14px;
  border-radius: 12px;
}

.canvas-root__galaxy-stat {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text);
}

.canvas-root__galaxy-hint {
  flex: 1;
  min-width: 120px;
  font-size: 11px;
  color: var(--color-text-secondary);
}

.canvas-root__galaxy-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 10px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.85);
  font: inherit;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
}

.canvas-root__galaxy-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.canvas-root__galaxy-legend {
  position: absolute;
  right: 12px;
  bottom: calc(var(--float-gap, 12px) + 56px);
  max-width: 180px;
  padding: 10px 12px;
  border-radius: 12px;
}

.canvas-root__galaxy-legend-title {
  margin: 0 0 6px;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--color-text-secondary);
}

.canvas-root__galaxy-legend-ul {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.canvas-root__galaxy-legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--color-text);
}

.canvas-root__galaxy-legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
</style>
