<script setup lang="ts">
import Icon from '@/components/common/Icon.vue'

withDefaults(
  defineProps<{
    /** 图标名，对应 public/icons */
    icon?: string
    iconSize?: number
    /** 进行中（例如流式生成）：显示绕边旋转光效 */
    pending?: boolean
    /** 不可点击（内容与 pending 共同决定） */
    disabled?: boolean
    ariaLabel?: string
  }>(),
  {
    icon: 'send',
    iconSize: 18,
    pending: false,
    disabled: false,
    ariaLabel: '提交',
  },
)

const emit = defineEmits<{
  click: []
}>()

function onClick() {
  emit('click')
}
</script>

<template>
  <button
    type="button"
    class="icon-circle-btn"
    :class="{
      'icon-circle-btn--pending': pending,
      'icon-circle-btn--muted': disabled && !pending,
    }"
    :disabled="disabled"
    :aria-busy="pending"
    :aria-label="ariaLabel"
    @click="onClick"
  >
    <span v-if="pending" class="icon-circle-btn__orbit" aria-hidden="true" />
    <Icon class="icon-circle-btn__icon" :name="icon" :size="iconSize" />
  </button>
</template>

<style scoped>
.icon-circle-btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 34px;
  height: 34px;
  padding: 0;
  border: none;
  border-radius: 50%;
  cursor: pointer;
  background: var(--color-accent);
  color: #fff;
  box-shadow: 0 4px 14px rgba(0, 122, 255, 0.35);
  transition:
    transform 0.14s ease,
    filter 0.16s ease,
    opacity 0.16s ease,
    box-shadow 0.16s ease;
}

.icon-circle-btn__icon {
  position: relative;
  z-index: 1;
  display: block;
  color: inherit;
  opacity: 0.95;
}

.icon-circle-btn:not(:disabled):hover {
  filter: brightness(1.05);
}

.icon-circle-btn:not(:disabled):active {
  transform: scale(0.93);
}

.icon-circle-btn:focus-visible {
  outline: none;
  box-shadow:
    0 4px 14px rgba(0, 122, 255, 0.35),
    0 0 0 3px rgba(0, 122, 255, 0.22);
}

.icon-circle-btn:disabled {
  cursor: not-allowed;
  transform: none;
  filter: none;
}

.icon-circle-btn--muted:disabled {
  opacity: 0.45;
  box-shadow: none;
}

.icon-circle-btn--pending:disabled {
  opacity: 1;
  box-shadow: 0 2px 10px rgba(0, 122, 255, 0.28);
}

/* 绕圆周旋转的高亮光段（conic 渐变 + 环形 mask） */
.icon-circle-btn__orbit {
  position: absolute;
  inset: -3px;
  z-index: 0;
  border-radius: 50%;
  pointer-events: none;
  background: conic-gradient(
    from 0deg,
    rgba(255, 255, 255, 0) 0deg,
    rgba(255, 255, 255, 0.15) 40deg,
    rgba(180, 220, 255, 0.95) 95deg,
    #ffffff 115deg,
    rgba(120, 190, 255, 1) 140deg,
    rgba(0, 122, 255, 0.85) 175deg,
    rgba(0, 90, 220, 0.35) 220deg,
    rgba(255, 255, 255, 0) 360deg
  );
  animation: icon-circle-orbit-spin 1.05s linear infinite;
  -webkit-mask: radial-gradient(
    farthest-side,
    transparent calc(100% - 3px),
    #000 calc(100% - 2.5px),
    #000 100%,
    transparent 100%
  );
  mask: radial-gradient(
    farthest-side,
    transparent calc(100% - 3px),
    #000 calc(100% - 2.5px),
    #000 100%,
    transparent 100%
  );
}

@keyframes icon-circle-orbit-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .icon-circle-btn__orbit {
    animation-duration: 2.4s;
  }

  .icon-circle-btn:not(:disabled):active {
    transform: none;
  }
}
</style>
