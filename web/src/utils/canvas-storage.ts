import type { CanvasViewState } from '@/types/canvas'

const STORAGE_KEY = 'graydraft:canvas:view:v1'

export function loadCanvasViewState(): Partial<CanvasViewState> | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const o = JSON.parse(raw) as Partial<CanvasViewState>
    if (!o || typeof o !== 'object') return null
    return o
  } catch {
    return null
  }
}

export function saveCanvasViewState(state: CanvasViewState) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  } catch {
    /* quota */
  }
}
