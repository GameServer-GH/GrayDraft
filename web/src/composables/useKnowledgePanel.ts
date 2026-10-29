import { computed } from 'vue'
import { useWorkspaceSessionStore } from '@/stores/workspaceSession'
import {
  createKnowledgeEntry,
  deleteKnowledgeEntry,
  getKnowledgeEntry,
  listKnowledgeEntries,
  updateKnowledgeEntry,
  waitForIngestTask,
} from '@/services/knowledge'
import { formatNexusError } from '@/utils/nexusError'
import { listNotebooks } from '@/services/notebook'
import type { NotebookMeta } from '@/types/notebook'

/** 是否使用 GrayDraft 知识库 API（工作区已绑定 nexusKbType） */
export function useKnowledgePanel() {
  const session = useWorkspaceSessionStore()

  const enabled = computed(() => Boolean(session.nexusKbType?.trim()))

  async function loadEntries(): Promise<NotebookMeta[]> {
    if (enabled.value) {
      return listKnowledgeEntries()
    }
    return listNotebooks()
  }

  async function removeEntry(id: string): Promise<void> {
    if (enabled.value) {
      await deleteKnowledgeEntry(id)
      return
    }
    const { deleteNotebook } = await import('@/services/notebook')
    await deleteNotebook({ id })
  }

  async function addEntry(body: {
    title: string
    content: string
    source_url?: string
    extensions?: Record<string, unknown>
  }): Promise<{ docId?: string }> {
    if (!enabled.value) {
      throw new Error('knowledge panel not enabled')
    }
    const queued = await createKnowledgeEntry(body)
    const final = await waitForIngestTask(queued.task_id)
    if (final.status === 'failed') {
      throw new Error(final.error_message || 'ingest failed')
    }
    return { docId: final.doc_id || queued.doc_id }
  }

  async function fetchEntry(id: string) {
    return getKnowledgeEntry(id)
  }

  async function saveEntry(body: {
    id: string
    title: string
    content: string
    source_url?: string
    extensions?: Record<string, unknown>
  }): Promise<{ docId?: string }> {
    const queued = await updateKnowledgeEntry(body)
    const final = await waitForIngestTask(queued.task_id)
    if (final.status === 'failed') {
      throw new Error(final.error_message || 'update ingest failed')
    }
    return { docId: final.doc_id || body.id }
  }

  function formatError(err: unknown): string {
    return formatNexusError(err, { nexusConfigured: enabled.value })
  }

  return { enabled, loadEntries, removeEntry, addEntry, fetchEntry, saveEntry, formatError }
}
