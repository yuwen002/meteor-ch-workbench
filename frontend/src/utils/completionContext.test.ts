import { describe, expect, it } from 'vitest'
import {
  extractTableRefs,
  filterByPrefix,
  getCompletionContext,
  isCursorInComment,
  resolveQualifier,
  unquoteIdentifier,
} from './completionContext'

describe('getCompletionContext', () => {
  const cases: { name: string; sql: string; expected: { kind: string; prefix: string; qualifier: string } }[] = [
    { name: '空语句', sql: '', expected: { kind: 'keyword', prefix: '', qualifier: '' } },
    { name: 'FROM 之后待补全表名', sql: 'SELECT * FROM ', expected: { kind: 'table', prefix: '', qualifier: '' } },
    { name: 'FROM 之后输入表名前缀', sql: 'SELECT * FROM sys', expected: { kind: 'table', prefix: 'sys', qualifier: '' } },
    { name: 'JOIN 之后待补全表名', sql: 'SELECT * FROM a JOIN ', expected: { kind: 'table', prefix: '', qualifier: '' } },
    {
      name: 'JOIN 之后输入表名前缀',
      sql: 'SELECT * FROM a JOIN system',
      expected: { kind: 'table', prefix: 'system', qualifier: '' },
    },
    { name: 'FROM 之后逗号分隔多表', sql: 'SELECT * FROM a, ', expected: { kind: 'table', prefix: '', qualifier: '' } },
    { name: '库名前缀后点', sql: 'SELECT * FROM system.', expected: { kind: 'column', prefix: '', qualifier: 'system' } },
    { name: '库表前缀', sql: 'SELECT * FROM system.que', expected: { kind: 'column', prefix: 'que', qualifier: 'system' } },
    { name: 'SELECT 列表中的列补全', sql: 'SELECT na', expected: { kind: 'column', prefix: 'na', qualifier: '' } },
    { name: '别名限定列', sql: 'SELECT t.na', expected: { kind: 'column', prefix: 'na', qualifier: 't' } },
    {
      name: 'WHERE 中列补全',
      sql: 'SELECT * FROM system.query_log WHERE qu',
      expected: { kind: 'column', prefix: 'qu', qualifier: '' },
    },
    {
      name: 'FROM 表名已完成后不再推荐表',
      sql: 'SELECT * FROM system.query_log WHERE ',
      expected: { kind: 'keyword', prefix: '', qualifier: '' },
    },
    { name: '行注释内不补全', sql: 'SELECT 1 -- sel', expected: { kind: 'keyword', prefix: 'sel', qualifier: '' } },
    { name: '块注释内不补全', sql: '/* sel', expected: { kind: 'keyword', prefix: 'sel', qualifier: '' } },
    { name: '只取最后一条语句', sql: 'SELECT 1; SELECT * FROM ', expected: { kind: 'table', prefix: '', qualifier: '' } },
    { name: '反引号库名前缀', sql: 'SELECT * FROM `system`.', expected: { kind: 'column', prefix: '', qualifier: 'system' } },
  ]

  it.each(cases)('$name', ({ sql, expected }) => {
    const ctx = getCompletionContext(sql, sql.length)
    expect(ctx.kind).toBe(expected.kind)
    expect(ctx.prefix).toBe(expected.prefix)
    expect(ctx.qualifier).toBe(expected.qualifier)
  })

  it.each(cases)('$name（光标必须落在末尾）', ({ sql }) => {
    const ctx = getCompletionContext(sql, sql.length)
    expect(ctx.preferTable || ctx.preferColumn).toBeTypeOf('boolean')
  })
})

describe('isCursorInComment', () => {
  const cases: { name: string; sql: string; expected: boolean }[] = [
    { name: '普通语句', sql: 'SELECT 1', expected: false },
    { name: '行注释内', sql: 'SELECT 1 -- abc', expected: true },
    { name: '井号注释内', sql: 'SELECT 1 # abc', expected: true },
    { name: '块注释未闭合', sql: '/* abc', expected: true },
    { name: '块注释已闭合', sql: '/* abc */ SELECT 1', expected: false },
    { name: '空文本', sql: '', expected: false },
    { name: '负数光标', sql: 'SELECT 1', expected: false },
  ]

  it.each(cases)('$name', ({ sql, expected }) => {
    expect(isCursorInComment(sql, sql.length)).toBe(expected)
  })
})

describe('unquoteIdentifier', () => {
  const cases: { name: string; input: string; expected: string }[] = [
    { name: '无反引号', input: 'system.query_log', expected: 'system.query_log' },
    { name: '单段反引号', input: '`query_log`', expected: 'query_log' },
    { name: '两段反引号', input: '`system`.`query_log`', expected: 'system.query_log' },
    { name: '双引号', input: '"system"."query_log"', expected: 'system.query_log' },
    { name: '反引号转义', input: '`a``b`', expected: 'a`b' },
    { name: '带空白', input: '  system . query_log  ', expected: 'system.query_log' },
  ]

  it.each(cases)('$name', ({ input, expected }) => {
    expect(unquoteIdentifier(input)).toBe(expected)
  })
})

describe('extractTableRefs', () => {
  const cases: {
    name: string
    sql: string
    expected: { database: string; table: string; alias: string | null }[]
  }[] = [
    {
      name: '单表无别名',
      sql: 'SELECT * FROM system.query_log',
      expected: [{ database: 'system', table: 'query_log', alias: null }],
    },
    {
      name: '显式 AS 别名',
      sql: 'SELECT * FROM system.query_log AS q',
      expected: [{ database: 'system', table: 'query_log', alias: 'q' }],
    },
    {
      name: '隐式别名',
      sql: 'SELECT * FROM system.query_log q WHERE q.event_time > 0',
      expected: [{ database: 'system', table: 'query_log', alias: 'q' }],
    },
    {
      name: '别名不误取关键字',
      sql: 'SELECT * FROM system.query_log WHERE event_time > 0',
      expected: [{ database: 'system', table: 'query_log', alias: null }],
    },
    {
      name: '多表 JOIN',
      sql: 'SELECT * FROM db1.t1 a JOIN db2.t2 b ON a.id = b.id',
      expected: [
        { database: 'db1', table: 't1', alias: 'a' },
        { database: 'db2', table: 't2', alias: 'b' },
      ],
    },
    {
      name: '反引号表名',
      sql: 'SELECT * FROM `system`.`query_log`',
      expected: [{ database: 'system', table: 'query_log', alias: null }],
    },
    {
      name: '单段表名忽略',
      sql: 'SELECT * FROM query_log',
      expected: [],
    },
    { name: '空文本', sql: '', expected: [] },
  ]

  it.each(cases)('$name', ({ sql, expected }) => {
    expect(extractTableRefs(sql)).toEqual(expected)
  })
})

describe('resolveQualifier', () => {
  const sql = 'SELECT * FROM system.query_log q JOIN db2.t2 b ON q.id = b.id'
  const cases: { name: string; qualifier: string; expected: string | null }[] = [
    { name: '按别名解析', qualifier: 'q', expected: 'query_log' },
    { name: '按表名解析', qualifier: 'query_log', expected: 'query_log' },
    { name: '按库表全名解析', qualifier: 'system.query_log', expected: 'query_log' },
    { name: '第二个表的别名', qualifier: 'b', expected: 't2' },
    { name: '无法解析返回 null', qualifier: 'zzz', expected: null },
    { name: '空限定符返回 null', qualifier: '', expected: null },
  ]

  it.each(cases)('$name', ({ qualifier, expected }) => {
    const ref = resolveQualifier(sql, qualifier)
    expect(ref ? ref.table : null).toBe(expected)
  })
})

describe('filterByPrefix', () => {
  const items = [{ label: 'query_log' }, { label: 'QueryMetricLog' }, { label: 'events' }]
  const cases: { name: string; prefix: string; expected: string[] }[] = [
    { name: '空前缀返回全部', prefix: '', expected: ['query_log', 'QueryMetricLog', 'events'] },
    { name: '前缀匹配', prefix: 'query_', expected: ['query_log'] },
    { name: '忽略大小写', prefix: 'QUR', expected: [] },
    { name: '大写输入匹配小写项', prefix: 'QUERY_L', expected: ['query_log'] },
    { name: '无匹配返回空', prefix: 'zzz', expected: [] },
  ]

  it.each(cases)('$name', ({ prefix, expected }) => {
    expect(filterByPrefix(items, prefix).map((i) => i.label)).toEqual(expected)
  })
})