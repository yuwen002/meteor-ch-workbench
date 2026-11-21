export const MENU_MARGIN = 8

export interface MenuViewport {
  width: number
  height: number
}

export interface MenuPoint {
  x: number
  y: number
}

export function clampMenuPosition(
  x: number,
  y: number,
  menuWidth: number,
  menuHeight: number,
  viewport: MenuViewport,
  margin = MENU_MARGIN,
): MenuPoint {
  const maxX = Math.max(margin, viewport.width - menuWidth - margin)
  const maxY = Math.max(margin, viewport.height - menuHeight - margin)
  return {
    x: Math.min(Math.max(x, margin), maxX),
    y: Math.min(Math.max(y, margin), maxY),
  }
}