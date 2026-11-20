export type ExplainMode = 'PLAN' | 'AST' | 'PLAN_DETAIL' | 'PIPELINE'

export interface ExplainModeOption {
  id: ExplainMode
  keyword: string
  labelKey: string
}

export const EXPLAIN_MODES: ExplainModeOption[] = [
  { id: 'PLAN', keyword: 'EXPLAIN', labelKey: 'editor.explainPlan' },
  { id: 'AST', keyword: 'EXPLAIN AST', labelKey: 'editor.explainAst' },
  { id: 'PLAN_DETAIL', keyword: 'EXPLAIN PLAN', labelKey: 'editor.explainPlanDetail' },
  { id: 'PIPELINE', keyword: 'EXPLAIN PIPELINE', labelKey: 'editor.explainPipeline' },
]

const BLOCKED_PREFIXES = [
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
]

function stripComments(sql: string): string {
  return sql
    .replace(/\/\*[\s\S]*?\*\//g, ' ')
    .replace(/--[^\n]*/g, ' ')
    .replace(/#[^\n]*/g, ' ')
    .trim()
}

export function splitStatements(sql: string): string[] {
  return stripComments(sql)
    .split(';')
    .map((s) => s.trim())
    .filter((s) => s.length > 0)
}

export function canExplain(sql: string): boolean {
  const statements = splitStatements(sql)
  if (statements.length !== 1) return false
  const head = statements[0].replace(/\s+/g, ' ').toUpperCase()
  if (!head) return false
  return !BLOCKED_PREFIXES.some((p) => head === p || head.startsWith(p + ' ') || head.startsWith(p + '('))
}

export function getExplainKeyword(mode: ExplainMode): string {
  const found = EXPLAIN_MODES.find((m) => m.id === mode)
  return found ? found.keyword : 'EXPLAIN'
}

export function buildExplainSQL(sql: string, mode: ExplainMode = 'PLAN'): string {
  const statements = splitStatements(sql)
  const body = statements[0] ?? ''
  if (!body) return ''
  const keyword = getExplainKeyword(mode)
  return `${keyword} ${body}`
}