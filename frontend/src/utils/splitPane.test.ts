import { describe, it, expect } from 'vitest'
import {
  clampSplitRatio,
  splitRatioFromY,
  splitTopHeight,
  clampPaneWidth,
  paneWidthFromDelta,
  SPLIT_MIN_TOP,
  SPLIT_MIN_BOTTOM,
  PANE_MIN_WIDTH,
  PANE_MAX_WIDTH,
} from './splitPane'

describe('clampSplitRatio', () => {
  it('容器高度为 0 时返回默认 0.5', () => {
    expect(clampSplitRatio(0.9, 0)).toBe(0.5)
  })

  it('非法比例返回默认 0.5', () => {
    expect(clampSplitRatio(Number.NaN, 600)).toBe(0.5)
  })

  it('正常比例原样返回', () => {
    expect(clampSplitRatio(0.5, 600)).toBeCloseTo(0.5, 5)
  })

  it('超出上界时压到留出底部最小高度', () => {
    const ratio = clampSplitRatio(0.98, 600)
    expect(ratio * 600).toBeCloseTo(600 - SPLIT_MIN_BOTTOM, 5)
  })

  it('超出下界时抬到顶部最小高度', () => {
    const ratio = clampSplitRatio(0.01, 600)
    expect(ratio * 600).toBeCloseTo(SPLIT_MIN_TOP, 5)
  })

  it('容器过矮时退化为顶部最小高度', () => {
    const ratio = clampSplitRatio(0.5, 200)
    expect(ratio * 200).toBeCloseTo(SPLIT_MIN_TOP, 5)
  })
})

describe('splitRatioFromY', () => {
  it('向下拖动增大顶部占比', () => {
    expect(splitRatioFromY(300, 0.5, 60, 600)).toBeCloseTo(0.6, 5)
  })

  it('向上拖动减小顶部占比', () => {
    expect(splitRatioFromY(300, 0.5, -60, 600)).toBeCloseTo(0.4, 5)
  })

  it('拖动超过边界时被夹住', () => {
    const ratio = splitRatioFromY(300, 0.5, 1000, 600)
    expect(ratio * 600).toBeCloseTo(600 - SPLIT_MIN_BOTTOM, 5)
  })
})

describe('splitTopHeight', () => {
  it('按比例换算像素高度', () => {
    expect(splitTopHeight(0.5, 600)).toBe(300)
  })

  it('容器高度为 0 时返回 0', () => {
    expect(splitTopHeight(0.5, 0)).toBe(0)
  })

  it('结果受最小高度约束', () => {
    expect(splitTopHeight(0.1, 600)).toBe(SPLIT_MIN_TOP)
  })
})

describe('clampPaneWidth', () => {
  it('普通宽度四舍五入', () => {
    expect(clampPaneWidth(223.6, 9999)).toBe(224)
  })

  it('小于最小值时抬到最小值', () => {
    expect(clampPaneWidth(10, 9999)).toBe(PANE_MIN_WIDTH)
  })

  it('大于最大宽度时压到最大值', () => {
    expect(clampPaneWidth(9999, 9999)).toBe(PANE_MAX_WIDTH)
  })

  it('可用空间不足时跟随可用空间', () => {
    expect(clampPaneWidth(400, 200)).toBe(200)
  })

  it('可用空间小于最小值时仍保证最小值', () => {
    expect(clampPaneWidth(400, 100)).toBe(PANE_MIN_WIDTH)
  })

  it('非法数值返回最小值', () => {
    expect(clampPaneWidth(Number.NaN, 9999)).toBe(PANE_MIN_WIDTH)
  })
})

describe('paneWidthFromDelta', () => {
  it('向右拖动变宽', () => {
    expect(paneWidthFromDelta(224, 60, 9999)).toBe(284)
  })

  it('向左拖动变窄并在最小值处停住', () => {
    expect(paneWidthFromDelta(224, -200, 9999)).toBe(PANE_MIN_WIDTH)
  })

  it('拖动过程中受可用空间限制', () => {
    expect(paneWidthFromDelta(224, 600, 400)).toBe(400)
  })
})