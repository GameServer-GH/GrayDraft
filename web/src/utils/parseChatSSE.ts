import type { ChatStreamEvent } from '@/types/knowledge'

/** 从单行 SSE data 解析事件（支持 `data: ` 与 `data:`） */
function parseDataLine(line: string): ChatStreamEvent | null {
  const trimmed = line.trim()
  if (!trimmed.startsWith('data:')) return null
  const json = trimmed.startsWith('data: ')
    ? trimmed.slice(6)
    : trimmed.slice(5).trimStart()
  if (!json) return null
  try {
    return JSON.parse(json) as ChatStreamEvent
  } catch {
    return null
  }
}

/** 按行消费完整行，返回未读完的尾部（可能是不完整行） */
export function consumeChatSSELines(
  buffer: string,
  onEvent: (ev: ChatStreamEvent) => void,
): string {
  const normalized = buffer.replace(/\r\n/g, '\n')
  const lastNl = normalized.lastIndexOf('\n')
  if (lastNl < 0) return normalized

  const complete = normalized.slice(0, lastNl + 1)
  let tail = normalized.slice(lastNl + 1)

  for (const line of complete.split('\n')) {
    const ev = parseDataLine(line)
    if (ev) onEvent(ev)
  }
  return tail
}

/** 流结束后处理剩余缓冲（含无尾换行的一行） */
export function flushChatSSEBuffer(buffer: string, onEvent: (ev: ChatStreamEvent) => void): void {
  const tail = buffer.replace(/\r\n/g, '\n').trimEnd()
  if (!tail) return
  for (const line of tail.split('\n')) {
    const ev = parseDataLine(line)
    if (ev) onEvent(ev)
  }
}
