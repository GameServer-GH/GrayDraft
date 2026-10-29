import { streamKnowledgeChat } from '@/services/knowledgeChat'
import { formatNexusError } from '@/utils/nexusError'
import { stripEmoji } from '@/utils/stripEmoji'
import { normalizeKBMarkdown } from '@/utils/normalizeKBMarkdown'
import type {
  ChatCitation,
  ChatSource,
  KnowledgeChatMode,
  QuerySemantic4D,
  RetrievalTrace,
} from '@/types/knowledge'
import { useI18n } from '@/composables/useI18n'
import { useAgentStore } from '@/stores/agent'
import { useUiStore } from '@/stores/ui'

export function useKnowledgeChat() {
  const agent = useAgentStore()
  const ui = useUiStore()
  const { t } = useI18n()

  async function ask(
    threadId: string,
    query: string,
    chatMode: KnowledgeChatMode,
    init?: {
      signal?: AbortSignal
      /** Image base64 (preprocessed) */
      imageBase64?: string
      imageMimeType?: string
      /** Video base64 */
      videoBase64?: string
      videoMimeType?: string
      /** Public URL to image/video */
      mediaUrl?: string
    },
  ): Promise<void> {
    const thread = agent.ensureThread(threadId)
    const thinkingMode = thread.thinkingMode ?? true
    agent.setLastError(threadId, null)
    agent.appendUserMessage(threadId, stripEmoji(query))
    agent.beginStreamingAssistant(threadId, { thinkingMode })
    let answer = ''
    let citations: ChatCitation[] | undefined
    let sources: ChatSource[] | undefined
    let strategy: string | undefined
    let confidence: number | undefined
    let chunksUsed: number | undefined
    let querySemantic4D: QuerySemantic4D | undefined
    let semantic4DFilterApplied: boolean | undefined
    let retrievalTrace: RetrievalTrace | undefined

    try {
      await streamKnowledgeChat({
        query,
        chatMode,
        sessionId: thread.sessionId ?? undefined,
        agentModel: ui.selectedAgentModel,
        thinkingMode,
        signal: init?.signal,
        imageBase64: init?.imageBase64,
        imageMimeType: init?.imageMimeType,
        videoBase64: init?.videoBase64,
        videoMimeType: init?.videoMimeType,
        mediaUrl: init?.mediaUrl,
        onEvent(ev) {
          if (ev.type === 'thinking') {
            if (!thinkingMode) return
            // llm reasoning 为逐 token 增量，保留原始空格；其余状态文案可 trim
            const raw = ev.message || ''
            const isLLM = ev.thinking_phase === 'llm'
            const text = isLLM ? raw : raw.trim()
            if (text) {
              agent.appendThinkingStep(threadId, {
                text,
                phase: ev.thinking_phase,
                tool: ev.tool_name,
                step: ev.step_id,
              })
            }
            return
          }
          if (ev.type === 'meta' || ev.type === 'done') {
            if (ev.citations?.length) citations = ev.citations
            if (ev.sources?.length) sources = ev.sources
            if (ev.strategy) strategy = ev.strategy
            if (typeof ev.confidence === 'number' && Number.isFinite(ev.confidence)) {
              confidence = ev.confidence
            }
            if (typeof ev.chunks_used === 'number' && ev.chunks_used > 0) {
              chunksUsed = ev.chunks_used
            }
            if (ev.query_semantic_4d) querySemantic4D = ev.query_semantic_4d
            if (ev.semantic_4d_filter_applied != null) {
              semantic4DFilterApplied = ev.semantic_4d_filter_applied
            }
            if (ev.retrieval_trace) retrievalTrace = ev.retrieval_trace
          }
          if (ev.type === 'chunk' && ev.delta) {
            answer = stripEmoji(answer + ev.delta)
            answer = normalizeKBMarkdown(answer)
            agent.patchLastAssistantMessage(threadId, { body: answer, streaming: true })
          }
          // done.message 为权威终态（含门禁拒答替换草稿）
          if (ev.type === 'done' && typeof ev.message === 'string' && ev.message.trim()) {
            answer = normalizeKBMarkdown(stripEmoji(ev.message))
            agent.patchLastAssistantMessage(threadId, { body: answer, streaming: true })
          }
          if (ev.type === 'error') {
            throw new Error(ev.message || 'chat error')
          }
        },
      })
      if (!answer.trim()) {
        answer = t('agentDock.chatNoTextReply')
      }
      answer = normalizeKBMarkdown(stripEmoji(answer))
      agent.patchLastAssistantMessage(threadId, {
        body: answer,
        citations,
        sources,
        strategy,
        confidence,
        chunksUsed,
        querySemantic4D,
        semantic4DFilterApplied,
        retrievalTrace,
        streaming: false,
      })
      agent.finalizeThinking(threadId, true)
    } catch (e) {
      const msg = formatNexusError(e)
      agent.patchLastAssistantMessage(threadId, {
        body: answer.trim() || msg,
        streaming: false,
      })
      agent.finalizeThinking(threadId)
      agent.setLastError(threadId, msg)
      throw e
    }
  }

  return { ask }
}
