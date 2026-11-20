import { describe, expect, it } from 'vitest'
import {
  DEFAULT_PAGE_SIZE,
  buildPageSQL,
  currentPageNumber,
  hasNextPage,
  hasPrevPage,
  headKeyword,
  isSingleSelect,
  nextPage,
  prevPage,
  resetPage,
  splitStatements,
  type PageState,
} from './pagination'

describe('splitStatements', () => {
  const cases: { name: string; input: string; expected: string[] }[] = [
    { name: '单条语句', input: 'SELECT 1', expected: ['SELECT 1'] },
    { name: '结尾分号被剥离', input: 'SELECT 1;', expected: ['SELECT 1'] },
    { name: '多条语句按分号切分', input: 'SELECT 1; SELECT 2', expected: ['SELECT 1', 'SELECT 2'] },
    { name: '空字符串返回空数组', input: '', expected: [] },
    { name: '仅空白与分号返回空数组', input: '  ;;  ', expected: [] },
    { name: '行注释被剥离', input: '-- 注释\nSELECT 1', expected: ['SELECT 1'] },
    { name: '井号注释被剥离', input: '# 注释\nSELECT 1', expected: ['SELECT 1'] },
    { name: '块注释被剥离', input: '/* 注释 */ SELECT 1', expected: ['SELECT 1'] },
    {
      name: '多行块注释跨行剥离',
      input: '/*\n多行\n注释\n*/SELECT 1',
      expected: ['SELECT 1'],
    },
    { name: '注释中的分号不切分', input: 'SELECT 1 -- a;b', expected: ['SELECT 1'] },
  ]

  it.each(cases)('$name', ({ input, expected }) => {
    expect(splitStatements(input)).toEqual(expected)
  })
})

describe('headKeyword', () => {
  const cases: { name: string; input: string; expected: string }[] = [
    { name: '普通 SELECT', input: 'SELECT * FROM t', expected: 'SELECT' },
    { name: '小写转大写', input: 'select 1', expected: 'SELECT' },
    { name: '前导空白', input: '   SELECT 1', expected: 'SELECT' },
    { name: '带括号的标识符', input: 'count(1)', expected: 'COUNT' },
    { name: 'WITH 开头', input: 'WITH x AS (SELECT 1) SELECT * FROM x', expected: 'WITH' },
    { name: '空字符串', input: '', expected: '' },
    { name: '仅注释', input: '-- 只有注释', expected: '' },
  ]

  it.each(cases)('$name', ({ input, expected }) => {
    expect(headKeyword(input)).toBe(expected)
  })
})

describe('isSingleSelect', () => {
  const cases: { name: string; input: string; expected: boolean }[] = [
    { name: '单条 SELECT', input: 'SELECT 1', expected: true },
    { name: '小写 SELECT', input: 'select 1', expected: true },
    { name: '带注释的单条 SELECT', input: '-- hi\nSELECT 1', expected: true },
    { name: '带子查询的 SELECT', input: 'SELECT * FROM (SELECT 1)', expected: true },
    { name: 'WITH 开头可翻页', input: 'WITH x AS (SELECT 1) SELECT * FROM x', expected: true },
    { name: '多条语句不可翻页', input: 'SELECT 1; SELECT 2', expected: false },
    { name: '空字符串不可翻页', input: '', expected: false },
    { name: 'INSERT 不可翻页', input: 'INSERT INTO t VALUES (1)', expected: false },
    { name: 'CREATE 不可翻页', input: 'CREATE TABLE t (a Int8)', expected: false },
    { name: 'DROP 不可翻页', input: 'DROP TABLE t', expected: false },
    { name: 'ALTER 不可翻页', input: 'ALTER TABLE t ADD COLUMN b Int8', expected: false },
    { name: 'EXPLAIN 不可翻页', input: 'EXPLAIN SELECT 1', expected: false },
    { name: 'SHOW 不可翻页', input: 'SHOW TABLES', expected: false },
    { name: 'DESCRIBE 不可翻页', input: 'DESCRIBE t', expected: false },
    { name: 'SET 不可翻页', input: 'SET max_threads = 4', expected: false },
    { name: 'TRUNCATE 不可翻页', input: 'TRUNCATE TABLE t', expected: false },
    { name: 'SYSTEM 不可翻页', input: 'SYSTEM FLUSH LOGS', expected: false },
  ]

  it.each(cases)('$name', ({ input, expected }) => {
    expect(isSingleSelect(input)).toBe(expected)
  })
})

describe('buildPageSQL', () => {
  const cases: { name: string; input: string; offset: number; pageSize: number; expected: string }[] = [
    {
      name: '首页追加 LIMIT OFFSET',
      input: 'SELECT 1',
      offset: 0,
      pageSize: 50000,
      expected: 'SELECT 1 LIMIT 50000 OFFSET 0',
    },
    {
      name: '第二页偏移正确',
      input: 'SELECT 1',
      offset: 50000,
      pageSize: 50000,
      expected: 'SELECT 1 LIMIT 50000 OFFSET 50000',
    },
    {
      name: '剥离尾部空分号',
      input: 'SELECT 1 ;',
      offset: 0,
      pageSize: 100,
      expected: 'SELECT 1 LIMIT 100 OFFSET 0',
    },
    {
      name: '多条语句只取第一条',
      input: 'SELECT 1; SELECT 2',
      offset: 0,
      pageSize: 100,
      expected: 'SELECT 1 LIMIT 100 OFFSET 0',
    },
    {
      name: '空字符串返回空串',
      input: '',
      offset: 0,
      pageSize: 100,
      expected: '',
    },
    {
      name: '仅注释返回空串',
      input: '-- 注释',
      offset: 0,
      pageSize: 100,
      expected: '',
    },
    {
      name: '负偏移归零',
      input: 'SELECT 1',
      offset: -10,
      pageSize: 100,
      expected: 'SELECT 1 LIMIT 100 OFFSET 0',
    },
    {
      name: '零页大小归为 1',
      input: 'SELECT 1',
      offset: 0,
      pageSize: 0,
      expected: 'SELECT 1 LIMIT 1 OFFSET 0',
    },
    {
      name: '小数参数取整',
      input: 'SELECT 1',
      offset: 10.7,
      pageSize: 100.3,
      expected: 'SELECT 1 LIMIT 100 OFFSET 10',
    },
    {
      name: '小写 select 保留原样拼接',
      input: 'select 1',
      offset: 0,
      pageSize: 10,
      expected: 'select 1 LIMIT 10 OFFSET 0',
    },
  ]

  it.each(cases)('$name', ({ input, offset, pageSize, expected }) => {
    expect(buildPageSQL(input, offset, pageSize)).toBe(expected)
  })
})

describe('分页状态推进', () => {
  const base: PageState = { offset: 0, pageSize: DEFAULT_PAGE_SIZE }

  const cases: {
    name: string
    state: PageState
    expected: PageState
  }[] = [
    { name: '首页下一页', state: base, expected: { offset: 50000, pageSize: 50000 } },
    { name: '第二页下一页', state: { offset: 50000, pageSize: 50000 }, expected: { offset: 100000, pageSize: 50000 } },
    { name: '自定页大小下一页', state: { offset: 100, pageSize: 100 }, expected: { offset: 200, pageSize: 100 } },
    { name: '零大小下一页不产生非法值', state: { offset: 0, pageSize: 0 }, expected: { offset: 0, pageSize: 0 } },
  ]

  it.each(cases)('nextPage $name', ({ state, expected }) => {
    expect(nextPage(state)).toEqual(expected)
  })

  const prevCases: { name: string; state: PageState; expected: PageState }[] = [
    { name: '第二页回到首页', state: { offset: 50000, pageSize: 50000 }, expected: { offset: 0, pageSize: 50000 } },
    { name: '首页上一页保持 0', state: base, expected: { offset: 0, pageSize: 50000 } },
    { name: '第三页回到第二页', state: { offset: 100000, pageSize: 50000 }, expected: { offset: 50000, pageSize: 50000 } },
  ]

  it.each(prevCases)('prevPage $name', ({ state, expected }) => {
    expect(prevPage(state)).toEqual(expected)
  })
})

describe('页码与按钮可用性', () => {
  const pageCases: { name: string; state: PageState; expected: number }[] = [
    { name: '首页为第 1 页', state: { offset: 0, pageSize: 50000 }, expected: 1 },
    { name: '偏移一页为第 2 页', state: { offset: 50000, pageSize: 50000 }, expected: 2 },
    { name: '偏移两页为第 3 页', state: { offset: 100000, pageSize: 50000 }, expected: 3 },
    { name: '零页大小回退第 1 页', state: { offset: 0, pageSize: 0 }, expected: 1 },
  ]

  it.each(pageCases)('currentPageNumber $name', ({ state, expected }) => {
    expect(currentPageNumber(state)).toBe(expected)
  })

  const prevable: { name: string; state: PageState; expected: boolean }[] = [
    { name: '首页无上一页', state: { offset: 0, pageSize: 50000 }, expected: false },
    { name: '第二页有上一页', state: { offset: 50000, pageSize: 50000 }, expected: true },
  ]

  it.each(prevable)('hasPrevPage $name', ({ state, expected }) => {
    expect(hasPrevPage(state)).toBe(expected)
  })

  const nextable: { name: string; state: PageState; rowCount: number; expected: boolean }[] = [
    { name: '满页可下一页', state: { offset: 0, pageSize: 100 }, rowCount: 100, expected: true },
    { name: '超满页可下一页', state: { offset: 0, pageSize: 100 }, rowCount: 150, expected: true },
    { name: '不满页不可下一页', state: { offset: 0, pageSize: 100 }, rowCount: 99, expected: false },
    { name: '零行不可下一页', state: { offset: 0, pageSize: 100 }, rowCount: 0, expected: false },
  ]

  it.each(nextable)('hasNextPage $name', ({ state, rowCount, expected }) => {
    expect(hasNextPage(state, rowCount)).toBe(expected)
  })

  it('resetPage 默认页大小', () => {
    expect(resetPage()).toEqual({ offset: 0, pageSize: DEFAULT_PAGE_SIZE })
  })

  it('resetPage 指定页大小', () => {
    expect(resetPage(200)).toEqual({ offset: 0, pageSize: 200 })
  })
})