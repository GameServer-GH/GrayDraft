import { migrateGalaxyViewScale } from '@/utils/knowledgeGalaxy'

const VIEW_SCALE_MIN = 0.05
const VIEW_SCALE_MAX = 3

function clampViewScale(s: number): number {
  const x = Number.isFinite(s) ? s : 1
  return Math.min(VIEW_SCALE_MAX, Math.max(VIEW_SCALE_MIN, Math.round(x * 1000) / 1000))
}

const KEY_PREFIX = 'graydraft:canvas-view-v1:'

export type StoredCanvasView = {
  v: 1
  panX: number
  panY: number
  viewScale: number
  /** 写入时是否为知识库/星系模式，用于读取时决定是否迁移旧缩放基准 */
  galaxy?: boolean
}

function storageKey(workspaceKey: string): string {
  return `${KEY_PREFIX}${workspaceKey.trim()}`
}

export function readCanvasViewLocal(workspaceKey: string | null): StoredCanvasView | null {
  if (!workspaceKey?.trim()) return null
  try {
    const raw = localStorage.getItem(storageKey(workspaceKey))
    if (!raw) return null
    const parsed = JSON.parse(raw) as StoredCanvasView
    if (parsed?.v !== 1) return null
    if (!Number.isFinite(parsed.panX) || !Number.isFinite(parsed.panY) || !Number.isFinite(parsed.viewScale)) {
      return null
    }
    return parsed
  } catch {
    return null
  }
}

export function writeCanvasViewLocal(workspaceKey: string | null, view: Omit<StoredCanvasView, 'v'>) {
  if (!workspaceKey?.trim()) return
  const payload: StoredCanvasView = {
    v: 1,
    panX: view.panX,
    panY: view.panY,
    viewScale: clampViewScale(view.viewScale),
    galaxy: view.galaxy,
  }
  try {
    localStorage.setItem(storageKey(workspaceKey), JSON.stringify(payload))
  } catch {
    /* quota / private mode */
  }
}

/** 将本地缓存应用到画布视口（pan / zoom）。 */
export function canvasViewFromLocal(
  stored: StoredCanvasView,
  galaxyMode: boolean,
): { panX: number; panY: number; viewScale: number } {
  let viewScale = stored.viewScale
  if (galaxyMode) {
    viewScale = stored.galaxy ? viewScale : migrateGalaxyViewScale(viewScale)
  }
  return {
    panX: stored.panX,
    panY: stored.panY,
    viewScale: clampViewScale(viewScale),
  }
}

export function clearCanvasViewLocal(workspaceKey: string | null) {
  if (!workspaceKey?.trim()) return
  try {
    localStorage.removeItem(storageKey(workspaceKey))
  } catch {
    /* ignore */
  }
}
