import type * as Monaco from 'monaco-editor/editor/editor.api.js'
import { api } from '../api'
import {
  extractTableRefs,
  getCompletionContext,
  resolveQualifier,
} from '../utils/completionContext'
import {
  buildCompletionItems,
  type ColumnSource,
  type CompletionItem,
  type TableSource,
} from '../utils/completionItems'

const TABLES_TTL = 120_000
const COLUMNS_TTL = 120_000

interface TablesEntry {
  connectionId: string
  at: number
  sources: TableSource[]
  databases: string[]
}

interface ColumnsEntry {
  at: number
  columns: { name: string; type: string }[]
}

const KIND_MAP: Record<CompletionItem['kind'], Monaco.languages.CompletionItemKind> = {
  keyword: 14,
  function: 1,
  table: 6,
  column: 3,
}

export interface SqlCompletionOptions {
  // 当前活动连接 ID，用于按连接缓存库表列清单
  getConnectionId: () => string | null
  // 当前编辑器全文，用于解析 FROM / JOIN 引用的表
  getText: () => string
}

export interface SqlCompletionHandle {
  register: () => Monaco.IDisposable | null
  invalidate: (connectionId?: string) => void
  dispose: () => void
}

export function createSqlCompletion(
  monaco: typeof Monaco,
  options: SqlCompletionOptions
): SqlCompletionHandle {
  let tablesCache: TablesEntry | null = null
  const columnsCache = new Map<string, ColumnsEntry>()
  const tablesInflight = new Map<string, Promise<TablesEntry | null>>()
  const columnsInflight = new Map<string, Promise<ColumnsEntry | null>>()
  let provider: Monaco.IDisposable | null = null

  function invalidate(connectionId?: string) {
    if (!connectionId || !tablesCache || tablesCache.connectionId === connectionId) {
      tablesCache = null
      columnsCache.clear()
      tablesInflight.clear()
      columnsInflight.clear()
    }
  }

  async function loadTables(connectionId: string): Promise<TablesEntry | null> {
    if (tablesCache && tablesCache.connectionId === connectionId && Date.now() - tablesCache.at < TABLES_TTL) {
      return tablesCache
    }
    const cached = tablesInflight.get(connectionId)
    if (cached) return cached
    const task = (async (): Promise<TablesEntry | null> => {
      try {
        const databases = await api.GetDatabases(connectionId)
        const results = await Promise.all(
          databases.map((db) =>
            api
              .GetTables(connectionId, db)
              .then((tables) => ({ database: db, tables: tables.map((t) => t.name) }))
              .catch(() => ({ database: db, tables: [] as string[] }))
          )
        )
        const entry: TablesEntry = {
          connectionId,
          at: Date.now(),
          sources: results.filter((r) => r.tables.length > 0),
          databases,
        }
        tablesCache = entry
        return entry
      } catch {
        return null
      } finally {
        tablesInflight.delete(connectionId)
      }
    })()
    tablesInflight.set(connectionId, task)
    return task
  }

  async function loadColumns(connectionId: string, database: string, table: string): Promise<ColumnsEntry | null> {
    const key = `${connectionId}|${database}|${table}`
    const hit = columnsCache.get(key)
    if (hit && Date.now() - hit.at < COLUMNS_TTL) return hit
    const running = columnsInflight.get(key)
    if (running) return running
    const task = (async (): Promise<ColumnsEntry | null> => {
      try {
        const schema = await api.GetTableSchema(connectionId, database, table)
        const entry: ColumnsEntry = {
          at: Date.now(),
          columns: schema.map((c) => ({ name: c.name, type: c.type })),
        }
        columnsCache.set(key, entry)
        return entry
      } catch {
        return null
      } finally {
        columnsInflight.delete(key)
      }
    })()
    columnsInflight.set(key, task)
    return task
  }

  function toMonacoItem(item: CompletionItem, range: Monaco.IRange): Monaco.languages.CompletionItem {
    return {
      label: item.label,
      kind: KIND_MAP[item.kind],
      detail: item.detail,
      insertText: item.insertText ?? item.label,
      range,
    }
  }

  async function provide(
    model: Monaco.editor.ITextModel,
    position: Monaco.Position
  ): Promise<Monaco.languages.CompletionList> {
    const text = model.getValue()
    const offset = model.getOffsetAt(position)
    const context = getCompletionContext(text, offset)
    if (context.kind === 'keyword' && !context.prefix) {
      return { suggestions: [] }
    }

    const connectionId = options.getConnectionId()
    const tableEntry = connectionId ? await loadTables(connectionId) : null

    let columnSources: ColumnSource[] = []
    if (context.kind === 'column' && context.preferColumn && connectionId) {
      const refs = extractTableRefs(text)
      const qualifierRef = context.qualifier ? resolveQualifier(text, context.qualifier) : null
      const targets = qualifierRef ? [qualifierRef] : refs
      const loaded = await Promise.all(
        targets.map(async (ref) => {
          if (!ref.database) return null
          const columns = await loadColumns(connectionId, ref.database, ref.table)
          if (!columns) return null
          return { table: ref.table, columns: columns.columns } as ColumnSource
        })
      )
      columnSources = loaded.filter((c): c is ColumnSource => c !== null)
    }

    const prefixLength = context.prefix.length
    const startColumn = Math.max(1, position.column - prefixLength)
    const range: Monaco.IRange = {
      startLineNumber: position.lineNumber,
      startColumn,
      endLineNumber: position.lineNumber,
      endColumn: position.column,
    }

    const items = buildCompletionItems({
      context,
      tableSources: tableEntry?.sources ?? [],
      databaseNames: tableEntry?.databases ?? [],
      columnSources,
    })

    return { suggestions: items.slice(0, 200).map((item) => toMonacoItem(item, range)) }
  }

  function register(): Monaco.IDisposable | null {
    if (provider) return provider
    provider = monaco.languages.registerCompletionItemProvider('sql', {
      triggerCharacters: ['.', '`', '"'],
      provideCompletionItems: (model, position) => provide(model, position),
    })
    return provider
  }

  function dispose() {
    provider?.dispose()
    provider = null
    tablesCache = null
    columnsCache.clear()
    tablesInflight.clear()
    columnsInflight.clear()
  }

  return { register, invalidate, dispose }
}