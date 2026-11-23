import type { QueryResult } from '../types'
import type { PageState } from './pagination'

export interface QueryTab {
  id: string
  title: string
  sql: string
  result: QueryResult | null
  error: string | null
  querying: boolean
  queryId: string | null
  pendingQueryIds: string[]
  canceled: boolean
  resultScrollTop: number
  viewState: unknown | null
  page: PageState | null
  lastExecutedSql: string
}

export function createTab(index: number, sql = '-- ClickHouse SQL\nSELECT version()'): QueryTab {
  return {
    id: genTabID(),
    title: `Query ${index}`,
    sql,
    result: null,
    error: null,
    querying: false,
    queryId: null,
    pendingQueryIds: [],
    canceled: false,
    resultScrollTop: 0,
    viewState: null,
    page: null,
    lastExecutedSql: '',
  }
}

function genTabID(): string {
  return 'tab-' + Math.random().toString(16).slice(2, 10)
}

export interface TabsSnapshot {
  tabs: ({ id?: string; title?: string; sql?: string } | null)[]
  active_tab_id: string
}

export interface RestoredTabs {
  tabs: QueryTab[]
  activeId: string
}

export function restoreTabs(snapshot: TabsSnapshot): RestoredTabs {
  const restored: QueryTab[] = (snapshot.tabs ?? []).map((t, i) => ({
    id: t?.id || 'tab-' + i,
    title: t?.title || `Query ${i + 1}`,
    sql: t?.sql ?? '',
    result: null,
    error: null,
    querying: false,
    queryId: null,
    pendingQueryIds: [],
    canceled: false,
    resultScrollTop: 0,
    viewState: null,
    page: null,
    lastExecutedSql: '',
  }))
  if (restored.length === 0) restored.push(createTab(1))
  const active = restored.find((t) => t.id === snapshot.active_tab_id) || restored[0]
  return { tabs: restored, activeId: active.id }
}