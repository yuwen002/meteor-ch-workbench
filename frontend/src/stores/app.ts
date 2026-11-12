import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '../api'
import type { Connection, Folder, DataFile } from '../types'

export const useAppStore = defineStore('app', () => {
  const folders = ref<Folder[]>([])
  const connections = ref<Connection[]>([])
  const activeConnectionId = ref<string | null>(null)
  const collapsedFolders = ref<Set<string>>(new Set())
  const sidebarCollapsed = ref(false)

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

  return {
    folders,
    connections,
    activeConnectionId,
    activeConnection,
    collapsedFolders,
    sidebarCollapsed,
    groupedConnections,
    loadAll,
    saveConnection,
    deleteConnection,
    saveFolder,
    deleteFolder,
    toggleFolder,
    toggleSidebar,
  }
})