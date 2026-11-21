import { describe, expect, it } from 'vitest'
import { clampMenuPosition, MENU_MARGIN, type MenuPoint } from './menuPosition'

describe('clampMenuPosition', () => {
  const viewport = { width: 1024, height: 768 }
  const cases: Array<{
    name: string
    x: number
    y: number
    w: number
    h: number
    want: MenuPoint
  }> = [
    { name: '正常坐标原样返回', x: 200, y: 150, w: 220, h: 320, want: { x: 200, y: 150 } },
    { name: '贴左 / 贴上边时压到 margin', x: 0, y: 0, w: 220, h: 320, want: { x: MENU_MARGIN, y: MENU_MARGIN } },
    { name: '负坐标压到 margin', x: -50, y: -20, w: 220, h: 320, want: { x: MENU_MARGIN, y: MENU_MARGIN } },
    { name: '右侧越界时向左收缩', x: 1000, y: 100, w: 220, h: 320, want: { x: 1024 - 220 - MENU_MARGIN, y: 100 } },
    { name: '底部越界时向上收缩', x: 100, y: 700, w: 220, h: 320, want: { x: 100, y: 768 - 320 - MENU_MARGIN } },
    {
      name: '菜单超宽时退化为左侧 margin',
      x: 500,
      y: 500,
      w: 2000,
      h: 2000,
      want: { x: MENU_MARGIN, y: MENU_MARGIN },
    },
    { name: '恰好边界不越界', x: 1024 - 220 - MENU_MARGIN, y: 768 - 320 - MENU_MARGIN, w: 220, h: 320, want: { x: 1024 - 220 - MENU_MARGIN, y: 768 - 320 - MENU_MARGIN } },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(clampMenuPosition(c.x, c.y, c.w, c.h, viewport)).toEqual(c.want)
    })
  }
})