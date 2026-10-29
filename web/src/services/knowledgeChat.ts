import { knowledgeChatStreamUrl } from '@/config/env'
import { getWorkspaceContextKey } from '@/services/workspaceContext'
import type { ChatStreamEvent, KnowledgeChatMode } from '@/types/knowledge'
import { agentModelField } from '@/utils/agentModelRequest'
import { consumeChatSSELines, flushChatSSEBuffer } from '@/utils/parseChatSSE'

export async function streamKnowledgeChat(opts: {
  query: string
  chatMode?: KnowledgeChatMode
  sessionId?: string
  topK?: number
  agentModel?: string
  thinkingMode?: boolean
  signal?: AbortSignal
  /** Image base64 data (preprocessed), sent for server-side recognition */
  imageBase64?: string
  /** Image MIME type, e.g. "image/png" */
  imageMimeType?: string
  /** Video base64 data, sent for server-side recognition */
  videoBase64?: string
  /** Video MIME type, e.g. "video/mp4" */
  videoMimeType?: string
  /** Public URL to an image or video, passed directly to VLM */
  mediaUrl?: string
  onEvent: (ev: ChatStreamEvent) => void
}): Promise<void> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'text/event-stream',
  }
  const k = getWorkspaceContextKey()
  if (k) headers['X-Graydraft-Workspace-Key'] = k

  const body: Record<string, unknown> = {
    query: opts.query,
    chat_mode: opts.chatMode ?? 'hybrid',
    session_id: opts.sessionId ?? '',
    top_k: opts.topK ?? 5,
    thinking_mode: opts.thinkingMode ?? true,
    ...agentModelField(opts.agentModel ?? 'auto'),
  }
  if (opts.imageBase64) {
    body.image_base64 = opts.imageBase64
    body.image_mime_type = opts.imageMimeType ?? 'image/png'
  }
  if (opts.videoBase64) {
    body.video_base64 = opts.videoBase64
    body.video_mime_type = opts.videoMimeType ?? 'video/mp4'
  }
  if (opts.mediaUrl) {
    body.media_url = opts.mediaUrl
  }

  const res = await fetch(knowledgeChatStreamUrl(), {
    method: 'POST',
    headers,
    body: JSON.stringify(body),
    signal: opts.signal,
  })

  if (!res.ok) {
    let detail = ''
    try {
      const t = await res.text()
      if (t.trim()) detail = ` — ${t.trim().slice(0, 240)}`
    } catch {
      /* ignore */
    }
    throw new Error(`Knowledge chat failed: ${res.status}${detail}`)
  }

  const reader = res.body?.getReader()
  if (!reader) throw new Error('Knowledge chat: empty response body')

  const dec = new TextDecoder()
  let buf = ''
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buf += dec.decode(value, { stream: true })
    buf = consumeChatSSELines(buf, opts.onEvent)
  }
  buf += dec.decode()
  flushChatSSEBuffer(buf, opts.onEvent)
}
