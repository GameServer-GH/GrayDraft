import { computed } from 'vue'
import { useUiStore } from '@/stores/ui'
import { agentDockWidthBand } from '@/utils/agentDockLayout'

/** Match Workspace.vue floating panel geometry (knowledge sidebar = 320px). */
const KNOWLEDGE_SIDEBAR_BAND = 'min(320px, calc(100vw - 2 * var(--float-gap, 12px)))'

/**
 * Dynamic padding for KnowledgeLibraryView so content clears left list + right Agent dock.
 */
export function useKnowledgeLibraryInsets() {
  const ui = useUiStore()

  const libraryInsetStyle = computed(() => {
    const gap = 'var(--float-gap, 12px)'
    const paddingLeft = ui.sidebarOpen
      ? `calc(${gap} + ${KNOWLEDGE_SIDEBAR_BAND} + ${gap})`
      : gap
    const paddingRight = ui.agentConversationOpen
      ? `calc(${gap} + ${agentDockWidthBand(ui.agentConversationExpanded)} + ${gap})`
      : gap
    return {
      paddingTop: `calc(var(--topbar-h, 56px) + ${gap} + 8px)`,
      paddingBottom: gap,
      paddingLeft,
      paddingRight,
    }
  })

  return { libraryInsetStyle }
}
