import { escapeSqlString, quoteIdent, sqlLiteral } from './sqlLiteral'

export interface ValueTarget {
  column: string
  value: unknown
}

export interface SortCondition {
  column: string
  dir: 'ASC' | 'DESC'
}

export const FILTER_EQ = 'eq'
export const FILTER_NE = 'ne'
export const FILTER_GT = 'gt'
export const FILTER_GE = 'ge'
export const FILTER_LT = 'lt'
export const FILTER_LE = 'le'
export const FILTER_LIKE = 'like'
export const FILTER_NOT_LIKE = 'not_like'

export type FilterOp =
  | typeof FILTER_EQ
  | typeof FILTER_NE
  | typeof FILTER_GT
  | typeof FILTER_GE
  | typeof FILTER_LT
  | typeof FILTER_LE
  | typeof FILTER_LIKE
  | typeof FILTER_NOT_LIKE

const OP_SQL: Record<FilterOp, string> = {
  [FILTER_EQ]: '=',
  [FILTER_NE]: '<>',
  [FILTER_GT]: '>',
  [FILTER_GE]: '>=',
  [FILTER_LT]: '<',
  [FILTER_LE]: '<=',
  [FILTER_LIKE]: 'LIKE',
  [FILTER_NOT_LIKE]: 'NOT LIKE',
}

export const FILTER_OP_ORDER: FilterOp[] = [
  FILTER_EQ,
  FILTER_NE,
  FILTER_GT,
  FILTER_GE,
  FILTER_LT,
  FILTER_LE,
  FILTER_LIKE,
  FILTER_NOT_LIKE,
]

export const PREVIEW_MAX_LENGTH = 18

export function isNullValue(v: unknown): boolean {
  return v === null || v === undefined
}

export function valuePreview(v: unknown, maxLength = PREVIEW_MAX_LENGTH): string {
  if (isNullValue(v)) return 'NULL'
  const text = String(v)
  if (text.length <= maxLength) return text
  const head = Math.max(1, Math.floor((maxLength - 3) / 2))
  const tail = Math.max(1, maxLength - 3 - head)
  return text.slice(0, head) + '...' + text.slice(text.length - tail)
}

export function opSymbol(op: FilterOp): string {
  if (op === FILTER_NOT_LIKE) return 'NOT LIKE'
  return OP_SQL[op]
}

export function filterExpression(
  column: string,
  op: FilterOp,
  value: unknown,
  percent = false
): string {
  const col = quoteColumn(column)
  if (isNullValue(value)) {
    return op === FILTER_NE ? `${col} IS NOT NULL` : `${col} IS NULL`
  }
  if (percent) {
    return `${col} ${opSymbol(op)} ${escapeSqlString('%' + String(value) + '%')}`
  }
  if (op === FILTER_LIKE || op === FILTER_NOT_LIKE) {
    const text = String(value)
    const pattern = text.includes('%') || text.includes('_') ? text : '%' + text + '%'
    return `${col} ${opSymbol(op)} ${escapeSqlString(pattern)}`
  }
  return `${col} ${opSymbol(op)} ${sqlLiteral(value)}`
}

export function conditionPreview(column: string, op: FilterOp, value: unknown): string {
  return `${column} ${opSymbol(op)} ${valuePreview(value)}`
}

export function quoteColumn(column: string): string {
  return quoteIdent(column)
}

export function isConditionable(statement: string): boolean {
  const trimmed = statement.trim().toUpperCase()
  if (!trimmed) return false
  if (trimmed.startsWith('WITH')) return true
  return trimmed.startsWith('SELECT')
}

function baseStatement(sql: string): string {
  const trimmed = sql.trim()
  if (!trimmed) return ''
  const noSemicolon = trimmed.replace(/;\s*$/, '').trim()
  if (noSemicolon.includes(';')) return ''
  const core = noSemicolon
    .replace(/\/\*[\s\S]*?\*\//g, ' ')
    .replace(/--[^\n]*/g, '\n')
    .replace(/#[^\n]*/g, '\n')
    .trim()
  if (!isConditionable(core)) return ''
  return noSemicolon
}

function takeLeadingComment(body: string): { comment: string; rest: string } {
  const m = body.match(/^(\s*(?:\/\*[\s\S]*?\*\/\s*|--[^\n]*\n\s*|#[^\n]*\n\s*)*)/)
  const comment = m ? m[1] : ''
  return { comment, rest: body.slice(comment.length) }
}

const TRAILING_CLAUSES = ['FORMAT', 'SETTINGS', 'INTO OUTFILE']

function clauseIndexOf(rest: string, clauses: string[]): number {
  const upper = rest.toUpperCase()
  let best = -1
  for (const clause of clauses) {
    const re = new RegExp(`\\b${clause.replace(/\s+/g, '\\s+')}\\b`)
    const m = re.exec(upper)
    if (m && (best === -1 || m.index < best)) best = m.index
  }
  return best
}

function joinClauses(comment: string, head: string, suffix: string): string {
  const trimmedHead = head.trimEnd()
  const trimmedSuffix = suffix.trim()
  if (!trimmedSuffix) return comment + trimmedHead
  return comment + trimmedHead + '\n' + trimmedSuffix
}

export function appendSort(sql: string, sorts: SortCondition[]): string {
  const body = baseStatement(sql)
  if (!body) return ''
  if (sorts.length === 0) return body

  const { comment, rest } = takeLeadingComment(body)
  const limitIdx = clauseIndexOf(rest, ['LIMIT'])
  const formatIdx = clauseIndexOf(rest, TRAILING_CLAUSES)

  let coreEnd = rest.length
  if (limitIdx !== -1 && limitIdx < coreEnd) coreEnd = limitIdx
  if (formatIdx !== -1 && formatIdx < coreEnd) coreEnd = formatIdx
  const core = rest.slice(0, coreEnd)
  const orderInCore = clauseIndexOf(core, ['ORDER BY'])
  const head = orderInCore === -1 ? core : core.slice(0, orderInCore)
  const tailPart = rest.slice(coreEnd)

  const orderBy = 'ORDER BY ' + sorts.map((s) => `${quoteColumn(s.column)} ${s.dir}`).join(', ')
  return joinClauses(comment, head, orderBy + (tailPart ? '\n' + tailPart.trim() : ''))
}

export function appendWhere(sql: string, conditions: string[]): string {
  const body = baseStatement(sql)
  if (!body) return ''
  if (conditions.length === 0) return body

  const { comment, rest } = takeLeadingComment(body)
  const tail = clauseIndexOf(rest, ['GROUP BY', 'ORDER BY', 'LIMIT', 'FORMAT', 'SETTINGS'])
  const head = tail === -1 ? rest : rest.slice(0, tail)
  const suffix = tail === -1 ? '' : rest.slice(tail)

  const wherePart = conditions.join(' AND ')
  const hasWhere = /\bWHERE\b/i.test(head)
  const merged = hasWhere
    ? `${head.trimEnd()} AND ${wherePart}`
    : `${head.trimEnd()}\nWHERE ${wherePart}`

  return joinClauses(comment, merged, suffix)
}

export const FILTER_JOIN_AND = 'AND'
export const FILTER_JOIN_OR = 'OR'

export type FilterJoin = typeof FILTER_JOIN_AND | typeof FILTER_JOIN_OR

export interface FilterRule {
  op: FilterOp
  value: string
  numeric?: boolean
}

export interface FilterCondition {
  column: string
  join: FilterJoin
  rules: FilterRule[]
}

export const FILTER_JOIN_ORDER: FilterJoin[] = [FILTER_JOIN_AND, FILTER_JOIN_OR]

const NUMERIC_TYPE_RE = /^(?:Nullable\()?(?:U?Int|Float|Decimal)/i

export function isNumericType(type: string | undefined | null): boolean {
  if (!type) return false
  return NUMERIC_TYPE_RE.test(type.trim())
}

function likePattern(value: string): string {
  const text = value.trim()
  if (text.includes('%') || text.includes('_')) return text
  return '%' + text + '%'
}

export function rawFilterExpression(
  column: string,
  op: FilterOp,
  value: string,
  numeric = false
): string {
  const col = quoteColumn(column)
  if (op === FILTER_LIKE || op === FILTER_NOT_LIKE) {
    return `${col} ${opSymbol(op)} ${escapeSqlString(likePattern(value))}`
  }
  const trimmed = value.trim()
  const literal = numeric && isNumericLiteral(trimmed)
    ? trimmed
    : escapeSqlString(trimmed)
  return `${col} ${opSymbol(op)} ${literal}`
}

function isNumericLiteral(text: string): boolean {
  return /^[-+]?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?$/.test(text)
}

export function conditionExpression(condition: FilterCondition): string {
  const parts = condition.rules.filter((r) => r.value.trim() !== '')
  if (parts.length === 0) return ''
  const group = parts.length === 1
    ? rawFilterExpression(condition.column, parts[0].op, parts[0].value, parts[0].numeric)
    : '(' +
      parts
        .map((r) => rawFilterExpression(condition.column, r.op, r.value, r.numeric))
        .join(' OR ') +
      ')'
  return group
}

export function buildFilterWhere(conditions: FilterCondition[]): string {
  const groups = conditions
    .map((c) => ({ join: c.join, expr: conditionExpression(c) }))
    .filter((g) => g.expr !== '')
  if (groups.length === 0) return ''
  return groups.map((g, i) => (i === 0 ? g.expr : `${g.join} ${g.expr}`)).join(' ')
}

export function countActiveRules(conditions: FilterCondition[]): number {
  return conditions.reduce(
    (sum, c) => sum + c.rules.filter((r) => r.value.trim() !== '').length,
    0
  )
}