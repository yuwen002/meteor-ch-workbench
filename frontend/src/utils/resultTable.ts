import type { ColumnInfo } from '../types'

export const ROW_HEIGHT = 28
export const VIRTUAL_THRESHOLD = 200
export const CELL_PADDING_X = 24
export const MIN_COL_WIDTH = 80
export const MAX_COL_WIDTH = 420
export const COL_WIDTH_SAMPLE_ROWS = 200
const CHAR_WIDTH = 7
const NULL_WIDTH = 4

export function measureColWidths(columns: ColumnInfo[], rows: unknown[][]): number[] {
  if (columns.length === 0) return []
  const sample = rows.slice(0, COL_WIDTH_SAMPLE_ROWS)
  return columns.map((col, ci) => {
    let max = col.name.length
    for (const row of sample) {
      const v = row[ci]
      const len = v === null || v === undefined ? NULL_WIDTH : String(v).length
      if (len > max) max = len
    }
    const width = max * CHAR_WIDTH + CELL_PADDING_X
    return Math.min(MAX_COL_WIDTH, Math.max(MIN_COL_WIDTH, width))
  })
}

export function buildGridStyle(colWidths: number[], columnCount: number): string {
  const cols =
    colWidths.length === columnCount && columnCount > 0
      ? colWidths.map((w) => w + 'px').join(' ')
      : new Array(columnCount).fill('minmax(120px, 1fr)').join(' ')
  return `grid-template-columns: ${cols};`
}

export function shouldVirtualize(rowCount: number): boolean {
  return rowCount > VIRTUAL_THRESHOLD
}