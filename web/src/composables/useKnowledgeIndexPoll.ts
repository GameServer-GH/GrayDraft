import { onUnmounted, watch } from 'vue'
import { useCanvasStore } from '@/stores/canvas'
import { useKnowledgeDetailStore } from '@/stores/knowledgeDetail'

const POLL_MS = 10000
const MAX_TICKS = 18

/** 列表中存在「索引中」条目时轮询刷新，直至全部就绪或超时 */
export function useKnowledgeIndexPoll(enabled: () => boolean, reload: () => Promise<void>) {
  const canvas = useCanvasStore()
  const detailStore = useKnowledgeDetailStore()
  let timer: ReturnType<typeof setInterval> | null = null
  let ticks = 0

  function stop() {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
    ticks = 0
  }

  function hasIndexing() {
    return canvas.notebooks.some((n) => n.indexReady === false)
  }

  function start() {
    stop()
    if (!enabled() || !hasIndexing()) return
    timer = setInterval(() => {
      ticks++
      if (!enabled() || !hasIndexing() || ticks >= MAX_TICKS) {
        stop()
        return
      }
      void reload().then(() => {
        detailStore.invalidateAll()
        if (!hasIndexing()) stop()
      })
    }, POLL_MS)
  }

  watch(
    () => [enabled(), canvas.notebooks.map((n) => `${n.id}:${n.indexReady}`).join('|')] as const,
    () => {
      if (enabled() && hasIndexing()) start()
      else stop()
    },
    { immediate: true },
  )

  onUnmounted(stop)
}
