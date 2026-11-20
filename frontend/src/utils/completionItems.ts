import { filterByPrefix, type CompletionContext } from './completionContext'

export const SQL_KEYWORDS: string[] = [
  'SELECT', 'FROM', 'WHERE', 'PREWHERE', 'GROUP BY', 'ORDER BY', 'HAVING', 'LIMIT', 'OFFSET',
  'JOIN', 'INNER JOIN', 'LEFT JOIN', 'RIGHT JOIN', 'FULL JOIN', 'CROSS JOIN', 'GLOBAL JOIN',
  'ON', 'USING', 'AS', 'WITH', 'UNION ALL', 'UNION', 'INTERSECT', 'EXCEPT',
  'INSERT INTO', 'CREATE TABLE', 'CREATE DATABASE', 'CREATE VIEW', 'ALTER TABLE', 'DROP TABLE',
  'TRUNCATE TABLE', 'RENAME TABLE', 'DESCRIBE', 'SHOW TABLES', 'SHOW DATABASES', 'EXPLAIN',
  'DISTINCT', 'FINAL', 'SAMPLE', 'ARRAY JOIN', 'FORMAT', 'SETTINGS', 'PARTITION BY',
  'ENGINE', 'NULLS FIRST', 'NULLS LAST', 'ASC', 'DESC', 'IN', 'NOT IN', 'BETWEEN',
  'CASE WHEN', 'THEN', 'ELSE', 'END', 'INTERVAL', 'OPTIMIZE TABLE', 'SYSTEM',
]

export const SQL_FUNCTIONS: { name: string; detail: string }[] = [
  { name: 'count', detail: '聚合 → UInt64' },
  { name: 'countDistinct', detail: '聚合 → UInt64' },
  { name: 'sum', detail: '聚合 → 数值' },
  { name: 'sumIf', detail: '聚合 → 数值' },
  { name: 'avg', detail: '聚合 → Float64' },
  { name: 'min', detail: '聚合' },
  { name: 'max', detail: '聚合' },
  { name: 'uniq', detail: '聚合 → 近似去重计数' },
  { name: 'uniqExact', detail: '聚合 → 精确去重计数' },
  { name: 'any', detail: '聚合' },
  { name: 'anyLast', detail: '聚合' },
  { name: 'groupArray', detail: '聚合 → Array' },
  { name: 'quantile', detail: '聚合 → 分位数' },
  { name: 'now', detail: '→ DateTime' },
  { name: 'today', detail: '→ Date' },
  { name: 'toDate', detail: '→ Date' },
  { name: 'toDateTime', detail: '→ DateTime' },
  { name: 'toString', detail: '→ String' },
  { name: 'toInt64', detail: '→ Int64' },
  { name: 'toUInt64', detail: '→ UInt64' },
  { name: 'toFloat64', detail: '→ Float64' },
  { name: 'toFixedString', detail: '→ FixedString' },
  { name: 'formatDateTime', detail: '→ String' },
  { name: 'dateDiff', detail: '→ Int64' },
  { name: 'dateTrunc', detail: '→ Date/DateTime' },
  { name: 'if', detail: '条件表达式' },
  { name: 'multiIf', detail: '多分支条件' },
  { name: 'coalesce', detail: '取首个非 NULL' },
  { name: 'ifNull', detail: 'NULL 回退' },
  { name: 'nullIf', detail: '相等时返回 NULL' },
  { name: 'isNull', detail: '→ UInt8' },
  { name: 'isNotNull', detail: '→ UInt8' },
  { name: 'assumeNotNull', detail: '剥离 Nullable' },
  { name: 'lower', detail: '→ String' },
  { name: 'upper', detail: '→ String' },
  { name: 'trim', detail: '→ String' },
  { name: 'concat', detail: '→ String' },
  { name: 'substring', detail: '→ String' },
  { name: 'splitByChar', detail: '→ Array' },
  { name: 'length', detail: '→ UInt64' },
  { name: 'position', detail: '→ UInt64' },
  { name: 'arrayJoin', detail: '展开数组' },
  { name: 'arrayMap', detail: '高阶函数' },
  { name: 'arrayFilter', detail: '高阶函数' },
  { name: 'has', detail: '→ UInt8' },
  { name: 'rowNumberInAllBlocks', detail: '窗口辅助' },
  { name: 'version', detail: '→ String' },
  { name: 'currentDatabase', detail: '→ String' },
  { name: 'hostName', detail: '→ String' },
  { name: 'toTypeName', detail: '→ String' },
]

export type CompletionKind = 'keyword' | 'function' | 'table' | 'column'

export interface CompletionItem {
  label: string
  kind: CompletionKind
  detail: string
  insertText?: string
}

export interface TableSource {
  database: string
  tables: string[]
}

export interface ColumnSource {
  table: string
  columns: { name: string; type: string }[]
}

export interface BuildCompletionOptions {
  context: CompletionContext
  keywords?: string[]
  functions?: { name: string; detail: string }[]
  tableSources?: TableSource[]
  columnSources?: ColumnSource[]
  databaseNames?: string[]
}

function tableItems(
  sources: TableSource[],
  qualifier: string,
  prefix: string,
  databaseNames: string[]
): CompletionItem[] {
  const q = qualifier.trim()
  if (q) {
    const dot = q.indexOf('.')
    const dbFilter = dot >= 0 ? q.slice(0, dot) : ''
    const dbName = dot >= 0 ? q.slice(dot + 1) : q
    const items: CompletionItem[] = []
    for (const source of sources) {
      if (dbFilter && source.database.toLowerCase() !== dbFilter.toLowerCase()) continue
      if (!dbFilter && source.database.toLowerCase() !== dbName.toLowerCase()) continue
      for (const table of source.tables) {
        items.push({
          label: table,
          kind: 'table',
          detail: `${source.database}.${table}`,
          insertText: `\`${table.replace(/`/g, '``')}\``,
        })
      }
    }
    return filterByPrefix(items, prefix)
  }
  const items: CompletionItem[] = []
  for (const name of databaseNames) {
    items.push({
      label: name,
      kind: 'table',
      detail: '数据库',
      insertText: `\`${name.replace(/`/g, '``')}\`.`,
    })
  }
  for (const source of sources) {
    for (const table of source.tables) {
      items.push({
        label: table,
        kind: 'table',
        detail: source.database,
        insertText: `\`${source.database.replace(/`/g, '``')}\`.\`${table.replace(/`/g, '``')}\``,
      })
    }
  }
  return filterByPrefix(items, prefix)
}

function columnItems(sources: ColumnSource[], qualifier: string, prefix: string): CompletionItem[] {
  const q = qualifier.trim()
  const matched = q ? sources.filter((s) => s.table.toLowerCase() === q.toLowerCase()) : sources
  const items: CompletionItem[] = []
  for (const source of matched) {
    for (const column of source.columns) {
      items.push({
        label: column.name,
        kind: 'column',
        detail: column.type ? `${column.type}${q ? '' : ` · ${source.table}`}` : source.table,
        insertText: q ? `${column.name}` : column.name,
      })
    }
  }
  return filterByPrefix(items, prefix)
}

export function buildCompletionItems(options: BuildCompletionOptions): CompletionItem[] {
  const { context, keywords = SQL_KEYWORDS, functions = SQL_FUNCTIONS, tableSources = [], columnSources = [], databaseNames = [] } = options
  const items: CompletionItem[] = []

  if (context.kind === 'table') {
    items.push(...tableItems(tableSources, context.qualifier, context.prefix, databaseNames))
  } else if (context.kind === 'column') {
    items.push(...columnItems(columnSources, context.qualifier, context.prefix))
    if (context.preferTable && !context.qualifier) {
      items.push(...tableItems(tableSources, '', context.prefix, databaseNames))
    }
  }

  if (context.kind !== 'table' && !context.preferColumn) {
    items.push(
      ...filterByPrefix(
        keywords.map((k) => ({ label: k, kind: 'keyword' as CompletionKind, detail: '关键字' })),
        context.prefix
      )
    )
  }

  if (context.kind !== 'table' && !context.preferTable) {
    items.push(
      ...filterByPrefix(
        functions.map((f) => ({ label: f.name, kind: 'function' as CompletionKind, detail: f.detail, insertText: `${f.name}(` })),
        context.prefix
      )
    )
  }

  return items
}