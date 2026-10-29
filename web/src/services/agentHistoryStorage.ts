import type { AgentMessage, AgentNotebookThread, KnowledgeChatMode } from '@/stores/agent'
const STORAGE_PREFIX = 'graydraft:agent-history:v1:'

function workspaceKeyPrefix(workspaceKey: string): string {
  const ws = encodeURIComponent(workspaceKey.trim()).slice(0, 160)
  return STORAGE_PREFIX + ws + '_'
}

export type AgentHistoryStorageStats = {
  threads: number
  bytes: number
}

export function countWorkspaceAgentHistory(workspaceKey: string): AgentHistoryStorageStats {
  const prefix = workspaceKeyPrefix(workspaceKey)
  let threads = 0
  let bytes = 0
  try {
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i)
      if (!key || !key.startsWith(prefix)) continue
      threads++
      const val = localStorage.getItem(key)
      if (val) bytes += key.length + val.length
    }
  } catch {
    /* private mode */
  }
  return { threads, bytes }
}

export function clearWorkspaceAgentHistory(workspaceKey: string): number {
  const prefix = workspaceKeyPrefix(workspaceKey)
  const toRemove: string[] = []
  try {
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i)
      if (key?.startsWith(prefix)) toRemove.push(key)
    }
    for (const key of toRemove) localStorage.removeItem(key)
  } catch {
    /* ignore */
  }
  return toRemove.length
}

export type PersistedAgentThread = {
  messages: AgentMessage[]
  sessionId: string | null
  knowledgeChatMode?: KnowledgeChatMode
  thinkingMode?: boolean
  knowledgeSearchTopK?: number
  updatedAt?: number
}

function storageKey(threadKey: string): string {
  return STORAGE_PREFIX + threadKey
}

function stripForPersist(messages: AgentMessage[]): AgentMessage[] {
  return messages
    .filter((m) => {
      if (m.role === 'user') {
        return Boolean(m.body.trim()) || Boolean(m.attachmentNames?.length)
      }
      return Boolean(m.body.trim() || m.footer?.trim() || (m.thinking?.length ?? 0) > 0)
    })
    .map((m) => ({
      ...m,
      streaming: false,
      thinkingActive: false,
      thinkingOpen: false,
      collapsed: m.role === 'assistant' ? Boolean(m.collapsed) : undefined,
      feedback: m.role === 'assistant' && (m.feedback === 'up' || m.feedback === 'down') ? m.feedback : undefined,
      thinking:
        m.role === 'assistant' && Array.isArray(m.thinking) && m.thinking.length
          ? m.thinking.map((s) => ({
              text: s.text,
              phase: s.phase,
              tool: s.tool,
              step: s.step,
            }))
          : undefined,
    }))
}

export function loadPersistedAgentThread(threadKey: string): PersistedAgentThread | null {
  try {
    const raw = localStorage.getItem(storageKey(threadKey))
    if (!raw) return null
    const data = JSON.parse(raw) as PersistedAgentThread
    if (!data || !Array.isArray(data.messages)) return null
    return {
      messages: data.messages
        .filter((m) => m && (m.role === 'user' || m.role === 'assistant') && typeof m.body === 'string')
        .map((m) => ({
          ...m,
          collapsed: m.role === 'assistant' ? Boolean(m.collapsed) : undefined,
          feedback:
            m.role === 'assistant' && (m.feedback === 'up' || m.feedback === 'down')
              ? m.feedback
              : undefined,
        })),
      sessionId: typeof data.sessionId === 'string' ? data.sessionId : null,
      knowledgeChatMode: data.knowledgeChatMode,
      thinkingMode: typeof data.thinkingMode === 'boolean' ? data.thinkingMode : undefined,
      knowledgeSearchTopK: data.knowledgeSearchTopK,
      updatedAt: typeof data.updatedAt === 'number' ? data.updatedAt : undefined,
    }
  } catch {
    return null
  }
}

export function savePersistedAgentThread(
  threadKey: string,
  thread: AgentNotebookThread,
  updatedAt = Date.now(),
): PersistedAgentThread | null {
  try {
    const payload: PersistedAgentThread = {
      messages: stripForPersist(thread.messages),
      sessionId: thread.sessionId,
      updatedAt,
    }
    if (thread.knowledgeChatMode) payload.knowledgeChatMode = thread.knowledgeChatMode
    if (typeof thread.thinkingMode === 'boolean') payload.thinkingMode = thread.thinkingMode
    if (thread.knowledgeSearchTopK != null) payload.knowledgeSearchTopK = thread.knowledgeSearchTopK
    localStorage.setItem(storageKey(threadKey), JSON.stringify(payload))
    return payload
  } catch {
    return null
  }
}

export function clearPersistedAgentThread(threadKey: string): void {
  try {
    localStorage.removeItem(storageKey(threadKey))
  } catch {
    /* ignore */
  }
}
