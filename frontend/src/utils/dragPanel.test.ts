import { describe, it, expect } from 'vitest'
import { clampPanelPosition, dragPanelPosition, PANEL_MARGIN } from './dragPanel'

const viewport = { width: 1000, height: 800 }
const size = { width: 380, height: 360 }

describe('clampPanelPosition', () => {
  it('范围内位置保持不变', () => {
    expect(clampPanelPosition({ x: 100, y: 100 }, size, viewport)).toEqual({ x: 100, y: 100 })
  })

  it('超出左边界时贴住边距', () => {
    const p = clampPanelPosition({ x: -50, y: 0 }, size, viewport)
    expect(p.x).toBe(PANEL_MARGIN)
    expect(p.y).toBe(PANEL_MARGIN)
  })

  it('超出右边界时被拉回', () => {
    const p = clampPanelPosition({ x: 900, y: 100 }, size, viewport)
    expect(p.x).toBe(viewport.width - size.width - PANEL_MARGIN)
  })

  it('超出下边界时被拉回', () => {
    const p = clampPanelPosition({ x: 100, y: 700 }, size, viewport)
    expect(p.y).toBe(viewport.height - size.height - PANEL_MARGIN)
  })

  it('视口小于面板时退化为边距', () => {
    const p = clampPanelPosition({ x: 200, y: 200 }, { width: 900, height: 900 }, { width: 500, height: 500 })
    expect(p).toEqual({ x: PANEL_MARGIN, y: PANEL_MARGIN })
  })
})

describe('dragPanelPosition', () => {
  it('按指针位移平移原点', () => {
    const p = dragPanelPosition({ x: 100, y: 100 }, { x: 200, y: 200 }, { x: 260, y: 240 }, size, viewport)
    expect(p).toEqual({ x: 160, y: 140 })
  })

  it('拖动时仍受视口约束', () => {
    const p = dragPanelPosition({ x: 100, y: 100 }, { x: 200, y: 200 }, { x: 900, y: 900 }, size, viewport)
    expect(p.x).toBe(viewport.width - size.width - PANEL_MARGIN)
    expect(p.y).toBe(viewport.height - size.height - PANEL_MARGIN)
  })

  it('反向拖动可回到左上边界', () => {
    const p = dragPanelPosition({ x: 300, y: 300 }, { x: 300, y: 300 }, { x: -400, y: -400 }, size, viewport)
    expect(p).toEqual({ x: PANEL_MARGIN, y: PANEL_MARGIN })
  })
})