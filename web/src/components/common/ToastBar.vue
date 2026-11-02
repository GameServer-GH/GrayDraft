<script setup lang="ts">
import { useToast } from '@/composables/useToast'

const { current } = useToast()
</script>

<template>
  <Teleport to="body">
    <Transition name="toast-bar">
      <div v-if="current" class="toast-bar" role="status" aria-live="polite">
        {{ current }}
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.toast-bar {
  position: fixed;
  z-index: 1000000;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  pointer-events: none;
  box-sizing: border-box;
  min-width: min(240px, calc(100vw - 48px));
  max-width: min(440px, calc(100vw - 48px));
  margin: 0;
  padding: 14px 32px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.72);
  background: rgba(255, 255, 255, 0.94);
  box-shadow: var(--shadow-float-lg);
  backdrop-filter: blur(24px) saturate(1.35);
  -webkit-backdrop-filter: blur(24px) saturate(1.35);
  color: var(--color-text);
  font-family:
    'Inter', ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  font-size: 14px;
  font-weight: 600;
  line-height: 1.35;
  letter-spacing: -0.01em;
  text-align: center;
  white-space: nowrap;
}

.toast-bar-enter-active {
  animation: toast-bar-in 0.18s ease;
}

.toast-bar-leave-active {
  animation: toast-bar-in 0.18s ease reverse;
}

@keyframes toast-bar-in {
  from { opacity: 0; }
  to { opacity: 1; }
}

@media (prefers-reduced-motion: reduce) {
  .toast-bar-enter-active,
  .toast-bar-leave-active {
    animation: none;
  }
}
</style>
