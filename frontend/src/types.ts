export interface Connection {
  id: string
  folder_id?: string
  name: string
  host: string
  port: number
  protocol: 'native' | 'http'
  user: string
  password: string
  database: string
  secure: boolean
  use_compress: boolean
}

export interface Folder {
  id: string
  name: string
}

export interface DataFile {
  folders: Folder[]
  connections: Connection[]
}

export interface ExportFormat {
  id: string
  name: string
  extension: string
  mime: string
  description: string
}

export interface ImportFormat {
  id: string
  name: string
  extension: string
  description: string
}

export interface ImportOptions {
  format: string
  mode: 'replace' | 'merge'
}

export interface ImportResult {
  mode: string
  format: string
  importedFolders: number
  importedConns: number
  totalFolders: number
  totalConns: number
}

export interface ColumnInfo {
  name: string
  type: string
  default?: string
  comment?: string
  is_primary?: boolean
  has_default?: boolean
}

export interface TableInfo {
  name: string
  engine?: string
  comment?: string
}

export interface ClusterInfo {
  cluster: string
  version: string
  host_name: string
  host_address: string
  port: number
  is_local: boolean
  shard_num: number
  replica_num: number
  uuid: string
}

export interface QueryResult {
  columns: ColumnInfo[]
  rows: any[][] | null
  row_count: number
  elapsed_ms: number
  truncated: boolean
  row_limit: number
  error?: string
}

export interface ResultExportFormat {
  id: string
  name: string
  extension: string
  mime: string
  description: string
}

export interface TabState {
  id: string
  title: string
  sql: string
}

export interface TabsFile {
  tabs: TabState[]
  active_tab_id: string
}

export interface HistoryEntry {
  id: string
  sql: string
  connection_id: string
  timestamp: string
  duration_ms: number
  success: boolean
}

export interface SavedSQL {
  id: string
  name: string
  sql: string
  connection_id: string
  created_at: string
  updated_at: string
}

export interface ExecResult {
  rows_affected: number
  error?: string
}