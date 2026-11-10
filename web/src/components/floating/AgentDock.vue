<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import Icon from '@/components/common/Icon.vue'
import { useKnowledgePanel } from '@/composables/useKnowledgePanel'
import { buildSmartImportPayload, runKnowledgeSmartImport } from '@/composables/useKnowledgeAgent'
import { getWorkspaceContextKey } from '@/services/workspaceContext'
import { buildAgentThreadKey, buildServerThreadKey } from '@/utils/agentThreadKey'
import { useKnowledgeDetailStore } from '@/stores/knowledgeDetail'
import { useKnowledgeChat } from '@/composables/useKnowledgeChat'
import { useKnowledgeRetrieval } from '@/composables/useKnowledgeRetrieval'
import { formatNexusError } from '@/utils/nexusError'
import { useAgentStore } from '@/stores/agent'
import { useCanvasStore } from '@/stores/canvas'
import { useUiStore } from '@/stores/ui'
import type {
  AgentMessage,
  AgentMessageFeedback,
  KnowledgeChatMode,
  KnowledgeDockMode,
} from '@/stores/agent'
import AgentInput from '@/components/floating/AgentInput.vue'
import { useI18n } from '@/composables/useI18n'
import { normalizeKBMarkdown } from '@/utils/normalizeKBMarkdown'
import { MarkdownRender } from 'markstream-vue'
import 'markstream-vue/index.css'
import { buildAgentMessageMeta, formatAgentStrategyLabel } from '@/utils/agentMessageMeta'
import { stripEmoji } from '@/utils/stripEmoji'
import { recognizeAndCleanImage } from '@/services/imageRecognize'
import { preprocessImage } from '@/utils/imagePreprocess'
import { useVoiceInput } from '@/composables/useVoiceInput'
import { useToast } from '@/composables/useToast'
import type { PendingImage } from '@/stores/agent'

const agent = useAgentStore()
const knowledgeDetail = useKnowledgeDetailStore()
const canvas = useCanvasStore()
const ui = useUiStore()
const { enabled: knowledgePanelEnabled } = useKnowledgePanel()
const { ask: askKnowledgeChat } = useKnowledgeChat()
const { search: runKnowledgeSearch, focusDocument } = useKnowledgeRetrieval()
const { t } = useI18n()
const { show: showToast } = useToast()

const activeNotebookId = computed(() => canvas.activeNotebookId)

/** 导入 | 问答 | 检索 各自独立历史（本地 key：工作区_笔记|all_模式） */
const knowledgeDockMode = ref<KnowledgeDockMode>('import')

const agentThreadId = computed(() => {
  if (!knowledgePanelEnabled.value) return activeNotebookId.value
  const ws = getWorkspaceContextKey()
  if (!ws) return null
  return buildAgentThreadKey(ws, activeNotebookId.value, knowledgeDockMode.value)
})

watch(agentThreadId, (id) => {
  if (id) agent.ensureThread(id)
}, { immediate: true })

const lastError = computed(() => {
  const id = agentThreadId.value
  if (!id) return null as string | null
  return agent.threads[id]?.lastError ?? null
})

const knowledgeChatMode = computed({
  get(): KnowledgeChatMode {
    const id = agentThreadId.value
    if (!id) return 'hybrid'
    return agent.threads[id]?.knowledgeChatMode ?? 'hybrid'
  },
  set(mode: KnowledgeChatMode) {
    const id = agentThreadId.value
    if (!id) return
    agent.setKnowledgeChatMode(id, mode)
  },
})

const thinkingMode = computed({
  get(): boolean {
    const id = agentThreadId.value
    if (!id) return true
    return agent.threads[id]?.thinkingMode ?? true
  },
  set(enabled: boolean) {
    const id = agentThreadId.value
    if (!id) return
    agent.setThinkingMode(id, enabled)
  },
})

const threadMessages = computed((): AgentMessage[] => {
  const id = agentThreadId.value
  if (!id) return []
  return agent.threads[id]?.messages ?? []
})

const conversationViewportEl = ref<HTMLElement | null>(null)

function scrollConversationToBottom() {
  const el = conversationViewportEl.value
  if (!el) return
  el.scrollTop = el.scrollHeight
}

function scheduleScrollToBottom() {
  void nextTick(() => {
    requestAnimationFrame(() => scrollConversationToBottom())
  })
}

watch(
  () =>
    [
      threadMessages.value.length,
      threadMessages.value.at(-1)?.id,
      threadMessages.value.at(-1)?.body?.length ?? 0,
      threadMessages.value.at(-1)?.streaming,
      threadMessages.value.at(-1)?.footer,
      threadMessages.value.at(-1)?.thinking?.length ?? 0,
      threadMessages.value.at(-1)?.thinking?.at(-1)?.text?.length ?? 0,
      threadMessages.value.at(-1)?.thinkingActive,
    ] as const,
  () => scheduleScrollToBottom(),
)

watch(
  [agentThreadId, knowledgeDockMode],
  () => {
    scheduleScrollToBottom()
    const id = agentThreadId.value
    if (!id) return
    agent.bindServerThread(id, buildServerThreadKey(activeNotebookId.value, knowledgeDockMode.value))
  },
  { immediate: true },
)

onMounted(() => scheduleScrollToBottom())

const copiedMsgId = ref<string | null>(null)
let copiedResetTimer: ReturnType<typeof setTimeout> | null = null

/** 旧版导入消息把附件名拼在正文末尾，展示时拆出为标签 */
const LEGACY_USER_ATTACH_SUFFIX =
  /\n\n[（(](?:已附加文件|Attached files)[:：]\s*(.+?)[）)]\s*$/u

function userMessageParts(m: AgentMessage): { text: string; attachments: string[] } {
  if (m.attachmentNames?.length) {
    return { text: stripEmoji(m.body?.trim() ?? ''), attachments: m.attachmentNames }
  }
  const raw = m.body ?? ''
  const match = raw.match(LEGACY_USER_ATTACH_SUFFIX)
  if (!match) {
    return { text: stripEmoji(raw.trim()), attachments: [] }
  }
  const attachments = match[1]
    .split(/[、,]\s*/)
    .map((n) => n.trim())
    .filter(Boolean)
  return { text: stripEmoji(raw.slice(0, match.index).trim()), attachments }
}

function messageBodyText(m: AgentMessage): string {
  if (m.role === 'user') {
    return userMessageParts(m).text
  }
  return stripEmoji(m.body || (m.streaming ? '…' : t('agentDock.emptyReply')))
}

function collapsedPreview(m: AgentMessage): string {
  return messageBodyText(m).replace(/\s+/g, ' ').trim()
}

function messageAttachmentNames(m: AgentMessage): string[] {
  if (m.role !== 'user') return []
  return userMessageParts(m).attachments
}

function markdownContent(m: AgentMessage): string {
  return normalizeKBMarkdown(m.body?.trim() ?? '')
}

function markdownFinal(m: AgentMessage): boolean {
  return !m.streaming
}

/** Agent 处理中时，对触发本轮请求的最后一条用户消息叠加载蒙层 */
const awaitingUserMessageId = computed(() => {
  if (!agent.streaming) return null
  const tid = agentThreadId.value
  if (!tid || agent.streamingNotebookId !== tid) return null
  const msgs = threadMessages.value
  for (let i = msgs.length - 1; i >= 0; i--) {
    if (msgs[i].role === 'user') return msgs[i].id
  }
  return null
})

function isUserMessageAwaitingAgent(m: AgentMessage): boolean {
  return m.role === 'user' && awaitingUserMessageId.value === m.id
}

const agentStreamingStatusLabel = computed(() => {
  if (agent.streamingPhase === 'import') return t('agentInput.stateImporting')
  if (agent.streamingPhase === 'search') return t('agentInput.stateSearching')
  if (agent.streamingPhase === 'chat') return t('agentInput.stateChatting')
  return t('agentDock.interpreting')
})

function messageMeta(m: AgentMessage) {
  return buildAgentMessageMeta(m)
}

function hasMessageMeta(m: AgentMessage): boolean {
  const meta = messageMeta(m)
  return Boolean(
    meta.strategy || meta.citations.length || meta.query4dText || meta.retrievalTraceText,
  )
}

function onCitationClick(citation: { docId: string; chunkId?: string; mediaUrl?: string }) {
  if (!citation.docId) return
  knowledgeDetail.openCitation(citation.docId, citation.chunkId, citation.mediaUrl)
  focusDocument(citation.docId)
}

function isMessageCollapsed(m: AgentMessage): boolean {
  return Boolean(m.collapsed)
}

function toggleMessageCollapsed(m: AgentMessage) {
  const tid = agentThreadId.value
  if (!tid) return
  const collapsed = !m.collapsed
  agent.patchMessage(tid, m.id, { collapsed })
  showToast(collapsed ? t('agentDock.msgCollapsed') : t('agentDock.msgExpanded'))
}

function toggleThinking(m: AgentMessage) {
  const tid = agentThreadId.value
  if (!tid) return
  agent.patchMessage(tid, m.id, { thinkingOpen: !m.thinkingOpen })
}

function hasThinking(m: AgentMessage): boolean {
  return Array.isArray(m.thinking) && m.thinking.length > 0
}

function thinkingStepCount(m: AgentMessage): number {
  if (!Array.isArray(m.thinking)) return 0
  const stepIds = new Set(
    m.thinking
      .map((item) => Number(item.step))
      .filter((step) => Number.isInteger(step) && step > 0),
  )
  return stepIds.size || m.thinking.filter((item) => item.phase === 'step').length
}

function setMessageFeedback(m: AgentMessage, feedback: AgentMessageFeedback) {
  const tid = agentThreadId.value
  if (!tid) return
  const clearing = m.feedback === feedback
  agent.patchMessage(tid, m.id, { feedback: clearing ? null : feedback })
  if (feedback === 'up') {
    showToast(clearing ? t('agentDock.msgLikeCleared') : t('agentDock.msgLikeDone'))
  } else {
    showToast(clearing ? t('agentDock.msgDislikeCleared') : t('agentDock.msgDislikeDone'))
  }
}

async function copyMessageBody(m: AgentMessage) {
  const names = messageAttachmentNames(m)
  const attachSuffix =
    names.length > 0
      ? `${t('agentDock.attachmentsLead')}${names.join(t('agentDock.attachmentsJoin'))}${t('agentDock.attachmentsEnd')}`
      : ''
  const text = [m.body?.trim(), attachSuffix.trim()].filter(Boolean).join('\n\n')
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    copiedMsgId.value = m.id
    showToast(t('agentDock.msgCopied'))
    if (copiedResetTimer) clearTimeout(copiedResetTimer)
    copiedResetTimer = setTimeout(() => {
      copiedMsgId.value = null
    }, 1600)
  } catch {
    showToast(t('agentDock.msgCopyFailed'))
  }
}

const knowledgeChatView = computed(
  () => knowledgePanelEnabled.value && knowledgeDockMode.value === 'chat',
)

const knowledgeSearchView = computed(
  () => knowledgePanelEnabled.value && knowledgeDockMode.value === 'search',
)

const knowledgeMessageView = computed(() => knowledgePanelEnabled.value)

const knowledgeSearchTopK = computed({
  get(): number {
    const id = agentThreadId.value
    if (!id) return 5
    return agent.threads[id]?.knowledgeSearchTopK ?? 5
  },
  set(v: number) {
    const id = agentThreadId.value
    if (!id) return
    agent.setKnowledgeSearchTopK(id, v)
  },
})

const searchTopKOptions = [
  { value: 5, label: '5' },
  { value: 10, label: '10' },
  { value: 20, label: '20' },
]

const canClearConversation = computed(() => {
  if (agent.streaming) return false
  const id = agentThreadId.value
  if (!id) return false
  return threadMessages.value.length > 0
})

const pendingAttachments = computed(() => {
  const id = agentThreadId.value
  if (!id) return [] as File[]
  return agent.threads[id]?.pendingAttachments ?? []
})

// ── Voice input ──
const { recording: voiceRecording, startRecording, stopRecording, cancelRecording } = useVoiceInput()

// ── Pending images ──
const pendingImages = computed<PendingImage[]>(() => {
  const id = agentThreadId.value
  if (!id) return []
  return agent.threads[id]?.pendingImages ?? []
})

const showImageUploadButton = computed(() => {
  return knowledgePanelEnabled.value &&
    (knowledgeDockMode.value === 'chat' || knowledgeDockMode.value === 'search')
})

const composerDraft = computed({
  get: () => {
    const id = agentThreadId.value
    if (!id) return ''
    return agent.threads[id]?.composerDraft ?? ''
  },
  set: (v: string) => {
    const id = agentThreadId.value
    if (!id) return
    agent.setComposerDraft(id, v)
  },
})

const dockRootRef = ref<HTMLElement | null>(null)

// ── Left-edge drag resize ──
const dockDragging = ref(false)
function onDockResizeStart(e: MouseEvent) {
  e.preventDefault()
  dockDragging.value = true
  const host = dockRootRef.value?.parentElement as HTMLElement | null
  if (host) host.style.transition = 'none'
  const startX = e.clientX
  const startWidth = ui.agentDockWidthPx
  const onMove = (ev: MouseEvent) => {
    const delta = startX - ev.clientX
    ui.setAgentDockWidth(startWidth + delta)
  }
  const onUp = () => {
    dockDragging.value = false
    if (host) host.style.transition = ''
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
  }
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}
const streamAbort = ref<AbortController | null>(null)

function cancelAgentStream() {
  streamAbort.value?.abort()
}

watch(composerDraft, (text, prev) => {
  const now = text.trim().length > 0
  const before = (prev ?? '').trim().length > 0
  if (now && !before) ui.setAgentConversationOpen(true)
})

function onAttachFiles(files: File[]) {
  const id = agentThreadId.value
  if (!id) return
  agent.addPendingAttachments(id, files)
}

async function onAttachImages(files: File[]) {
  const id = agentThreadId.value
  if (!id) return
  for (const file of files) {
    if (!file.type.startsWith('image/')) continue
    let img: PendingImage = {
      id: `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
      base64: '',
      thumbnail: '',
      mimeType: file.type,
      filename: file.name,
      status: 'pending',
      mediaType: 'image',
    }
    agent.addPendingImage(id, img)
    try {
      const preprocessed = await preprocessImage(file)
      agent.updatePendingImage(id, img.id, {
        base64: preprocessed.base64,
        thumbnail: preprocessed.thumbnail,
        mimeType: preprocessed.mimeType,
        status: 'pending',
      })
    } catch {
      agent.updatePendingImage(id, img.id, { status: 'error', error: '预处理失败' })
    }
  }
}

async function onAttachVideos(files: File[]) {
  const id = agentThreadId.value
  if (!id) return
  for (const file of files) {
    if (!file.type.startsWith('video/')) continue
    if (file.size > 100 * 1024 * 1024) {
      agent.setLastError(id, '视频过大，最大支持 100 MB')
      continue
    }
    let vid: PendingImage = {
      id: `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
      base64: '',
      thumbnail: '',
      mimeType: file.type,
      filename: file.name,
      status: 'pending',
      mediaType: 'video',
    }
    agent.addPendingImage(id, vid)
    try {
      // Generate thumbnail from first frame
      const thumb = await generateVideoThumbnail(file)
      const reader = new FileReader()
      const base64Promise = new Promise<string>((resolve, reject) => {
        reader.onload = () => resolve((reader.result as string).split(',')[1] ?? '')
        reader.onerror = () => reject(new Error('read failed'))
      })
      reader.readAsDataURL(file)
      const base64 = await base64Promise
      agent.updatePendingImage(id, vid.id, {
        base64,
        thumbnail: thumb,
        status: 'pending',
      })
    } catch {
      agent.updatePendingImage(id, vid.id, { status: 'error', error: '视频预处理失败' })
    }
  }
}

async function generateVideoThumbnail(file: File): Promise<string> {
  return new Promise((resolve) => {
    const video = document.createElement('video')
    video.preload = 'metadata'
    video.muted = true
    video.playsInline = true
    const url = URL.createObjectURL(file)
    video.src = url
    video.onloadeddata = () => {
      video.currentTime = 1 // Seek to 1s
    }
    video.onseeked = () => {
      const canvas = document.createElement('canvas')
      canvas.width = 120
      canvas.height = 120
      const ctx = canvas.getContext('2d')
      if (ctx) {
        const scale = Math.max(120 / video.videoWidth, 120 / video.videoHeight)
        const sw = 120 / scale
        const sh = 120 / scale
        const sx = (video.videoWidth - sw) / 2
        const sy = (video.videoHeight - sh) / 2
        ctx.drawImage(video, sx, sy, sw, sh, 0, 0, 120, 120)
      }
      URL.revokeObjectURL(url)
      resolve(canvas.toDataURL('image/jpeg', 0.7))
    }
    video.onerror = () => {
      URL.revokeObjectURL(url)
      resolve('')
    }
  })
}

function onRemoveImage(imageId: string) {
  const id = agentThreadId.value
  if (!id) return
  agent.removePendingImage(id, imageId)
}

async function onStartVoice() {
  try {
    await startRecording()
  } catch {
    // microphone denied or unavailable
  }
}

async function onStopVoice() {
  try {
    const text = await stopRecording()
    if (text) {
      const id = agentThreadId.value
      if (id) {
        const current = agent.threads[id]?.composerDraft ?? ''
        agent.setComposerDraft(id, (current + ' ' + text).trim())
      }
    }
  } catch {
    // ignore
  }
}

function onCancelVoice() {
  cancelRecording()
}

const agentNotebookDisplayTitle = computed(() => {
  const n = canvas.activeNotebook
  if (!canvas.activeNotebookId || !n) return null as string | null
  const title = n.title.trim()
  return title || t('common.untitledNotebook')
})

function clearConversation() {
  const id = agentThreadId.value
  if (!id) return
  agent.clearMessages(id)
}

async function onKnowledgeSearchSubmit(threadId: string, query: string) {
  agent.setLastError(threadId, null)
  agent.setComposerDraft(threadId, '')
  agent.setStreaming(true, threadId, 'search')
  const ac = new AbortController()
  streamAbort.value = ac
  try {
    await runKnowledgeSearch(threadId, query, {
      topK: knowledgeSearchTopK.value,
      signal: ac.signal,
    })
  } catch (e) {
    const aborted = ac.signal.aborted || (e instanceof DOMException && e.name === 'AbortError')
    if (!aborted) {
      agent.setLastError(threadId, formatNexusError(e))
    }
  } finally {
    streamAbort.value = null
    agent.setStreaming(false)
  }
}

async function onKnowledgeChatSubmit(
  threadId: string,
  query: string,
  imageBase64?: string,
  imageMimeType?: string,
  videoBase64?: string,
  videoMimeType?: string,
  mediaUrl?: string,
) {
  agent.setLastError(threadId, null)
  agent.setComposerDraft(threadId, '')
  agent.setStreaming(true, threadId, 'chat')
  const ac = new AbortController()
  streamAbort.value = ac
  try {
    await askKnowledgeChat(threadId, query, knowledgeChatMode.value, {
      signal: ac.signal,
      imageBase64,
      imageMimeType,
      videoBase64,
      videoMimeType,
      mediaUrl,
    })
  } catch (e) {
    const aborted = ac.signal.aborted || (e instanceof DOMException && e.name === 'AbortError')
    if (!aborted) {
      agent.setLastError(threadId, formatNexusError(e))
    }
  } finally {
    streamAbort.value = null
    agent.setStreaming(false)
  }
}

function isInvalidImportContent(err: unknown): boolean {
  const raw = err instanceof Error ? err.message : String(err)
  return (
    /importable=0\/total=0/.test(raw) ||
    /content too short/i.test(raw) ||
    /no segments after split/i.test(raw)
  )
}

function reportImportError(threadId: string, err: unknown) {
  if (isInvalidImportContent(err)) {
    showToast(t('agentDock.importContentInvalid'))
    return
  }
  agent.setLastError(threadId, formatNexusError(err))
}

async function onKnowledgeSubmit(
  threadId: string,
  rawUser: string,
  files: File[],
  attachmentNames: string[] = [],
) {
  agent.setLastError(threadId, null)
  agent.appendUserMessage(threadId, stripEmoji(rawUser), attachmentNames)
  agent.clearPendingAttachments(threadId)
  agent.setComposerDraft(threadId, '')
  agent.setStreaming(true, threadId, 'import')
  const ac = new AbortController()
  streamAbort.value = ac
  try {
    const built = await buildSmartImportPayload(rawUser, files)
    const { summary } = await runKnowledgeSmartImport({
      rawText: built.rawText,
      filename: built.filename,
      attachments: built.attachments,
      targetDocId: canvas.activeNotebookId,
    })
    agent.appendAssistantMessage(threadId, summary)
  } catch (e) {
    const aborted = ac.signal.aborted || (e instanceof DOMException && e.name === 'AbortError')
    if (!aborted) {
      reportImportError(threadId, e)
    }
  } finally {
    streamAbort.value = null
    agent.setStreaming(false)
  }
}

async function onSubmit() {
  const text = composerDraft.value.trim()
  const hasFiles = pendingAttachments.value.length > 0
  const hasImages = pendingImages.value.length > 0
  if ((!text && !hasFiles && !hasImages) || agent.streaming) return

  if (!knowledgePanelEnabled.value) return

  const threadId = agentThreadId.value
  if (!threadId) return
  agent.ensureThread(threadId)

  // ── Media path: chat mode passes image/video/URL to stream for server-side recognition ──
  if (hasImages && knowledgeDockMode.value === 'chat') {
    // Collect the first valid media
    const img = pendingImages.value.find(
      (i) => i.status !== 'error' && (i.base64 || i.mediaUrl),
    )
    agent.clearPendingImages(threadId)
    agent.setComposerDraft(threadId, '')
    if (!img) {
      agent.setLastError(threadId, '媒体数据无效，请重试')
      return
    }
    if (img.mediaUrl) {
      await onKnowledgeChatSubmit(threadId, text, undefined, undefined, undefined, undefined, img.mediaUrl)
    } else if (img.mediaType === 'video') {
      await onKnowledgeChatSubmit(threadId, text, undefined, undefined, img.base64, img.mimeType)
    } else {
      await onKnowledgeChatSubmit(threadId, text, img.base64, img.mimeType)
    }
    return
  }
  // ── Image path: search mode still uses separate recognition ──
  if (hasImages && knowledgeDockMode.value === 'search') {
    const cleanedParts: string[] = []
    for (const img of pendingImages.value) {
      if (img.status === 'error' || img.status === 'recognizing') continue
      agent.updatePendingImage(threadId, img.id, { status: 'recognizing' })
      try {
        const result = await recognizeAndCleanImage({
          imageBase64: img.base64,
          mimeType: img.mimeType,
          userText: text,
        })
        cleanedParts.push(result.cleanedQuestion)
        agent.updatePendingImage(threadId, img.id, { status: 'done' })
      } catch (e) {
        agent.updatePendingImage(threadId, img.id, {
          status: 'error',
          error: formatNexusError(e),
        })
      }
    }
    // Clear images after recognition.
    agent.clearPendingImages(threadId)
    agent.setComposerDraft(threadId, '')

    const finalQuestion = [...cleanedParts, text].filter(Boolean).join('\n\n')
    if (!finalQuestion.trim()) {
      agent.setLastError(threadId, '图片识别失败，请重试')
      return
    }
    await onKnowledgeSearchSubmit(threadId, finalQuestion)
    return
  }

  // ── Media path: import mode passes image/video/URL to smart-import for server-side VLM recognition ──
  if (hasImages && knowledgeDockMode.value === 'import') {
    const img = pendingImages.value.find(
      (i) => i.status !== 'error' && (i.base64 || i.mediaUrl),
    )
    agent.clearPendingImages(threadId)
    agent.setComposerDraft(threadId, '')
    if (!img) {
      agent.setLastError(threadId, '媒体数据无效，请重试')
      return
    }
    agent.setStreaming(true, threadId, 'import')
    const ac = new AbortController()
    streamAbort.value = ac
    try {
      const built = await buildSmartImportPayload(text)
      const { summary } = await runKnowledgeSmartImport({
        rawText: built.rawText,
        filename: built.filename,
        targetDocId: canvas.activeNotebookId,
        mediaImageBase64: img.mediaType === 'image' && !img.mediaUrl ? img.base64 : undefined,
        mediaImageMimeType: img.mediaType === 'image' && !img.mediaUrl ? img.mimeType : undefined,
        mediaVideoBase64: img.mediaType === 'video' ? img.base64 : undefined,
        mediaVideoMimeType: img.mediaType === 'video' ? img.mimeType : undefined,
        mediaUrl: img.mediaUrl ?? undefined,
      })
      agent.appendAssistantMessage(threadId, summary)
    } catch (e: any) {
      const aborted = ac.signal.aborted || (e instanceof DOMException && e.name === 'AbortError')
      if (!aborted) {
        reportImportError(threadId, e)
      }
    } finally {
      streamAbort.value = null
      agent.setStreaming(false)
    }
    return
  }

  const attachmentNames = pendingAttachments.value.map((f) => f.name)

  if (knowledgeDockMode.value === 'chat') {
    await onKnowledgeChatSubmit(threadId, text)
    return
  }
  if (knowledgeDockMode.value === 'search') {
    await onKnowledgeSearchSubmit(threadId, text)
    return
  }
  await onKnowledgeSubmit(threadId, text, [...pendingAttachments.value], attachmentNames)
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    const hasFiles = pendingAttachments.value.length > 0
    const hasImages = pendingImages.value.length > 0
    if ((!composerDraft.value.trim() && !hasFiles && !hasImages) || !knowledgePanelEnabled.value) return
    e.preventDefault()
    onSubmit()
  }
}
</script>

<template>
  <aside
    ref="dockRootRef"
    class="agent-dock glass-panel"
    :class="{ 'agent-dock--expanded': ui.agentConversationExpanded, 'agent-dock--dragging': dockDragging }"
    :aria-label="t('agentDock.aria')"
  >
    <div
      class="agent-dock__resize-handle"
      @mousedown="onDockResizeStart"
    />
    <header class="agent-dock__head">
      <div class="agent-dock__head-row">
        <div class="agent-dock__head-text">
          <h2 class="agent-dock__title">
            <Icon class="agent-dock__title-icon" name="messages" :size="18" />
            {{ t('agentDock.title') }}
          </h2>
        </div>
        <div class="agent-dock__head-actions">
        <button
          type="button"
          class="agent-dock__clear"
          :disabled="!canClearConversation"
          :title="t('agentDock.clearTitle')"
          @click="clearConversation"
        >
          <Icon class="agent-dock__clear-icon" name="trash" :size="14" />
          {{ t('agentDock.clear') }}
        </button>
        <button
          type="button"
          class="agent-dock__close"
          :aria-label="
            ui.agentConversationExpanded ? t('agentDock.collapseWidthAria') : t('agentDock.expandWidthAria')
          "
          :title="
            ui.agentConversationExpanded ? t('agentDock.collapseWidthTitle') : t('agentDock.expandWidthTitle')
          "
          :aria-pressed="ui.agentConversationExpanded"
          @click="ui.toggleAgentConversationExpanded()"
        >
          <Icon
            :name="ui.agentConversationExpanded ? 'arrows-diagonal-minimize-2' : 'arrows-diagonal'"
            :size="16"
          />
        </button>
        <button
          type="button"
          class="agent-dock__close"
          :aria-label="t('agentDock.closeAria')"
          :title="t('agentDock.closeTitle')"
          @click="ui.setAgentConversationOpen(false)"
        >
          <Icon name="x" :size="16" />
        </button>
        </div>
      </div>
      <div v-if="knowledgePanelEnabled" class="agent-dock__mode-tabs" role="tablist">
        <button
          type="button"
          role="tab"
          class="agent-dock__mode-tab"
          :class="{ 'agent-dock__mode-tab--active': knowledgeDockMode === 'import' }"
          :aria-selected="knowledgeDockMode === 'import'"
          @click="knowledgeDockMode = 'import'"
        >
          {{ t('agentDock.knowledgeModeImport') }}
        </button>
        <button
          type="button"
          role="tab"
          class="agent-dock__mode-tab"
          :class="{ 'agent-dock__mode-tab--active': knowledgeDockMode === 'chat' }"
          :aria-selected="knowledgeDockMode === 'chat'"
          @click="knowledgeDockMode = 'chat'"
        >
          {{ t('agentDock.knowledgeModeChat') }}
        </button>
        <button
          type="button"
          role="tab"
          class="agent-dock__mode-tab"
          :class="{ 'agent-dock__mode-tab--active': knowledgeDockMode === 'search' }"
          :aria-selected="knowledgeDockMode === 'search'"
          @click="knowledgeDockMode = 'search'"
        >
          {{ t('agentDock.knowledgeModeSearch') }}
        </button>
      </div>
    </header>
    <div
      v-if="knowledgePanelEnabled && knowledgeSearchView"
      class="agent-dock__chat-toolbar"
      role="group"
      :aria-label="t('agentDock.searchToolbarAria')"
    >
      <span class="agent-dock__search-label">{{ t('agentDock.searchTopK') }}</span>
      <button
        v-for="opt in searchTopKOptions"
        :key="opt.value"
        type="button"
        class="agent-dock__chat-mode"
        :class="{ 'agent-dock__chat-mode--active': knowledgeSearchTopK === opt.value }"
        @click="knowledgeSearchTopK = opt.value"
      >
        {{ opt.label }}
      </button>
    </div>
    <div
      v-else-if="knowledgePanelEnabled && knowledgeChatView"
      class="agent-dock__chat-toolbar"
      role="group"
      :aria-label="t('agentDock.chatMetaAria')"
    >
      <button
        type="button"
        class="agent-dock__chat-mode"
        :class="{ 'agent-dock__chat-mode--active': knowledgeChatMode === 'hybrid' }"
        @click="knowledgeChatMode = 'hybrid'"
      >
        {{ t('agentDock.chatModeHybrid') }}
      </button>
      <button
        type="button"
        class="agent-dock__chat-mode"
        :class="{ 'agent-dock__chat-mode--active': knowledgeChatMode === 'kb_only' }"
        @click="knowledgeChatMode = 'kb_only'"
      >
        {{ t('agentDock.chatModeKbOnly') }}
      </button>
      <span class="agent-dock__toolbar-spacer" aria-hidden="true" />
      <button
        type="button"
        class="agent-dock__chat-mode agent-dock__chat-mode--thinking"
        :class="{ 'agent-dock__chat-mode--active': thinkingMode }"
        :aria-pressed="thinkingMode"
        :title="thinkingMode ? t('agentDock.thinkingModeOnHint') : t('agentDock.thinkingModeOffHint')"
        @click="thinkingMode = !thinkingMode"
      >
        {{ thinkingMode ? t('agentDock.thinkingModeOn') : t('agentDock.thinkingModeOff') }}
      </button>
    </div>
    <div ref="conversationViewportEl" class="agent-dock__viewport" tabindex="0">
        <div class="agent-dock__content">
          <p v-if="!knowledgePanelEnabled" class="agent-dock__empty">
            <Icon class="agent-dock__empty-icon" name="message-circle" :size="18" />
            {{ t('agentDock.legacyDisabled') }}
          </p>
          <template v-else-if="knowledgeMessageView">
            <p v-if="threadMessages.length === 0" class="agent-dock__empty" aria-hidden="true">
              <Icon class="agent-dock__empty-icon" name="message-circle" :size="18" />
            </p>
            <ul v-else class="agent-dock__messages" role="log" aria-live="polite">
              <li
                v-for="m in threadMessages"
                :key="m.id"
                class="agent-dock__msg"
                :class="`agent-dock__msg--${m.role}`"
              >
                <span class="agent-dock__msg-role">
                  {{
                    m.role === 'user' ? t('agentDock.roleUser') : t('agentDock.roleAssistant')
                  }}
                </span>
                <div
                  v-if="m.role === 'assistant'"
                  class="agent-dock__msg-bubble agent-dock__msg-bubble--assistant"
                >
                  <div
                    v-if="hasThinking(m)"
                    class="agent-dock__thinking"
                    :class="{ 'agent-dock__thinking--active': m.thinkingActive }"
                  >
                    <button
                      type="button"
                      class="agent-dock__thinking-toggle"
                      :aria-expanded="Boolean(m.thinkingOpen)"
                      @click="toggleThinking(m)"
                    >
                      <Icon
                        class="agent-dock__thinking-chevron"
                        :class="{ 'agent-dock__thinking-chevron--open': m.thinkingOpen }"
                        name="chevron-right"
                        :size="12"
                      />
                      <span class="agent-dock__thinking-label">
                        {{
                          m.thinkingActive
                            ? t('agentDock.thinkingActive')
                            : t('agentDock.thinkingDone')
                        }}
                      </span>
                      <span v-if="m.thinkingActive" class="agent-dock__thinking-pulse" aria-hidden="true" />
                      <span class="agent-dock__thinking-count">
                        {{ t('agentDock.thinkingSteps', { count: String(thinkingStepCount(m)) }) }}
                      </span>
                    </button>
                    <div v-show="m.thinkingOpen" class="agent-dock__thinking-body">
                      <div
                        v-for="(step, si) in m.thinking"
                        :key="si"
                        class="agent-dock__thinking-step"
                        :class="{
                          'agent-dock__thinking-step--latest':
                            m.thinkingActive && si === m.thinking!.length - 1,
                          'agent-dock__thinking-step--llm': step.phase === 'llm',
                        }"
                      >
                        {{ step.text }}
                      </div>
                    </div>
                  </div>
                  <div
                    v-if="markdownContent(m) || (!hasThinking(m) || !m.streaming)"
                    class="agent-dock__msg-body"
                    :class="{
                      'agent-dock__msg-body--md': Boolean(markdownContent(m)),
                      'agent-dock__msg-body--collapsed': isMessageCollapsed(m),
                    }"
                  >
                    <p v-if="isMessageCollapsed(m)" class="agent-dock__msg-body-line">
                      {{ collapsedPreview(m) }}
                    </p>
                    <div v-show="!isMessageCollapsed(m)" class="agent-dock__msg-body-main">
                      <MarkdownRender
                        v-if="markdownContent(m)"
                        class="agent-dock__msg-md markstream-vue"
                        :content="markdownContent(m)"
                        :final="markdownFinal(m)"
                        :max-live-nodes="0"
                        :batch-rendering="true"
                        :render-batch-size="4"
                        :render-batch-delay="20"
                        :render-batch-budget-ms="6"
                        :fade="true"
                        :typewriter="true"
                      />
                      <template v-else>{{ messageBodyText(m) }}</template>
                    </div>
                    <div
                      v-if="!isMessageCollapsed(m) && !m.streaming && hasMessageMeta(m)"
                      class="agent-dock__msg-meta"
                      :aria-label="t('agentDock.chatMetaAria')"
                    >
                      <div v-if="messageMeta(m).strategy" class="agent-dock__strategy">
                        <span class="agent-dock__meta-label">{{ t('agentDock.strategyLabel') }}</span>
                        <span class="agent-dock__strategy-tag">
                          <Icon class="agent-dock__strategy-icon" name="stack-2" :size="12" />
                          {{ formatAgentStrategyLabel(messageMeta(m).strategy!) }}
                        </span>
                      </div>
                      <div
                        v-if="messageMeta(m).query4dText"
                        class="agent-dock__query-4d"
                      >
                        <span class="agent-dock__meta-label">{{ t('agentDock.query4dLabel') }}</span>
                        <span class="agent-dock__query-4d-tag">
                          {{ messageMeta(m).query4dText }}
                          <template v-if="messageMeta(m).semantic4DFilterApplied">
                            · {{ t('agentDock.query4dFilterOn') }}
                          </template>
                        </span>
                      </div>
                      <div
                        v-if="messageMeta(m).retrievalTraceText"
                        class="agent-dock__query-4d"
                      >
                        <span class="agent-dock__meta-label">{{ t('agentDock.retrievalTraceLabel') }}</span>
                        <span class="agent-dock__query-4d-tag">
                          {{ messageMeta(m).retrievalTraceText }}
                        </span>
                      </div>
                      <div v-if="messageMeta(m).citations.length" class="agent-dock__citations">
                        <span class="agent-dock__meta-label">{{ t('agentDock.citationsLabel') }}</span>
                        <ul class="agent-dock__citation-tags">
                          <li v-for="c in messageMeta(m).citations" :key="c.key">
                            <button
                              type="button"
                              class="agent-dock__citation-tag"
                              :disabled="!c.docId"
                              :title="c.chunkId ? `${c.title} · ${c.chunkId}` : c.title"
                              @click="onCitationClick(c)"
                            >
                              <Icon name="file-description" :size="12" />
                              <span class="agent-dock__citation-tag-text">{{ c.title }}</span>
                              <span
                                v-if="c.chunkCount && c.chunkCount > 1"
                                class="agent-dock__citation-tag-count"
                              >
                                ×{{ c.chunkCount }}
                              </span>
                            </button>
                          </li>
                        </ul>
                      </div>
                    </div>
                  </div>
                  <div v-if="!m.streaming && knowledgeChatView" class="agent-dock__msg-actions">
                    <button
                      type="button"
                      class="agent-dock__msg-action"
                      :class="{ 'agent-dock__msg-action--active': m.feedback === 'up' }"
                      :aria-label="t('agentDock.msgLikeAria')"
                      :title="t('agentDock.msgLikeAria')"
                      @click="setMessageFeedback(m, 'up')"
                    >
                      <Icon name="thumb-up" :size="14" />
                    </button>
                    <button
                      type="button"
                      class="agent-dock__msg-action"
                      :class="{ 'agent-dock__msg-action--active': m.feedback === 'down' }"
                      :aria-label="t('agentDock.msgDislikeAria')"
                      :title="t('agentDock.msgDislikeAria')"
                      @click="setMessageFeedback(m, 'down')"
                    >
                      <Icon name="thumb-down" :size="14" />
                    </button>
                    <button
                      type="button"
                      class="agent-dock__msg-action"
                      :class="{ 'agent-dock__msg-action--active': copiedMsgId === m.id }"
                      :aria-label="t('agentDock.msgCopyAria')"
                      :title="copiedMsgId === m.id ? t('agentDock.msgCopied') : t('agentDock.msgCopyAria')"
                      @click="copyMessageBody(m)"
                    >
                      <Icon name="copy" :size="14" />
                    </button>
                    <button
                      type="button"
                      class="agent-dock__msg-action"
                      :class="{ 'agent-dock__msg-action--active': isMessageCollapsed(m) }"
                      :aria-label="
                        isMessageCollapsed(m) ? t('agentDock.msgExpandAria') : t('agentDock.msgCollapseAria')
                      "
                      :title="
                        isMessageCollapsed(m) ? t('agentDock.msgExpandAria') : t('agentDock.msgCollapseAria')
                      "
                      @click="toggleMessageCollapsed(m)"
                    >
                      <Icon
                        :name="isMessageCollapsed(m) ? 'chevron-down' : 'arrows-diagonal-minimize-2'"
                        :size="14"
                      />
                    </button>
                  </div>
                </div>
                <div
                  v-else
                  class="agent-dock__msg-user"
                  :class="{ 'agent-dock__msg-user--busy': isUserMessageAwaitingAgent(m) }"
                  :aria-busy="isUserMessageAwaitingAgent(m) || undefined"
                >
                  <p v-if="messageBodyText(m)" class="agent-dock__msg-body agent-dock__msg-body--user">
                    {{ messageBodyText(m) }}
                  </p>
                  <ul
                    v-if="messageAttachmentNames(m).length"
                    class="agent-dock__msg-attachments"
                    :aria-label="t('agentDock.messageAttachmentsAria')"
                  >
                    <li
                      v-for="(name, ai) in messageAttachmentNames(m)"
                      :key="`${m.id}-attach-${ai}-${name}`"
                      class="agent-dock__msg-attachment-tag"
                      :title="name"
                    >
                      <Icon name="file-description" :size="12" />
                      <span class="agent-dock__msg-attachment-tag-text">{{ name }}</span>
                    </li>
                  </ul>
                  <div
                    v-if="isUserMessageAwaitingAgent(m)"
                    class="agent-dock__msg-user-veil"
                    role="status"
                    :aria-label="agentStreamingStatusLabel"
                  />
                </div>
                <p
                  v-if="messageMeta(m).statsText"
                  class="agent-dock__msg-footer"
                  :aria-label="t('agentDock.chatMetaAria')"
                >
                  {{ messageMeta(m).statsText }}
                </p>
              </li>
            </ul>
          </template>
          <p v-if="lastError" class="agent-dock__err" role="alert">
            <Icon class="agent-dock__err-icon" name="exclamation-circle" :size="16" />
            {{ lastError }}
          </p>
        </div>
    </div>

    <Teleport to="#gd-compose-root">
      <div class="agent-dock__compose">
        <div
          v-if="pendingAttachments.length"
          class="agent-dock__pending-files"
          role="status"
          aria-live="polite"
        >
          <span class="agent-dock__pending-label">{{ t('agentDock.pendingFiles') }}</span>
          <ul class="agent-dock__pending-list">
            <li
              v-for="(f, i) in pendingAttachments"
              :key="`${f.name}-${i}-${f.size}`"
              class="agent-dock__pending-item"
            >
              <span class="agent-dock__pending-name">{{ f.name }}</span>
              <button
                type="button"
                class="agent-dock__pending-remove"
                :aria-label="t('agentDock.removeFileAria', { name: f.name })"
                @click="agentThreadId && agent.removePendingAttachment(agentThreadId, i)"
              >
                <Icon name="x" :size="12" />
              </button>
            </li>
          </ul>
        </div>
        <AgentInput
          v-model="composerDraft"
          :notebook-display-title="agentNotebookDisplayTitle"
          :knowledge-mode="knowledgePanelEnabled"
          :knowledge-dock-mode="knowledgePanelEnabled ? knowledgeDockMode : undefined"
          :show-attach="knowledgePanelEnabled && knowledgeDockMode === 'import'"
          :show-image-upload="showImageUploadButton"
          :has-attachments="pendingAttachments.length > 0"
          :voice-recording="voiceRecording"
          :pending-images="pendingImages"
          :disabled="agent.streaming || !knowledgePanelEnabled"
          :placeholder="knowledgePanelEnabled ? undefined : undefined"
          @keydown="onKeydown"
          @submit="onSubmit"
          @cancel="cancelAgentStream"
          @attach-files="onAttachFiles"
          @attach-images="onAttachImages"
          @attach-videos="onAttachVideos"
          @remove-image="onRemoveImage"
          @start-voice="onStartVoice"
          @stop-voice="onStopVoice"
          @cancel-voice="onCancelVoice"
        />
      </div>
    </Teleport>

  </aside>
</template>

<style scoped>
.agent-dock {
  --agent-font:
    'Inter', ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;

  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  border-radius: 16px;
  padding: var(--space-4) var(--space-4) var(--space-3);
  font-family: var(--agent-font);
  font-weight: 600;
  -webkit-font-smoothing: antialiased;
}

/** 侧栏整体：轻卡片阴影，避免「对话框」式过重浮层 */
.agent-dock.glass-panel {
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05), 0 4px 14px rgba(0, 0, 0, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.5);
}

.agent-dock--expanded.glass-panel {
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.06), 0 12px 32px rgba(0, 0, 0, 0.1);
  border-color: rgba(255, 255, 255, 0.65);
}

/* Left-edge drag resize handle */
.agent-dock__resize-handle {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 8px;
  cursor: col-resize;
  z-index: 2;
  background: transparent;
  transition: background 0.15s ease;
  border-radius: 16px 0 0 16px;
}

.agent-dock__resize-handle:hover {
  background: linear-gradient(135deg, #6366f11a, #6366f10a);
}

.agent-dock--dragging .agent-dock__resize-handle {
  background: linear-gradient(135deg, #6366f11a, #6366f10a);
}

/* Grip dot indicators */
.agent-dock__resize-handle::after {
  content: '';
  position: absolute;
  top: 50%;
  left: 2px;
  transform: translateY(-50%);
  width: 2px;
  height: 28px;
  border-radius: 1px;
  background: rgba(60, 60, 67, 0.15);
  opacity: 0;
  transition: opacity 0.15s ease;
}

.agent-dock__resize-handle:hover::after,
.agent-dock--dragging .agent-dock__resize-handle::after {
  opacity: 1;
}

.agent-dock__head {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding-bottom: var(--space-1);
  flex-shrink: 0;
}

.agent-dock__head-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3);
}

.agent-dock__mode-tabs {
  display: flex;
  gap: 6px;
  padding: 3px;
  border-radius: 10px;
  background: rgba(60, 60, 67, 0.06);
}

.agent-dock__mode-tab {
  flex: 1;
  border: none;
  border-radius: 8px;
  padding: 6px 10px;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
  background: transparent;
  cursor: pointer;
}

.agent-dock__mode-tab--active {
  color: var(--color-text);
  background: #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
}

.agent-dock__chat-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin-bottom: var(--space-3);
  flex-shrink: 0;
}

.agent-dock__chat-mode {
  border: 1px solid rgba(60, 60, 67, 0.12);
  border-radius: 999px;
  padding: 4px 10px;
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-secondary);
  background: rgba(255, 255, 255, 0.6);
  cursor: pointer;
}

.agent-dock__chat-mode--active {
  color: var(--color-accent);
  border-color: rgba(0, 122, 255, 0.32);
  background: #fff;
  box-shadow: 0 1px 3px rgba(0, 122, 255, 0.14);
}

.agent-dock__toolbar-spacer {
  flex: 1 1 auto;
  min-width: 8px;
}

.agent-dock__chat-mode--thinking {
  margin-left: auto;
}

.agent-dock__search-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-secondary);
  align-self: center;
  margin-right: 2px;
}

.agent-dock__messages {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.agent-dock__msg {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-width: 100%;
}

.agent-dock__msg--user {
  align-items: flex-end;
}

.agent-dock__msg--assistant {
  align-items: flex-start;
}

.agent-dock__msg-role {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--color-text-secondary);
  opacity: 0.75;
}

.agent-dock__msg-bubble {
  position: relative;
  max-width: 100%;
}

.agent-dock__msg-bubble--assistant {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 8px;
}

.agent-dock__thinking {
  border-radius: 10px;
  background: color-mix(in srgb, var(--color-text-secondary) 6%, transparent);
  border: 1px solid color-mix(in srgb, var(--color-text-secondary) 12%, transparent);
  overflow: hidden;
}
.agent-dock__thinking--active {
  background: color-mix(in srgb, var(--color-text-secondary) 8%, transparent);
  border-color: color-mix(in srgb, var(--color-text-secondary) 18%, transparent);
}
.agent-dock__thinking-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 7px 10px;
  border: none;
  background: transparent;
  cursor: pointer;
  color: var(--color-text-secondary);
  font-size: 11px;
  font-weight: 600;
  text-align: left;
}
.agent-dock__thinking-toggle:hover {
  background: color-mix(in srgb, var(--color-text-secondary) 6%, transparent);
}
.agent-dock__thinking-chevron {
  flex-shrink: 0;
  opacity: 0.7;
  transition: transform 0.2s ease;
}
.agent-dock__thinking-chevron--open {
  transform: rotate(90deg);
}
.agent-dock__thinking-label {
  flex: 0 0 auto;
}
.agent-dock__thinking-count {
  margin-left: auto;
  font-weight: 600;
  opacity: 0.75;
  font-variant-numeric: tabular-nums;
}
.agent-dock__thinking-pulse {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: color-mix(in srgb, var(--color-text-secondary) 70%, transparent);
  animation: agent-dock-thinking-pulse 1.2s ease-in-out infinite;
}
@keyframes agent-dock-thinking-pulse {
  0%, 100% { opacity: 0.35; transform: scale(0.85); }
  50% { opacity: 1; transform: scale(1.15); }
}
.agent-dock__thinking-body {
  padding: 0 10px 8px 28px;
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.agent-dock__thinking-step {
  font-size: 11px;
  line-height: 1.45;
  color: color-mix(in srgb, var(--color-text-secondary) 85%, transparent);
  font-weight: 600;
}
.agent-dock__thinking-step--llm {
  white-space: pre-wrap;
  word-break: break-word;
  font-weight: 600;
}
.agent-dock__thinking-step--latest {
  color: var(--color-text);
}
.agent-dock__thinking--active .agent-dock__thinking-step--latest {
  animation: agent-dock-thinking-fade 0.3s ease;
}
@keyframes agent-dock-thinking-fade {
  from { opacity: 0; transform: translateY(3px); }
  to { opacity: 1; transform: translateY(0); }
}

.agent-dock__msg-body {
  margin: 0;
  padding: 10px 12px;
  border-radius: 12px;
  font-size: 13px;
  font-weight: 700;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
}

.agent-dock__msg-body.agent-dock__msg-body--collapsed {
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.agent-dock__msg-body-line {
  margin: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  font-weight: 600;
  line-height: 1.55;
}

.agent-dock__msg-body-main {
  min-width: 0;
}

.agent-dock__msg-bubble--assistant .agent-dock__msg-body {
  padding-bottom: 10px;
  border-radius: 12px;
}

.agent-dock__msg-user {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 6px;
  max-width: 100%;
}

.agent-dock__msg-user-veil {
  position: absolute;
  inset: 0;
  z-index: 1;
  border-radius: 12px;
  pointer-events: none;
  overflow: hidden;
  background: linear-gradient(
    118deg,
    rgba(255, 255, 255, 0.5) 0%,
    rgba(0, 122, 255, 0.12) 40%,
    rgba(255, 255, 255, 0.46) 100%
  );
  background-size: 200% 200%;
  backdrop-filter: blur(1.5px) saturate(1.1);
  -webkit-backdrop-filter: blur(1.5px) saturate(1.1);
  animation: agent-dock-user-veil 2.2s ease-in-out infinite;
}

.agent-dock__msg-user-veil::after {
  content: '';
  position: absolute;
  inset: -35%;
  border-radius: 40%;
  background: radial-gradient(circle at 32% 42%, rgba(255, 255, 255, 0.62) 0%, transparent 52%);
  animation: agent-dock-user-veil-drift 1.65s ease-in-out infinite;
}

@keyframes agent-dock-user-veil {
  0%,
  100% {
    background-position: 0% 50%;
    opacity: 0.82;
  }
  50% {
    background-position: 100% 45%;
    opacity: 1;
  }
}

@keyframes agent-dock-user-veil-drift {
  0%,
  100% {
    transform: translate(-6%, 3%);
  }
  50% {
    transform: translate(10%, -5%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .agent-dock__msg-user-veil,
  .agent-dock__msg-user-veil::after {
    animation: none;
  }
}

.agent-dock__msg-attachments {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
  max-width: 100%;
}

.agent-dock__msg-attachment-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 100%;
  padding: 4px 10px;
  border: 1px solid rgba(0, 122, 255, 0.22);
  border-radius: 999px;
  background: rgba(0, 122, 255, 0.08);
  color: var(--color-text);
  font-size: 11px;
  font-weight: 600;
  line-height: 1.3;
}

.agent-dock__msg-attachment-tag-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: min(220px, 72vw);
}

.agent-dock__msg--user .agent-dock__msg-body {
  background: linear-gradient(135deg, rgba(0, 122, 255, 0.16) 0%, rgba(0, 122, 255, 0.08) 100%);
  color: var(--color-text);
  box-shadow: 0 1px 2px rgba(0, 122, 255, 0.06), inset 0 1px 0 rgba(255, 255, 255, 0.45);
}

.agent-dock__msg-bubble--assistant .agent-dock__msg-body {
  background: linear-gradient(180deg, rgba(60, 60, 67, 0.04) 0%, rgba(60, 60, 67, 0.01) 100%);
  color: var(--color-text);
  border: 1px solid rgba(60, 60, 67, 0.07);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03);
}

.agent-dock__msg-body--md {
  font-weight: 600;
  white-space: normal;
}

.agent-dock__msg-meta {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
  padding-top: 8px;
}

.agent-dock__meta-label {
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--color-text-secondary);
}

.agent-dock__strategy {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.agent-dock__strategy-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  align-self: flex-start;
  padding: 4px 10px;
  border-radius: 999px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.1) 0%, rgba(99, 102, 241, 0.04) 100%);
  border: 1px solid rgba(99, 102, 241, 0.18);
  font-size: 11px;
  font-weight: 600;
  line-height: 1.3;
  color: var(--color-text);
  box-shadow: 0 1px 2px rgba(99, 102, 241, 0.06);
}

.agent-dock__query-4d {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.agent-dock__query-4d-tag {
  font-size: 11px;
  line-height: 1.4;
  color: rgba(30, 130, 60, 0.9);
  padding: 4px 10px;
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(52, 199, 89, 0.1) 0%, rgba(52, 199, 89, 0.04) 100%);
  border: 1px solid rgba(52, 199, 89, 0.18);
  box-shadow: 0 1px 2px rgba(52, 199, 89, 0.06);
  align-self: flex-start;
  font-weight: 600;
}

.agent-dock__strategy-icon {
  flex-shrink: 0;
  color: rgba(99, 102, 241, 0.9);
}

.agent-dock__citations {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.agent-dock__citation-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.agent-dock__citation-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 100%;
  padding: 4px 10px;
  border: 1px solid rgba(0, 122, 255, 0.18);
  border-radius: 999px;
  background: linear-gradient(135deg, rgba(0, 122, 255, 0.09) 0%, rgba(0, 122, 255, 0.03) 100%);
  color: var(--color-text);
  font-size: 11px;
  font-weight: 600;
  line-height: 1.3;
  cursor: pointer;
  box-shadow: 0 1px 2px rgba(0, 122, 255, 0.05);
  transition: background 0.15s ease, border-color 0.15s ease, box-shadow 0.15s ease;
}

.agent-dock__citation-tag:disabled {
  cursor: default;
  opacity: 0.85;
}

.agent-dock__citation-tag:not(:disabled):hover {
  background: linear-gradient(135deg, rgba(0, 122, 255, 0.15) 0%, rgba(0, 122, 255, 0.06) 100%);
  border-color: rgba(0, 122, 255, 0.30);
  box-shadow: 0 2px 4px rgba(0, 122, 255, 0.10);
}

.agent-dock__citation-tag-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 180px;
}

.agent-dock__citation-tag-count {
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.agent-dock__msg-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 2px;
  margin: 0 4px;
  padding: 6px 4px 2px;
}

.agent-dock__msg-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  padding: 0;
  border-radius: 6px;
  border: 1px solid rgba(60, 60, 67, 0.1);
  background: rgba(255, 255, 255, 0.5);
  color: var(--color-text-secondary);
  cursor: pointer;
}

.agent-dock__msg-action:hover {
  background: rgba(60, 60, 67, 0.08);
  color: var(--color-text);
}

.agent-dock__msg-action--active {
  color: var(--color-text);
  background: var(--color-selected-soft);
  border-color: var(--color-selected-line);
}

.agent-dock__msg-footer {
  margin: 0;
  font-size: 11px;
  line-height: 1.45;
  color: var(--color-text-secondary);
  white-space: pre-wrap;
}

.agent-dock__head-text {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.agent-dock__head-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  padding-top: 1px;
}

.agent-dock__title {
  margin: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 650;
  letter-spacing: -0.02em;
  color: var(--color-text);
}

.agent-dock__subtitle {
  margin: 0;
  font-size: 12px;
  line-height: 1.35;
  font-weight: 600;
  color: var(--color-text-secondary);
  letter-spacing: -0.01em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 42ch;
}

.agent-dock__title-icon {
  flex-shrink: 0;
  opacity: 0.78;
}

.agent-dock__clear {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-family: var(--agent-font);
  font-size: 12px;
  font-weight: 600;
  padding: 7px 12px;
  border-radius: 8px;
  border: 1px solid rgba(60, 60, 67, 0.12);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.92), rgba(250, 250, 252, 0.72));
  color: var(--color-text-secondary);
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.9) inset;
  cursor: pointer;
  transition:
    background 0.15s ease,
    color 0.15s ease,
    box-shadow 0.15s ease;
}

.agent-dock__clear-icon {
  flex-shrink: 0;
  opacity: 0.75;
}

.agent-dock__clear:disabled {
  opacity: 0.38;
  cursor: not-allowed;
  box-shadow: none;
}

.agent-dock__clear:not(:disabled):hover {
  background: #fff;
  color: var(--color-text);
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 1) inset,
    0 2px 8px rgba(0, 0, 0, 0.06);
}

.agent-dock__close[aria-pressed='true'] {
  color: var(--color-text);
  border-color: var(--color-selected-line);
  background: var(--color-selected-soft);
}

.agent-dock__close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  border-radius: 8px;
  border: 1px solid rgba(60, 60, 67, 0.12);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.92), rgba(250, 250, 252, 0.72));
  color: var(--color-text-secondary);
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.9) inset;
  cursor: pointer;
  flex-shrink: 0;
  transition:
    background 0.15s ease,
    color 0.15s ease,
    box-shadow 0.15s ease;
}

.agent-dock__close:hover {
  background: #fff;
  color: var(--color-text);
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 1) inset,
    0 2px 8px rgba(0, 0, 0, 0.06);
}

.agent-dock__history-toolbar {
  flex-shrink: 0;
  margin-bottom: var(--space-3);
  min-width: 0;
}

.agent-dock__history-toolbar :deep(.form-select) {
  display: flex;
  width: 100%;
  min-width: 0;
}

.agent-dock__history-toolbar :deep(.form-select__trigger) {
  width: 100%;
  min-width: 0;
}

.agent-dock__history-toolbar :deep(.form-select__trigger-text) {
  flex: 1;
  text-align: left;
}

.agent-dock__viewport {
  position: relative;
  flex: 1;
  min-height: 0;
  width: 100%;
  height: 75vh;
  max-height: 75vh;
  overflow-x: hidden;
  overflow-y: auto;
  overscroll-behavior: contain;
  border-radius: 12px;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.agent-dock__viewport::-webkit-scrollbar {
  display: none;
  width: 0;
  height: 0;
}

.agent-dock__content {
  /* 底部让出固定输入条，避免最后一条消息的操作钮被挡住点不到 */
  padding: var(--space-2) var(--space-2) 200px var(--space-1);
  min-height: 120px;
}

.agent-dock__empty {
  margin: var(--space-4) 0 0;
  display: flex;
  align-items: flex-start;
  gap: 12px;
  font-size: 13px;
  line-height: 1.55;
  font-weight: 600;
  color: var(--color-text-secondary);
  letter-spacing: -0.01em;
  max-width: 36rem;
}

.agent-dock__empty-icon {
  flex-shrink: 0;
  margin-top: 2px;
  opacity: 0.5;
}

.agent-dock__enrichment {
  margin: var(--space-2) 0 0;
  padding: 14px 16px;
  border-radius: 14px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98) 0%, rgba(252, 252, 254, 0.94) 100%);
  border: 1px solid rgba(60, 60, 67, 0.1);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.agent-dock__enrichment-block + .agent-dock__enrichment-block {
  padding-top: 16px;
  border-top: 1px solid rgba(60, 60, 67, 0.08);
}

.agent-dock__enrichment-title {
  margin: 0 0 10px;
  font-size: 11px;
  font-weight: 650;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: rgba(60, 60, 67, 0.72);
}

.agent-dock__enrichment-body,
.agent-dock__enrichment-loading,
.agent-dock__enrichment-muted {
  margin: 0;
  font-size: 13px;
  line-height: 1.55;
  font-weight: 600;
  letter-spacing: -0.015em;
  white-space: pre-wrap;
  word-break: break-word;
}

.agent-dock__enrichment-body {
  color: var(--color-text);
}

.agent-dock__enrichment-loading {
  color: var(--color-text-secondary);
}

.agent-dock__enrichment-muted {
  color: var(--color-text-secondary);
}

.agent-dock__err {
  margin: 0 var(--space-1) var(--space-3);
  display: flex;
  align-items: flex-start;
  gap: 10px;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.45;
  color: #c41e1e;
  padding: 10px 12px;
  border-radius: 12px;
  background: rgba(196, 30, 30, 0.06);
  border: 1px solid rgba(196, 30, 30, 0.14);
}

.agent-dock__err-icon {
  flex-shrink: 0;
  margin-top: 1px;
}

.agent-dock__compose {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  width: 100%;
}

.agent-dock__pending-files {
  padding: var(--space-2) var(--space-3);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.82);
  border: 1px solid rgba(60, 60, 67, 0.1);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
}

.agent-dock__pending-label {
  display: block;
  margin-bottom: 6px;
  font-family: var(--agent-font);
  font-size: 11px;
  font-weight: 650;
  letter-spacing: 0.06em;
  color: var(--color-text-secondary);
  text-transform: uppercase;
}

.agent-dock__pending-list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.agent-dock__pending-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  font-size: 12px;
  line-height: 1.35;
  font-weight: 600;
}

.agent-dock__pending-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--color-text);
}

.agent-dock__pending-remove {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  border: none;
  border-radius: 6px;
  font-size: 16px;
  line-height: 1;
  color: var(--color-text-secondary);
  background: rgba(60, 60, 67, 0.08);
  cursor: pointer;
  transition:
    background 0.12s ease,
    color 0.12s ease;
}

.agent-dock__pending-remove:hover {
  background: rgba(60, 60, 67, 0.14);
  color: var(--color-text);
}

</style>

<!-- markstream-vue overrides (non-scoped to reach into component tree) -->
<style>
.agent-dock__msg-md.markstream-vue {
  --link-color: #007aff;
  --table-border: rgba(60, 60, 67, 0.15);
  --table-header-bg: rgba(60, 60, 67, 0.04);
  --blockquote-border: rgba(60, 60, 67, 0.18);
  --blockquote-fg: var(--color-text-secondary, rgba(60, 60, 67, 0.65));
  --code-bg: rgba(60, 60, 67, 0.07);
  --code-border: rgba(60, 60, 67, 0.08);
  --inline-code-bg: rgba(60, 60, 67, 0.1);
  --hr-border: rgba(60, 60, 67, 0.12);
  /* Match user message bubble (13px) */
  font-size: 13px;
  line-height: 1.55;
  font-weight: 600;
}

/* Headings — override markstream-vue rem-based defaults */
.agent-dock__msg-md.markstream-vue .heading-1 { font-size: 1.15em; }
.agent-dock__msg-md.markstream-vue .heading-2 { font-size: 1.08em; }
.agent-dock__msg-md.markstream-vue .heading-3,
.agent-dock__msg-md.markstream-vue .heading-4 { font-size: 1em; }
.agent-dock__msg-md.markstream-vue .heading-5,
.agent-dock__msg-md.markstream-vue .heading-6 { font-size: 0.92em; }

/* Paragraph / list spacing */
.agent-dock__msg-md.markstream-vue .paragraph-node {
  margin: 0.35em 0;
  font-weight: 600;
  color: var(--color-text);
  font-size: 14px;
}
.agent-dock__msg-md.markstream-vue .list-node { margin: 0.2em 0; }
</style>
