import { describe, it, expect } from 'vitest'
import { buildCreateTableSQL, buildAlterTableSQL, buildDropTableSQL, type CreateTableDef } from './ddl'

describe('buildDropTableSQL', () => {
  it('生成带 IF EXISTS 的 DROP 语句', () => {
    expect(buildDropTableSQL('db', 't')).toBe('DROP TABLE IF EXISTS `db`.`t`')
  })
})

describe('buildCreateTableSQL', () => {
  const base: CreateTableDef = {
    database: 'db',
    table: 't',
    engine: 'MergeTree',
    columns: [
      { name: 'id', type: 'UInt64' },
      { name: 'name', type: 'String' },
    ],
  }

  it('无表名返回空字符串', () => {
    expect(buildCreateTableSQL({ ...base, table: '' })).toBe('')
  })

  it('无列返回空字符串', () => {
    expect(buildCreateTableSQL({ ...base, columns: [] })).toBe('')
  })

  it('MergeTree 无 ORDER BY 时补 tuple()', () => {
    const sql = buildCreateTableSQL(base)
    expect(sql).toContain('ENGINE = MergeTree')
    expect(sql).toContain('ORDER BY tuple()')
    expect(sql).toContain('`id` UInt64')
  })

  it('给定 ORDER BY 时按指定列排序', () => {
    const sql = buildCreateTableSQL({ ...base, orderBy: 'id' })
    expect(sql).toContain('ORDER BY (id)')
    expect(sql).not.toContain('ORDER BY tuple()')
  })

  it('Memory 引擎不补 ORDER BY', () => {
    const sql = buildCreateTableSQL({ ...base, engine: 'Memory' })
    expect(sql).not.toContain('ORDER BY')
  })

  it('补充默认值、注释、分区、主键与 SETTINGS', () => {
    const sql = buildCreateTableSQL({
      ...base,
      columns: [{ name: 'id', type: 'UInt64', default: '0', comment: "it's" }],
      orderBy: 'id',
      partitionBy: 'toYYYYMM(id)',
      primaryKey: 'id',
      comment: 'demo',
      settings: 'index_granularity = 8192',
    })
    expect(sql).toContain('DEFAULT 0')
    expect(sql).toContain("COMMENT 'it''s'")
    expect(sql).toContain('PARTITION BY toYYYYMM(id)')
    expect(sql).toContain('PRIMARY KEY (id)')
    expect(sql).toContain("COMMENT 'demo'")
    expect(sql).toContain('SETTINGS index_granularity = 8192')
  })

  it('缺列名或类型的列被过滤', () => {
    const sql = buildCreateTableSQL({
      ...base,
      columns: [
        { name: 'a', type: 'Int32' },
        { name: '', type: 'Int32' },
        { name: 'b', type: '' },
      ],
    })
    expect(sql).toContain('`a` Int32')
    expect(sql).not.toContain('`b`')
  })
})

describe('buildAlterTableSQL', () => {
  it('DROP COLUMN', () => {
    expect(buildAlterTableSQL('db', 't', [{ type: 'drop', column: 'c' }])).toEqual([
      'ALTER TABLE `db`.`t` DROP COLUMN `c`',
    ])
  })

  it('RENAME COLUMN', () => {
    expect(buildAlterTableSQL('db', 't', [{ type: 'rename', column: 'a', newName: 'b' }])).toEqual([
      'ALTER TABLE `db`.`t` RENAME COLUMN `a` TO `b`',
    ])
  })

  it('RENAME 缺新名时跳过', () => {
    expect(buildAlterTableSQL('db', 't', [{ type: 'rename', column: 'a' }])).toEqual([])
  })

  it('MODIFY COLUMN 组合类型、默认值与注释', () => {
    expect(
      buildAlterTableSQL('db', 't', [
        { type: 'modify', column: 'a', newType: 'Int64', newDefault: '0', newComment: "x'y" },
      ]),
    ).toEqual(["ALTER TABLE `db`.`t` MODIFY COLUMN `a` TYPE Int64 DEFAULT 0 COMMENT 'x''y'"])
  })

  it('MODIFY 无任何变更时跳过', () => {
    expect(buildAlterTableSQL('db', 't', [{ type: 'modify', column: 'a' }])).toEqual([])
  })

  it('ADD COLUMN 带 FIRST 位置', () => {
    expect(
      buildAlterTableSQL('db', 't', [
        { type: 'add', column: 'a', newType: 'Int32', position: { first: true } },
      ]),
    ).toEqual(['ALTER TABLE `db`.`t` ADD COLUMN `a` Int32 FIRST'])
  })

  it('ADD COLUMN 带 AFTER 位置与默认值', () => {
    expect(
      buildAlterTableSQL('db', 't', [
        { type: 'add', column: 'a', newDefault: '1', position: { after: 'b' } },
      ]),
    ).toEqual(['ALTER TABLE `db`.`t` ADD COLUMN `a` String DEFAULT 1 AFTER `b`'])
  })

  it('多条变更按顺序生成多条语句', () => {
    const out = buildAlterTableSQL('db', 't', [
      { type: 'drop', column: 'a' },
      { type: 'add', column: 'b', newType: 'Int32' },
    ])
    expect(out.length).toBe(2)
  })

  it('空变更返回空数组', () => {
    expect(buildAlterTableSQL('db', 't', [])).toEqual([])
  })
})