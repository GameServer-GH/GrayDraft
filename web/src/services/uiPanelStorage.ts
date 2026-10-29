const KEY_PREFIX = 'graydraft:ui-panels-v1:'

export type StoredUiPanels = {
  v: 1
  sidebarOpen: boolean
  agentConversationOpen: boolean
}

function storageKey(workspaceKey: string): string {
  return `${KEY_PREFIX}${encodeURIComponent(workspaceKey.trim()).slice(0, 160)}`
}

export function readUiPanelsLocal(workspaceKey: string | null): StoredUiPanels | null {
  if (!workspaceKey?.trim()) return null
  try {
    const raw = localStorage.getItem(storageKey(workspaceKey))
    if (!raw) return null
    const data = JSON.parse(raw) as StoredUiPanels
    if (data?.v !== 1) return null
    if (typeof data.sidebarOpen !== 'boolean' || typeof data.agentConversationOpen !== 'boolean') {
      return null
    }
    return data
  } catch {
    return null
  }
}

export function writeUiPanelsLocal(
  workspaceKey: string | null,
  panels: Pick<StoredUiPanels, 'sidebarOpen' | 'agentConversationOpen'>,
) {
  if (!workspaceKey?.trim()) return
  const payload: StoredUiPanels = {
    v: 1,
    sidebarOpen: panels.sidebarOpen,
    agentConversationOpen: panels.agentConversationOpen,
  }
  try {
    localStorage.setItem(storageKey(workspaceKey), JSON.stringify(payload))
  } catch {
    /* quota / private mode */
  }
}

export function clearUiPanelsLocal(workspaceKey: string | null) {
  if (!workspaceKey?.trim()) return
  try {
    localStorage.removeItem(storageKey(workspaceKey))
  } catch {
    /* ignore */
  }
}
