export const SPLIT_MIN_TOP = 120
export const SPLIT_MIN_BOTTOM = 120

export function clampSplitRatio(ratio: number, containerHeight: number): number {
  if (!Number.isFinite(ratio) || containerHeight <= 0) return 0.5
  const top = ratio * containerHeight
  const maxTop = containerHeight - SPLIT_MIN_BOTTOM
  const minTop = SPLIT_MIN_TOP
  if (maxTop <= minTop) return minTop / containerHeight
  const clampedTop = Math.min(maxTop, Math.max(minTop, top))
  return clampedTop / containerHeight
}

export function splitRatioFromY(startY: number, startRatio: number, deltaY: number, containerHeight: number): number {
  if (containerHeight <= 0) return startRatio
  return clampSplitRatio(startRatio + deltaY / containerHeight, containerHeight)
}

export function splitTopHeight(ratio: number, containerHeight: number): number {
  if (containerHeight <= 0) return 0
  return Math.round(clampSplitRatio(ratio, containerHeight) * containerHeight)
}

export const PANE_MIN_WIDTH = 140
export const PANE_MAX_WIDTH = 520

export function clampPaneWidth(width: number, maxWidth: number): number {
  if (!Number.isFinite(width)) return PANE_MIN_WIDTH
  const upper = Math.max(PANE_MIN_WIDTH, Math.min(PANE_MAX_WIDTH, maxWidth))
  return Math.round(Math.min(upper, Math.max(PANE_MIN_WIDTH, width)))
}

export function paneWidthFromDelta(startWidth: number, deltaX: number, maxWidth: number): number {
  return clampPaneWidth(startWidth + deltaX, maxWidth)
}