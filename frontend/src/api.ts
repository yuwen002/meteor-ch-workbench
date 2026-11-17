import { Call } from '@wailsio/runtime'
import type { Connection, Folder, DataFile, QueryResult, ExecResult, TableInfo, ColumnInfo, ClusterInfo, ExportFormat, ImportFormat, ImportOptions, ImportResult } from './types'

type AnyError = { message?: string; cause?: unknown; name?: string }

function handleError(e: unknown): Error {
  const err = e as AnyError
  return new Error(err?.message || String(e))
}

function call<T>(methodName: string, ...args: any[]): Promise<T> {
  return Call.ByName(methodName, ...args).catch((e) => {
    throw handleError(e)
  }) as Promise<T>
}

export const api = {
  GetAll(): Promise<DataFile> {
    return call<DataFile>('main.App.GetAll')
  },

  SaveConnection(conn: Connection): Promise<void> {
    return call<void>('main.App.SaveConnection', conn)
  },

  DeleteConnection(id: string): Promise<void> {
    return call<void>('main.App.DeleteConnection', id)
  },

  TestConnection(conn: Connection): Promise<void> {
    return call<void>('main.App.TestConnection', conn)
  },

  Disconnect(connectionID: string): Promise<void> {
    return call<void>('main.App.Disconnect', connectionID)
  },

  SaveFolder(folder: Folder): Promise<void> {
    return call<void>('main.App.SaveFolder', folder)
  },

  DeleteFolder(id: string): Promise<void> {
    return call<void>('main.App.DeleteFolder', id)
  },

  ReorderFolders(ids: string[]): Promise<void> {
    return call<void>('main.App.ReorderFolders', ids)
  },

  ReorderConnections(ids: string[]): Promise<void> {
    return call<void>('main.App.ReorderConnections', ids)
  },

  ExportConfig(): Promise<string> {
    return call<string>('main.App.ExportConfig')
  },

  GetExportFormats(): Promise<ExportFormat[]> {
    return call<ExportFormat[]>('main.App.GetExportFormats')
  },

  ExportConfigByFormat(format: string, includePassword: boolean): Promise<[string, string]> {
    return call<[string, string]>('main.App.ExportConfigByFormat', format, includePassword)
  },

  SaveExportAs(format: string, includePassword: boolean): Promise<string> {
    return call<string>('main.App.SaveExportAs', format, includePassword)
  },

  ImportConfig(raw: string): Promise<void> {
    return call<void>('main.App.ImportConfig', raw)
  },

  GetImportFormats(): Promise<ImportFormat[]> {
    return call<ImportFormat[]>('main.App.GetImportFormats')
  },

  PickImportFile(): Promise<[string, string]> {
    return call<[string, string]>('main.App.PickImportFile')
  },

  ImportConfigWithOptions(raw: string, opts: ImportOptions): Promise<ImportResult> {
    return call<ImportResult>('main.App.ImportConfigWithOptions', raw, opts)
  },

  Query(connectionID: string, sql: string): Promise<QueryResult> {
    return call<QueryResult>('main.App.Query', connectionID, sql)
  },

  Execute(connectionID: string, sql: string): Promise<ExecResult> {
    return call<ExecResult>('main.App.Execute', connectionID, sql)
  },

  GetDatabases(connectionID: string): Promise<string[]> {
    return call<string[]>('main.App.GetDatabases', connectionID)
  },

  GetTables(connectionID: string, database: string): Promise<TableInfo[]> {
    return call<TableInfo[]>('main.App.GetTables', connectionID, database)
  },

  GetViews(connectionID: string, database: string): Promise<TableInfo[]> {
    return call<TableInfo[]>('main.App.GetViews', connectionID, database)
  },

  GetTableSchema(connectionID: string, database: string, table: string): Promise<ColumnInfo[]> {
    return call<ColumnInfo[]>('main.App.GetTableSchema', connectionID, database, table)
  },

  GetClusters(connectionID: string): Promise<ClusterInfo[]> {
    return call<ClusterInfo[]>('main.App.GetClusters', connectionID)
  },

  OpenConfigFolder(): Promise<void> {
    return call<void>('main.App.OpenConfigFolder')
  },

  GetAppVersion(): Promise<string> {
    return call<string>('main.App.GetAppVersion')
  },
}