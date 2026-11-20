import { describe, it, expect } from 'vitest'
import {
  FILTER_EQ,
  FILTER_NE,
  FILTER_GT,
  FILTER_GE,
  FILTER_LT,
  FILTER_LE,
  FILTER_LIKE,
  FILTER_NOT_LIKE,
  FILTER_OP_ORDER,
  FILTER_JOIN_AND,
  FILTER_JOIN_OR,
  appendSort,
  appendWhere,
  buildFilterWhere,
  conditionExpression,
  conditionPreview,
  countActiveRules,
  filterExpression,
  isConditionable,
  isNumericType,
  opSymbol,
  rawFilterExpression,
  valuePreview,
} from './resultFilter'

describe('valuePreview', () => {
  const cases: { name: string; value: unknown; max?: number; want: string }[] = [
    { name: 'null 显示 NULL', value: null, want: 'NULL' },
    { name: 'undefined 显示 NULL', value: undefined, want: 'NULL' },
    { name: '短字符串原样', value: 'abc', want: 'abc' },
    { name: '数字转字符串', value: 42, want: '42' },
    { name: '正好等于上限不截断', value: '0123456789abcdefgh', want: '0123456789abcdefgh' },
    { name: '长字符串中间省略', value: '0123456789abcdefghijk', want: '0123456...defghijk' },
    { name: '自定义长度', value: 'abcdefgh', max: 5, want: 'a...h' },
    { name: '空字符串', value: '', want: '' },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(valuePreview(c.value, c.max)).toBe(c.want)
    })
  }
})

describe('opSymbol', () => {
  it('等于与不等于', () => {
    expect(opSymbol(FILTER_EQ)).toBe('=')
    expect(opSymbol(FILTER_NE)).toBe('<>')
  })
  it('大于小于', () => {
    expect(opSymbol(FILTER_GT)).toBe('>')
    expect(opSymbol(FILTER_LT)).toBe('<')
  })
  it('LIKE 系列带不带 NOT', () => {
    expect(opSymbol(FILTER_LIKE)).toBe('LIKE')
    expect(opSymbol(FILTER_NOT_LIKE)).toBe('NOT LIKE')
  })
  it('操作符顺序固定', () => {
    expect(FILTER_OP_ORDER).toEqual([
      FILTER_EQ,
      FILTER_NE,
      FILTER_GT,
      FILTER_GE,
      FILTER_LT,
      FILTER_LE,
      FILTER_LIKE,
      FILTER_NOT_LIKE,
    ])
  })
})

describe('filterExpression', () => {
  const cases: { name: string; col: string; op: any; value: unknown; pct?: boolean; want: string }[] =
    [
      { name: '字符串等值带引号', col: 'log_id', op: FILTER_EQ, value: 'abc', want: "`log_id` = 'abc'" },
      { name: '数字等值不带引号', col: 'n', op: FILTER_EQ, value: 12, want: '`n` = 12' },
      { name: '单引号转义', col: 'c', op: FILTER_EQ, value: "a'b", want: "`c` = 'a\\'b'" },
      { name: '空值转 IS NULL', col: 'c', op: FILTER_EQ, value: null, want: '`c` IS NULL' },
      { name: '不等于空值转 IS NOT NULL', col: 'c', op: FILTER_NE, value: null, want: '`c` IS NOT NULL' },
      { name: 'LIKE 自动补通配符', col: 'log_id', op: FILTER_LIKE, value: 'abc', want: "`log_id` LIKE '%abc%'" },
      { name: 'LIKE 已含通配符不重复补', col: 'log_id', op: FILTER_LIKE, value: '%abc', want: "`log_id` LIKE '%abc'" },
      { name: 'NOT LIKE', col: 'log_id', op: FILTER_NOT_LIKE, value: 'abc', want: "`log_id` NOT LIKE '%abc%'" },
      { name: '百分号形式带引号转义', col: 'log_id', op: FILTER_LIKE, value: 'abc', pct: true, want: "`log_id` LIKE '%abc%'" },
      { name: '百分号形式值含单引号', col: 'c', op: FILTER_LIKE, value: "a'b", pct: true, want: "`c` LIKE '%a\\'b%'" },
      { name: '大于数字', col: 'n', op: FILTER_GT, value: 5, want: '`n` > 5' },
      { name: '列名含特殊字符转义', col: 'my col', op: FILTER_EQ, value: 'x', want: "`my col` = 'x'" },
    ]
  for (const c of cases) {
    it(c.name, () => {
      expect(filterExpression(c.col, c.op, c.value, c.pct)).toBe(c.want)
    })
  }
})

describe('conditionPreview', () => {
  it('等值预览截断长值', () => {
    expect(conditionPreview('log_id', FILTER_EQ, 'a510809-4b7c-4d1e-9a2f-722297fc6')).toBe(
      'log_id = a510809...22297fc6'
    )
  })
  it('LIKE 预览', () => {
    expect(conditionPreview('log_id', FILTER_LIKE, 'abc')).toBe('log_id LIKE abc')
  })
  it('空值预览', () => {
    expect(conditionPreview('log_id', FILTER_EQ, null)).toBe('log_id = NULL')
  })
})

describe('isConditionable', () => {
  const cases: { name: string; sql: string; want: boolean }[] = [
    { name: 'SELECT 可加条件', sql: 'SELECT 1', want: true },
    { name: 'WITH 可加条件', sql: 'WITH x AS (SELECT 1) SELECT 1', want: true },
    { name: '小写 select 可加条件', sql: ' select 1', want: true },
    { name: 'SHOW 不可加条件', sql: 'SHOW TABLES', want: false },
    { name: 'DDL 不可加条件', sql: 'CREATE TABLE t (a Int8)', want: false },
    { name: '空语句不可加条件', sql: '   ', want: false },
    { name: 'EXPLAIN 不可加条件', sql: 'EXPLAIN SELECT 1', want: false },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(isConditionable(c.sql)).toBe(c.want)
    })
  }
})

describe('appendSort', () => {
  const cases: { name: string; sql: string; sorts: any[]; want: string }[] = [
    {
      name: '无 ORDER BY 时追加',
      sql: 'SELECT a, b FROM t',
      sorts: [{ column: 'export_time', dir: 'ASC' }],
      want: 'SELECT a, b FROM t\nORDER BY `export_time` ASC',
    },
    {
      name: '降序追加',
      sql: 'SELECT a FROM t',
      sorts: [{ column: 'export_time', dir: 'DESC' }],
      want: 'SELECT a FROM t\nORDER BY `export_time` DESC',
    },
    {
      name: '多列排序',
      sql: 'SELECT a FROM t',
      sorts: [
        { column: 'x', dir: 'ASC' },
        { column: 'y', dir: 'DESC' },
      ],
      want: 'SELECT a FROM t\nORDER BY `x` ASC, `y` DESC',
    },
    {
      name: '替换已有 ORDER BY',
      sql: 'SELECT a FROM t ORDER BY a DESC',
      sorts: [{ column: 'b', dir: 'ASC' }],
      want: 'SELECT a FROM t\nORDER BY `b` ASC',
    },
    {
      name: 'LIMIT 前插入',
      sql: 'SELECT a FROM t LIMIT 10',
      sorts: [{ column: 'a', dir: 'ASC' }],
      want: 'SELECT a FROM t\nORDER BY `a` ASC\nLIMIT 10',
    },
    {
      name: '带 FORMAT 时排在 FORMAT 前',
      sql: 'SELECT a FROM t FORMAT JSON',
      sorts: [{ column: 'a', dir: 'ASC' }],
      want: 'SELECT a FROM t\nORDER BY `a` ASC\nFORMAT JSON',
    },
    { name: '非 SELECT 返回空串', sql: 'SHOW TABLES', sorts: [{ column: 'a', dir: 'ASC' }], want: '' },
    {
      name: '多语句返回空串',
      sql: 'SELECT 1; SELECT 2',
      sorts: [{ column: 'a', dir: 'ASC' }],
      want: '',
    },
    { name: '空排序列表只返回原语句', sql: 'SELECT a FROM t', sorts: [], want: 'SELECT a FROM t' },
    {
      name: '保留结尾分号外的空白',
      sql: 'SELECT a FROM t;',
      sorts: [{ column: 'a', dir: 'ASC' }],
      want: 'SELECT a FROM t\nORDER BY `a` ASC',
    },
    {
      name: '保留前置注释',
      sql: '-- note\nSELECT a FROM t',
      sorts: [{ column: 'a', dir: 'ASC' }],
      want: '-- note\nSELECT a FROM t\nORDER BY `a` ASC',
    },
    {
      name: '列名含反引号转义',
      sql: 'SELECT 1 FROM t',
      sorts: [{ column: 'a`b', dir: 'ASC' }],
      want: 'SELECT 1 FROM t\nORDER BY `a``b` ASC',
    },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(appendSort(c.sql, c.sorts)).toBe(c.want)
    })
  }
})

describe('appendWhere', () => {
  const cases: { name: string; sql: string; conds: string[]; want: string }[] = [
    {
      name: '无 WHERE 追加',
      sql: 'SELECT a FROM t',
      conds: ["log_id = 'x'"],
      want: "SELECT a FROM t\nWHERE log_id = 'x'",
    },
    {
      name: '已有 WHERE 用 AND 合并',
      sql: 'SELECT a FROM t WHERE a > 0',
      conds: ["b = 'y'"],
      want: "SELECT a FROM t WHERE a > 0 AND b = 'y'",
    },
    {
      name: '多个条件 AND 连接',
      sql: 'SELECT a FROM t',
      conds: ['a = 1', 'b = 2'],
      want: 'SELECT a FROM t\nWHERE a = 1 AND b = 2',
    },
    {
      name: 'WHERE 插在 GROUP BY 之前',
      sql: 'SELECT a, count() FROM t GROUP BY a',
      conds: ['a = 1'],
      want: 'SELECT a, count() FROM t\nWHERE a = 1\nGROUP BY a',
    },
    {
      name: '已有 WHERE 且插入在 ORDER BY 之前',
      sql: 'SELECT a FROM t WHERE a > 0 ORDER BY a',
      conds: ['b = 1'],
      want: 'SELECT a FROM t WHERE a > 0 AND b = 1\nORDER BY a',
    },
    {
      name: 'LIMIT 前插入',
      sql: 'SELECT a FROM t LIMIT 5',
      conds: ['a = 1'],
      want: 'SELECT a FROM t\nWHERE a = 1\nLIMIT 5',
    },
    { name: '非 SELECT 返回空串', sql: 'SHOW TABLES', conds: ['a = 1'], want: '' },
    { name: '空条件只返回原语句', sql: 'SELECT a FROM t', conds: [], want: 'SELECT a FROM t' },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(appendWhere(c.sql, c.conds)).toBe(c.want)
    })
  }
})

describe('多条件筛选（Navicat 风格）', () => {
  it('新增比较操作符符号正确', () => {
    expect(opSymbol(FILTER_GE)).toBe('>=')
    expect(opSymbol(FILTER_LE)).toBe('<=')
  })

  it('菜单操作符顺序固定', () => {
    expect(FILTER_OP_ORDER).toEqual([
      FILTER_EQ,
      FILTER_NE,
      FILTER_GT,
      FILTER_GE,
      FILTER_LT,
      FILTER_LE,
      FILTER_LIKE,
      FILTER_NOT_LIKE,
    ])
  })

  const exprCases: { name: string; column: string; rules: { op: any; value: string; numeric?: boolean }[]; want: string }[] = [
    {
      name: '单条规则生成一个条件',
      column: 'name',
      rules: [{ op: FILTER_EQ, value: 'abc' }],
      want: "`name` = 'abc'",
    },
    {
      name: '同列多条规则用 OR 组合并加括号',
      column: 'name',
      rules: [{ op: FILTER_EQ, value: 'abc' }, { op: FILTER_EQ, value: 'def' }],
      want: "(`name` = 'abc' OR `name` = 'def')",
    },
    {
      name: '空值规则被忽略',
      column: 'name',
      rules: [{ op: FILTER_EQ, value: '  ' }, { op: FILTER_EQ, value: 'abc' }],
      want: "`name` = 'abc'",
    },
    {
      name: '单引号转义',
      column: 'name',
      rules: [{ op: FILTER_EQ, value: "a'b" }],
      want: "`name` = 'a\\'b'",
    },
    {
      name: '文本列数字仍加引号',
      column: 'age',
      rules: [{ op: FILTER_GT, value: '18' }],
      want: "`age` > '18'",
    },
    {
      name: '数值列不加引号',
      column: 'age',
      rules: [{ op: FILTER_GT, value: '18', numeric: true }],
      want: '`age` > 18',
    },
    {
      name: '数值列但输入非数字时回退加引号',
      column: 'age',
      rules: [{ op: FILTER_EQ, value: 'abc', numeric: true }],
      want: "`age` = 'abc'",
    },
    {
      name: 'LIKE 保留用户输入的百分号',
      column: 'name',
      rules: [{ op: FILTER_LIKE, value: '%ab%' }],
      want: "`name` LIKE '%ab%'",
    },
    {
      name: 'LIKE 未含通配时自动补',
      column: 'name',
      rules: [{ op: FILTER_LIKE, value: 'ab' }],
      want: "`name` LIKE '%ab%'",
    },
    {
      name: 'NOT LIKE 自动补通配',
      column: 'name',
      rules: [{ op: FILTER_NOT_LIKE, value: 'ab' }],
      want: "`name` NOT LIKE '%ab%'",
    },
    {
      name: '列名含特殊字符时反引号转义',
      column: 'my col',
      rules: [{ op: FILTER_EQ, value: '1' }],
      want: "`my col` = '1'",
    },
    {
      name: '数值列含百分号仍按 LIKE 处理',
      column: 'age',
      rules: [{ op: FILTER_LIKE, value: '18', numeric: true }],
      want: "`age` LIKE '%18%'",
    },
  ]
  for (const c of exprCases) {
    it(c.name, () => {
      expect(conditionExpression({ column: c.column, join: FILTER_JOIN_AND, rules: c.rules })).toBe(c.want)
    })
  }

  it('全部规则为空返回空串', () => {
    expect(conditionExpression({ column: 'name', join: FILTER_JOIN_AND, rules: [{ op: FILTER_EQ, value: '' }] })).toBe('')
  })

  const whereCases: { name: string; conditions: any[]; want: string }[] = [
    {
      name: '单条件无连接词前缀',
      conditions: [{ column: 'a', join: FILTER_JOIN_AND, rules: [{ op: FILTER_EQ, value: '1' }] }],
      want: "`a` = '1'",
    },
    {
      name: '两个条件用 AND 连接',
      conditions: [
        { column: 'a', join: FILTER_JOIN_AND, rules: [{ op: FILTER_EQ, value: '1' }] },
        { column: 'b', join: FILTER_JOIN_AND, rules: [{ op: FILTER_EQ, value: '2' }] },
      ],
      want: "`a` = '1' AND `b` = '2'",
    },
    {
      name: 'OR 连接生效',
      conditions: [
        { column: 'a', join: FILTER_JOIN_AND, rules: [{ op: FILTER_EQ, value: '1' }] },
        { column: 'b', join: FILTER_JOIN_OR, rules: [{ op: FILTER_EQ, value: '2' }] },
      ],
      want: "`a` = '1' OR `b` = '2'",
    },
    {
      name: '跳过空条件组',
      conditions: [
        { column: 'a', join: FILTER_JOIN_AND, rules: [{ op: FILTER_EQ, value: '' }] },
        { column: 'b', join: FILTER_JOIN_AND, rules: [{ op: FILTER_EQ, value: 'x' }] },
      ],
      want: "`b` = 'x'",
    },
    {
      name: '数值列与文本列混合',
      conditions: [
        { column: 'age', join: FILTER_JOIN_AND, rules: [{ op: FILTER_GE, value: '18', numeric: true }] },
        { column: 'name', join: FILTER_JOIN_AND, rules: [{ op: FILTER_EQ, value: 'abc' }] },
      ],
      want: "`age` >= 18 AND `name` = 'abc'",
    },
  ]
  for (const c of whereCases) {
    it(c.name, () => {
      expect(buildFilterWhere(c.conditions)).toBe(c.want)
    })
  }

  it('无有效条件返回空串', () => {
    expect(buildFilterWhere([])).toBe('')
  })

  it('统计有效规则数量', () => {
    expect(
      countActiveRules([
        { column: 'a', join: FILTER_JOIN_AND, rules: [{ op: FILTER_EQ, value: '1' }, { op: FILTER_EQ, value: ' ' }] },
        { column: 'b', join: FILTER_JOIN_AND, rules: [{ op: FILTER_EQ, value: '2' }] },
      ])
    ).toBe(2)
  })

  it('按值过滤与多条件筛选语义一致（LIKE 自动补通配）', () => {
    expect(filterExpression('name', FILTER_LIKE, 'ab')).toBe("`name` LIKE '%ab%'")
    expect(rawFilterExpression('name', FILTER_LIKE, 'ab')).toBe("`name` LIKE '%ab%'")
  })

  it('rawFilterExpression 数值列不带引号', () => {
    expect(rawFilterExpression('age', FILTER_GT, '18', true)).toBe('`age` > 18')
    expect(rawFilterExpression('age', FILTER_GT, '18', false)).toBe("`age` > '18'")
  })

  const numericTypeCases: { name: string; type: string | null | undefined; want: boolean }[] = [
    { name: 'UInt8 是数值', type: 'UInt8', want: true },
    { name: 'Nullable(Int64) 是数值', type: 'Nullable(Int64)', want: true },
    { name: 'Float64 是数值', type: 'Float64', want: true },
    { name: 'Decimal(10, 2) 是数值', type: 'Decimal(10, 2)', want: true },
    { name: 'String 不是数值', type: 'String', want: false },
    { name: 'DateTime 不是数值', type: 'DateTime', want: false },
    { name: '类型缺失按文本', type: undefined, want: false },
    { name: '空类型按文本', type: '', want: false },
  ]
  for (const c of numericTypeCases) {
    it(c.name, () => {
      expect(isNumericType(c.type)).toBe(c.want)
    })
  }

  it('组合条件可直接用于 appendWhere', () => {
    const where = buildFilterWhere([
      { column: 'a', join: FILTER_JOIN_AND, rules: [{ op: FILTER_EQ, value: '1' }] },
    ])
    expect(appendWhere('SELECT a FROM t', [where])).toBe("SELECT a FROM t\nWHERE `a` = '1'")
  })
})