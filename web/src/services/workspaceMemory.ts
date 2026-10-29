import { callBackend } from '@/services/bridge'

export type WorkspaceMemory = {
  memory_id: string
  kind: string
  subject: string
  value?: Record<string, unknown> | null
  confidence?: number
  status?: string
}

export async function listMemories(): Promise<WorkspaceMemory[]> {
  const res = await callBackend<{ items?: WorkspaceMemory[] }>('ListMemories', {})
  return Array.isArray(res.items) ? res.items : []
}

export async function revokeMemory(memoryId: string): Promise<void> {
  await callBackend('RevokeMemory', { memory_id: memoryId })
}

export function memorySummary(item: WorkspaceMemory): string {
  const value = item.value
  if (value && typeof value === 'object') {
    for (const key of ['text', 'content', 'summary', 'value', 'note']) {
      const v = value[key]
      if (typeof v === 'string' && v.trim()) return v.trim()
    }
    const raw = JSON.stringify(value)
    if (raw && raw !== '{}') return raw.length > 180 ? `${raw.slice(0, 180)}…` : raw
  }
  return item.subject
}
