import { agentSmartImport, waitForIngestTask, type SmartImportAttachmentInput } from '@/services/knowledge'
import { prepareSmartImportFiles } from '@/utils/smartImportFiles'
import type { SmartImportResponse, SmartImportSegmentOverride } from '@/types/knowledge'
import { afterKnowledgeBulkChange } from '@/composables/useKnowledgeLibrarySync'
import { useCanvasStore } from '@/stores/canvas'
import { useUiStore } from '@/stores/ui'
import { formatNexusError } from '@/utils/nexusError'
import { stripEmoji } from '@/utils/stripEmoji'

export type SmartImportPayload = {
  rawText: string
  filename?: string
  /** 选中知识卡片时传入，smart-import 将更新该文档 */
  targetDocId?: string | null
  /** 写入 kb_documents.extensions（与 smart-import 锚点字段合并） */
  extensions?: Record<string, unknown>
  source_url?: string
  /** 图片/视频/PDF 等由 Core 多模态管线处理 */
  attachments?: SmartImportAttachmentInput[]
  /** 图片 base64，服务端 VLM 识别后拼接到 raw_text 底部 */
  mediaImageBase64?: string
  mediaImageMimeType?: string
  /** 视频 base64，服务端 VLM 识别后拼接到 raw_text 底部 */
  mediaVideoBase64?: string
  mediaVideoMimeType?: string
  /** 公开可访问的图片或视频链接 */
  mediaUrl?: string
}

export type SmartImportConfirmPayload = {
  segmentsOverride?: SmartImportSegmentOverride[]
  extensions?: Record<string, unknown>
}

function collectTaskIds(res: SmartImportResponse): string[] {
  const ids: string[] = []
  if (res.task?.task_id) ids.push(res.task.task_id)
  for (const t of res.tasks ?? []) {
    if (t.task_id) ids.push(t.task_id)
  }
  return [...new Set(ids)]
}

function formatImportSummary(res: SmartImportResponse, completed: number, failed: number): string {
  const conf = res.confidence ?? res.preview?.confidence
  const lines = [
    res.message,
    conf != null ? `confidence: ${conf.toFixed(2)}` : '',
    res.split ? `segments: ${(res.items ?? []).length}` : '',
    `ingest: ${completed} ok, ${failed} failed`,
  ]
    .filter((line): line is string => Boolean(line))
    .map(stripEmoji)
  return lines.join('\n')
}

export async function buildSmartImportRawText(rawText: string, files?: File[]): Promise<string> {
  let raw = stripEmoji(rawText.trim())
  if (files?.length) {
    const { textParts } = await prepareSmartImportFiles(files)
    const fileText = textParts.filter((p) => p.trim()).join('\n\n---\n\n')
    raw = [raw, fileText].filter(Boolean).join('\n\n---\n\n')
  }
  if (!raw) throw new Error('empty content')
  return raw
}

/** Builds smart-import payload including multimodal attachments for binary files. */
export async function buildSmartImportPayload(
  rawText: string,
  files?: File[],
): Promise<{ rawText: string; filename?: string; attachments?: SmartImportAttachmentInput[] }> {
  let raw = stripEmoji(rawText.trim())
  let attachments: SmartImportAttachmentInput[] | undefined
  let filename: string | undefined

  if (files?.length) {
    const prepared = await prepareSmartImportFiles(files)
    const fileText = prepared.textParts.filter((p) => p.trim()).join('\n\n---\n\n')
    raw = [raw, fileText].filter(Boolean).join('\n\n---\n\n')
    if (prepared.attachments.length > 0) {
      attachments = prepared.attachments
      filename = prepared.attachments[0]?.filename
    } else if (files[0]) {
      filename = files[0].name
    }
  }

  if (!raw.trim() && (!attachments || attachments.length === 0)) {
    throw new Error('empty content')
  }
  return { rawText: raw, filename, attachments }
}

function selectedAgentModel(): string {
  return useUiStore().selectedAgentModel || 'auto'
}

function smartImportBody(
  payload: SmartImportPayload,
  submit: boolean,
  confirm?: SmartImportConfirmPayload,
) {
  const target = payload.targetDocId?.trim() || undefined
  const extensions = confirm?.extensions ?? payload.extensions
  const body: Parameters<typeof agentSmartImport>[0] = {
    raw_text: payload.rawText,
    filename: payload.filename ?? '',
    target_doc_id: target,
    split: !target,
    submit,
    use_llm: true,
    agent_model: selectedAgentModel(),
    segments_override: confirm?.segmentsOverride?.length ? confirm.segmentsOverride : undefined,
    attachments: payload.attachments,
    media_image_base64: payload.mediaImageBase64,
    media_image_mime_type: payload.mediaImageMimeType,
    media_video_base64: payload.mediaVideoBase64,
    media_video_mime_type: payload.mediaVideoMimeType,
    media_url: payload.mediaUrl,
  }
  if (extensions && Object.keys(extensions).length > 0) {
    body.extensions = extensions
  }
  const sourceURL = payload.source_url?.trim()
  if (sourceURL) body.source_url = sourceURL
  return body
}

/** 分析并提交入库，轮询 ingest 任务直至完成 */
export async function runKnowledgeSmartImport(
  payload: SmartImportPayload,
  confirm?: SmartImportConfirmPayload,
): Promise<{ summary: string; response: SmartImportResponse }> {
  try {
    const submitted = await agentSmartImport(smartImportBody(payload, true, confirm))

    // Show masked placeholder cards in the knowledge list during import
    const canvas = useCanvasStore()
    const items = submitted.items ?? []
    if (items.length > 0) {
        canvas.setImportingCards(
            items.map((item, i) => ({
                title: item.preview?.title ?? 'Importing...',
                segment: i + 1,
                total: items.length,
                status: 'queued' as const,
            })),
        )
    }

    const taskIds = collectTaskIds(submitted)
    if (taskIds.length === 0) {
      if (submitted.preview?.action === 'unchanged') {
        canvas.clearImportingCards()
    await afterKnowledgeBulkChange()
        return { summary: submitted.message || 'unchanged', response: submitted }
      }
      throw new Error(submitted.message || 'no ingest tasks created')
    }

    let completed = 0
    let failed = 0
    for (let idx = 0; idx < taskIds.length; idx++) {
      const st = await waitForIngestTask(taskIds[idx])
      if (st.status === 'completed') {
        completed++
        const cards = canvas.importingCards
        if (cards[idx]) cards[idx].status = 'completed'
      } else {
        failed++
        const cards = canvas.importingCards
        if (cards[idx]) cards[idx].status = 'failed'
      }
    }

    canvas.clearImportingCards()
    await afterKnowledgeBulkChange()

    return {
      summary: formatImportSummary(submitted, completed, failed),
      response: submitted,
    }
  } catch (e) {
    throw new Error(formatNexusError(e))
  }
}
