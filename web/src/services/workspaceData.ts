import { callBackend } from '@/services/bridge'
import {
  clearWorkspaceAgentHistory,
  countWorkspaceAgentHistory,
  type AgentHistoryStorageStats,
} from '@/services/agentHistoryStorage'
import { getWorkspaceContextKey } from '@/services/workspaceContext'
import { deleteAllServerThreads } from '@/services/agentServerHistory'
import { useAgentStore } from '@/stores/agent'

export type WorkspaceDataStats = {
  local: AgentHistoryStorageStats
  knowledge_documents: number
  redis_retrieval_entries: number
  redis_answer_entries: number
  nexus_available: boolean
}

type RemoteWorkspaceDataStats = {
  knowledge_documents?: number
  redis_retrieval_entries?: number
  redis_answer_entries?: number
}

export async function fetchWorkspaceDataStats(): Promise<WorkspaceDataStats> {
  const wsKey = getWorkspaceContextKey() ?? ''
  const local = wsKey ? countWorkspaceAgentHistory(wsKey) : { threads: 0, bytes: 0 }

  try {
    const remote = await callBackend<RemoteWorkspaceDataStats>('GetWorkspaceDataStats', {})
    return {
      local,
      knowledge_documents: remote.knowledge_documents ?? 0,
      redis_retrieval_entries: remote.redis_retrieval_entries ?? 0,
      redis_answer_entries: remote.redis_answer_entries ?? 0,
      nexus_available: true,
    }
  } catch {
    return {
      local,
      knowledge_documents: 0,
      redis_retrieval_entries: 0,
      redis_answer_entries: 0,
      nexus_available: false,
    }
  }
}

export type ClearWorkspaceDataTarget = 'local' | 'knowledge' | 'redis'

export async function clearWorkspaceData(target: ClearWorkspaceDataTarget): Promise<void> {
  if (target === 'local') {
    const wsKey = getWorkspaceContextKey()
    if (!wsKey) return
    let serverError: unknown = null
    try {
      await deleteAllServerThreads()
    } catch (e) {
      serverError = e
    }
    clearWorkspaceAgentHistory(wsKey)
    useAgentStore().resetInMemoryThreads()
    if (serverError) throw serverError
    return
  }
  await callBackend('ClearWorkspaceData', { target })
}

export function formatStorageBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}
