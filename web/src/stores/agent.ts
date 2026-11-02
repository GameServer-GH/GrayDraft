import { defineStore } from 'pinia'
import {
  clearPersistedAgentThread,
  loadPersistedAgentThread,
  savePersistedAgentThread,
  type PersistedAgentThread,
} from '@/services/agentHistoryStorage'
import { deleteServerThread, loadServerThread, saveServerThread } from '@/services/agentServerHistory'
import type {
  ChatCitation,
  ChatSource,
  QuerySemantic4D,
  RetrievalSearchResponse,
  RetrievalTrace,
} from '@/types/knowledge'

export type AgentMessageRole = 'user' | 'assistant'

export type AgentMessageFeedback = 'up' | 'down'

/** Agent Loop 思考步骤（SSE thinking 事件） */
export interface AgentThinkingStep {
  text: string
  phase?: string
  tool?: string
  step?: number
}

export interface AgentMessage {
  id: string
  role: AgentMessageRole
  body: string
  /** 导入模式用户消息附带的文件名（对话中以标签展示，不写入正文） */
  attachmentNames?: string[]
  createdAt: number
  footer?: string
  streaming?: boolean
  /** 助手回复是否收起正文（持久化到本地 agent 历史） */
  collapsed?: boolean
  feedback?: AgentMessageFeedback | null
  /** GrayDraft chat done 事件原样透传 */
  citations?: ChatCitation[]
  sources?: ChatSource[]
  strategy?: string
  confidence?: number
  chunksUsed?: number
  /** GrayDraft retrieval/search 原样透传 */
  retrieval?: RetrievalSearchResponse
  /** Chat meta / 检索响应中的 query 4D */
  querySemantic4D?: QuerySemantic4D
  semantic4DFilterApplied?: boolean
  retrievalTrace?: RetrievalTrace
  /** Agent Loop 思考过程步骤 */
  thinking?: AgentThinkingStep[]
  /** 思考面板是否展开（思考中默认展开，结束后收起） */
  thinkingOpen?: boolean
  /** 是否仍在思考（流式进行中） */
  thinkingActive?: boolean
}

export type KnowledgeDockMode = 'import' | 'chat' | 'search'
export type KnowledgeChatMode = 'hybrid' | 'kb_only'

/** Pending media for recognition — transient, never persisted.
 *  Supports images (upload), videos (upload), and media URLs. */
export interface PendingImage {
  id: string
  base64: string
  thumbnail: string
  mimeType: string
  filename: string
  status: 'pending' | 'recognizing' | 'done' | 'error'
  error?: string
  /** 'image' or 'video' — default 'image' */
  mediaType?: 'image' | 'video'
  /** Direct URL to the media (for URL paste, no upload needed) */
  mediaUrl?: string
}

export interface AgentNotebookThread {
  messages: AgentMessage[]
  sessionId: string | null
  lastError: string | null
  composerDraft: string
  pendingAttachments: File[]
  /** Pending images/videos before recognition (transient, not persisted). */
  pendingImages: PendingImage[]
  knowledgeDockMode?: KnowledgeDockMode
  knowledgeChatMode?: KnowledgeChatMode
  thinkingMode?: boolean
  knowledgeSearchTopK?: number
}

function msgId() {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`
}

function emptyThread(persisted?: PersistedAgentThread | null): AgentNotebookThread {
  return {
    messages: persisted?.messages ? [...persisted.messages] : [],
    sessionId: persisted?.sessionId ?? null,
    lastError: null,
    composerDraft: '',
    pendingAttachments: [],
    pendingImages: [],
    knowledgeChatMode: persisted?.knowledgeChatMode,
    thinkingMode: persisted?.thinkingMode ?? true,
    knowledgeSearchTopK: persisted?.knowledgeSearchTopK,
  }
}

export type AgentStreamingPhase = 'import' | 'chat' | 'search'

const persistTimers = new Map<string, ReturnType<typeof setTimeout>>()
const serverThreadKeys = new Map<string, string>()
const hydrateTokens = new Map<string, number>()

export const useAgentStore = defineStore('agent', {
  state: () => ({
    threads: {} as Record<string, AgentNotebookThread>,
    streaming: false,
    streamingNotebookId: null as string | null,
    streamingPhase: null as AgentStreamingPhase | null,
  }),
  actions: {
    schedulePersist(threadId: string) {
      const existing = persistTimers.get(threadId)
      if (existing) clearTimeout(existing)
      persistTimers.set(
        threadId,
        setTimeout(() => {
          persistTimers.delete(threadId)
          const t = this.threads[threadId]
          if (!t) return
          const saved = savePersistedAgentThread(threadId, t)
          const serverKey = serverThreadKeys.get(threadId)
          if (saved && serverKey) void saveServerThread(serverKey, saved).catch(() => undefined)
        }, 280),
      )
    },

    bindServerThread(localId: string, serverKey: string) {
      serverThreadKeys.set(localId, serverKey)
      void this.hydrateFromServer(localId)
    },

    async hydrateFromServer(localId: string) {
      const serverKey = serverThreadKeys.get(localId)
      if (!serverKey) return
      const token = (hydrateTokens.get(localId) ?? 0) + 1
      hydrateTokens.set(localId, token)
      let remote: PersistedAgentThread | null = null
      try {
        remote = await loadServerThread(serverKey)
      } catch {
        return
      }
      if (hydrateTokens.get(localId) !== token || this.streaming) return
      const local = loadPersistedAgentThread(localId)
      const localAt = local?.updatedAt ?? 0
      const remoteAt = remote?.updatedAt ?? 0
      if (!remote) {
        if (local && local.messages.length > 0) void saveServerThread(serverKey, local).catch(() => undefined)
        return
      }
      if (!local || local.messages.length === 0 || remoteAt > localAt) {
        const t = this.ensureThread(localId)
        t.messages = remote.messages
        t.sessionId = remote.sessionId
        if (remote.knowledgeChatMode) t.knowledgeChatMode = remote.knowledgeChatMode
        if (typeof remote.thinkingMode === 'boolean') t.thinkingMode = remote.thinkingMode
        if (remote.knowledgeSearchTopK != null) t.knowledgeSearchTopK = remote.knowledgeSearchTopK
        savePersistedAgentThread(localId, t, remoteAt || Date.now())
        return
      }
      if (localAt > remoteAt) void saveServerThread(serverKey, local).catch(() => undefined)
    },

    ensureThread(threadId: string): AgentNotebookThread {
      let t = this.threads[threadId]
      if (!t) {
        const persisted = loadPersistedAgentThread(threadId)
        t = emptyThread(persisted)
        this.threads[threadId] = t
      }
      return t
    },

    resetInMemoryThreads() {
      for (const id of persistTimers.keys()) {
        const t = persistTimers.get(id)
        if (t) clearTimeout(t)
      }
      persistTimers.clear()
      serverThreadKeys.clear()
      hydrateTokens.clear()
      this.threads = {}
      this.streaming = false
      this.streamingNotebookId = null
      this.streamingPhase = null
    },

    discardThread(threadId: string) {
      if (!this.threads[threadId]) return
      const next = { ...this.threads }
      delete next[threadId]
      this.threads = next
      clearPersistedAgentThread(threadId)
      const serverKey = serverThreadKeys.get(threadId)
      if (serverKey) void deleteServerThread(serverKey).catch(() => undefined)
      serverThreadKeys.delete(threadId)
    },

    setStreaming(
      v: boolean,
      notebookId: string | null = null,
      phase: AgentStreamingPhase | null = null,
    ) {
      this.streaming = v
      this.streamingNotebookId = v ? notebookId : null
      this.streamingPhase = v ? phase : null
    },

    setSessionId(threadId: string, id: string | null) {
      const t = this.ensureThread(threadId)
      t.sessionId = id
      this.schedulePersist(threadId)
    },

    setLastError(threadId: string, msg: string | null) {
      const t = this.ensureThread(threadId)
      t.lastError = msg
    },

    appendUserMessage(threadId: string, text: string, attachmentNames?: string[]) {
      const t = this.ensureThread(threadId)
      const names = attachmentNames?.map((n) => n.trim()).filter(Boolean)
      t.messages.push({
        id: msgId(),
        role: 'user',
        body: text,
        ...(names?.length ? { attachmentNames: names } : {}),
        createdAt: Date.now(),
      })
      this.schedulePersist(threadId)
    },

    appendAssistantMessage(threadId: string, text: string, footer?: string) {
      const t = this.ensureThread(threadId)
      t.messages.push({
        id: msgId(),
        role: 'assistant',
        body: text,
        createdAt: Date.now(),
        footer,
      })
      this.schedulePersist(threadId)
    },

    beginStreamingAssistant(threadId: string, opts?: { thinkingMode?: boolean }) {
      const t = this.ensureThread(threadId)
      const thinkingMode = opts?.thinkingMode ?? true
      t.messages.push({
        id: msgId(),
        role: 'assistant',
        body: '',
        createdAt: Date.now(),
        streaming: true,
        ...(thinkingMode
          ? {
              thinking: [],
              thinkingOpen: true,
              thinkingActive: true,
            }
          : {}),
      })
    },

    appendThinkingStep(threadId: string, step: AgentThinkingStep) {
      const t = this.ensureThread(threadId)
      const last = t.messages[t.messages.length - 1]
      if (!last || last.role !== 'assistant') return
      if (!Array.isArray(last.thinking)) last.thinking = []
      // 连续 llm reasoning 增量合并到同一条，避免一步一个 token
      if (step.phase === 'llm' && step.text) {
        const prev = last.thinking[last.thinking.length - 1]
        if (prev?.phase === 'llm') {
          prev.text += step.text
          last.thinkingOpen = true
          last.thinkingActive = true
          return
        }
      }
      last.thinking.push(step)
      last.thinkingOpen = true
      last.thinkingActive = true
    },

    // 流结束时收尾：成功时将生成状态改为完成态，失败时移除瞬时状态
    finalizeThinking(threadId: string, answerCompleted = false) {
      const t = this.ensureThread(threadId)
      const last = t.messages[t.messages.length - 1]
      if (!last || last.role !== 'assistant') return
      last.thinkingActive = false
      last.thinkingOpen = false
      if (Array.isArray(last.thinking)) {
        last.thinking = last.thinking.filter((s) => s.phase !== 'llm')
        const answerStep = [...last.thinking].reverse().find((s) => s.phase === 'answer')
        if (answerCompleted && answerStep) {
          answerStep.text = '已生成回答'
        } else if (!answerCompleted) {
          last.thinking = last.thinking.filter((s) => s.phase !== 'answer')
        }
      }
      this.schedulePersist(threadId)
    },

    patchLastAssistantMessage(threadId: string, patch: Partial<AgentMessage>) {
      const t = this.ensureThread(threadId)
      const last = t.messages[t.messages.length - 1]
      if (!last || last.role !== 'assistant') return
      Object.assign(last, patch)
      if (
        patch.streaming === false ||
        patch.body !== undefined ||
        patch.footer !== undefined ||
        patch.citations !== undefined ||
        patch.sources !== undefined ||
        patch.strategy !== undefined ||
        patch.confidence !== undefined ||
        patch.chunksUsed !== undefined ||
        patch.retrieval !== undefined ||
        patch.thinking !== undefined
      ) {
        this.schedulePersist(threadId)
      }
    },

    setKnowledgeDockMode(threadId: string, mode: KnowledgeDockMode) {
      const t = this.ensureThread(threadId)
      t.knowledgeDockMode = mode
    },

    setKnowledgeChatMode(threadId: string, mode: KnowledgeChatMode) {
      const t = this.ensureThread(threadId)
      t.knowledgeChatMode = mode
      this.schedulePersist(threadId)
    },

    setThinkingMode(threadId: string, enabled: boolean) {
      const t = this.ensureThread(threadId)
      t.thinkingMode = enabled
      this.schedulePersist(threadId)
    },

    setKnowledgeSearchTopK(threadId: string, topK: number) {
      const t = this.ensureThread(threadId)
      t.knowledgeSearchTopK = topK
      this.schedulePersist(threadId)
    },

    patchMessage(threadId: string, messageId: string, patch: Partial<AgentMessage>) {
      const t = this.ensureThread(threadId)
      const msg = t.messages.find((m) => m.id === messageId)
      if (!msg) return
      Object.assign(msg, patch)
      if (patch.collapsed !== undefined || patch.feedback !== undefined) {
        this.schedulePersist(threadId)
      }
    },

    clearMessages(threadId: string) {
      const t = this.ensureThread(threadId)
      t.messages = []
      t.lastError = null
      t.pendingImages = []
      clearPersistedAgentThread(threadId)
      const serverKey = serverThreadKeys.get(threadId)
      if (serverKey) void deleteServerThread(serverKey).catch(() => undefined)
    },

    setComposerDraft(threadId: string, v: string) {
      const t = this.ensureThread(threadId)
      t.composerDraft = v
    },

    addPendingAttachments(threadId: string, files: File[]) {
      const t = this.ensureThread(threadId)
      t.pendingAttachments.push(...files)
    },

    removePendingAttachment(threadId: string, index: number) {
      const t = this.ensureThread(threadId)
      t.pendingAttachments.splice(index, 1)
    },

    clearPendingAttachments(threadId: string) {
      const t = this.ensureThread(threadId)
      t.pendingAttachments = []
    },

    // ── Pending images (transient, never persisted) ──

    addPendingImage(threadId: string, img: PendingImage) {
      const t = this.ensureThread(threadId)
      t.pendingImages.push(img)
    },

    removePendingImage(threadId: string, imageId: string) {
      const t = this.ensureThread(threadId)
      t.pendingImages = t.pendingImages.filter((i) => i.id !== imageId)
    },

    updatePendingImage(threadId: string, imageId: string, patch: Partial<PendingImage>) {
      const t = this.ensureThread(threadId)
      const img = t.pendingImages.find((i) => i.id === imageId)
      if (img) Object.assign(img, patch)
    },

    clearPendingImages(threadId: string) {
      const t = this.ensureThread(threadId)
      t.pendingImages = []
    },
  },
})
