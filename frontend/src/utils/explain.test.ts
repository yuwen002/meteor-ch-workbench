import { describe, it, expect } from 'vitest'
import { canExplain, buildExplainSQL, getExplainKeyword, splitStatements, EXPLAIN_MODES } from './explain'

describe('splitStatements', () => {
  const cases: { name: string; sql: string; want: string[] }[] = [
    { name: '单条语句', sql: 'SELECT 1', want: ['SELECT 1'] },
    { name: '两条语句', sql: 'SELECT 1; SELECT 2', want: ['SELECT 1', 'SELECT 2'] },
    { name: '尾部多余分号', sql: 'SELECT 1;;  ', want: ['SELECT 1'] },
    { name: '空字符串', sql: '   ', want: [] },
    { name: '行注释与块注释被剥离', sql: '-- 注释\nSELECT 1 /* 块注释 */', want: ['SELECT 1'] },
    { name: '注释里的分号不影响切分', sql: 'SELECT 1 -- ;\n', want: ['SELECT 1'] },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(splitStatements(c.sql)).toEqual(c.want)
    })
  }
})

describe('canExplain', () => {
  const cases: { name: string; sql: string; want: boolean }[] = [
    { name: '单条 SELECT 可解释', sql: 'SELECT * FROM t', want: true },
    { name: '带 WITH 的查询可解释', sql: 'WITH 1 AS x SELECT x', want: true },
    { name: '前后空白的单条 SELECT', sql: '  \n SELECT 1\n ', want: true },
    { name: '空内容不可解释', sql: '', want: false },
    { name: '仅注释不可解释', sql: '-- nothing here', want: false },
    { name: '多条语句不可解释', sql: 'SELECT 1; SELECT 2', want: false },
    { name: 'INSERT 不可解释', sql: 'INSERT INTO t VALUES (1)', want: false },
    { name: 'CREATE 不可解释', sql: 'CREATE TABLE t (a Int32) ENGINE = Memory', want: false },
    { name: 'DROP 不可解释', sql: 'DROP TABLE t', want: false },
    { name: '已有 EXPLAIN 不可再嵌套', sql: 'EXPLAIN SELECT 1', want: false },
    { name: '小写 insert 不可解释', sql: 'insert into t values (1)', want: false },
    { name: '非 SELECT 但可解释的 SHOW 允许', sql: 'SHOW TABLES', want: true },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(canExplain(c.sql)).toBe(c.want)
    })
  }
})

describe('getExplainKeyword', () => {
  const cases: { name: string; mode: (typeof EXPLAIN_MODES)[number]['id']; want: string }[] = [
    { name: '默认 PLAN', mode: 'PLAN', want: 'EXPLAIN' },
    { name: 'AST 模式', mode: 'AST', want: 'EXPLAIN AST' },
    { name: 'PLAN 详情模式', mode: 'PLAN_DETAIL', want: 'EXPLAIN PLAN' },
    { name: 'PIPELINE 模式', mode: 'PIPELINE', want: 'EXPLAIN PIPELINE' },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(getExplainKeyword(c.mode)).toBe(c.want)
    })
  }
})

describe('buildExplainSQL', () => {
  const cases: { name: string; sql: string; mode: (typeof EXPLAIN_MODES)[number]['id']; want: string }[] = [
    { name: '默认模式拼接', sql: 'SELECT * FROM t', mode: 'PLAN', want: 'EXPLAIN SELECT * FROM t' },
    { name: 'AST 模式拼接', sql: 'SELECT 1', mode: 'AST', want: 'EXPLAIN AST SELECT 1' },
    { name: 'PIPELINE 模式拼接', sql: 'SELECT 1', mode: 'PIPELINE', want: 'EXPLAIN PIPELINE SELECT 1' },
    { name: '去除尾部多余分号', sql: 'SELECT 1;', mode: 'PLAN', want: 'EXPLAIN SELECT 1' },
    { name: '多语句只取首条', sql: 'SELECT 1; SELECT 2', mode: 'PLAN', want: 'EXPLAIN SELECT 1' },
    { name: '剥离前置注释', sql: '-- 注释\nSELECT 1', mode: 'PLAN', want: 'EXPLAIN SELECT 1' },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(buildExplainSQL(c.sql, c.mode)).toBe(c.want)
    })
  }

  it('空内容返回空字符串', () => {
    expect(buildExplainSQL('', 'PLAN')).toBe('')
    expect(buildExplainSQL('   ', 'PLAN')).toBe('')
  })

  it('未指定模式退化为默认 PLAN', () => {
    expect(buildExplainSQL('SELECT 1')).toBe('EXPLAIN SELECT 1')
  })
})