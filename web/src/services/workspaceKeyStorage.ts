const STORAGE_KEY = 'graydraft:workspace-key-v1'

export function readStoredWorkspaceKey(): string | null {
  try {
    const v = localStorage.getItem(STORAGE_KEY)?.trim()
    return v || null
  } catch {
    return null
  }
}

export function writeStoredWorkspaceKey(key: string) {
  try {
    localStorage.setItem(STORAGE_KEY, key.trim())
  } catch {
    /* ignore */
  }
}

export function clearStoredWorkspaceKey() {
  try {
    localStorage.removeItem(STORAGE_KEY)
  } catch {
    /* ignore */
  }
}
