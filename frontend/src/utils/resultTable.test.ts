import { describe, it, expect } from 'vitest'
import type { ColumnInfo } from '../types'
import {
  ROW_HEIGHT,
  VIRTUAL_THRESHOLD,
  MIN_COL_WIDTH,
  MAX_COL_WIDTH,
  measureColWidths,
  buildGridStyle,
  shouldVirtualize,
} from './resultTable'

describe('measureColWidths', () => {
  it('空列返回空数组', () => {
    expect(measureColWidths([], [])).toEqual([])
  })

  it('按列名与内容取最大宽度', () => {
    const columns: ColumnInfo[] = [
      { name: 'id', type: 'UInt64' },
      { name: 'name', type: 'String' },
    ]
    const rows: unknown[][] = [
      [1, 'a'],
      [22, 'much longer value'],
    ]
    const widths = measureColWidths(columns, rows)
    expect(widths.length).toBe(2)
    expect(widths[0]).toBeGreaterThanOrEqual(MIN_COL_WIDTH)
    expect(widths[1]).toBeGreaterThan(widths[0])
  })

  it('超长内容被限制在最大宽度', () => {
    const columns: ColumnInfo[] = [{ name: 'v', type: 'String' }]
    const rows: unknown[][] = [['x'.repeat(1000)]]
    expect(measureColWidths(columns, rows)[0]).toBe(MAX_COL_WIDTH)
  })

  it('极短内容不低于最小宽度', () => {
    const columns: ColumnInfo[] = [{ name: 'v', type: 'String' }]
    expect(measureColWidths(columns, [[1]])[0]).toBe(MIN_COL_WIDTH)
  })

  it('null 与 undefined 按占位宽度计算', () => {
    const columns: ColumnInfo[] = [{ name: 'v', type: 'String' }]
    expect(measureColWidths(columns, [[null], [undefined]])[0]).toBe(MIN_COL_WIDTH)
  })
})

describe('buildGridStyle', () => {
  it('列宽数量与列数一致时使用固定列宽', () => {
    expect(buildGridStyle([100, 200], 2)).toBe('grid-template-columns: 100px 200px;')
  })
  it('列宽数量不匹配时回退为自适应列', () => {
    expect(buildGridStyle([100], 2)).toBe('grid-template-columns: minmax(120px, 1fr) minmax(120px, 1fr);')
  })
  it('列数为零时返回空模板', () => {
    expect(buildGridStyle([], 0)).toBe('grid-template-columns: ;')
  })
})

describe('shouldVirtualize', () => {
  it('等于阈值不虚拟化', () => {
    expect(shouldVirtualize(VIRTUAL_THRESHOLD)).toBe(false)
  })
  it('超过阈值一行的边界虚拟化', () => {
    expect(shouldVirtualize(VIRTUAL_THRESHOLD + 1)).toBe(true)
  })
  it('零行不虚拟化', () => {
    expect(shouldVirtualize(0)).toBe(false)
  })
})

describe('常量', () => {
  it('行高固定为 28px', () => {
    expect(ROW_HEIGHT).toBe(28)
  })
})