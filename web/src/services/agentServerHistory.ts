import { callBackend } from '@/services/bridge'
import type { PersistedAgentThread } from '@/services/agentHistoryStorage'

type ThreadResponse =
  | { thread: null }
  | {
      thread_key?: string
      payload?: PersistedAgentThread
      updated_at?: string
    }

export async function loadServerThread(threadKey: string): Promise<PersistedAgentThread | null> {
  const res = await callBackend<ThreadResponse>('GetAgentThread', { thread_key: threadKey })
  if (!res || !('payload' in res) || !res.payload || !Array.isArray(res.payload.messages)) {
    return null
  }
  const payload = res.payload
  if (typeof payload.updatedAt !== 'number' && res.updated_at) {
    const parsed = Date.parse(res.updated_at)
    if (Number.isFinite(parsed)) payload.updatedAt = parsed
  }
  return payload
}

export async function saveServerThread(threadKey: string, payload: PersistedAgentThread): Promise<void> {
  await callBackend('SaveAgentThread', { thread_key: threadKey, payload })
}

export async function deleteServerThread(threadKey: string): Promise<void> {
  await callBackend('DeleteAgentThreads', { thread_key: threadKey })
}

export async function deleteAllServerThreads(): Promise<void> {
  await callBackend('DeleteAgentThreads', {})
}
