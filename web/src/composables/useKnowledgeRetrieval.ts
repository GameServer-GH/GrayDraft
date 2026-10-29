import { retrievalSearch } from '@/services/knowledge'
import { formatNexusError } from '@/utils/nexusError'
import { stripEmoji } from '@/utils/stripEmoji'
import type { RetrievalSearchResponse } from '@/types/knowledge'
import { formatQuerySemantic4D, formatRetrievalTrace } from '@/utils/semantic4d'
import { useAgentStore } from '@/stores/agent'
import { useCanvasStore } from '@/stores/canvas'

function chunkPreview(text: string, max = 280): string {
  const s = stripEmoji(text.trim())
  if (s.length <= max) return s
  return `${s.slice(0, max)}…`
}

function formatSearchBody(res: RetrievalSearchResponse): string {
  const chunks = res.chunks ?? []
  if (chunks.length === 0) {
    return '（无命中分块，请确认知识已索引或调整查询）'
  }
  const lines: string[] = []
  chunks.forEach((c, i) => {
    const title = (c.doc_title || c.doc_id || '').trim()
    const head = [
      `#${i + 1}`,
      `score ${c.score.toFixed(3)}`,
      title ? `「${title}」` : c.doc_id.slice(0, 8),
      `chunk ${c.chunk_index + 1}`,
      c.source || '',
    ]
      .filter(Boolean)
      .join(' · ')
    lines.push(head)
    lines.push(chunkPreview(c.content))
    lines.push('')
  })
  return lines.join('\n').trimEnd()
}

function formatSearchFooter(res: RetrievalSearchResponse): string {
  const parts: string[] = []
  parts.push(`total: ${res.total}`)
  if (res.cache_hit) parts.push(`cache: ${res.cache_hit}`)
  if (res.query_intent) parts.push(`intent: ${res.query_intent}`)
  if (res.catalog_version != null) parts.push(`catalog v${res.catalog_version}`)
  if (res.semantic_4d_filter_applied) {
    const q4d = formatQuerySemantic4D(res.query_semantic_4d)
    parts.push(q4d ? `4D filter: ${q4d}` : '4D filter: on')
  }
  const trace = formatRetrievalTrace(res.retrieval_trace)
  if (trace) parts.push(`trace: ${trace}`)
  return parts.join('\n')
}

export function useKnowledgeRetrieval() {
  const agent = useAgentStore()
  const canvas = useCanvasStore()

  function focusDocument(docId: string) {
    if (!canvas.notebooks.some((n) => n.id === docId)) return
    canvas.selectNotebook(docId)
    canvas.recenterViewToActiveNotebookOrOrigin()
  }

  async function search(
    threadId: string,
    query: string,
    opts?: { topK?: number; skipCache?: boolean; signal?: AbortSignal },
  ): Promise<RetrievalSearchResponse | null> {
    agent.setLastError(threadId, null)
    agent.appendUserMessage(threadId, stripEmoji(query))
    try {
      const res = await retrievalSearch({
        query,
        top_k: opts?.topK ?? 5,
        skip_cache: opts?.skipCache,
      })
      if (opts?.signal?.aborted) return null
      agent.appendAssistantMessage(threadId, formatSearchBody(res))
      const footer = formatSearchFooter(res)
      agent.patchLastAssistantMessage(threadId, {
        footer: footer || undefined,
        retrieval: res,
      })
      return res
    } catch (e) {
      if (opts?.signal?.aborted) return null
      const msg = formatNexusError(e)
      agent.appendAssistantMessage(threadId, msg)
      agent.setLastError(threadId, msg)
      throw e
    }
  }

  return { search, focusDocument, formatSearchBody, formatSearchFooter }
}
