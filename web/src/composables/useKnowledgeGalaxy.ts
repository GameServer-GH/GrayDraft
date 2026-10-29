import { storeToRefs } from 'pinia'
import { watch } from 'vue'
import { useGalaxyGraphStore } from '@/stores/galaxyGraph'

export type { GalaxySyncOp } from '@/utils/galaxySync'

/** 语义星系图：状态在 {@link useGalaxyGraphStore}，支持全量/增量同步。 */
export function useKnowledgeGalaxy(enabled: () => boolean) {
  const store = useGalaxyGraphStore()
  const { loading, error, layout, nodeCount, edgeCount } = storeToRefs(store)

  watch(
    enabled,
    (on) => {
      store.setActive(on)
    },
    { immediate: true },
  )

  return {
    loading,
    error,
    layout,
    nodeCount,
    edgeCount,
    reload: () => store.reload(),
    sync: store.sync,
  }
}
