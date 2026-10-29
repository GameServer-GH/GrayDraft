import { callBackend } from '@/services/bridge'
import { agentModelField } from '@/utils/agentModelRequest'
import type { NotebookMeta } from '@/types/notebook'
import type {
  CreateKnowledgeResponse,
  IngestTaskResponse,
  KnowledgeDocumentItem,
  KnowledgeEntryDetail,
  KnowledgeListResponse,
  KnowledgeGraphResponse,
  RetrievalSearchResponse,
  SmartImportResponse,
} from '@/types/knowledge'

function docToNotebookMeta(d: KnowledgeDocumentItem): NotebookMeta {
  return {
    id: d.id,
    title: d.title || 'Untitled',
    updatedAt: d.updated_at,
    preview: d.preview,
    indexReady: d.index_ready,
    qualityScore: d.quality_score,
    chunkCount: d.chunk_count,
    extensions: d.extensions,
  }
}

export async function listKnowledgeEntries(opts?: { limit?: number; offset?: number }): Promise<NotebookMeta[]> {
  const res = await callBackend<KnowledgeListResponse>('ListKnowledgeEntries', opts ?? {})
  return (res.items ?? []).map(docToNotebookMeta)
}

export async function createKnowledgeEntry(body: {
  title: string
  content: string
  source_url?: string
  extensions?: Record<string, unknown>
}): Promise<CreateKnowledgeResponse> {
  return callBackend<CreateKnowledgeResponse>('CreateKnowledgeEntry', body)
}

export async function getIngestTask(taskId: string): Promise<IngestTaskResponse> {
  return callBackend<IngestTaskResponse>('GetIngestTask', { task_id: taskId })
}

export async function deleteKnowledgeEntry(id: string): Promise<void> {
  await callBackend<{ ok?: boolean }>('DeleteKnowledgeEntry', { id })
}

export async function getKnowledgeEntry(id: string): Promise<KnowledgeEntryDetail> {
  return callBackend<KnowledgeEntryDetail>('GetKnowledgeEntry', { id })
}

export async function updateKnowledgeEntry(body: {
  id: string
  title: string
  content: string
  source_url?: string
  extensions?: Record<string, unknown>
}): Promise<CreateKnowledgeResponse> {
  return callBackend<CreateKnowledgeResponse>('UpdateKnowledgeEntry', body)
}

export async function getKnowledgeGraph(opts?: { limit?: number }): Promise<KnowledgeGraphResponse> {
  return callBackend<KnowledgeGraphResponse>('GetKnowledgeGraph', { limit: opts?.limit ?? 120 })
}

export async function nexusHealth(): Promise<{ status?: string; nexus?: string; nexusError?: string }> {
  return callBackend('NexusHealth', {})
}

export async function retrievalSearch(body: {
  query: string
  top_k?: number
  skip_cache?: boolean
}): Promise<RetrievalSearchResponse> {
  return callBackend<RetrievalSearchResponse>('RetrievalSearch', {
    top_k: 5,
    ...body,
  })
}

export type SmartImportAttachmentInput = {
  filename: string
  mime_type?: string
  data_base64: string
}

export type MultimodalPromptDefaultsResponse = {
  base_prompt: string
  workspace_prompt?: string
  enabled?: boolean
}

export async function getMultimodalPromptDefaults(): Promise<MultimodalPromptDefaultsResponse> {
  return callBackend<MultimodalPromptDefaultsResponse>('GetMultimodalPromptDefaults', {})
}

export async function saveMultimodalWorkspacePrompt(workspace_prompt: string): Promise<void> {
  await callBackend<{ ok?: boolean }>('SaveMultimodalWorkspacePrompt', { workspace_prompt })
}

export async function getWorkspaceContext(): Promise<import('@/services/workspace').WorkspaceContextProfile> {
  return callBackend('GetWorkspaceContext', {})
}

export async function saveWorkspaceContext(
  profile: import('@/services/workspace').WorkspaceContextProfile,
): Promise<void> {
  await callBackend<{ ok?: boolean }>('SaveWorkspaceContext', profile)
}

/** 用一句话描述，由 LLM 扩展为完整结构化工作区背景并保存 */
export async function enrichWorkspaceContext(description: string): Promise<
  import('@/services/workspace').WorkspaceContextProfile
> {
  const res = await callBackend<{
    ok?: boolean
    profile?: import('@/services/workspace').WorkspaceContextProfile
  }>('EnrichWorkspaceContext', { description })
  return res.profile ?? { description: description.trim() }
}

export async function agentSmartImport(body: {
  raw_text: string
  filename?: string
  source_url?: string
  target_doc_id?: string
  split?: boolean
  submit?: boolean
  use_llm?: boolean
  agent_model?: string
  extensions?: Record<string, unknown>
  segments_override?: { index: number; line_range: [number, number] }[]
  attachments?: SmartImportAttachmentInput[]
  /** Image base64 for server-side VLM recognition before import */
  media_image_base64?: string
  media_image_mime_type?: string
  /** Video base64 for server-side VLM recognition before import */
  media_video_base64?: string
  media_video_mime_type?: string
  /** Publicly accessible image or video URL */
  media_url?: string
}): Promise<SmartImportResponse> {
  return callBackend<SmartImportResponse>('AgentSmartImport', {
    split: true,
    use_llm: true,
    generate_qa: true,
    ...body,
    ...agentModelField(body.agent_model ?? 'auto'),
  })
}

const POLL_MS = 1200
const POLL_MAX = 90

/** 轮询 UGC 任务直至完成或失败 */
export async function waitForIngestTask(
  taskId: string,
  onStatus?: (st: IngestTaskResponse) => void,
): Promise<IngestTaskResponse> {
  for (let i = 0; i < POLL_MAX; i++) {
    const st = await getIngestTask(taskId)
    onStatus?.(st)
    if (st.status === 'completed' || st.status === 'failed') {
      return st
    }
    await new Promise((r) => setTimeout(r, POLL_MS))
  }
  throw new Error('ingest task timed out')
}

// ── SSE subscription for real-time knowledge library updates ──

/**
 * Subscribe to knowledge library SSE events.
 * Returns a cleanup function to close the connection.
 */
export function subscribeKnowledgeLibrary(onEvent: (event: { type: string; data: string }) => void): () => void {
    const url = '/api/knowledge/library/events'
    const es = new EventSource(url)

    es.addEventListener('library-changed', (e: MessageEvent) => {
        onEvent({ type: 'library-changed', data: e.data })
    })

    es.onerror = () => {
        // EventSource auto-reconnects; no action needed.
    }

    return () => es.close()
}
