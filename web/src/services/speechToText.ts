import { speechTranscribeUrl } from '@/config/env'

/**
 * Transcribe audio blob to text via the BFF speech-to-text endpoint.
 */
export async function transcribeAudio(blob: Blob, language = 'zh'): Promise<string> {
  const base64 = await blobToBase64(blob)

  // Determine format from MIME type.
  let format = 'webm'
  const mime = blob.type.toLowerCase()
  if (mime.includes('wav')) format = 'wav'
  else if (mime.includes('mp4') || mime.includes('m4a')) format = 'mp4'

  const res = await fetch(speechTranscribeUrl(), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      audio_base64: base64,
      format,
      language,
    }),
  })

  if (!res.ok) {
    let detail = ''
    try {
      const t = await res.text()
      if (t.trim()) detail = ` — ${t.trim().slice(0, 240)}`
    } catch {
      /* ignore */
    }
    throw new Error(`Speech transcribe failed: ${res.status}${detail}`)
  }

  const data = await res.json()
  return (data.text ?? '').trim()
}

function blobToBase64(blob: Blob): Promise<string> {
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
    reader.readAsDataURL(blob)
  })
}
