import { describe, it, expect } from 'vitest'
import {
  escapeSqlString,
  sqlLiteral,
  quoteIdent,
  rowToTSV,
  columnsToCSV,
  buildInsertStatement,
  buildInsertBatch,
  insertTarget,
  PENDING_TABLE_PLACEHOLDER,
} from './sqlLiteral'

describe('escapeSqlString', () => {
  const cases: { name: string; input: string; want: string }[] = [
    { name: '普通字符串', input: 'abc', want: "'abc'" },
    { name: '单引号转义', input: "a'b", want: "'a\\'b'" },
    { name: '反斜杠转义', input: 'a\\b', want: "'a\\\\b'" },
    { name: '换行保持原样但包裹引号', input: 'a\nb', want: "'a\nb'" },
    { name: '反斜杠优先于单引号', input: "\\'", want: "'\\\\\\''" },
    { name: '空字符串', input: '', want: "''" },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(escapeSqlString(c.input)).toBe(c.want)
    })
  }
})

describe('sqlLiteral', () => {
  const cases: { name: string; input: unknown; want: string }[] = [
    { name: 'null 转 NULL', input: null, want: 'NULL' },
    { name: 'undefined 转 NULL', input: undefined, want: 'NULL' },
    { name: '整数', input: 42, want: '42' },
    { name: '浮点数', input: 1.5, want: '1.5' },
    { name: 'NaN 转 NULL', input: Number.NaN, want: 'NULL' },
    { name: 'Infinity 转 NULL', input: Number.POSITIVE_INFINITY, want: 'NULL' },
    { name: 'true 转 1', input: true, want: '1' },
    { name: 'false 转 0', input: false, want: '0' },
    { name: '字符串加引号', input: 'hello', want: "'hello'" },
    { name: '字符串含单引号', input: "it's", want: "'it\\'s'" },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(sqlLiteral(c.input)).toBe(c.want)
    })
  }
})

describe('quoteIdent', () => {
  const cases: { name: string; input: string; want: string }[] = [
    { name: '普通标识符', input: 'users', want: '`users`' },
    { name: '含反引号标识符', input: 'a`b', want: '`a``b`' },
    { name: '空标识符', input: '', want: '``' },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(quoteIdent(c.input)).toBe(c.want)
    })
  }
})

describe('rowToTSV', () => {
  it('普通行以制表符连接', () => {
    expect(rowToTSV([1, 'a', true])).toBe('1\ta\ttrue')
  })
  it('null 与 undefined 转为空', () => {
    expect(rowToTSV([1, null, undefined, 2])).toBe('1\t\t\t2')
  })
})

describe('columnsToCSV', () => {
  it('列名逗号分隔', () => {
    expect(columnsToCSV(['a', 'b', 'c'])).toBe('a, b, c')
  })
  it('空列返回空字符串', () => {
    expect(columnsToCSV([])).toBe('')
  })
})

describe('buildInsertStatement', () => {
  it('生成单行 INSERT', () => {
    expect(buildInsertStatement('`db`.`t`', ['a', 'b'], [1, 'x'])).toBe(
      "INSERT INTO `db`.`t` (`a`, `b`) VALUES (1, 'x');",
    )
  })
  it('null 值渲染为 NULL', () => {
    expect(buildInsertStatement('t', ['a'], [null])).toBe('INSERT INTO t (`a`) VALUES (NULL);')
  })
})

describe('buildInsertBatch', () => {
  it('多行以逗号分隔、末行分号', () => {
    const out = buildInsertBatch('t', ['a'], [[1], [2], [3]])
    expect(out).toBe('INSERT INTO t (`a`) VALUES\n  (1),\n  (2),\n  (3);')
  })
  it('单行批次末行带分号', () => {
    expect(buildInsertBatch('t', ['a'], [[1]])).toBe('INSERT INTO t (`a`) VALUES\n  (1);')
  })
  it('空批次只有头部', () => {
    expect(buildInsertBatch('t', ['a'], [])).toBe('INSERT INTO t (`a`) VALUES\n')
  })
})

describe('insertTarget', () => {
  it('库表都存在时返回限定名', () => {
    expect(insertTarget('db', 't')).toBe('`db`.`t`')
  })
  it('仅有表名时返回表名', () => {
    expect(insertTarget(null, 't')).toBe('`t`')
  })
  it('无表名时返回占位符', () => {
    expect(insertTarget(null, null)).toBe(PENDING_TABLE_PLACEHOLDER)
    expect(insertTarget('db', null)).toBe(PENDING_TABLE_PLACEHOLDER)
  })
})