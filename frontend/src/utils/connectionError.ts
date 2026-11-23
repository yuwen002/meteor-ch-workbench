const DISCONNECT_PATTERNS: RegExp[] = [
  /connection\s+(is\s+)?closed/i,
  /\bEOF\b/i,
  /broken\s+pipe/i,
  /connection\s+refused/i,
  /connection\s+reset\s+by\s+peer/i,
  /i\/o\s+timeout/i,
  /server\s+closed\s+the\s+connection/i,
  /clickhouse:\s*exception:\s*close/i,
]

export function isDisconnectedError(text: string | null | undefined): boolean {
  const raw = (text ?? '').trim()
  if (raw === '') return false
  return DISCONNECT_PATTERNS.some((pattern) => pattern.test(raw))
}