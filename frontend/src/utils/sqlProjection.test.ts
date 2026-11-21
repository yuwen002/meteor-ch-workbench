import { describe, it, expect } from 'vitest'
import {
  findSelectList,
  splitTopLevel,
  parseProjection,
  projectionOutputNames,
  rewriteProjection,
  rewriteProjectionToColumns,
  stripPageTail,
  quoteIdentifier,
} from './sqlProjection'

describe('stripPageTail', () => {
  it('去掉分页 LIMIT / OFFSET 尾部', () => {
    expect(stripPageTail('SELECT a FROM t LIMIT 50000 OFFSET 50000')).toBe('SELECT a FROM t')
  })

  it('大小写与多余空白不影响', () => {
    expect(stripPageTail('SELECT a FROM t   limit 10   offset 0  ')).toBe('SELECT a FROM t')
  })

  it('同时去掉结尾分号', () => {
    expect(stripPageTail('SELECT a FROM t LIMIT 10 OFFSET 0;')).toBe('SELECT a FROM t')
  })

  it('没有分页尾时原样返回', () => {
    expect(stripPageTail('SELECT a FROM t')).toBe('SELECT a FROM t')
  })

  it('普通 LIMIT 不误删（非现有分页格式）', () => {
    expect(stripPageTail('SELECT a FROM t LIMIT 10')).toBe('SELECT a FROM t LIMIT 10')
  })

  it('空输入返回空串', () => {
    expect(stripPageTail('   ')).toBe('')
  })
})

describe('findSelectList', () => {
  it('识别简单 SELECT 的投影区间', () => {
    const sql = 'SELECT a, b FROM t'
    const list = findSelectList(sql)
    expect(list).not.toBeNull()
    expect(sql.slice(list!.start, list!.end).trim()).toBe('a, b')
  })

  it('忽略括号内子查询中的 FROM', () => {
    const sql = 'SELECT a, (SELECT max(x) FROM s) AS m FROM t'
    const list = findSelectList(sql)
    expect(sql.slice(list!.start, list!.end).trim()).toBe('a, (SELECT max(x) FROM s) AS m')
  })

  it('函数参数中的逗号不影响区间', () => {
    const sql = 'SELECT concat(a, b), c FROM t'
    const list = findSelectList(sql)
    expect(sql.slice(list!.start, list!.end).trim()).toBe('concat(a, b), c')
  })

  it('字符串字面量中的 FROM 不算关键字', () => {
    const sql = "SELECT 'from', a FROM t"
    const list = findSelectList(sql)
    expect(sql.slice(list!.start, list!.end).trim()).toBe("'from', a")
  })

  it('WITH 子句返回主查询投影区间', () => {
    const sql = 'WITH x AS (SELECT 1 AS k) SELECT a, b FROM x'
    const list = findSelectList(sql)
    expect(sql.slice(list!.start, list!.end).trim()).toBe('a, b')
  })

  it('UNION 查询判定为不支持', () => {
    expect(findSelectList('SELECT a FROM t UNION SELECT b FROM u')).toBeNull()
  })

  it('多语句判定为不支持', () => {
    expect(findSelectList('SELECT a FROM t; SELECT b FROM u')).toBeNull()
  })

  it('非查询语句判定为不支持', () => {
    expect(findSelectList('INSERT INTO t SELECT a FROM u')).toBeNull()
    expect(findSelectList('EXPLAIN SELECT a FROM t')).toBeNull()
  })

  it('SELECT 无 FROM 判定为不支持', () => {
    expect(findSelectList('SELECT 1')).toBeNull()
  })

  it('空的查询返回 null', () => {
    expect(findSelectList('')).toBeNull()
  })
})

describe('splitTopLevel', () => {
  it('按顶层逗号切分', () => {
    expect(splitTopLevel('a, b, c')).toEqual(['a', 'b', 'c'])
  })

  it('不切分函数参数里的逗号', () => {
    expect(splitTopLevel('concat(a, b), c')).toEqual(['concat(a, b)', 'c'])
  })

  it('不切分引号内的逗号', () => {
    expect(splitTopLevel("'a,b', c")).toEqual(["'a,b'", 'c'])
  })

  it('空输入返回空数组', () => {
    expect(splitTopLevel('')).toEqual([])
    expect(splitTopLevel(' , ')).toEqual([])
  })
})

describe('parseProjection', () => {
  it('解析 AS 别名', () => {
    expect(parseProjection('a AS x')).toEqual({ expr: 'a', alias: 'x' })
  })

  it('解析省略 AS 的别名', () => {
    expect(parseProjection('a x')).toEqual({ expr: 'a', alias: 'x' })
  })

  it('裸列名别名即列名', () => {
    expect(parseProjection('a')).toEqual({ expr: 'a', alias: 'a' })
  })

  it('限定的列名取最后一段', () => {
    expect(parseProjection('t.a')).toEqual({ expr: 't.a', alias: 'a' })
  })

  it('反引号标识符去引号', () => {
    expect(parseProjection('`a b` AS `x y`')).toEqual({ expr: '`a b`', alias: 'x y' })
  })

  it('无别名的表达式返回 null', () => {
    expect(parseProjection('count(*)').alias).toBeNull()
    expect(parseProjection('a + b').alias).toBeNull()
  })
})

describe('projectionOutputNames', () => {
  it('批量解析输出列名', () => {
    expect(projectionOutputNames(['a AS x', 'count(*)', 't.b'])).toEqual(['x', null, 'b'])
  })

  it('空输入返回空数组', () => {
    expect(projectionOutputNames([])).toEqual([])
  })
})

describe('rewriteProjection', () => {
  it('删除指定索引的列', () => {
    const r = rewriteProjection('SELECT a, b, c FROM t', [1])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql).toBe('SELECT a, c FROM t')
  })

  it('删除多列并保留其余顺序', () => {
    const r = rewriteProjection('SELECT a, b, c, d FROM t', [0, 2])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql).toBe('SELECT b, d FROM t')
  })

  it('保留 WHERE / ORDER BY / LIMIT 等尾部', () => {
    const r = rewriteProjection('SELECT a, b FROM t WHERE x = 1 ORDER BY a LIMIT 10', [1])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql).toBe('SELECT a FROM t WHERE x = 1 ORDER BY a LIMIT 10')
  })

  it('保留原有换行缩进的语义等价', () => {
    const r = rewriteProjection('SELECT\n  a,\n  b\nFROM t', [1])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql.replace(/\s+/g, ' ')).toBe('SELECT a FROM t')
  })

  it('不带分号时输出也不带分号', () => {
    const r = rewriteProjection('SELECT a, b FROM t;', [0])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql.endsWith(';')).toBe(false)
  })

  it('空索引列表保持原语句', () => {
    const r = rewriteProjection('SELECT a, b FROM t', [])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql).toBe('SELECT a, b FROM t')
  })

  it('越界索引被忽略', () => {
    const r = rewriteProjection('SELECT a FROM t', [5, -1])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql).toBe('SELECT a FROM t')
  })

  it('删除全部列判定为不支持', () => {
    const r = rewriteProjection('SELECT a, b FROM t', [0, 1])
    expect(r.ok).toBe(false)
  })

  it('不支持语句返回 unsupported', () => {
    const r = rewriteProjection('INSERT INTO t SELECT a FROM u', [0])
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.reason).toBe('unsupported')
  })

  it('保留子查询内部的 FROM 结构', () => {
    const r = rewriteProjection('SELECT a, (SELECT max(x) FROM s) AS m FROM t', [0])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql).toBe('SELECT (SELECT max(x) FROM s) AS m FROM t')
  })
})

describe('rewriteProjectionToColumns', () => {
  it('按列名删除对应表达式', () => {
    const r = rewriteProjectionToColumns('SELECT a AS x, b AS y FROM t', ['y'])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql).toBe('SELECT a AS x FROM t')
  })

  it('裸列名也可按名删除', () => {
    const r = rewriteProjectionToColumns('SELECT a, t.b FROM t', ['b'])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql).toBe('SELECT a FROM t')
  })

  it('列名不存在时原样返回', () => {
    const r = rewriteProjectionToColumns('SELECT a, b FROM t', ['zz'])
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.reason).toBe('not-found')
  })

  it('空列表原语句返回', () => {
    const r = rewriteProjectionToColumns('SELECT a, b FROM t', [])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql).toBe('SELECT a, b FROM t')
  })

  it('删除全部列判定为不支持', () => {
    const r = rewriteProjectionToColumns('SELECT a FROM t', ['a'])
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.reason).toBe('unsupported')
  })
})

describe('rewriteProjectionToColumns 处理 SELECT *', () => {
  it('单列 * 按结果列名展开并删除目标列', () => {
    const r = rewriteProjectionToColumns('SELECT * FROM t WHERE x = 1', ['b'], ['a', 'b', 'c'])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql).toBe('SELECT a, c FROM t WHERE x = 1')
  })

  it('带表前缀的 t.* 展开时保留前缀', () => {
    const r = rewriteProjectionToColumns('SELECT t.* FROM t', ['b'], ['a', 'b', 'c'])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql).toBe('SELECT t.a, t.c FROM t')
  })

  it('WITH 子句里的主查询 * 也能展开', () => {
    const r = rewriteProjectionToColumns(
      'WITH x AS (SELECT 1) SELECT * FROM x',
      ['b'],
      ['a', 'b', 'c'],
    )
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql).toBe('WITH x AS (SELECT 1) SELECT a, c FROM x')
  })

  it('多表 JOIN 的 a.*, b.* 各自展开（前缀各自保留）', () => {
    const r = rewriteProjectionToColumns(
      'SELECT a.*, b.* FROM a JOIN b ON a.id = b.id',
      ['c'],
      ['a', 'b', 'c'],
    )
    expect(r.ok).toBe(true)
    if (r.ok) {
      expect(r.sql).toBe('SELECT a.a, a.b, b.a, b.b FROM a JOIN b ON a.id = b.id')
    }
  })

  it('缺少结果列名时返回 not-found 而不是生成错误 SQL', () => {
    const r = rewriteProjectionToColumns('SELECT * FROM t', ['b'], [])
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.reason).toBe('not-found')
  })

  it('要删的列不在结果集中时：单列 * 仍会展开并保持全部列', () => {
    const r = rewriteProjectionToColumns('SELECT * FROM t', ['zz'], ['a', 'b'])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql).toBe('SELECT a, b FROM t')
  })

  it('特殊列名自动加反引号', () => {
    const r = rewriteProjectionToColumns('SELECT * FROM t', ['b'], ['a', 'b', 'c d'])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql).toBe('SELECT a, `c d` FROM t')
  })

  it('混合投影按列名删除，保留函数表达式', () => {
    const r = rewriteProjectionToColumns('SELECT count(*), a FROM t', ['a'], ['a', 'b'])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql).toBe('SELECT count(*) FROM t')
  })

  it('显式 * 与表达式混排时不展开（安全失败）', () => {
    const r = rewriteProjectionToColumns('SELECT *, 1 AS k FROM t', ['a'], ['a', 'b'])
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.reason).toBe('not-found')
  })

  it('显式 * 与表达式混排时删除表达式列仍可行', () => {
    const r = rewriteProjectionToColumns('SELECT *, 1 AS k FROM t', ['k'], ['a', 'b'])
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.sql).toBe('SELECT * FROM t')
  })
})

describe('quoteIdentifier', () => {
  it('普通标识符不加引号', () => {
    expect(quoteIdentifier('abc')).toBe('abc')
  })
  it('含空格加反引号', () => {
    expect(quoteIdentifier('a b')).toBe('`a b`')
  })
  it('含反引号转义', () => {
    expect(quoteIdentifier('a`b')).toBe('`a``b`')
  })
  it('中文列名加反引号', () => {
    expect(quoteIdentifier('用户名')).toBe('`用户名`')
  })
})