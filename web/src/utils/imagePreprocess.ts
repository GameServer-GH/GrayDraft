/** Maximum image dimension (width or height) before downscaling. */
const MAX_IMAGE_DIMENSION = 2048

/** Maximum size in bytes after compression (5 MB). */
const MAX_IMAGE_BYTES = 5 * 1024 * 1024

/** Thumbnail size (square). */
const THUMBNAIL_SIZE = 120

export interface PreprocessedImage {
  /** Compressed image as base64 (no data URL prefix). */
  base64: string
  /** Thumbnail data URL for UI preview. */
  thumbnail: string
  /** MIME type, e.g. "image/jpeg". */
  mimeType: string
  /** Original filename. */
  filename: string
}

/**
 * Preprocess an image file: resize if needed, compress to JPEG, generate thumbnail.
 * Returns base64 data ready for the /api/image/recognize endpoint.
 */
export async function preprocessImage(file: File): Promise<PreprocessedImage> {
  const mimeType = resolveOutputMime(file.type)
  const bitmap = await createImageBitmap(file)
  const { width, height } = computeSize(bitmap.width, bitmap.height)

  // --- Full-size compressed image ---
  const fullCanvas = document.createElement('canvas')
  fullCanvas.width = width
  fullCanvas.height = height
  const fullCtx = fullCanvas.getContext('2d')
  if (!fullCtx) throw new Error('Cannot create canvas context')
  fullCtx.drawImage(bitmap, 0, 0, width, height)

  // Compress with quality adjustment.
  let quality = 0.85
  let base64 = ''
  for (let attempt = 0; attempt < 3; attempt++) {
    base64 = fullCanvas.toDataURL(mimeType, quality).split(',')[1] ?? ''
    const byteSize = Math.ceil((base64.length * 3) / 4)
    if (byteSize <= MAX_IMAGE_BYTES) break
    quality -= 0.15
    if (quality < 0.3) quality = 0.3
  }

  // --- Thumbnail ---
  const thumbCanvas = document.createElement('canvas')
  thumbCanvas.width = THUMBNAIL_SIZE
  thumbCanvas.height = THUMBNAIL_SIZE
  const thumbCtx = thumbCanvas.getContext('2d')
  if (!thumbCtx) throw new Error('Cannot create thumbnail context')
  // Cover crop.
  const scale = Math.max(THUMBNAIL_SIZE / bitmap.width, THUMBNAIL_SIZE / bitmap.height)
  const sw = THUMBNAIL_SIZE / scale
  const sh = THUMBNAIL_SIZE / scale
  const sx = (bitmap.width - sw) / 2
  const sy = (bitmap.height - sh) / 2
  thumbCtx.drawImage(bitmap, sx, sy, sw, sh, 0, 0, THUMBNAIL_SIZE, THUMBNAIL_SIZE)
  const thumbnail = thumbCanvas.toDataURL('image/jpeg', 0.75)

  bitmap.close()

  return {
    base64,
    thumbnail,
    mimeType,
    filename: file.name,
  }
}

function computeSize(w: number, h: number): { width: number; height: number } {
  if (w <= MAX_IMAGE_DIMENSION && h <= MAX_IMAGE_DIMENSION) {
    return { width: w, height: h }
  }
  const ratio = Math.min(MAX_IMAGE_DIMENSION / w, MAX_IMAGE_DIMENSION / h)
  return {
    width: Math.round(w * ratio),
    height: Math.round(h * ratio),
  }
}

function resolveOutputMime(originalType: string): string {
  const t = originalType.toLowerCase()
  if (t === 'image/png' || t === 'image/jpeg' || t === 'image/webp') return t
  return 'image/jpeg'
}
