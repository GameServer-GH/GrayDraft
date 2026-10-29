export const AGENT_DOCK_MIN_WIDTH_PX = 360
export const AGENT_DOCK_MAX_WIDTH_PX = 1024

/** 最大宽度占视口的比例（拖拽上限） */
export const AGENT_DOCK_MAX_VW_RATIO = 0.95

const VIEWPORT_BAND = 'calc(100vw - 2 * var(--float-gap, 12px))'

/** 与 Workspace 右侧 Agent 浮窗宽度一致，供画布 inset 等复用 */
export function agentDockWidthBand(expanded: boolean): string {
  if (expanded) {
    return `min(70vh, ${AGENT_DOCK_MAX_WIDTH_PX}px, ${VIEWPORT_BAND})`
  }
  return `min(${AGENT_DOCK_MIN_WIDTH_PX}px, ${VIEWPORT_BAND})`
}

/** 根据拖拽像素值 + 视口约束生成 CSS width 值 */
export function agentDockClampedWidth(px: number): string {
  const maxPx = Math.floor(window.innerWidth * AGENT_DOCK_MAX_VW_RATIO)
  const clamped = Math.max(AGENT_DOCK_MIN_WIDTH_PX, Math.min(px, maxPx))
  return `${clamped}px`
}
