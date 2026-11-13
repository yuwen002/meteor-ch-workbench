import { Call } from '@wailsio/runtime'
import type { Connection, Folder, DataFile, QueryResult, TableInfo, ClusterInfo, ColumnInfo } from './types'

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

  SaveFolder(folder: Folder): Promise<void> {
    return call<void>('main.App.SaveFolder', folder)
  },

  DeleteFolder(id: string): Promise<void> {
    return call<void>('main.App.DeleteFolder', id)
  },

  Query(connectionID: string, sql: string): Promise<QueryResult> {
    return call<QueryResult>('main.App.Query', connectionID, sql)
  },

  GetDatabases(connectionID: string): Promise<string[]> {
    return call<string[]>('main.App.GetDatabases', connectionID)
  },

  GetTables(connectionID: string, database: string): Promise<TableInfo[]> {
    return call<TableInfo[]>('main.App.GetTables', connectionID, database)
  },

  GetClusters(connectionID: string): Promise<ClusterInfo[]> {
    return call<ClusterInfo[]>('main.App.GetClusters', connectionID)
      },

      GetTableSchema(connectionID: string, database: string, table: string): Promise<ColumnInfo[]> {
        return call<ColumnInfo[]>('main.App.GetTableSchema', connectionID, database, table)
  },

  OpenConfigFolder(): Promise<void> {
    return call<void>('main.App.OpenConfigFolder')
  },

  GetAppVersion(): Promise<string> {
    return call<string>('main.App.GetAppVersion')
  },
}