const SELECT_HEADS = ['SELECT', 'WITH']

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

export function stripPageTail(sql: string): string {
  return stripTrailingSemicolon(sql).replace(/\s+LIMIT\s+\d+\s+OFFSET\s+\d+\s*$/i, '').trim()
}

function headKeyword(statement: string): string {
  const cleaned = stripComments(statement).replace(/\s+/g, ' ').trim()
  const withParen = cleaned.match(/^([A-Za-z_]+)\s*\(/)
  if (withParen) return withParen[1].toUpperCase()
  return (cleaned.split(' ')[0] ?? '').toUpperCase()
}

function hasSingleStatement(sql: string): boolean {
  return stripComments(sql).split(';').filter((s) => s.trim().length > 0).length === 1
}

function skipQuote(s: string, i: number): number {
  const quote = s[i]
  i++
  while (i < s.length) {
    if (s[i] === '\\') {
      i += 2
      continue
    }
    if (s[i] === quote) return i + 1
    i++
  }
  return i
}

export interface SelectList {
  start: number
  end: number
}

export type ProjectionRewrite =
  | { ok: true; sql: string }
  | { ok: false; reason: 'unsupported' | 'not-found' }

export function findSelectList(sql: string): SelectList | null {
  const s = stripTrailingSemicolon(sql)
  if (!hasSingleStatement(s)) return null
  if (!SELECT_HEADS.includes(headKeyword(s))) return null

  let depth = 0
  let blockStart = -1
  let fromIndex = -1
  let i = 0

  while (i < s.length) {
    const ch = s[i]
    if (ch === "'" || ch === '"' || ch === '`') {
      i = skipQuote(s, i)
      continue
    }
    if (ch === '(') {
      depth++
      i++
      continue
    }
    if (ch === ')') {
      depth--
      i++
      continue
    }
    if (/[A-Za-z_]/.test(ch)) {
      let j = i
      while (j < s.length && /[A-Za-z0-9_$]/.test(s[j])) j++
      const kw = s.slice(i, j).toUpperCase()
      if (depth === 0) {
        if (kw === 'SELECT' && blockStart < 0) {
          blockStart = j
        } else if (kw === 'FROM' && blockStart >= 0) {
          fromIndex = i
          break
        }
      }
      i = j
      continue
    }
    i++
  }

  if (blockStart < 0 || fromIndex < 0 || fromIndex < blockStart) return null

  let d = 0
  let k = fromIndex
  while (k < s.length) {
    const ch = s[k]
    if (ch === "'" || ch === '"' || ch === '`') {
      k = skipQuote(s, k)
      continue
    }
    if (ch === '(') {
      d++
      k++
      continue
    }
    if (ch === ')') {
      d--
      k++
      continue
    }
    if (/[A-Za-z_]/.test(ch)) {
      let j = k
      while (j < s.length && /[A-Za-z0-9_$]/.test(s[j])) j++
      const kw = s.slice(k, j).toUpperCase()
      if (d === 0 && (kw === 'UNION' || kw === 'INTERSECT' || kw === 'EXCEPT')) return null
      k = j
      continue
    }
    k++
  }

  return { start: blockStart, end: fromIndex }
}

export function splitTopLevel(expr: string): string[] {
  const parts: string[] = []
  let depth = 0
  let start = 0
  let i = 0
  while (i < expr.length) {
    const ch = expr[i]
    if (ch === "'" || ch === '"' || ch === '`') {
      i = skipQuote(expr, i)
      continue
    }
    if (ch === '(') {
      depth++
      i++
      continue
    }
    if (ch === ')') {
      depth--
      i++
      continue
    }
    if (ch === ',' && depth === 0) {
      parts.push(expr.slice(start, i))
      start = i + 1
      i++
      continue
    }
    i++
  }
  parts.push(expr.slice(start))
  return parts.map((p) => p.trim()).filter((p) => p.length > 0)
}

function unquote(name: string): string {
  const trimmed = name.trim()
  if (trimmed.length >= 2 && trimmed.startsWith('`') && trimmed.endsWith('`')) {
    return trimmed.slice(1, -1).replace(/``/g, '`')
  }
  if (trimmed.length >= 2 && trimmed.startsWith('"') && trimmed.endsWith('"')) {
    return trimmed.slice(1, -1)
  }
  return trimmed
}

function isPlainIdentifier(s: string): boolean {
  return /^[A-Za-z_][A-Za-z0-9_$]*$/.test(s)
}

export interface ProjectionItem {
  expr: string
  alias: string | null
}

export function parseProjection(expr: string): ProjectionItem {
  const cleaned = expr.trim()
  const alias = extractAlias(cleaned)
  if (alias) return alias
  if (!cleaned.includes('(')) {
    const direct = unquote(cleaned)
    const tail = qualifiedTail(cleaned)
    if (isPlainIdentifier(direct)) return { expr: cleaned, alias: direct }
    if (tail) return { expr: cleaned, alias: tail }
  }
  return { expr: cleaned, alias: null }
}

function extractAlias(s: string): ProjectionItem | null {
  let i = s.length - 1
  while (i >= 0 && /\s/.test(s[i])) i--

  let aliasStart = i
  let aliasEnd = i
  let quotedAlias = false
  if (s[i] === '`' || s[i] === '"') {
    const quote = s[i]
    let j = i - 1
    while (j >= 0 && s[j] !== quote) j--
    if (j < 0) return null
    aliasStart = j
    aliasEnd = i
    i = j - 1
    quotedAlias = true
  } else {
    while (aliasStart >= 0 && /[A-Za-z0-9_$]/.test(s[aliasStart])) aliasStart--
    aliasStart++
    if (aliasStart > aliasEnd) return null
    i = aliasStart - 1
    if (i < 0 || !/\s/.test(s[i])) return null
  }
  while (i >= 0 && /\s/.test(s[i])) i--
  if (i < 0) return null

  const aliasName = unquote(s.slice(aliasStart, aliasEnd + 1))
  const rest = s.slice(0, i + 1)

  const asMatch = rest.match(/(^|[\s)])AS\b/i)
  if (asMatch && asMatch.index !== undefined && asMatch.index + asMatch[0].length === rest.length) {
    const start = asMatch.index + (asMatch[1] ? 1 : 0)
    return { expr: s.slice(0, start).trim(), alias: aliasName }
  }
  if (!isPlainIdentifier(aliasName) && !quotedAlias) return null
  if (!isImplicitAliasHead(rest)) return null
  return { expr: rest.trim(), alias: aliasName }
}

function isImplicitAliasHead(rest: string): boolean {
  const trimmed = rest.trim()
  if (!trimmed) return false
  const last = trimmed[trimmed.length - 1]
  if (/[+\-*/%<>=|&^!.,(]/.test(last)) return false
  if (trimmed === '*' || /^\*$/i.test(trimmed)) return false
  if (/^\d/.test(trimmed.replace(/^[`"](.*)[`"]$/, ''))) return false
  if (trimmed.endsWith(')')) return true
  return /^[`"]?[A-Za-z_][A-Za-z0-9_$]*[`"]?$/.test(trimmed) ||
    /^[`"]?[A-Za-z_][A-Za-z0-9_$]*[`"]?\s*\.[`"]?[A-Za-z_][A-Za-z0-9_$]*[`"]?$/.test(trimmed) ||
    /^[`"]?[A-Za-z_][A-Za-z0-9_$]*[`"]?\s*\([\s\S]*\)$/.test(trimmed)
}

function qualifiedTail(expr: string): string | null {
  const lastDot = expr.lastIndexOf('.')
  if (lastDot < 0) return null
  const tail = unquote(expr.slice(lastDot + 1))
  return isPlainIdentifier(tail) ? tail : null
}

export function projectionOutputNames(exprs: string[]): Array<string | null> {
  return exprs.map((e) => parseProjection(e).alias)
}

function applyRemoval(sql: string, drop: Set<number>, emptyReason: 'keep' | 'not-found'): ProjectionRewrite {
  const list = findSelectList(sql)
  if (!list) return { ok: false, reason: 'unsupported' }
  const body = stripTrailingSemicolon(sql)
  const items = splitTopLevel(body.slice(list.start, list.end))
  if (items.length === 0) return { ok: false, reason: 'unsupported' }
  if (drop.size === 0) {
    return emptyReason === 'keep' ? { ok: true, sql: body } : { ok: false, reason: 'not-found' }
  }
  const kept = items.filter((_, i) => !drop.has(i))
  if (kept.length === 0) return { ok: false, reason: 'unsupported' }
  const head = body.slice(0, list.start).replace(/\s+$/, '')
  const tail = body.slice(list.end)
  const sep = /\s/.test(tail[0] ?? '') ? '' : ' '
  return { ok: true, sql: `${head} ${kept.join(', ')}${sep}${tail}` }
}

export function rewriteProjection(sql: string, removeIndexes: number[]): ProjectionRewrite {
  const drop = new Set<number>()
  for (const idx of removeIndexes) {
    if (Number.isInteger(idx) && idx >= 0) drop.add(idx)
  }
  return applyRemoval(sql, drop, 'keep')
}

export function rewriteProjectionToColumns(
  sql: string,
  removedNames: string[],
  allNames: string[] = [],
): ProjectionRewrite {
  const names = new Set(removedNames.filter((n) => n.trim().length > 0))
  if (names.size === 0) return { ok: true, sql: stripTrailingSemicolon(sql) }
  const list = findSelectList(sql)
  if (!list) return { ok: false, reason: 'unsupported' }
  const body = stripTrailingSemicolon(sql)
  const items = splitTopLevel(body.slice(list.start, list.end))
  if (items.length === 0) return { ok: false, reason: 'unsupported' }

  const onlyStar = items.every((item) => isStarItem(item))
  if (onlyStar) {
    return expandStar(body, list, items, names, allNames)
  }

  const drop = new Set<number>()
  items.forEach((item, i) => {
    const alias = parseProjection(item).alias
    if (alias && names.has(alias)) drop.add(i)
  })
  return applyRemoval(sql, drop, 'not-found')
}

function isStarItem(expr: string): boolean {
  const alias = extractAlias(expr)
  const core = alias ? alias.expr : expr.trim()
  return /^`?[A-Za-z_][A-Za-z0-9_$]*`?\s*\.[\s]*\*$/.test(core) || core.trim() === '*'
}

function expandStar(
  body: string,
  list: SelectList,
  items: string[],
  removed: Set<string>,
  allNames: string[],
): ProjectionRewrite {
  const names = allNames.map((n) => n.trim()).filter((n) => n.length > 0)
  if (names.length === 0) return { ok: false, reason: 'not-found' }
  const kept = names.filter((n) => !removed.has(n))
  if (kept.length === 0) return { ok: false, reason: 'unsupported' }
  const rendered: string[] = []
  for (const item of items) {
    const prefix = starQualifier([item])
    for (const n of kept) {
      rendered.push(prefix ? `${prefix}.${quoteIdentifier(n)}` : quoteIdentifier(n))
    }
  }
  const head = body.slice(0, list.start).replace(/\s+$/, '')
  const tail = body.slice(list.end)
  const sep = /\s/.test(tail[0] ?? '') ? '' : ' '
  return { ok: true, sql: `${head} ${rendered.join(', ')}${sep}${tail}` }
}

function starQualifier(items: string[]): string | null {
  for (const item of items) {
    const core = (extractAlias(item)?.expr ?? item).trim()
    const m = core.match(/^(`?[A-Za-z_][A-Za-z0-9_$]*`?)\s*\./)
    if (m) return m[1]
  }
  return null
}

export function quoteIdentifier(name: string): string {
  if (/^[A-Za-z_][A-Za-z0-9_$]*$/.test(name)) return name
  return '`' + name.replace(/`/g, '``') + '`'
}