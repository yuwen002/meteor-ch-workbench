import { describe, expect, it } from 'vitest'
import { SQL_FUNCTIONS, SQL_KEYWORDS, buildCompletionItems } from './completionItems'
import type { CompletionContext } from './completionContext'

function ctx(partial: Partial<CompletionContext>): CompletionContext {
  return { kind: 'keyword', prefix: '', qualifier: '', preferTable: false, preferColumn: false, ...partial }
}

describe('buildCompletionItems', () => {
  const tableSources = [
    { database: 'system', tables: ['query_log', 'processes'] },
    { database: 'default', tables: ['events'] },
  ]
  const databaseNames = ['system', 'default']
  const columnSources = [
    { table: 'query_log', columns: [{ name: 'event_time', type: 'DateTime' }, { name: 'query', type: 'String' }] },
    { table: 'processes', columns: [{ name: 'pid', type: 'UInt64' }] },
  ]

  const cases: {
    name: string
    context: CompletionContext
    expectedLabels: string[]
    excludedLabels?: string[]
  }[] = [
    {
      name: 'FROM 后表名位置给出库名与表名',
      context: ctx({ kind: 'table', preferTable: true }),
      expectedLabels: ['system', 'default', 'query_log', 'processes', 'events'],
      excludedLabels: ['SELECT'],
    },
    {
      name: '表名前缀过滤',
      context: ctx({ kind: 'table', prefix: 'que', preferTable: true }),
      expectedLabels: ['query_log'],
    },
    {
      name: '库限定后只给该库的表',
      context: ctx({ kind: 'table', qualifier: 'system', preferTable: true }),
      expectedLabels: ['query_log', 'processes'],
      excludedLabels: ['events', 'system'],
    },
    {
      name: '库限定带前缀',
      context: ctx({ kind: 'table', qualifier: 'default', prefix: 'ev', preferTable: true }),
      expectedLabels: ['events'],
    },
    {
      name: '列位置给出全部表列',
      context: ctx({ kind: 'column', preferColumn: true }),
      expectedLabels: ['event_time', 'query', 'pid'],
    },
    {
      name: '别名限定只给对应表列',
      context: ctx({ kind: 'column', qualifier: 'query_log', preferColumn: true }),
      expectedLabels: ['event_time', 'query'],
      excludedLabels: ['pid'],
    },
    {
      name: '列前缀过滤',
      context: ctx({ kind: 'column', prefix: 'ev', preferColumn: true }),
      expectedLabels: ['event_time'],
    },
    {
      name: '无关位置不给出表名与列名',
      context: ctx({ kind: 'keyword', prefix: 'SEL' }),
      expectedLabels: ['SELECT'],
      excludedLabels: ['events', 'event_time'],
    },
    {
      name: 'SELECT 列表同时给出列与关键字',
      context: ctx({ kind: 'column', prefix: 'q', preferTable: true, preferColumn: true }),
      expectedLabels: ['query', 'query_log'],
    },
  ]

  it.each(cases)('$name', ({ context, expectedLabels, excludedLabels }) => {
    const items = buildCompletionItems({ context, tableSources, databaseNames, columnSources })
    const labels = items.map((i) => i.label)
    for (const label of expectedLabels) {
      expect(labels).toContain(label)
    }
    for (const label of excludedLabels ?? []) {
      expect(labels).not.toContain(label)
    }
  })

  it('无数据源时不报错，仅返回关键字与函数', () => {
    const items = buildCompletionItems({ context: ctx({ kind: 'keyword', prefix: 'co' }) })
    expect(items.length).toBeGreaterThan(0)
    expect(items.every((i) => i.kind === 'keyword' || i.kind === 'function')).toBe(true)
  })

  it('函数补全带左括号插入文本', () => {
    const items = buildCompletionItems({ context: ctx({ kind: 'column', prefix: 'toDate' }) })
    const fn = items.find((i) => i.label === 'toDate')
    expect(fn?.insertText).toBe('toDate(')
    expect(fn?.detail).toContain('Date')
  })

  it('表名补全带反引号包裹的全名', () => {
    const items = buildCompletionItems({ context: ctx({ kind: 'table', prefix: 'query', preferTable: true }), tableSources })
    const table = items.find((i) => i.label === 'query_log')
    expect(table?.insertText).toBe('`system`.`query_log`')
    expect(table?.detail).toBe('system')
  })

  it('关键字与函数表非空且无重复', () => {
    expect(new Set(SQL_KEYWORDS).size).toBe(SQL_KEYWORDS.length)
    expect(new Set(SQL_FUNCTIONS.map((f) => f.name)).size).toBe(SQL_FUNCTIONS.length)
    expect(SQL_KEYWORDS.length).toBeGreaterThan(20)
    expect(SQL_FUNCTIONS.length).toBeGreaterThan(20)
  })
})