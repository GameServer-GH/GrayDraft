import { ref } from 'vue'

/**
 * 悬浮拖拽；拖拽时可将 --glass-blur 置 0 — 方案 composables/useFloatingDrag.ts
 */
export function useFloatingDrag() {
  const dragging = ref(false)
  return { dragging }
}
