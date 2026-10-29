/** 当前会话的工作区密钥（明文）；仅内存，与 localStorage 同步由 workspaceKeyStorage 负责。 */
let currentWorkspaceKey: string | null = null

export function setWorkspaceContextKey(key: string | null) {
  currentWorkspaceKey = key?.trim() ? key.trim() : null
}

export function getWorkspaceContextKey(): string | null {
  return currentWorkspaceKey
}
