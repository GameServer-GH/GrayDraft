<script setup>
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import Icon from '@/components/common/Icon.vue'

const props = defineProps({
  step: { type: Object, default: null },
  stepIndex: { type: Number, default: 0 },
  totalSteps: { type: Number, default: 1 },
  isFirst: { type: Boolean, default: true },
  isLast: { type: Boolean, default: false },
  targetSelector: { type: [String, Function, Object], default: null },
  spotlightPadding: { type: Number, default: 8 },
  cardPlacement: { type: String, default: 'auto' },
  cardOffset: { type: Number, default: 16 }
})

const emit = defineEmits(['next', 'prev', 'skip'])

const cardRef = ref(null)
const cardStyle = ref({})

const CARD_Z = 9002

function resolveTarget() {
  if (!props.targetSelector) return null
  if (typeof props.targetSelector === 'string') {
    return document.querySelector(props.targetSelector)
  }
  if (typeof props.targetSelector === 'function') {
    return props.targetSelector()
  }
  if (props.targetSelector instanceof Element) {
    return props.targetSelector
  }
  return null
}

function computeCardPosition() {
  const target = resolveTarget()
  if (!target) return

  nextTick(() => {
    const card = cardRef.value?.$el || cardRef.value
    if (!card) return

    const targetRect = target.getBoundingClientRect()
    const cardRect = card.getBoundingClientRect()
    const pad = props.spotlightPadding
    const offset = props.cardOffset
    const vw = window.innerWidth
    const vh = window.innerHeight

    let placement = props.cardPlacement
    if (placement === 'auto') {
      // prefer bottom on desktop, bottom on mobile
      placement = vw < 768 ? 'bottom' : 'right'
    }

    let top, left

    switch (placement) {
      case 'bottom':
        top = targetRect.bottom + pad + offset
        left = targetRect.left + targetRect.width / 2 - cardRect.width / 2
        break
      case 'top':
        top = targetRect.top - pad - offset - cardRect.height
        left = targetRect.left + targetRect.width / 2 - cardRect.width / 2
        break
      case 'right':
        top = targetRect.top + targetRect.height / 2 - cardRect.height / 2
        left = targetRect.right + pad + offset
        break
      case 'left':
        top = targetRect.top + targetRect.height / 2 - cardRect.height / 2
        left = targetRect.left - pad - offset - cardRect.width
        break
      default:
        top = targetRect.bottom + pad + offset
        left = targetRect.left + targetRect.width / 2 - cardRect.width / 2
    }

    // Clamp to viewport
    const maxLeft = vw - cardRect.width - 12
    const minLeft = 12
    const maxTop = vh - cardRect.height - 12
    const minTop = 12

    left = Math.max(minLeft, Math.min(maxLeft, left))
    top = Math.max(minTop, Math.min(maxTop, top))

    cardStyle.value = {
      position: 'fixed',
      top: `${top}px`,
      left: `${left}px`,
      zIndex: CARD_Z
    }
  })
}

let resizeTimer = null
function onResize() {
  if (resizeTimer) clearTimeout(resizeTimer)
  resizeTimer = setTimeout(computeCardPosition, 100)
}

watch(() => props.stepIndex, () => {
  nextTick(computeCardPosition)
})

onMounted(() => {
  window.addEventListener('resize', onResize)
  nextTick(computeCardPosition)
})

onUnmounted(() => {
  window.removeEventListener('resize', onResize)
  if (resizeTimer) clearTimeout(resizeTimer)
})
</script>

<template>
  <div
    v-if="step"
    ref="cardRef"
    class="tour-glass-card"
    :style="cardStyle"
  >
    <!-- Step badge -->
    <div class="tour-step-badge">
      {{ stepIndex + 1 }} / {{ totalSteps }}
    </div>

    <!-- Title -->
    <h3 class="tour-card-title">{{ step.title }}</h3>

    <!-- Content -->
    <p class="tour-card-content">{{ step.content }}</p>

    <!-- Actions -->
    <div class="tour-card-actions">
      <button
        v-if="!isFirst"
        class="tour-btn tour-btn-prev"
        @click.stop="emit('prev')"
        title="上一步"
      >
        <Icon name="chevron-left" :size="18" />
      </button>

      <button
        class="tour-btn tour-btn-skip"
        @click.stop="emit('skip')"
        title="跳过"
      >
        跳过
      </button>

      <button
        class="tour-btn tour-btn-next"
        @click.stop="emit('next')"
        :title="isLast ? '完成' : '下一步'"
      >
        <Icon v-if="!isLast" name="chevron-right" :size="18" />
        <Icon v-else name="check" :size="18" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.tour-glass-card {
  width: 280px;
  max-width: calc(100vw - 24px);
  padding: 20px;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.72);
  backdrop-filter: blur(24px) saturate(1.4);
  -webkit-backdrop-filter: blur(24px) saturate(1.4);
  border: 1px solid rgba(255, 255, 255, 0.6);
  box-shadow:
    0 8px 40px rgba(0, 0, 0, 0.12),
    0 2px 8px rgba(0, 0, 0, 0.06),
    inset 0 1px 1px rgba(255, 255, 255, 0.5);
  pointer-events: all;
  animation: tour-card-enter 0.4s cubic-bezier(0.22, 1, 0.36, 1);
  z-index: 9002 !important;
}

@keyframes tour-card-enter {
  from {
    opacity: 0;
    transform: translateY(12px) scale(0.96);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.tour-step-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 3px 10px;
  border-radius: 20px;
  background: rgba(58, 130, 247, 0.12);
  color: #3a82f7;
  font-size: 11px;
  font-weight: 600;
  margin-bottom: 12px;
  letter-spacing: 0.02em;
}

.tour-card-title {
  margin: 0 0 8px;
  font-size: 16px;
  font-weight: 700;
  color: #1c1e24;
  line-height: 1.4;
}

.tour-card-content {
  margin: 0 0 18px;
  font-size: 13px;
  font-weight: 600;
  color: #3d414a;
  line-height: 1.6;
}

.tour-card-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.tour-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  cursor: pointer;
  transition: all 0.2s ease;
  border-radius: 10px;
  -webkit-tap-highlight-color: transparent;
}

.tour-btn-prev,
.tour-btn-next {
  width: 36px;
  height: 36px;
  background: rgba(0, 0, 0, 0.04);
  color: #3d414a;
  border-radius: 50%;
}

.tour-btn-prev:hover,
.tour-btn-next:hover {
  background: rgba(58, 130, 247, 0.1);
  color: #3a82f7;
}

.tour-btn-skip {
  padding: 7px 16px;
  background: transparent;
  color: #8a8f9c;
  font-size: 12px;
  font-weight: 600;
}

.tour-btn-skip:hover {
  color: #5c6170;
  background: rgba(0, 0, 0, 0.04);
}

.tour-btn-next {
  background: linear-gradient(135deg, #3a82f7, #6a5acd);
  color: #fff;
}

.tour-btn-next:hover {
  opacity: 0.9;
  transform: scale(1.05);
}
</style>
