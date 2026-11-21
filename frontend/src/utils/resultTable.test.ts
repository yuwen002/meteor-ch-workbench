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
  visibleColumnIndexes,
  hiddenColumnIndexes,
  pickCellIndexes,
  gridWidthsFor,
  colWidthAt,
  clampColWidth,
  resizeColWidths,
} from './resultTable'

describe('visibleColumnIndexes', () => {
  it('无隐藏时返回全部索引', () => {
    expect(visibleColumnIndexes(4, [])).toEqual([0, 1, 2, 3])
  })

  it('跳过隐藏列且保持原始顺序', () => {
    expect(visibleColumnIndexes(5, [1, 3])).toEqual([0, 2, 4])
  })

  it('零列返回空数组', () => {
    expect(visibleColumnIndexes(0, [])).toEqual([])
  })

  it('隐藏索引越界时忽略', () => {
    expect(visibleColumnIndexes(3, [1, 9, -1])).toEqual([0, 2])
  })
})

describe('hiddenColumnIndexes', () => {
  it('去重并升序', () => {
    expect(hiddenColumnIndexes(5, [3, 1, 3])).toEqual([1, 3])
  })

  it('过滤越界与小数', () => {
    expect(hiddenColumnIndexes(4, [-1, 1.5, 4, 2])).toEqual([2])
  })

  it('空输入返回空数组', () => {
    expect(hiddenColumnIndexes(3, [])).toEqual([])
  })
})

describe('pickCellIndexes', () => {
  it('按可见列裁剪行数据', () => {
    expect(pickCellIndexes(['a', 'b', 'c'], [0, 2])).toEqual(['a', 'c'])
  })

  it('空索引返回空数组', () => {
    expect(pickCellIndexes(['a'], [])).toEqual([])
  })
})

describe('gridWidthsFor', () => {
  it('按索引取列宽', () => {
    expect(gridWidthsFor([0, 2], [100, 200, 300])).toEqual([100, 300])
  })

  it('索引越界时给出 undefined 而非崩溃', () => {
    expect(gridWidthsFor([5], [100])).toEqual([undefined])
  })
})

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

describe('colWidthAt', () => {
  it('返回对应列宽', () => {
    expect(colWidthAt([100, 200], 1)).toBe(200)
  })

  it('缺列时回落到最小宽度', () => {
    expect(colWidthAt([100], 2)).toBe(MIN_COL_WIDTH)
  })

  it('非法值回落到最小宽度', () => {
    expect(colWidthAt([Number.NaN], 0)).toBe(MIN_COL_WIDTH)
  })
})

describe('clampColWidth', () => {
  it('普通数值四舍五入', () => {
    expect(clampColWidth(123.6)).toBe(124)
  })

  it('小于最小值时抬到最小值', () => {
    expect(clampColWidth(10)).toBe(MIN_COL_WIDTH)
  })

  it('大于最大值时压到最大值', () => {
    expect(clampColWidth(9999)).toBe(MAX_COL_WIDTH)
  })

  it('非法数值返回最小值', () => {
    expect(clampColWidth(Number.NaN)).toBe(MIN_COL_WIDTH)
  })

  it('Infinity 返回最大值', () => {
    expect(clampColWidth(Infinity)).toBe(MAX_COL_WIDTH)
  })
})

describe('resizeColWidths', () => {
  it('向右拖动增大宽度', () => {
    expect(resizeColWidths([100, 200], [0], 100, 40)).toEqual([140, 200])
  })

  it('向左拖动减小宽度', () => {
    expect(resizeColWidths([100, 200], [1], 200, -60)).toEqual([100, 140])
  })

  it('拖动到负数被夹到最小宽度', () => {
    expect(resizeColWidths([100], [0], 100, -500)).toEqual([MIN_COL_WIDTH])
  })

  it('多列同时设置相同宽度', () => {
    expect(resizeColWidths([100, 100, 100], [0, 2], 100, 30)).toEqual([130, 100, 130])
  })

  it('空索引时原样返回', () => {
    const widths = [100, 200]
    expect(resizeColWidths(widths, [], 100, 50)).toBe(widths)
  })

  it('原数组不被修改', () => {
    const widths = [100, 200]
    resizeColWidths(widths, [0], 100, 40)
    expect(widths).toEqual([100, 200])
  })

  it('索引超出当前宽度数组时自动补齐', () => {
    expect(resizeColWidths([100], [3], 100, 20)).toEqual([100, MIN_COL_WIDTH, MIN_COL_WIDTH, 120])
  })
})