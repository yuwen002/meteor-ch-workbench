export interface Connection {
  id: string
  folder_id?: string
  name: string
  host: string
  port: number
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

export interface ColumnInfo {
  name: string
  type: string
  default?: string
  comment?: string
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
  columns: string[]
  rows: any[][]
  row_count: number
  elapsed_ms: number
  error?: string
}