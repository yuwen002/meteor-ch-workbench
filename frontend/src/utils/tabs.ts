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