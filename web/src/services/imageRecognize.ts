import { imageRecognizeUrl } from '@/config/env'

export interface ImageRecognizeResult {
  /** Cleaned complete question text. */
  cleanedQuestion: string
  /** Raw VLM image description (for debug / optional display). */
  imageDescription: string
}

/**
 * Upload an image for VLM recognition + question cleaning.
 * The backend immediately discards the image after processing.
 */
export async function recognizeAndCleanImage(opts: {
  imageBase64: string
  mimeType: string
  userText?: string
  signal?: AbortSignal
}): Promise<ImageRecognizeResult> {
  const res = await fetch(imageRecognizeUrl(), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      image_base64: opts.imageBase64,
      mime_type: opts.mimeType,
      user_text: opts.userText?.trim() || '',
    }),
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
    throw new Error(`Image recognize failed: ${res.status}${detail}`)
  }

  return res.json()
}
