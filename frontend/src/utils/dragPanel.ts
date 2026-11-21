export interface PanelPoint {
  x: number
  y: number
}

export interface PanelSize {
  width: number
  height: number
}

export interface PanelViewport {
  width: number
  height: number
}

export const PANEL_MARGIN = 8

export function clampPanelPosition(
  point: PanelPoint,
  size: PanelSize,
  viewport: PanelViewport,
  margin: number = PANEL_MARGIN,
): PanelPoint {
  const maxX = Math.max(margin, viewport.width - size.width - margin)
  const maxY = Math.max(margin, viewport.height - size.height - margin)
  return {
    x: Math.min(Math.max(point.x, margin), maxX),
    y: Math.min(Math.max(point.y, margin), maxY),
  }
}

export function dragPanelPosition(
  origin: PanelPoint,
  startCursor: PanelPoint,
  currentCursor: PanelPoint,
  size: PanelSize,
  viewport: PanelViewport,
  margin: number = PANEL_MARGIN,
): PanelPoint {
  const next = {
    x: origin.x + (currentCursor.x - startCursor.x),
    y: origin.y + (currentCursor.y - startCursor.y),
  }
  return clampPanelPosition(next, size, viewport, margin)
}