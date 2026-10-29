/** Max single attachment size aligned with Core multimodal.max_file_bytes (12MB). */
export const SMART_IMPORT_MAX_BYTES = 12 * 1024 * 1024

export type SmartImportAttachmentPayload = {
  filename: string
  mime_type?: string
  data_base64: string
}

function isTextLike(file: File): boolean {
  const t = (file.type || '').toLowerCase()
  if (t.startsWith('text/')) return true
  if (t === 'application/json' || t === 'application/markdown') return true
  const n = file.name.toLowerCase()
  return n.endsWith('.md') || n.endsWith('.txt') || n.endsWith('.json') || n.endsWith('.csv')
}

function readFileAsBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const result = reader.result
      if (typeof result !== 'string') {
        reject(new Error('read failed'))
        return
      }
      const comma = result.indexOf(',')
      resolve(comma >= 0 ? result.slice(comma + 1) : result)
    }
    reader.onerror = () => reject(reader.error ?? new Error('read failed'))
    reader.readAsDataURL(file)
  })
}

/**
 * Splits files into plain-text merge vs binary attachments for multimodal 4.0.
 */
export async function prepareSmartImportFiles(files: File[]): Promise<{
  textParts: string[]
  attachments: SmartImportAttachmentPayload[]
}> {
  const textParts: string[] = []
  const attachments: SmartImportAttachmentPayload[] = []

  for (const file of files) {
    if (file.size > SMART_IMPORT_MAX_BYTES) {
      throw new Error(`file too large (max ${Math.round(SMART_IMPORT_MAX_BYTES / (1024 * 1024))}MB): ${file.name}`)
    }
    if (isTextLike(file)) {
      try {
        const t = await file.text()
        if (t.trim()) textParts.push(t)
      } catch {
        /* skip */
      }
      continue
    }
    const data_base64 = await readFileAsBase64(file)
    attachments.push({
      filename: file.name,
      mime_type: file.type || undefined,
      data_base64,
    })
  }
  return { textParts, attachments }
}
