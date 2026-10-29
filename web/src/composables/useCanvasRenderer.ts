/**
 * Unified canvas renderer with damped interpolation.
 *
 * Runs a persistent rAF loop that reads Pinia raw state as "target",
 * applies lerp-based smoothing, then writes the resulting CSS transform
 * directly to DOM elements — completely bypassing Vue reactivity.
 *
 * Also exposes a shared `renderState` (non-reactive object) for SVG
 * child components to read the current interpolated pan/scale without
 * depending on Pinia or Vue reactivity.
 */
import { ref, onMounted, onBeforeUnmount, type Ref } from 'vue'
import type { useCanvasStore } from '@/stores/canvas'

export interface RenderState {
  panX: number
  panY: number
  scale: number
}

export function useCanvasRenderer(
  canvasStore: ReturnType<typeof useCanvasStore>,
  layerBg: Ref<HTMLElement | null>,
  layerContent: Ref<HTMLElement | null>,
  opts?: { lerpFactor?: number; velocityDecay?: number },
) {
  const lerpFactor = opts?.lerpFactor ?? 0.28
  const velocityDecay = opts?.velocityDecay ?? 0.88

  let rafId = 0
  let curPanX = 0
  let curPanY = 0
  let curScale = 1

  // Velocity for inertia feel (trackpad coasting)
  let velX = 0
  let velY = 0
  let velScale = 0

  // Shared non-reactive state for child components
  const renderState: RenderState = { panX: 0, panY: 0, scale: 1 }
  const renderStateRef = ref<RenderState>(renderState)

  let prevTx = ''
  let frameCount = 0

  function tick() {
    rafId = requestAnimationFrame(tick)

    const target = canvasStore.$state

    // Compute velocity from wheel events (approximate as target delta)
    const dpx = target.panX - curPanX
    const dpy = target.panY - curPanY
    const ds = target.viewScale - curScale

    // Blend velocity: keep portion of old + add new impulse
    velX = velX * velocityDecay + dpx * 0.15
    velY = velY * velocityDecay + dpy * 0.15
    velScale = velScale * velocityDecay + ds * 0.15

    // Damped interpolation toward target, plus velocity contribution
    curPanX += (target.panX - curPanX) * lerpFactor + velX
    curPanY += (target.panY - curPanY) * lerpFactor + velY
    curScale += (target.viewScale - curScale) * lerpFactor + velScale

    // Clamp scale to valid range
    curScale = Math.max(0.02, Math.min(5, curScale))

    // Update shared render state (mutate in place for zero-allocation reads)
    renderState.panX = curPanX
    renderState.panY = curPanY
    renderState.scale = curScale
    frameCount++

    const tx = `translate3d(${curPanX}px, ${curPanY}px, 0) scale(${curScale})`
    if (tx === prevTx) return
    prevTx = tx

    // Write to both layers atomically in the same frame
    const bg = layerBg.value
    const ct = layerContent.value
    if (bg) bg.style.transform = tx
    if (ct) ct.style.transform = tx
  }

  onMounted(() => {
    rafId = requestAnimationFrame(tick)
  })

  onBeforeUnmount(() => {
    cancelAnimationFrame(rafId)
  })

  return { renderState, renderStateRef }
}
