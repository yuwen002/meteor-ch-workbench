export function escapeSqlString(s: string): string {
  return "'" + s.replace(/\\/g, '\\\\').replace(/'/g, "\\'") + "'"
}

export function sqlLiteral(v: unknown): string {
  if (v === null || v === undefined) return 'NULL'
  if (typeof v === 'number') return Number.isFinite(v) ? String(v) : 'NULL'
  if (typeof v === 'boolean') return v ? '1' : '0'
  return escapeSqlString(String(v))
}

export function quoteIdent(name: string): string {
  return '`' + name.replace(/`/g, '``') + '`'
}

export function rowToTSV(row: unknown[]): string {
  return row
    .map((v) => (v === null || v === undefined ? '' : String(v)))
    .join('\t')
}

export function columnsToCSV(columns: string[]): string {
  return columns.join(', ')
}

export function buildInsertStatement(table: string, columns: string[], row: unknown[]): string {
  const cols = columns.map(quoteIdent).join(', ')
  const vals = row.map(sqlLiteral).join(', ')
  return `INSERT INTO ${table} (${cols}) VALUES (${vals});`
}

export function buildInsertBatch(table: string, columns: string[], rows: unknown[][]): string {
  const head = `INSERT INTO ${table} (${columns.map(quoteIdent).join(', ')}) VALUES\n`
  const body = rows.map((row, i) => {
    const vals = row.map(sqlLiteral).join(', ')
    return '  (' + vals + ')' + (i === rows.length - 1 ? ';' : ',')
  })
  return head + body.join('\n')
}

export function insertTarget(db: string | null, table: string | null): string {
  if (!table) return PENDING_TABLE_PLACEHOLDER
  const qualified = db ? quoteIdent(db) + '.' + quoteIdent(table) : quoteIdent(table)
  return qualified
}

export const PENDING_TABLE_PLACEHOLDER = 'your_table'