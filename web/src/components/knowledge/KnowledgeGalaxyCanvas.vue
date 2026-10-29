<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted, type Ref } from 'vue'
import { useI18n } from '@/composables/useI18n'
import type { RenderState } from '@/composables/useCanvasRenderer'
import { useCanvasStore } from '@/stores/canvas'
import {
  GALAXY_ACTIVE_RING_PAD,
  GALAXY_LABEL_OFFSET,
  GALAXY_LAYOUT_DENSITY,
  GALAXY_ORBIT_RING_BASE,
  GALAXY_ORBIT_RING_STEP,
  type GalaxyLayout,
} from '@/utils/knowledgeGalaxy'

const props = defineProps<{
  layout: GalaxyLayout | null
  renderState?: Ref<RenderState>
}>()

const emit = defineEmits<{
  openDetail: [docId: string]
}>()

const { t } = useI18n()
const canvas = useCanvasStore()
const hoveredId = ref<string | null>(null)

const activeId = computed(() => canvas.activeNotebookId)

// ── orbit animation ───────────────────────────────────────────────

interface OrbitParam {
  orbitRadius: number
  initialAngle: number
  speed: number       // rad/s
}

const orbitParams = ref(new Map<string, OrbitParam>())
const nodeTimeAcc = new Map<string, number>()  // accumulated time per node (non-reactive)
const tickCount = ref(0)
let rafId = 0

watch(() => props.layout?.nodes, (nodes) => {
  if (!nodes) return
  const map = new Map<string, OrbitParam>()
  // Find center
  let cx = 0, cy = 0
  for (const n of nodes) {
    if (n.clusterId === 0) { cx = n.x; cy = n.y; break }
  }
  for (const n of nodes) {
    const existing = orbitParams.value.get(n.id)
    if (existing) { map.set(n.id, existing); continue }
    if (n.clusterId === 0) {
      map.set(n.id, { orbitRadius: 0, initialAngle: 0, speed: 0 })
    } else {
      const dx = n.x - cx
      const dy = n.y - cy
      const r = Math.hypot(dx, dy)
      // Outer rings orbit slower, like real planets
      const speed = 0.15 / Math.sqrt(Math.max(1, n.clusterId))
      map.set(n.id, {
        orbitRadius: r,
        initialAngle: Math.atan2(dy, dx),
        speed,
      })
    }
  }
  orbitParams.value = map
  nodeTimeAcc.clear()
  tickCount.value = 0
}, { immediate: true })

onMounted(() => {
  let last = performance.now()
  const tick = (now: number) => {
    const dt = Math.min((now - last) / 1000, 0.1) // cap dt to avoid jump after tab switch
    last = now
    for (const [id, p] of orbitParams.value) {
      if (p.orbitRadius === 0) continue
      if (id === hoveredId.value || id === activeId.value) continue
      nodeTimeAcc.set(id, (nodeTimeAcc.get(id) ?? 0) + dt)
    }
    tickCount.value++
    rafId = requestAnimationFrame(tick)
  }
  rafId = requestAnimationFrame(tick)
})

onUnmounted(() => {
  cancelAnimationFrame(rafId)
})

// ── layout ─────────────────────────────────────────────────────────

const frame = computed(() => {
  const b = props.layout?.bounds
  if (!b) return { w: 900, h: 620, viewBox: '0 0 900 620' }
  const w = b.maxX - b.minX
  const h = b.maxY - b.minY
  return { w, h, viewBox: `${b.minX} ${b.minY} ${w} ${h}` }
})

const centerPos = computed(() => {
  for (const n of props.layout?.nodes ?? []) {
    if (n.clusterId === 0) return { x: n.x, y: n.y }
  }
  return { x: 0, y: 0 }
})

/** Compute orbit ring levels present in the layout for drawing faint guide circles. */
const orbitRings = computed(() => {
  const levels = new Set<number>()
  for (const n of props.layout?.nodes ?? []) {
    if (n.clusterId > 0) levels.add(n.clusterId)
  }
  return [...levels].sort((a, b) => a - b).map(lvl => ({
    level: lvl,
    radius: GALAXY_ORBIT_RING_BASE + (lvl - 1) * GALAXY_ORBIT_RING_STEP,
  }))
})

const animatedNodes = computed(() => {
  // tickCount triggers recomputation
  void tickCount.value
  const cx = centerPos.value.x
  const cy = centerPos.value.y

  return (props.layout?.nodes ?? []).map((n) => {
    const p = orbitParams.value.get(n.id)
    if (!p || p.orbitRadius === 0) return n

    const t = nodeTimeAcc.get(n.id) ?? 0
    const angle = p.initialAngle + p.speed * t
    return {
      ...n,
      x: cx + p.orbitRadius * Math.cos(angle),
      y: cy + p.orbitRadius * Math.sin(angle),
    }
  })
})

const animatedPosById = computed(() => {
  const map = new Map<string, { x: number; y: number }>()
  for (const n of animatedNodes.value) map.set(n.id, { x: n.x, y: n.y })
  return map
})

const edgeLines = computed(() =>
  (props.layout?.edges ?? []).flatMap((e) => {
    const a = animatedPosById.value.get(e.source)
    const b = animatedPosById.value.get(e.target)
    if (!a || !b) return []
    const t = (e.similarity - 0.55) / 0.45
    const opacity = Math.pow(Math.max(0, t), 1.5) * 0.35 + 0.1
    return [{ ...e, x1: a.x, y1: a.y, x2: b.x, y2: b.y, opacity }]
  }),
)

const labeledNodes = computed(() => {
  const nodes = animatedNodes.value
  const visible = nodes.filter((n) => n.id === activeId.value || n.id === hoveredId.value)
  return [...visible].sort((a, b) => {
    if (a.id === activeId.value) return 1
    if (b.id === activeId.value) return -1
    if (a.id === hoveredId.value) return 1
    if (b.id === hoveredId.value) return -1
    return 0
  })
})

function labelText(title: string) {
  return title.length > 20 ? `${title.slice(0, 20)}…` : title
}

// Expose animated node positions for auto-pan centering
defineExpose({
  getNodeAnimPos: (id: string): { x: number; y: number } | null =>
    animatedPosById.value.get(id) ?? null,
})

function onNodeClick(id: string) {
  if (activeId.value === id) {
    emit('openDetail', id)
    return
  }
  canvas.selectNotebook(id)
}
</script>

<template>
  <div
    v-if="layout?.nodes.length"
    class="kgalaxy-world"
    :style="{
      '--kg-density': String(GALAXY_LAYOUT_DENSITY),
      width: `${frame.w}px`,
      height: `${frame.h}px`,
      marginLeft: `${-frame.w / 2}px`,
      marginTop: `${-frame.h / 2}px`,
    }"
  >
    <svg
      class="kgalaxy-world__svg"
      :viewBox="frame.viewBox"
      :width="frame.w"
      :height="frame.h"
      role="img"
      :aria-label="t('knowledge.galaxy.aria')"
    >
      <!-- faint orbit guide rings -->
      <g class="kgalaxy-world__orbits">
        <circle
          v-for="ring in orbitRings"
          :key="`orbit-${ring.level}`"
          class="kgalaxy-world__orbit-ring"
          :cx="centerPos.x"
          :cy="centerPos.y"
          :r="ring.radius"
        />
      </g>

      <g class="kgalaxy-world__edges">
        <line
          v-for="(e, i) in edgeLines"
          :key="`e-${i}`"
          class="kgalaxy-world__edge"
          :x1="e.x1"
          :y1="e.y1"
          :x2="e.x2"
          :y2="e.y2"
          :stroke-opacity="e.opacity"
        />
      </g>

      <g class="kgalaxy-world__nodes">
        <g
          v-for="node in animatedNodes"
          :key="node.id"
          class="kg-node"
          :class="{
            'kg-node--active': node.id === activeId,
            'kg-node--hover': node.id === hoveredId,
            'kg-node--orphan': !node.hasEmbedding,
            'kg-node--center': node.clusterId === 0,
          }"
          :transform="`translate(${node.x}, ${node.y})`"
          @click.stop="onNodeClick(node.id)"
          @pointerenter="hoveredId = node.id"
          @pointerleave="hoveredId = null"
        >
          <circle :r="node.radius" class="kg-node__dot" />
          <circle
            v-if="node.hasSemantic4D"
            :r="Math.max(3, node.radius * 0.35)"
            class="kg-node__4d"
            :cx="node.radius * 0.72"
            :cy="-node.radius * 0.72"
          />
          <circle
            v-if="node.id === activeId"
            :r="node.radius + GALAXY_ACTIVE_RING_PAD"
            class="kg-node__ring"
            fill="none"
          />
          <title>{{ node.title }}</title>
        </g>
      </g>

      <g class="kgalaxy-world__labels" pointer-events="none">
        <g
          v-for="node in labeledNodes"
          :key="`label-${node.id}`"
          class="kg-node__label-wrap"
          :class="{
            'kg-node__label-wrap--active': node.id === activeId,
            'kg-node__label-wrap--hover': node.id === hoveredId,
          }"
          :transform="`translate(${node.x}, ${node.y})`"
        >
          <text
            class="kg-node__label"
            :y="-(node.radius + GALAXY_LABEL_OFFSET)"
            text-anchor="middle"
          >
            {{ labelText(node.title) }}
          </text>
        </g>
      </g>
    </svg>
  </div>
</template>

<style scoped>
.kgalaxy-world {
  position: absolute;
  left: 50%;
  top: 50%;
  pointer-events: auto;
}

.kgalaxy-world__svg {
  display: block;
  overflow: visible;
  shape-rendering: geometricPrecision;
  text-rendering: geometricPrecision;
}

.kgalaxy-world__orbit-ring {
  fill: none;
  stroke: rgba(60, 60, 67, 0.12);
  stroke-width: calc(0.3px * var(--kg-density, 4));
  pointer-events: none;
}

.kgalaxy-world__edge {
  stroke: rgba(60, 60, 67, 0.50);
  stroke-width: calc(0.5px * var(--kg-density, 4));
}

.kgalaxy-world__nodes {
  cursor: pointer;
}

.kgalaxy-world__labels {
  pointer-events: none;
}

.kg-node__dot {
  fill: #aeaeb2;
  stroke: rgba(255, 255, 255, 0.9);
  stroke-width: calc(0.5px * var(--kg-density, 4));
  transition: fill 0.2s ease;
}

.kg-node--center .kg-node__dot {
  fill: #f5a623;
  stroke: rgba(255, 255, 255, 0.95);
  stroke-width: calc(0.75px * var(--kg-density, 4));
}

.kg-node--hover .kg-node__dot {
  fill: #8e8e93;
}

.kg-node--active .kg-node__dot {
  fill: #636366;
}

.kg-node--orphan .kg-node__dot {
  fill: #c7c7cc;
  opacity: 0.7;
}

.kg-node__ring {
  stroke: #636366;
  stroke-width: calc(1.5px * var(--kg-density, 4));
}

.kg-node__4d {
  fill: #34c759;
  stroke: rgba(255, 255, 255, 0.95);
  stroke-width: calc(0.75px * var(--kg-density, 4));
  pointer-events: none;
}

.kg-node__label-wrap--active .kg-node__label {
  font-weight: 600;
  fill: #3a3a3c;
}

.kg-node__label {
  font-size: calc(11px * var(--kg-density, 4));
  font-weight: 600;
  fill: #636366;
  paint-order: stroke fill;
  stroke: rgba(255, 255, 255, 0.92);
  stroke-width: calc(3px * var(--kg-density, 4));
  stroke-linejoin: round;
}
</style>
