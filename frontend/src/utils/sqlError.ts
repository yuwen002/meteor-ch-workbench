export interface FriendlySqlError {
  code: number | null
  summary: string | null
  raw: string
}

const CODE_MESSAGES: Record<number, string> = {
  47: '字段或标识符不存在',
  48: '未知查询',
  60: '表不存在',
  62: 'SQL 语法错误',
  159: '查询超时',
  164: '查询被取消',
  202: '过多的语法元素',
  206: '表引擎不支持该操作',
  241: '内存超限，请减小查询规模或提高 max_memory_usage',
  242: '无法解析表结构',
  306: '内存限制超限',
  516: '认证失败',
  999: '未知错误',
}

const CODE_PATTERN = /Code:\s*(\d+)/i
const EXCEPTION_PATTERN = /DB::Exception:\s*([^\n]+?)(?:\s*\([A-Z0-9_]+\))?(?:\s*\(version[^)]*\))?\s*$/im

export function extractErrorCode(text: string): number | null {
  const m = CODE_PATTERN.exec(text)
  if (!m) return null
  const n = Number(m[1])
  return Number.isFinite(n) ? n : null
}

export function describeErrorCode(code: number | null): string | null {
  if (code === null) return null
  return CODE_MESSAGES[code] ?? null
}

export function firstLine(text: string): string {
  return text.split(/\r?\n/).find((line) => line.trim().length > 0)?.trim() ?? ''
}

export function parseSqlError(raw: string): FriendlySqlError {
  const text = raw ?? ''
  const code = extractErrorCode(text)

  let summary: string | null = null
  if (code !== null) {
    const mapped = describeErrorCode(code)
    const detail = EXCEPTION_PATTERN.exec(text)?.[1]?.trim().replace(/\.$/, '')
    const parts: string[] = []
    if (detail) parts.push(detail)
    if (mapped && mapped !== detail) parts.push(mapped)
    summary = parts.join(' — ') || null
  } else {
    const line = firstLine(text)
    summary = line || null
  }

  return { code, summary, raw: text }
}

export function isCanceledMessage(text: string, canceledText: string): boolean {
  return text.trim() === canceledText.trim()
}