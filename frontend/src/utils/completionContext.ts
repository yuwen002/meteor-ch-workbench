export type CompletionContextKind = 'table' | 'column' | 'keyword'

export interface CompletionContext {
  kind: CompletionContextKind
  prefix: string
  qualifier: string
  preferTable: boolean
  preferColumn: boolean
}

export interface TableRef {
  database: string | null
  table: string
  alias: string | null
}

const TABLE_INTRODUCERS = ['FROM', 'JOIN', 'INTO', 'UPDATE', 'DESCRIBE', 'DESC', 'TABLE']

// 出现在 FROM <表> 之后、表示「表名部分已结束」的关键字
const AFTER_TABLE_KEYWORDS = [
  'ON', 'USING', 'WHERE', 'PREWHERE', 'GROUP', 'ORDER', 'LIMIT', 'HAVING', 'SETTINGS',
  'UNION', 'INNER', 'LEFT', 'RIGHT', 'FULL', 'CROSS', 'AS', 'FINAL', 'SAMPLE', 'FORMAT',
  'ARRAY', 'GLOBAL', 'ANY', 'ALL', 'SEMI', 'ANTI', 'OUTER', 'SELECT', 'WINDOW', 'QUALIFY',
  'WITH', 'OFFSET', 'VALUES', 'TABLE',
]

const NOT_TABLE_NAMES = ['SELECT', 'UNION', 'WHERE', 'GROUP', 'ORDER', 'LIMIT', 'SETTINGS', 'ON']

const WORD_SPLIT = /[\s,();]+/
const MAX_PROBE = 4000

function trimBack(text: string): string {
  return text.replace(/[\s,();]+$/g, '')
}

function lastWord(text: string): string {
  const trimmed = trimBack(text)
  const parts = trimmed.split(WORD_SPLIT)
  return parts[parts.length - 1] ?? ''
}

function splitQualified(word: string): { qualifier: string; prefix: string } {
  const raw = word.replace(/^[`"[]+/, '')
  const dot = raw.lastIndexOf('.')
  if (dot < 0) return { qualifier: '', prefix: raw.replace(/[`"\]]+$/g, '') }
  return {
    qualifier: raw.slice(0, dot).replace(/[`"\]]+$/g, ''),
    prefix: raw.slice(dot + 1).replace(/[`"\]]+$/g, ''),
  }
}

export function unquoteIdentifier(raw: string): string {
  return raw
    .split('.')
    .map((part) => {
      const t = part.trim()
      if (t.length >= 2 && t.startsWith('`') && t.endsWith('`')) return t.slice(1, -1).replace(/``/g, '`')
      if (t.length >= 2 && t.startsWith('"') && t.endsWith('"')) return t.slice(1, -1).replace(/""/g, '"')
      return t
    })
    .filter((p) => p.length > 0)
    .join('.')
}

export function isCursorInComment(text: string, cursor: number): boolean {
  const safeText = typeof text === 'string' ? text : ''
  const safeCursor = typeof cursor === 'number' && cursor > 0 ? Math.min(cursor, safeText.length) : 0
  const before = safeText.slice(0, safeCursor)
  const blockOpen = before.lastIndexOf('/*')
  const blockClose = before.lastIndexOf('*/')
  if (blockOpen > blockClose) return true
  const lineStart = before.lastIndexOf('\n') + 1
  const line = before.slice(lineStart)
  const dash = line.indexOf('--')
  const hash = line.indexOf('#')
  if (dash < 0 && hash < 0) return false
  const at = Math.min(dash < 0 ? hash : dash, hash < 0 ? dash : hash)
  return at >= 0 && (line[at] === '-' || line[at] === '#')
}

const TABLE_CLAUSE_END = [
  ...AFTER_TABLE_KEYWORDS,
  ',',
  ')',
]

function clauseTokens(statement: string): string {
  return statement
    .replace(/[(),;]/g, (m) => (m === '(' || m === ')' ? ' (' : ' , '))
    .replace(/\s+/g, ' ')
    .trim()
}

function inTablePosition(statement: string): boolean {
  const normalized = clauseTokens(statement)
  if (!normalized) return false
  const words = normalized.split(' ').filter((w) => w.length > 0 && w !== ')')
  const lastIdx = words.length - 1
  const last = words[lastIdx] ?? ''
  if (last === '(' || last === '.' || last === ')') return false
  let introIdx = -1
  for (let i = 0; i < words.length; i++) {
    if (TABLE_INTRODUCERS.includes(words[i].toUpperCase())) introIdx = i
  }
  if (introIdx < 0) return false
  if (introIdx === lastIdx) return true
  const rest = words.slice(introIdx + 1).filter((w) => w !== '(')
  if (rest.length === 0) return false
  const tail = rest[rest.length - 1]
  if (tail === ',') {
    return !rest.some((w) => AFTER_TABLE_KEYWORDS.includes(w.toUpperCase()))
  }
  const hasAfter = rest.some((w) => TABLE_CLAUSE_END.includes(w.toUpperCase()) || TABLE_CLAUSE_END.includes(w))
  return !hasAfter
}

export function getCompletionContext(text: string, cursor: number): CompletionContext {
  const safeText = typeof text === 'string' ? text : ''
  const safeCursor = typeof cursor === 'number' && cursor > 0 ? Math.min(cursor, safeText.length) : 0
  const before = safeText.slice(0, safeCursor)
  const tokenRaw = (before.split(WORD_SPLIT).pop() ?? '').trim()
  const { qualifier, prefix } = splitQualified(tokenRaw)
  const inComment = isCursorInComment(before, before.length)

  const statement = before.slice(Math.max(before.lastIndexOf(';'), before.lastIndexOf('(')) + 1)
  const endsWithSeparator = /[\s,();]$/.test(before)
  const word = lastWord(statement)
  const upperWord = word.toUpperCase()

  if (endsWithSeparator || before.length === 0) {
    if (!inComment && inTablePosition(statement)) {
      return { kind: 'table', prefix, qualifier: '', preferTable: true, preferColumn: false }
    }
    return { kind: 'keyword', prefix, qualifier, preferTable: false, preferColumn: false }
  }

  if (inComment) {
    return { kind: 'keyword', prefix, qualifier, preferTable: false, preferColumn: false }
  }

  if (qualifier) {
    return { kind: 'column', prefix, qualifier, preferTable: false, preferColumn: true }
  }

  if (TABLE_INTRODUCERS.includes(upperWord)) {
    return { kind: 'table', prefix, qualifier: '', preferTable: true, preferColumn: false }
  }

  if (inTablePosition(statement)) {
    return { kind: 'table', prefix, qualifier: '', preferTable: true, preferColumn: false }
  }

  const inSelectList = /\bSELECT\b/i.test(statement) && !/\b(WHERE|GROUP|ORDER|HAVING|ON|LIMIT)\b/i.test(statement)
  if (inSelectList) {
    return { kind: 'column', prefix, qualifier: '', preferTable: true, preferColumn: true }
  }

  return { kind: 'column', prefix, qualifier: '', preferTable: false, preferColumn: true }
}

export function extractTableRefs(text: string): TableRef[] {
  const safe = typeof text === 'string' ? text : ''
  const refs: TableRef[] = []
  const re =
    /(?:FROM|JOIN|INTO|UPDATE|DESCRIBE|DESC)([^A-Za-z`"0-9_]+)((?:[`"][^`"]+[`"]|[A-Za-z0-9_$-]+)\.(?:[`"][^`"]+[`"]|[A-Za-z0-9_$-]+))/gi
  for (const match of safe.matchAll(re)) {
    const raw = match[2]
    const parts = unquoteIdentifier(raw).split('.')
    if (parts.length < 2) continue
    const database = parts[0]
    const table = parts[parts.length - 1]
    if (NOT_TABLE_NAMES.includes(table.toUpperCase())) continue
    const after = safe.slice((match.index ?? 0) + match[0].length, (match.index ?? 0) + match[0].length + 120)
    const aliasMatch = after.match(/^\s+(?:AS\s+)?([A-Za-z_][A-Za-z0-9_$]*)/i)
    const aliasCandidate = aliasMatch ? aliasMatch[1] : ''
    const alias =
      aliasCandidate && !AFTER_TABLE_KEYWORDS.includes(aliasCandidate.toUpperCase()) ? aliasCandidate : null
    refs.push({ database, table, alias })
  }
  return refs
}

export function resolveQualifier(text: string, qualifier: string): TableRef | null {
  if (!qualifier) return null
  const q = unquoteIdentifier(qualifier).toLowerCase()
  for (const ref of extractTableRefs(text)) {
    if (ref.alias && ref.alias.toLowerCase() === q) return ref
    if (ref.table.toLowerCase() === q) return ref
    if (`${ref.database}.${ref.table}`.toLowerCase() === q) return ref
  }
  return null
}

export function filterByPrefix<T extends { label: string }>(items: T[], prefix: string): T[] {
  const p = (prefix || '').toLowerCase()
  if (!p) return items
  return items.filter((item) => item.label.toLowerCase().startsWith(p))
}