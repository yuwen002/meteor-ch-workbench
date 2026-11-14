import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '../api'
import type { Connection, Folder, DataFile } from '../types'

export type ConnectionStatus = 'unknown' | 'connecting' | 'connected' | 'failed'

export const useAppStore = defineStore('app', () => {
  const folders = ref<Folder[]>([])
  const connections = ref<Connection[]>([])
  const activeConnectionId = ref<string | null>(null)
  const collapsedFolders = ref<Set<string>>(new Set())
  const sidebarCollapsed = ref(false)
  const connectionStatus = ref<Map<string, ConnectionStatus>>(new Map())
  const connectionError = ref<Map<string, string>>(new Map())

  const activeConnection = computed(() =>
    connections.value.find((c) => c.id === activeConnectionId.value) || null
  )

  const groupedConnections = computed(() => {
    const groups: { folder: Folder | null; conns: Connection[] }[] = []
    const folderMap = new Map<string, Connection[]>()
    const ungrouped: Connection[] = []

    for (const f of folders.value) {
      folderMap.set(f.id, [])
    }

    for (const c of connections.value) {
      if (c.folder_id && folderMap.has(c.folder_id)) {
        folderMap.get(c.folder_id)!.push(c)
      } else {
        ungrouped.push(c)
      }
    }

    for (const f of folders.value) {
      groups.push({ folder: f, conns: folderMap.get(f.id) || [] })
    }
    if (ungrouped.length > 0) {
      groups.push({ folder: null, conns: ungrouped })
    }
    return groups
  })

  async function loadAll() {
    try {
      const data: DataFile = await api.GetAll()
      folders.value = data.folders || []
      connections.value = data.connections || []
    } catch {
      folders.value = []
      connections.value = []
    }
  }

  async function saveConnection(conn: Connection) {
    await api.SaveConnection(conn)
    await loadAll()
  }

  async function deleteConnection(id: string) {
    await api.DeleteConnection(id)
    if (activeConnectionId.value === id) {
      activeConnectionId.value = null
    }
    await loadAll()
  }

  async function saveFolder(f: Folder) {
    await api.SaveFolder(f)
    await loadAll()
  }

  async function deleteFolder(id: string) {
    await api.DeleteFolder(id)
    await loadAll()
  }

  function toggleFolder(folderId: string) {
    if (collapsedFolders.value.has(folderId)) {
      collapsedFolders.value.delete(folderId)
    } else {
      collapsedFolders.value.add(folderId)
    }
  }

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  function getConnectionStatus(id: string): ConnectionStatus {
    return connectionStatus.value.get(id) || 'unknown'
  }

  function getConnectionError(id: string): string {
    return connectionError.value.get(id) || ''
  }

  async function checkConnection(id: string) {
    const conn = connections.value.find((c) => c.id === id)
    if (!conn) return
    connectionStatus.value.set(id, 'connecting')
    connectionError.value.delete(id)
    connectionStatus.value = new Map(connectionStatus.value)
    try {
      await api.TestConnection(conn)
      connectionStatus.value.set(id, 'connected')
      connectionError.value.delete(id)
    } catch (e: any) {
      connectionStatus.value.set(id, 'failed')
      connectionError.value.set(id, e?.message || 'Connection failed')
    }
    connectionStatus.value = new Map(connectionStatus.value)
  }

  async function checkAllConnections() {
    for (const c of connections.value) {
      checkConnection(c.id)
    }
  }

  return {
    folders,
    connections,
    activeConnectionId,
    activeConnection,
    collapsedFolders,
    sidebarCollapsed,
    connectionStatus,
    groupedConnections,
    loadAll,
    saveConnection,
    deleteConnection,
    saveFolder,
    deleteFolder,
    toggleFolder,
    toggleSidebar,
    getConnectionStatus,
    getConnectionError,
    checkConnection,
    checkAllConnections,
  }
})