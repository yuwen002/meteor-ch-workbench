export interface PageState {
  offset: number
  pageSize: number
}

export const DEFAULT_PAGE_SIZE = 50000

const NOT_PAGEABLE_PREFIXES = [
  'EXPLAIN',
  'INSERT',
  'CREATE',
  'ALTER',
  'DROP',
  'TRUNCATE',
  'RENAME',
  'OPTIMIZE',
  'SYSTEM',
  'USE',
  'SET',
  'ATTACH',
  'DETACH',
  'KILL',
  'GRANT',
  'REVOKE',
  'SHOW',
  'DESCRIBE',
  'DESC',
  'EXISTS',
]

function stripComments(sql: string): string {
  return sql
    .replace(/\/\*[\s\S]*?\*\//g, ' ')
    .replace(/--[^\n]*/g, ' ')
    .replace(/#[^\n]*/g, ' ')
    .trim()
}

function stripTrailingSemicolon(sql: string): string {
  return sql.replace(/;\s*$/, '').trim()
}

export function splitStatements(sql: string): string[] {
  return stripComments(sql)
    .split(';')
    .map((s) => s.trim())
    .filter((s) => s.length > 0)
}

export function headKeyword(statement: string): string {
  const cleaned = stripComments(statement).replace(/\s+/g, ' ').trim()
  const withParen = cleaned.match(/^([A-Za-z_]+)\s*\(/)
  if (withParen) return withParen[1].toUpperCase()
  const first = cleaned.split(' ')[0] ?? ''
  return first.toUpperCase()
}

export function isSingleSelect(sql: string): boolean {
  const statements = splitStatements(sql)
  if (statements.length !== 1) return false
  const head = headKeyword(statements[0])
  if (!head) return false
  if (NOT_PAGEABLE_PREFIXES.includes(head)) return false
  return head === 'SELECT' || head === 'WITH'
}

export function buildPageSQL(sql: string, offset: number, pageSize: number): string {
  const statements = splitStatements(sql)
  const body = stripTrailingSemicolon(statements[0] ?? '')
  if (!body) return ''
  const limit = Math.max(1, Math.floor(pageSize))
  const off = Math.max(0, Math.floor(offset))
  return `${body} LIMIT ${limit} OFFSET ${off}`
}

export function nextPage(state: PageState): PageState {
  return { offset: state.offset + state.pageSize, pageSize: state.pageSize }
}

export function prevPage(state: PageState): PageState {
  return { offset: Math.max(0, state.offset - state.pageSize), pageSize: state.pageSize }
}

export function currentPageNumber(state: PageState): number {
  if (state.pageSize <= 0) return 1
  return Math.floor(state.offset / state.pageSize) + 1
}

export function hasPrevPage(state: PageState): boolean {
  return state.offset > 0
}

export function hasNextPage(state: PageState, rowCount: number): boolean {
  if (rowCount <= 0) return false
  return rowCount >= state.pageSize
}

export function resetPage(pageSize = DEFAULT_PAGE_SIZE): PageState {
  return { offset: 0, pageSize }
}