/** 异步智能导入过程中展示的占位卡片 */
export interface ImportingCard {
  title: string
  segment: number
  total: number
  status: 'queued' | 'processing' | 'completed' | 'failed'
}

/** 笔记卡片在画布世界坐标中的位置（原点为视口中心，单位 px，与 viewScale 无关） */
export interface NotebookCardLayout {
  /** 卡片中心相对视口中心的水平偏移 */
  cx: number
  /** 卡片中心相对视口中心的垂直偏移 */
  cy: number
  w: number
  h: number
}

export interface CanvasViewState {
  panX: number
  panY: number
  /** 视口缩放：1 = 100%，合法范围 0.05～3（由 store 钳制） */
  viewScale: number
  notebookLayouts: Record<string, NotebookCardLayout>
}
