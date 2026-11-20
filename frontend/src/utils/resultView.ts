import type { ColumnInfo } from '../types'

export type CellValue = unknown

export function isEmptyCell(v: CellValue): boolean {
  return v === null || v === undefined
}

export function collectColumnValues(rows: CellValue[][], indices: number[], column: number): string[] {
  const out: string[] = []
  for (const ri of indices) {
    const v = rows[ri]?.[column]
    if (isEmptyCell(v)) continue
    out.push(String(v))
  }
  return out
}

export function distinctColumnValues(rows: CellValue[][], indices: number[], column: number): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const v of collectColumnValues(rows, indices, column)) {
    if (seen.has(v)) continue
    seen.add(v)
    out.push(v)
  }
  return out
}

export function columnValuesText(rows: CellValue[][], indices: number[], column: number): string {
  return distinctColumnValues(rows, indices, column).join('\n')
}

export function columnListText(columns: ColumnInfo[], index: number): string {
  return columns[index]?.name ?? ''
}

export function cellText(v: CellValue): string {
  return isEmptyCell(v) ? '' : String(v)
}

export function rowMatches(row: CellValue[], keyword: string, column = -1): boolean {
  const needle = keyword.trim().toLowerCase()
  if (!needle) return true
  if (column >= 0) {
    return cellText(row[column]).toLowerCase().includes(needle)
  }
  for (const v of row) {
    if (cellText(v).toLowerCase().includes(needle)) return true
  }
  return false
}

export function filterRowIndices(rows: CellValue[][], keyword: string, column = -1): number[] {
  const needle = keyword.trim()
  if (!needle) {
    const all: number[] = []
    for (let i = 0; i < rows.length; i += 1) all.push(i)
    return all
  }
  const out: number[] = []
  for (let i = 0; i < rows.length; i += 1) {
    if (rowMatches(rows[i] ?? [], needle, column)) out.push(i)
  }
  return out
}