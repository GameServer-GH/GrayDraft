import { defineStore } from 'pinia'
import type { NotebookMeta } from '@/types/notebook'
import type { ImportingCard, NotebookCardLayout } from '@/types/canvas'
import { writeCanvasViewLocal } from '@/services/canvasViewStorage'
import { getWorkspaceContextKey } from '@/services/workspaceContext'
import { requestWorkspaceSettingsSave } from '@/services/workspaceSettingsHub'
import { useWorkspaceSessionStore } from '@/stores/workspaceSession'
import {
  GALAXY_VIEW_BASE_SCALE,
  migrateGalaxyViewScale,
} from '@/utils/knowledgeGalaxy'

/** 画布视口缩放：笔记模式 1 = 100%；星系模式 UI 100% = {@link GALAXY_VIEW_BASE_SCALE}。最小 5%，最大 300%（UI% 在星系下可更高）。 */
export const VIEW_SCALE_MIN = 0.05
export const VIEW_SCALE_MAX = 3

/** 钳制 viewScale 并量化到千分位，减轻浮点误差 */
export function clampViewScale(s: number): number {
  const x = Number.isFinite(s) ? s : 1
  return Math.min(VIEW_SCALE_MAX, Math.max(VIEW_SCALE_MIN, Math.round(x * 1000) / 1000))
}

/** Ctrl/⌘ + 滚轮：每次刻度对 viewScale 的绝对增量（约 3% UI，原先 8% 过快） */
export const WHEEL_ZOOM_STEP = 0.03

const DEFAULT_W = 520
const DEFAULT_H = 420
const MIN_W = 240
const MIN_H = 160
const MAX_W = 2400
const MAX_H = 1800

/** 新卡片相对「列表中上一张已布局卡片」的中心偏移（画布坐标 px，用于列表同步时无锚点的兜底） */
const NEW_NOTEBOOK_FALLBACK_OFFSET = 20

/** 新建笔记相对锚点卡片：右缘 + 间隔 → 新卡片中心（画布坐标 px） */
const NEW_NOTEBOOK_GAP_PX = 20

/** 画布、笔记卡片布局与视口平移缩放 */
export const useCanvasStore = defineStore('canvas', {
  state: () => ({
    blockIds: [] as string[],
    selectedIds: [] as string[],
    notebooks: [] as NotebookMeta[],
    activeNotebookId: null as string | null,
    /** 用户显式取消选中（工作区维度对话）后，列表刷新不再自动回选第一个知识点 */
    _selectionCleared: false,
    notebookLoadError: null as string | null,
    /** Masked placeholder cards shown during async smart-import */
    importingCards: [] as ImportingCard[],
    viewScale: 1,
    panX: 0,
    panY: 0,
    notebookLayouts: {} as Record<string, NotebookCardLayout>,
    _persistT: null as ReturnType<typeof setTimeout> | null,
  }),
  getters: {
    activeNotebook(state): NotebookMeta | null {
      return state.notebooks.find((n) => n.id === state.activeNotebookId) ?? null
    },
    layoutFor:
      (state) =>
      (id: string): NotebookCardLayout => {
        const L = state.notebookLayouts[id]
        if (L) return L
        return { cx: 0, cy: 0, w: DEFAULT_W, h: DEFAULT_H }
      },
  },
  actions: {
    hydrateFromStorage() {
      /* 画布视口与布局改由工作区 settings.json 恢复，见 workspaceSession */
    },

    applyFromServer(raw: Record<string, unknown>, opts?: { galaxyMode?: boolean }) {
      const nl = raw.notebookLayouts
      if (nl && typeof nl === 'object' && !Array.isArray(nl)) {
        this.notebookLayouts = { ...(nl as Record<string, NotebookCardLayout>) }
      }
      this.applyViewport(raw, opts)
    },

    applyViewport(raw: Record<string, unknown>, opts?: { galaxyMode?: boolean }) {
      if (typeof raw.panX === 'number') this.panX = raw.panX
      if (typeof raw.panY === 'number') this.panY = raw.panY
      if (typeof raw.viewScale === 'number') {
        let s = Number.isFinite(raw.viewScale) ? raw.viewScale : 1
        if (opts?.galaxyMode) s = migrateGalaxyViewScale(s)
        this.viewScale = clampViewScale(s)
      } else if (opts?.galaxyMode) {
        this.viewScale = clampViewScale(GALAXY_VIEW_BASE_SCALE)
      }
    },

    _persistCanvasViewLocal() {
      const wsKey = getWorkspaceContextKey()
      if (!wsKey) return
      const galaxy = Boolean(useWorkspaceSessionStore().nexusKbType?.trim())
      writeCanvasViewLocal(wsKey, {
        panX: this.panX,
        panY: this.panY,
        viewScale: this.viewScale,
        galaxy,
      })
    },

    _schedulePersist() {
      this._persistCanvasViewLocal()
      if (this._persistT) clearTimeout(this._persistT)
      this._persistT = setTimeout(() => {
        this._persistT = null
        requestWorkspaceSettingsSave()
      }, 320)
    },

    persistViewNow() {
      this._persistCanvasViewLocal()
      if (this._persistT) {
        clearTimeout(this._persistT)
        this._persistT = null
      }
      requestWorkspaceSettingsSave()
    },

    setNotebooks(list: NotebookMeta[]) {
      this.notebooks = list
      this.notebookLoadError = null
      if (list.length === 0) {
        this.activeNotebookId = null
      } else if (this._selectionCleared) {
        if (this.activeNotebookId && !list.some((n) => n.id === this.activeNotebookId)) {
          this.activeNotebookId = null
        }
      } else if (!this.activeNotebookId || !list.some((n) => n.id === this.activeNotebookId)) {
        this.activeNotebookId = null
      }
      const nextLayouts = { ...this.notebookLayouts }
      for (const n of list) {
        if (nextLayouts[n.id]) continue
        let prevLayout: NotebookCardLayout | null = null
        for (const m of list) {
          if (m.id === n.id) break
          const Lm = nextLayouts[m.id]
          if (Lm) prevLayout = Lm
        }
        if (prevLayout) {
          nextLayouts[n.id] = {
            cx: prevLayout.cx + NEW_NOTEBOOK_FALLBACK_OFFSET,
            cy: prevLayout.cy + NEW_NOTEBOOK_FALLBACK_OFFSET,
            w: DEFAULT_W,
            h: DEFAULT_H,
          }
        } else {
          nextLayouts[n.id] = { cx: 0, cy: 0, w: DEFAULT_W, h: DEFAULT_H }
        }
      }
      for (const k of Object.keys(nextLayouts)) {
        if (!list.some((n) => n.id === k)) delete nextLayouts[k]
      }
      this.notebookLayouts = nextLayouts
      this._schedulePersist()
    },
    setImportingCards(cards: ImportingCard[]) {
      this.importingCards = cards
    },
    clearImportingCards() {
      this.importingCards = []
    },
    setNotebookLoadError(msg: string | null) {
      this.notebookLoadError = msg
    },
    /** 用服务端返回的元数据更新列表中的同 id 项（如重命名后）。 */
    patchNotebookMeta(meta: NotebookMeta) {
      const i = this.notebooks.findIndex((n) => n.id === meta.id)
      if (i < 0) return
      const next = [...this.notebooks]
      next[i] = { ...next[i], ...meta }
      this.notebooks = next
    },
    selectNotebook(id: string | null) {
      this.activeNotebookId = id
      this._selectionCleared = id === null
    },
    /**
     * 将新建笔记卡片放在锚点卡片右侧，两卡外缘间距 20px（纵向与锚点中心对齐）。
     * 无有效锚点时落在原点，由 {@link setNotebooks} 的兜底布局覆盖前可先调用本方法修正。
     */
    placeNewNotebookAfterAnchor(newId: string, anchorId: string | null) {
      const w = DEFAULT_W
      const h = DEFAULT_H
      let cx = 0
      let cy = 0
      if (
        anchorId &&
        anchorId !== newId &&
        this.notebooks.some((n) => n.id === anchorId)
      ) {
        const L = this.layoutFor(anchorId)
        cx = L.cx + L.w / 2 + NEW_NOTEBOOK_GAP_PX + w / 2
        cy = L.cy
      }
      this.notebookLayouts = {
        ...this.notebookLayouts,
        [newId]: { cx, cy, w, h },
      }
      this._schedulePersist()
    },
    patchNotebookLayout(id: string, patch: Partial<NotebookCardLayout>) {
      const cur = this.notebookLayouts[id] ?? {
        cx: 0,
        cy: 0,
        w: DEFAULT_W,
        h: DEFAULT_H,
      }
      let w = patch.w ?? cur.w
      let h = patch.h ?? cur.h
      w = Math.min(MAX_W, Math.max(MIN_W, w))
      h = Math.min(MAX_H, Math.max(MIN_H, h))
      this.notebookLayouts[id] = {
        cx: patch.cx ?? cur.cx,
        cy: patch.cy ?? cur.cy,
        w,
        h,
      }
      this._schedulePersist()
    },
    setViewScale(scale: number) {
      const s = Number.isFinite(scale) ? scale : 1
      this.viewScale = clampViewScale(s)
      this._schedulePersist()
    },
    /**
     * Ctrl / ⌘ + 滚轮缩放，锚点为视口内坐标（相对画布 viewport 左上角，px）。
     * 与 CanvasRoot 的 translate(pan)+scale(s)、transform-origin:center 一致：
     * 屏幕点 (vx,vy) 下画布内容在缩放前后保持在该点下方。
     * 缩放范围见 {@link VIEW_SCALE_MIN}、{@link VIEW_SCALE_MAX}。
     */
    zoomViewByWheelAt(
      deltaY: number,
      vx: number,
      vy: number,
      viewW: number,
      viewH: number,
      step = WHEEL_ZOOM_STEP,
    ) {
      const s0 = this.viewScale
      const dir = deltaY > 0 ? -1 : 1
      const s1 = clampViewScale(s0 + dir * step)
      if (s1 === s0) return
      const cx = viewW / 2
      const cy = viewH / 2
      this.panX = vx - cx - (s1 / s0) * (vx - this.panX - cx)
      this.panY = vy - cy - (s1 / s0) * (vy - this.panY - cy)
      this.viewScale = s1
      this._schedulePersist()
    },
    panBy(dx: number, dy: number) {
      this.panX += dx
      this.panY += dy
      this._schedulePersist()
    },
    /** 将视口对准画布坐标原点（视口中心），与卡片布局的 (cx,cy)=(0,0) 对齐 */
    recenterViewOnCanvasOrigin() {
      this.panX = 0
      this.panY = 0
      this.persistViewNow()
    },
    /**
     * 星系图模式：将视口对准指定节点（使用 galaxy layout 中 node 的 viewBox 坐标）。
     * frameW/frameH 为 galaxy frame 宽高，boundsMinX/boundsMinY 为 viewBox 起点。
     */
    focusGalaxyNode(
      nx: number,
      ny: number,
      frameW: number,
      frameH: number,
      boundsMinX: number,
      boundsMinY: number,
    ) {
      const s = this.viewScale
      this.panX = (frameW / 2 - nx + boundsMinX) * s
      this.panY = (frameH / 2 - ny + boundsMinY) * s
      this.persistViewNow()
    },
    /**
     * 优先将视口对准当前激活笔记卡片中心；无激活笔记时同 {@link recenterViewOnCanvasOrigin}。
     * 与 CanvasRoot 的 translate(pan)+scale(s)、transform-origin:center 一致：pan = -s·(cx,cy)。
     */
    recenterViewToActiveNotebookOrOrigin() {
      const id = this.activeNotebookId
      if (id && this.notebooks.some((n) => n.id === id)) {
        const L = this.layoutFor(id)
        const s = this.viewScale
        this.panX = -s * L.cx
        this.panY = -s * L.cy
      } else {
        this.panX = 0
        this.panY = 0
      }
      this.persistViewNow()
    },
  },
})
