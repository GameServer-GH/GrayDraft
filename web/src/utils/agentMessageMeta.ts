import type { AgentMessage } from '@/stores/agent'
import type { ChatCitation } from '@/types/knowledge'
import { formatQuerySemantic4D, formatRetrievalTrace } from '@/utils/semantic4d'

export type AgentMessageCitationView = {
  key: string
  docId: string
  title: string
  chunkId?: string
  mediaUrl?: string
  chunkCount?: number
}

export type AgentMessageMetaView = {
  strategy: string | null
  citations: AgentMessageCitationView[]
  /** Query 4D 预过滤标签（chat meta 或检索 footer） */
  query4dText: string | null
  semantic4DFilterApplied: boolean
  retrievalTraceText: string | null
  /** confidence / chunks / total 等仍用一行小字展示 */
  statsText: string | null
}

function citationTitle(c: ChatCitation): string {
  return (c.doc_title || c.doc_id || '').trim() || c.doc_id
}

function citationsFromList(cites: ChatCitation[]): AgentMessageCitationView[] {
  return cites.map((c, i) => ({
    key: `${c.doc_id}-${c.chunk_ids?.[0] ?? i}`,
    docId: c.doc_id,
    title: citationTitle(c),
    chunkId: c.chunk_ids?.[0],
    mediaUrl: c.media_url,
    chunkCount: c.chunk_ids?.length,
  }))
}

function parseLegacyFooter(footer: string): {
  strategy: string | null
  citations: AgentMessageCitationView[]
  statsLines: string[]
} {
  let strategy: string | null = null
  const citations: AgentMessageCitationView[] = []
  const statsLines: string[] = []

  for (const raw of footer.split('\n')) {
    const line = raw.trim()
    if (!line) continue
    if (line.startsWith('strategy: ')) {
      strategy = line.slice('strategy: '.length).trim() || null
      continue
    }
    if (line.startsWith('citations: ')) {
      const refs = line
        .slice('citations: '.length)
        .split('·')
        .map((s) => s.trim())
        .filter(Boolean)
      refs.forEach((title, i) => {
        citations.push({ key: `legacy-${i}-${title}`, docId: '', title })
      })
      continue
    }
    statsLines.push(line)
  }

  return { strategy, citations, statsLines }
}

/** Footer stat lines duplicated by structured confidence / chunksUsed fields. */
function isStructuredStatDuplicate(
  line: string,
  confidence: number | undefined,
  chunksUsed: number | undefined,
): boolean {
  const t = line.trim().toLowerCase()
  if (typeof confidence === 'number' && Number.isFinite(confidence) && t.startsWith('confidence:')) {
    return true
  }
  if (typeof chunksUsed === 'number' && chunksUsed > 0 && t.startsWith('chunks:')) {
    return true
  }
  return false
}

export function formatAgentStrategyLabel(strategy: string): string {
  return strategy
    .trim()
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase())
}

export function buildAgentMessageMeta(m: AgentMessage): AgentMessageMetaView {
  let strategy = m.strategy?.trim() || m.retrieval?.strategy?.trim() || null
  let citations: AgentMessageCitationView[] = m.citations?.length
    ? citationsFromList(m.citations)
    : []
  const statsLines: string[] = []

  if (typeof m.confidence === 'number' && Number.isFinite(m.confidence)) {
    statsLines.push(`confidence: ${m.confidence.toFixed(2)}`)
  }
  if (typeof m.chunksUsed === 'number' && m.chunksUsed > 0) {
    statsLines.push(`chunks: ${m.chunksUsed}`)
  }

  if (m.footer?.trim()) {
    const legacy = parseLegacyFooter(m.footer)
    if (!strategy) strategy = legacy.strategy
    if (!citations.length) citations = legacy.citations
    for (const line of legacy.statsLines) {
      if (isStructuredStatDuplicate(line, m.confidence, m.chunksUsed)) continue
      statsLines.push(line)
    }
  }

  const q4d =
    m.querySemantic4D ??
    (m.retrieval?.query_semantic_4d as AgentMessage['querySemantic4D'])
  const filterOn =
    m.semantic4DFilterApplied === true || m.retrieval?.semantic_4d_filter_applied === true
  const query4dText = formatQuerySemantic4D(q4d) || null

  const trace =
    m.retrievalTrace ??
    (m.retrieval?.retrieval_trace as AgentMessage['retrievalTrace'])
  const retrievalTraceText = formatRetrievalTrace(trace) || null

  const statsText = statsLines.length ? statsLines.join(' · ') : null
  return {
    strategy,
    citations,
    query4dText,
    semantic4DFilterApplied: filterOn,
    retrievalTraceText,
    statsText,
  }
}
