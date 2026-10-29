/**
 * Generates a static checkerboard pattern as a data URL.
 * Used as CSS background-image on the bg layer — never recomputed during zoom,
 * because the layer's CSS transform handles scaling.
 */
export function generateCheckerBoardDataURL(
  cellSize: number,
  colorA: string,
  colorB: string,
): string {
  const size = cellSize * 2
  const canvas = document.createElement('canvas')
  canvas.width = size
  canvas.height = size
  const ctx = canvas.getContext('2d')!
  ctx.fillStyle = colorA
  ctx.fillRect(0, 0, cellSize, cellSize)
  ctx.fillRect(cellSize, cellSize, cellSize, cellSize)
  ctx.fillStyle = colorB
  ctx.fillRect(cellSize, 0, cellSize, cellSize)
  ctx.fillRect(0, cellSize, cellSize, cellSize)
  return canvas.toDataURL()
}
