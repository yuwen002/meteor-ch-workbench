export interface ColumnDef {
  name: string
  type: string
  default?: string
  comment?: string
}

export interface CreateTableDef {
  database: string
  table: string
  comment?: string
  engine: string
  orderBy?: string
  partitionBy?: string
  primaryKey?: string
  settings?: string
  columns: ColumnDef[]
}

export interface EditTableChange {
  type: 'add' | 'modify' | 'drop' | 'rename'
  column: string
  newName?: string
  newType?: string
  newDefault?: string
  newComment?: string
  position?: { after?: string; first?: boolean }
}

const CH_ENGINES = [
  'MergeTree',
  'ReplacingMergeTree',
  'ReplacingMergeTree(version)',
  'CollapsingMergeTree(sign)',
  'SummingMergeTree',
  'AggregatingMergeTree',
  'VersionedCollapsingMergeTree(sign, version)',
  'Memory',
  'TinyLog',
  'Log',
  'Null',
  'View',
  'MaterializedView',
  'Dictionary',
  'Kafka',
  'RabbitMQ',
  'URL',
  'File(CSV)',
  'File(JSON)',
  'HDFS',
  'S3',
  'JDBC',
  'MySQL',
  'PostgreSQL',
  'MongoDB',
  'Redis',
  'Distributed',
]

export const CLICKHOUSE_ENGINES = CH_ENGINES

const CH_TYPES = [
  'String',
  'Int8', 'Int16', 'Int32', 'Int64', 'Int128', 'Int256',
  'UInt8', 'UInt16', 'UInt32', 'UInt64', 'UInt128', 'UInt256',
  'Float32', 'Float64',
  'Decimal(10, 0)', 'Decimal(10, 2)', 'Decimal(18, 4)', 'Decimal128(4)', 'Decimal256(8)',
  'Boolean',
  'Date', 'Date32',
  'DateTime', 'DateTime64(3)', 'DateTime64(6)',
  'UUID',
  'IPv4', 'IPv6',
  'JSON', 'Object',
  'Array(String)', 'Array(Int32)', 'Array(UInt64)', 'Array(Float64)',
  'Map(String, String)', 'Map(String, Int64)',
  'LowCardinality(String)', 'LowCardinality(Int32)',
  'Nullable(String)', 'Nullable(Int32)', 'Nullable(UInt64)', 'Nullable(Float64)', 'Nullable(DateTime)',
  'FixedString(16)', 'FixedString(32)', 'FixedString(64)',
  'Enum8(\'A\' = 1, \'B\' = 2)', 'Enum16',
]

export const CLICKHOUSE_TYPES = CH_TYPES

function quoteIdent(s: string): string {
  return '`' + s.replace(/`/g, '``') + '`'
}

export function buildCreateTableSQL(def: CreateTableDef): string {
  if (!def.table || def.columns.length === 0) return ''

  const cols = def.columns
    .filter((c) => c.name && c.type)
    .map((c) => {
      let line = `  ${quoteIdent(c.name)} ${c.type}`
      if (c.default !== undefined && c.default !== '') {
        line += ` DEFAULT ${c.default}`
      }
      if (c.comment) {
        line += ` COMMENT '${c.comment.replace(/'/g, "''")}'`
      }
      return line
    })
    .join(',\n')

  let sql = `CREATE TABLE ${quoteIdent(def.database)}.${quoteIdent(def.table)} (\n${cols}\n) ENGINE = ${def.engine || 'MergeTree'}`

  if (def.orderBy) {
    sql += `\nORDER BY (${def.orderBy})`
  } else if (['MergeTree', 'ReplacingMergeTree', 'ReplacingMergeTree(version)', 'CollapsingMergeTree(sign)', 'SummingMergeTree', 'AggregatingMergeTree', 'VersionedCollapsingMergeTree(sign, version)'].includes(def.engine || 'MergeTree')) {
    sql += `\nORDER BY tuple()`
  }

  if (def.partitionBy) {
    sql += `\nPARTITION BY ${def.partitionBy}`
  }
  if (def.primaryKey) {
    sql += `\nPRIMARY KEY (${def.primaryKey})`
  }
  if (def.comment) {
    sql += `\nCOMMENT '${def.comment.replace(/'/g, "''")}'`
  }
  if (def.settings) {
    sql += `\nSETTINGS ${def.settings}`
  }

  return sql
}

export function buildAlterTableSQL(
  database: string,
  table: string,
  changes: EditTableChange[]
): string[] {
  const fullName = `${quoteIdent(database)}.${quoteIdent(table)}`
  const stmts: string[] = []

  for (const c of changes) {
    if (c.type === 'drop') {
      stmts.push(`ALTER TABLE ${fullName} DROP COLUMN ${quoteIdent(c.column)}`)
    } else if (c.type === 'rename') {
      if (c.newName) {
        stmts.push(`ALTER TABLE ${fullName} RENAME COLUMN ${quoteIdent(c.column)} TO ${quoteIdent(c.newName)}`)
      }
    } else if (c.type === 'modify') {
      const parts: string[] = []
      if (c.newType) parts.push(`TYPE ${c.newType}`)
      if (c.newDefault !== undefined) parts.push(`DEFAULT ${c.newDefault}`)
      if (c.newComment !== undefined) parts.push(`COMMENT '${c.newComment.replace(/'/g, "''")}'`)
      if (parts.length > 0) {
        stmts.push(`ALTER TABLE ${fullName} MODIFY COLUMN ${quoteIdent(c.column)} ${parts.join(' ')}`)
      }
    } else if (c.type === 'add') {
      let pos = ''
      if (c.position?.first) {
        pos = ' FIRST'
      } else if (c.position?.after) {
        pos = ` AFTER ${quoteIdent(c.position.after)}`
      }
      let colDef = `${quoteIdent(c.column)} ${c.newType || 'String'}`
      if (c.newDefault !== undefined && c.newDefault !== '') {
        colDef += ` DEFAULT ${c.newDefault}`
      }
      if (c.newComment) {
        colDef += ` COMMENT '${c.newComment.replace(/'/g, "''")}'`
      }
      stmts.push(`ALTER TABLE ${fullName} ADD COLUMN ${colDef}${pos}`)
    }
  }

  return stmts
}

export function buildDropTableSQL(database: string, table: string): string {
  return `DROP TABLE IF EXISTS ${quoteIdent(database)}.${quoteIdent(table)}`
}