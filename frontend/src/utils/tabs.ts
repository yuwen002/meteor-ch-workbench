import type { QueryResult } from '../types'

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
  }
}

function genTabID(): string {
  return 'tab-' + Math.random().toString(16).slice(2, 10)
}