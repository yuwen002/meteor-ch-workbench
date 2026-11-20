import { describe, expect, it } from 'vitest'
import {
  buildAlterUpdateSQL,
  buildKeyCondition,
  buildRowKeys,
  canEditCell,
  detectKeyColumns,
  parseQuerySource,
  valueForEdit,
  type KeyColumn,
} from './cellEdit'

describe('parseQuerySource', () => {
  const cases: Array<{ name: string; sql: string; want: ReturnType<typeof parseQuerySource> }> = [
    { name: '带库名的简单查询', sql: 'SELECT * FROM maas_ops.dws_log LIMIT 10', want: { database: 'maas_ops', table: 'dws_log' } },
    { name: '不带库名', sql: 'SELECT id FROM dws_log', want: { database: null, table: 'dws_log' } },
    { name: '反引号包裹的标识符', sql: 'SELECT * FROM `maas_ops`.`dws_log`', want: { database: 'maas_ops', table: 'dws_log' } },
    { name: '小写 from', sql: 'select * from t1', want: { database: null, table: 't1' } },
    { name: '带注释干扰', sql: '-- 说明\n/* 块注释 */ SELECT * FROM db.t1', want: { database: 'db', table: 't1' } },
    { name: '含 JOIN 不定位', sql: 'SELECT * FROM a JOIN b ON a.id = b.id', want: null },
    { name: '含 UNION 不定位', sql: 'SELECT * FROM a UNION ALL SELECT * FROM b', want: null },
    { name: 'FROM 子查询不定位', sql: 'SELECT * FROM (SELECT 1)', want: null },
    { name: '表值函数 numbers 不定位', sql: 'SELECT * FROM numbers(10)', want: null },
    { name: '空语句', sql: '', want: null },
    { name: '无 FROM 子句', sql: 'SELECT 1', want: null },
    { name: 'UPDATE 语句可定位', sql: 'UPDATE db.t1 SET a = 1', want: { database: 'db', table: 't1' } },
  ]

  for (const c of cases) {
    it(c.name, () => {
      expect(parseQuerySource(c.sql)).toEqual(c.want)
    })
  }
})

describe('canEditCell', () => {
  const keys = (...vals: unknown[]): KeyColumn[] => vals.map((value, i) => ({ name: `k${i}`, value }))
  const cases: Array<{ name: string; source: ReturnType<typeof parseQuerySource>; keys: KeyColumn[]; want: boolean }> = [
    { name: '源表可定位且有主键值', source: { database: 'db', table: 't' }, keys: keys('a'), want: true },
    { name: '源表为 null 不可编辑', source: null, keys: keys('a'), want: false },
    { name: '无主键列不可编辑', source: { database: 'db', table: 't' }, keys: [], want: false },
    { name: '主键值含 NULL 不可编辑', source: { database: 'db', table: 't' }, keys: keys('a', null), want: false },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(canEditCell(c.source, c.keys)).toBe(c.want)
    })
  }
})

describe('buildKeyCondition', () => {
  const typeOf = (name: string): string | null => (name === 'id' ? 'UInt64' : name === 'name' ? 'String' : null)
  const cases: Array<{ name: string; keys: KeyColumn[]; want: string }> = [
    { name: '单字符串主键', keys: [{ name: 'name', value: "o'k" }], want: "`name` = 'o\\'k'" },
    { name: '数值类型不加引号', keys: [{ name: 'id', value: 42 }], want: '`id` = 42' },
    { name: '数值类型字符串值也裸写', keys: [{ name: 'id', value: '42' }], want: '`id` = 42' },
    { name: '多个主键用 AND 连接', keys: [{ name: 'id', value: 1 }, { name: 'name', value: 'a' }], want: '`id` = 1 AND `name` = \'a\'' },
    { name: '空主键返回空串', keys: [], want: '' },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(buildKeyCondition(c.keys, typeOf)).toBe(c.want)
    })
  }
})

describe('buildAlterUpdateSQL', () => {
  const typeOf = (name: string): string | null => (name === 'cnt' ? 'UInt32' : 'String')
  const cases: Array<{ name: string; args: Parameters<typeof buildAlterUpdateSQL>; want: string }> = [
    {
      name: '带库名生成 ALTER UPDATE',
      args: [{ database: 'db', table: 't' }, { column: 'name', oldValue: 'a', newValue: 'b' }, [{ name: 'id', value: 1 }], typeOf],
      want: "ALTER TABLE `db`.`t`\nUPDATE `name` = 'b'\nWHERE `id` = 1;",
    },
    {
      name: '不带库名',
      args: [{ database: null, table: 't' }, { column: 'name', oldValue: null, newValue: "x'y" }, [{ name: 'id', value: '7' }], typeOf],
      want: "ALTER TABLE `t`\nUPDATE `name` = 'x\\'y'\nWHERE `id` = '7';",
    },
    {
      name: '数值列不加引号',
      args: [{ database: 'db', table: 't' }, { column: 'cnt', oldValue: 1, newValue: '9' }, [{ name: 'cnt', value: 3 }], typeOf],
      want: 'ALTER TABLE `db`.`t`\nUPDATE `cnt` = 9\nWHERE `cnt` = 3;',
    },
    {
      name: '源表为空返回空串',
      args: [null, { column: 'a', oldValue: 1, newValue: '2' }, [{ name: 'id', value: 1 }], typeOf],
      want: '',
    },
    {
      name: '无主键返回空串',
      args: [{ database: 'db', table: 't' }, { column: 'a', oldValue: 1, newValue: '2' }, [], typeOf],
      want: '',
    },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(buildAlterUpdateSQL(...c.args)).toBe(c.want)
    })
  }
})

describe('valueForEdit', () => {
  const cases: Array<{ name: string; raw: unknown; want: string }> = [
    { name: '字符串原样', raw: 'abc', want: 'abc' },
    { name: '数字转字符串', raw: 123, want: '123' },
    { name: 'null 转空串', raw: null, want: '' },
    { name: 'undefined 转空串', raw: undefined, want: '' },
    { name: '布尔转字符串', raw: false, want: 'false' },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(valueForEdit(c.raw)).toBe(c.want)
    })
  }
})

describe('detectKeyColumns', () => {
  const cols = (...names: string[]) => names.map((name) => ({ name }))
  const cases: Array<{
    name: string
    input: Parameters<typeof detectKeyColumns>[0]
    want: ReturnType<typeof detectKeyColumns>
  }> = [
    {
      name: '有主键时优先使用主键',
      input: { columns: [{ name: 'id', is_primary: true }, { name: 'v', is_primary: true }, { name: 'name' }], rows: [['a', 'b', 'x']] },
      want: { indexes: [0, 1], reason: 'primary' },
    },
    {
      name: '无主键时使用首个唯一列',
      input: { columns: cols('v', 'id'), rows: [[1, 10], [2, 20], [3, 30]] },
      want: { indexes: [0], reason: 'unique' },
    },
    {
      name: '多列都唯一时取第一个',
      input: { columns: cols('a', 'b'), rows: [[1, 10], [2, 20]] },
      want: { indexes: [0], reason: 'unique' },
    },
    {
      name: '无主键无唯一列时退化为全部列',
      input: { columns: cols('a', 'b'), rows: [[1, 1], [1, 1]] },
      want: { indexes: [0, 1], reason: 'all' },
    },
    {
      name: '唯一列含 NULL 则不算唯一',
      input: { columns: cols('a', 'b'), rows: [[null, 1], [2, 1]] },
      want: { indexes: [0, 1], reason: 'all' },
    },
    {
      name: '无数据行时退化为全部列',
      input: { columns: cols('a', 'b'), rows: [] },
      want: { indexes: [0, 1], reason: 'all' },
    },
    {
      name: '无列时返回空',
      input: { columns: [], rows: [] },
      want: { indexes: [], reason: 'all' },
    },
    {
      name: '主键优先于唯一列',
      input: { columns: [{ name: 'id', is_primary: true }, { name: 'u' }], rows: [['k1', 'v1'], ['k2', 'v2']] },
      want: { indexes: [0], reason: 'primary' },
    },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(detectKeyColumns(c.input)).toEqual(c.want)
    })
  }
})

describe('buildRowKeys', () => {
  const cases: Array<{ name: string; indexes: number[]; row: unknown[]; want: KeyColumn[] }> = [
    {
      name: '按索引取列名与值',
      indexes: [0, 2],
      row: [1, 'x', 'y'],
      want: [{ name: 'c0', value: 1 }, { name: 'c2', value: 'y' }],
    },
    { name: '空索引返回空数组', indexes: [], row: [1], want: [] },
    {
      name: '越界索引被过滤',
      indexes: [0, 9],
      row: [1, 2],
      want: [{ name: 'c0', value: 1 }],
    },
  ]
  for (const c of cases) {
    it(c.name, () => {
      const cols = Array.from({ length: 3 }, (_, i) => ({ name: `c${i}` }))
      expect(buildRowKeys(c.indexes, cols, c.row)).toEqual(c.want)
    })
  }
})