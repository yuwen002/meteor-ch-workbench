import { quoteIdent, sqlLiteral } from './sqlLiteral'

export interface SourceRef {
  database: string | null
  table: string
}

export interface EditTarget {
  column: string
  oldValue: unknown
  newValue: string
}

export interface KeyColumn {
  name: string
  value: unknown
}

export const NO_SOURCE_HINT = '结果集无法定位源表（存在 JOIN / UNION / 子查询），不允许直接编辑'

function stripComments(sql: string): string {
  return sql
    .replace(/\/\*[\s\S]*?\*\//g, ' ')
    .replace(/--[^\n]*/g, ' ')
    .replace(/#[^\n]*/g, ' ')
}

function stripTrailing(sql: string): string {
  return sql.replace(/^[\s;]+|[\s;]+$/g, '')
}

function splitQualified(name: string): { database: string | null; table: string } | null {
  const parts = name.split('.').map((p) => p.trim()).filter(Boolean)
  if (parts.length === 0 || parts.length > 2) return null
  const unquote = (s: string) => s.replace(/^[`"']+|[`"']+$/g, '')
  if (parts.length === 1) {
    const table = unquote(parts[0])
    return table ? { database: null, table } : null
  }
  const database = unquote(parts[0])
  const table = unquote(parts[1])
  if (!database || !table) return null
  return { database, table }
}

export function parseQuerySource(sql: string): SourceRef | null {
  if (!sql) return null
  const body = stripComments(sql)
  if (/\bJOIN\b/i.test(body)) return null
  if (/\bUNION\b/i.test(body)) return null

  const match = body.match(/\b(?:FROM|UPDATE)\s+([A-Za-z0-9_`"'.]+)\s*([(]?)/i)
  if (!match) return null
  if (match[2] === '(') return null

  const candidate = match[1]
  if (/^(?:numbers|system|remote|cluster)(?:$|[(.])/i.test(candidate)) return null
  if (!candidate) return null

  return splitQualified(candidate)
}

export function canEditCell(source: SourceRef | null, keyColumns: KeyColumn[]): boolean {
  if (!source) return false
  if (!source.table) return false
  if (keyColumns.length === 0) return false
  return keyColumns.every((k) => k.value !== null && k.value !== undefined)
}

export function buildKeyCondition(keyColumns: KeyColumn[], columnType?: (name: string) => string | null): string {
  if (keyColumns.length === 0) return ''
  return keyColumns
    .map((k) => {
      const type = columnType ? columnType(k.name) : null
      const literal = isNumericType(type) && typeof k.value === 'string' ? numericLiteral(k.value) : sqlLiteral(k.value)
      return `${quoteIdent(k.name)} = ${literal}`
    })
    .join(' AND ')
}

function isNumericType(type: string | null): boolean {
  if (!type) return false
  return /^(?:U?Int(?:8|16|32|64|128|256)|Float32|Float64|Decimal|Bool)/i.test(type.trim())
}

function numericLiteral(raw: string): string {
  const t = raw.trim()
  return /^-?\d+(?:\.\d+)?$/.test(t) ? t : sqlLiteral(t)
}

export function buildAlterUpdateSQL(
  source: SourceRef | null,
  target: EditTarget,
  keyColumns: KeyColumn[],
  columnType?: (name: string) => string | null,
): string {
  if (!source) return ''
  if (keyColumns.length === 0) return ''
  const tableRef = source.database
    ? `${quoteIdent(source.database)}.${quoteIdent(source.table)}`
    : quoteIdent(source.table)
  const type = columnType ? columnType(target.column) : null
  const value = isNumericType(type) ? numericLiteral(target.newValue) : sqlLiteral(target.newValue)
  const where = buildKeyCondition(keyColumns, columnType)
  return `ALTER TABLE ${tableRef}\nUPDATE ${quoteIdent(target.column)} = ${value}\nWHERE ${where};`
}

export function valueForEdit(raw: unknown): string {
  if (raw === null || raw === undefined) return ''
  return String(raw)
}

export interface AutoKeyInput {
  columns: Array<{ name: string; is_primary?: boolean }>
  rows: unknown[][]
}

export interface AutoKeyResult {
  indexes: number[]
  reason: 'primary' | 'unique' | 'all'
}

export function detectKeyColumns(input: AutoKeyInput): AutoKeyResult {
  const { columns, rows } = input
  if (columns.length === 0) return { indexes: [], reason: 'all' }

  const primaries = columns.map((c, i) => (c.is_primary ? i : -1)).filter((i) => i >= 0)
  if (primaries.length > 0) return { indexes: primaries, reason: 'primary' }

  for (let ci = 0; ci < columns.length; ci++) {
    const seen = new Set<string>()
    let ok = rows.length > 0
    for (const row of rows) {
      const v = row[ci]
      if (v === null || v === undefined) {
        ok = false
        break
      }
      seen.add(String(v))
    }
    if (ok && seen.size === rows.length) return { indexes: [ci], reason: 'unique' }
  }

  return { indexes: columns.map((_, i) => i), reason: 'all' }
}

export function buildRowKeys(indexes: number[], columns: Array<{ name: string }>, row: unknown[]): KeyColumn[] {
  return indexes
    .filter((i) => i >= 0 && i < columns.length)
    .map((i) => ({ name: columns[i].name, value: row[i] }))
}