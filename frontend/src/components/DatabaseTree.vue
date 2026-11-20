<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { api } from '../api'
import type { QueryResult, ClusterInfo } from '../types'
import CreateTableModal from './CreateTableModal.vue'
import EditTableModal from './EditTableModal.vue'
import { buildDropTableSQL } from '../utils/ddl'

const props = defineProps<{
  active: boolean
  selectedDb: string | null
  selectedTable: string | null
}>()

const emit = defineEmits<{
  (e: 'preview', db: string, table: string): void
  (e: 'select-db', db: string): void
  (e: 'append-uuid', uuid: string): void
  (e: 'insert-ddl', sql: string): void
  (e: 'error', msg: string): void
}>()

const { t } = useI18n()
const store = useAppStore()

const databases = ref<string[]>([])
const expandedDBs = ref<Set<string>>(new Set())
const tablesByDB = ref<Map<string, { name: string; engine?: string }[]>>(new Map())
const viewsByDB = ref<Map<string, { name: string; engine?: string }[]>>(new Map())
const loadingTables = ref<string | null>(null)
const loadingMeta = ref(false)

const clusters = ref<ClusterInfo[]>([])
const expandedClusters = ref<Set<string>>(new Set())
const loadingClusters = ref(false)
const copiedUUID = ref<string | null>(null)

const tableMenu = ref<{ visible: boolean; x: number; y: number; db: string; table: string }>({
  visible: false, x: 0, y: 0, db: '', table: '',
})

const dbMenu = ref<{ visible: boolean; x: number; y: number; db: string }>({
  visible: false, x: 0, y: 0, db: '',
})

const ddlModal = ref<{ visible: boolean; loading: boolean; sql: string; copied: boolean; error: string; db: string; table: string }>({
  visible: false, loading: false, sql: '', copied: false, error: '', db: '', table: '',
})

const createModal = ref<{ visible: boolean; database: string }>({ visible: false, database: '' })
const editModal = ref<{ visible: boolean; database: string; table: string }>({ visible: false, database: '', table: '' })

const dropConfirm = ref<{ visible: boolean; db: string; table: string }>({ visible: false, db: '', table: '' })

const groupedClusters = computed(() => {
  const map = new Map<string, ClusterInfo[]>()
  for (const c of clusters.value) {
    if (!map.has(c.cluster)) map.set(c.cluster, [])
    map.get(c.cluster)!.push(c)
  }
  return map
})

function closeMenus() {
  tableMenu.value.visible = false
  dbMenu.value.visible = false
}

async function loadDatabases() {
  if (!store.activeConnectionId) return
  loadingMeta.value = true
  try {
    const dbs: string[] = await api.GetDatabases(store.activeConnectionId)
    databases.value = dbs
  } catch (e: any) {
    // swallow
  } finally {
    loadingMeta.value = false
  }
}

async function loadClusters() {
  if (!store.activeConnectionId) return
  loadingClusters.value = true
  try {
    const cs: ClusterInfo[] = await api.GetClusters(store.activeConnectionId)
    clusters.value = cs
  } catch (e: any) {
    clusters.value = []
  } finally {
    loadingClusters.value = false
  }
}

async function reload() {
  expandedDBs.value = new Set()
  tablesByDB.value = new Map()
  viewsByDB.value = new Map()
  expandedClusters.value = new Set()
  await Promise.all([loadDatabases(), loadClusters()])
}

defineExpose({ closeMenus, reload })

watch(
  () => store.activeConnectionId,
  async (id) => {
    if (id) await reload()
  },
  { immediate: true }
)

async function toggleDB(db: string) {
  emit('select-db', db)
  if (expandedDBs.value.has(db)) {
    expandedDBs.value.delete(db)
    expandedDBs.value = new Set(expandedDBs.value)
    return
  }
  expandedDBs.value.add(db)
  expandedDBs.value = new Set(expandedDBs.value)

  if (tablesByDB.value.has(db)) return
  if (!store.activeConnectionId) return

  loadingTables.value = db
  try {
    const tables: any[] = await api.GetTables(store.activeConnectionId, db)
    tablesByDB.value.set(db, tables || [])
    tablesByDB.value = new Map(tablesByDB.value)
  } catch (e: any) {
  }
  try {
    const views: any[] = await api.GetViews(store.activeConnectionId, db)
    viewsByDB.value.set(db, views || [])
    viewsByDB.value = new Map(viewsByDB.value)
  } catch (e: any) {
  } finally {
    loadingTables.value = null
  }
}

function toggleCluster(cluster: string) {
  if (expandedClusters.value.has(cluster)) {
    expandedClusters.value.delete(cluster)
  } else {
    expandedClusters.value.add(cluster)
  }
  expandedClusters.value = new Set(expandedClusters.value)
}

async function copyUUID(uuid: string) {
  if (!uuid) return
  try {
    await navigator.clipboard.writeText(uuid)
    copiedUUID.value = uuid
    setTimeout(() => { copiedUUID.value = null }, 1500)
  } catch {
    // ignore
  }
}

function showTableMenu(e: MouseEvent, db: string, table: string) {
  e.preventDefault()
  e.stopPropagation()
  tableMenu.value = { visible: true, x: e.clientX, y: e.clientY, db, table }
}

function closeTableMenu() {
  tableMenu.value.visible = false
}

function showDBMenu(e: MouseEvent, db: string) {
  e.preventDefault()
  e.stopPropagation()
  dbMenu.value = { visible: true, x: e.clientX, y: e.clientY, db }
}

function closeDBMenu() {
  dbMenu.value.visible = false
}

async function openTableDDL(db: string, table: string) {
  closeTableMenu()
  ddlModal.value = { visible: true, loading: true, sql: '', copied: false, error: '', db, table }
  try {
    const r: QueryResult = await api.Query(
      store.activeConnectionId!,
      `SHOW CREATE TABLE \`${db.replace(/`/g, '``')}\`.\`${table.replace(/`/g, '``')}\``,
      ''
    )
    if (r.error) {
      ddlModal.value.error = r.error
    } else if (r.rows && r.rows.length > 0 && r.rows[0]?.length > 0) {
      const val = r.rows[0][0]
      ddlModal.value.sql = typeof val === 'string' ? val : JSON.stringify(val)
    } else {
      ddlModal.value.error = t('editor.noResults')
    }
  } catch (e: any) {
    ddlModal.value.error = e?.message || t('editor.queryFailed')
  } finally {
    ddlModal.value.loading = false
  }
}

async function copyDDL() {
  if (!ddlModal.value.sql) return
  try {
    await navigator.clipboard.writeText(ddlModal.value.sql)
    ddlModal.value.copied = true
    setTimeout(() => { ddlModal.value.copied = false }, 1500)
  } catch {
    // ignore
  }
}

function ddlToEditor() {
  if (!ddlModal.value.sql) return
  emit('insert-ddl', ddlModal.value.sql)
  ddlModal.value.visible = false
}

function openCreateTable(db: string) {
  closeDBMenu()
  closeTableMenu()
  createModal.value = { visible: true, database: db }
}

function openEditTable(db: string, table: string) {
  closeTableMenu()
  editModal.value = { visible: true, database: db, table }
}

function confirmDropTable(db: string, table: string) {
  closeTableMenu()
  dropConfirm.value = { visible: true, db, table }
}

async function doDropTable() {
  if (!store.activeConnectionId) return
  const { db, table } = dropConfirm.value
  const sql = buildDropTableSQL(db, table)
  try {
    const r = await api.Execute(store.activeConnectionId, sql)
    if (r.error) {
      emit('error', r.error)
    } else {
      dropConfirm.value.visible = false
      tablesByDB.value.delete(db)
      tablesByDB.value = new Map(tablesByDB.value)
    }
  } catch (e: any) {
    emit('error', e?.message || 'Drop table failed')
  }
}

function refreshTables(db: string) {
  tablesByDB.value.delete(db)
  tablesByDB.value = new Map(tablesByDB.value)
  viewsByDB.value.delete(db)
  viewsByDB.value = new Map(viewsByDB.value)
  toggleDB(db)
}
</script>

<template>
  <div v-if="active" class="w-56 border-r border-ch-border bg-ch-panel flex flex-col shrink-0">
    <div class="flex-1 overflow-y-auto text-sm">
      <div class="px-3 py-2 text-xs font-semibold text-ch-muted uppercase tracking-wide border-b border-ch-border sticky top-0 bg-ch-panel z-10">
        {{ t('editor.databases') }}
      </div>
      <div class="py-1">
        <template v-if="databases.length === 0 && !loadingMeta">
          <div class="px-3 py-2 text-ch-muted text-xs">{{ t('editor.noDatabases') }}</div>
        </template>
        <div
          v-for="db in databases"
          :key="db"
          class="cursor-pointer group"
        >
          <div
            class="flex items-center gap-1 px-3 py-1 hover:bg-ch-hover border-l-2 border-l-transparent"
            :class="props.selectedDb === db ? 'bg-ch-accent/15 border-l-ch-accent text-ch-accent' : 'text-ch-text'"
            @click="toggleDB(db)"
            @contextmenu.prevent="showDBMenu($event, db)"
          >
            <svg
              width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
              :class="{ 'rotate-90': expandedDBs.has(db) }"
              class="transition-transform shrink-0"
            >
              <polyline points="9 18 15 12 9 6"/>
            </svg>
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="text-ch-muted shrink-0"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14a9 3 0 0 0 18 0V5"/><path d="M3 12a9 3 0 0 0 18 0"/></svg>
            <span class="truncate flex-1">{{ db }}</span>
            <button
              class="opacity-0 group-hover:opacity-100 hover:opacity-100 text-ch-muted hover:text-ch-accent transition-opacity p-0.5"
              :title="t('editor.addTable')"
              @click.stop="openCreateTable(db)"
            >
              <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
            </button>
          </div>
          <div v-show="expandedDBs.has(db)" class="pl-5">
            <div v-if="loadingTables === db" class="px-3 py-1 text-xs text-ch-muted">{{ t('editor.loading') }}</div>
            <div
              v-for="tb in (tablesByDB.get(db) || [])"
              :key="tb.name"
              class="flex items-center gap-1 px-2 py-1 hover:bg-ch-hover cursor-pointer text-xs border-l-2 border-l-transparent"
              :class="props.selectedDb === db && props.selectedTable === tb.name ? 'bg-ch-accent/15 border-l-ch-accent text-ch-accent' : 'text-ch-text'"
              @click="emit('preview', db, tb.name)"
              @contextmenu.prevent="showTableMenu($event, db, tb.name)"
              :title="tb.engine || (t('editor.clickToPreview'))"
            >
              <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="text-ch-muted"><rect x="3" y="3" width="18" height="18" rx="2"/><line x1="3" y1="9" x2="21" y2="9"/><line x1="3" y1="15" x2="21" y2="15"/><line x1="9" y1="3" x2="9" y2="21"/><line x1="15" y1="3" x2="15" y2="21"/></svg>
              <span class="truncate">{{ tb.name }}</span>
            </div>
            <div v-if="(tablesByDB.get(db) || []).length === 0 && loadingTables !== db" class="px-3 py-1 text-[10px] text-ch-muted italic">
              {{ t('editor.noResults') }}
            </div>
            <div v-if="(viewsByDB.get(db) || []).length > 0" class="mt-1 px-2 pt-1 pb-0.5 text-[10px] font-semibold uppercase tracking-wide text-ch-muted flex items-center gap-1">
              <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
              {{ t('editor.views') }}
            </div>
            <div
              v-for="v in (viewsByDB.get(db) || [])"
              :key="'v-' + v.name"
              class="flex items-center gap-1 px-2 py-1 hover:bg-ch-hover cursor-pointer text-xs border-l-2 border-l-transparent"
              :class="props.selectedDb === db && props.selectedTable === v.name ? 'bg-ch-accent/15 border-l-ch-accent text-ch-accent' : 'text-ch-text'"
              @click="emit('preview', db, v.name)"
              @contextmenu.prevent="showTableMenu($event, db, v.name)"
              :title="v.engine || (t('editor.clickToPreview'))"
            >
              <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-ch-muted"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
              <span class="truncate">{{ v.name }}</span>
            </div>
          </div>
        </div>
      </div>

      <div class="px-3 py-2 text-xs font-semibold text-ch-muted uppercase tracking-wide border-y border-ch-border bg-ch-panel z-10">
        {{ t('editor.clusters') }}
      </div>
      <div class="py-1 pb-2">
        <template v-if="loadingClusters">
          <div class="px-3 py-2 text-xs text-ch-muted">{{ t('editor.loading') }}</div>
        </template>
        <template v-else-if="clusters.length === 0">
          <div class="px-3 py-2 text-ch-muted text-xs">{{ t('editor.noClusters') }}</div>
        </template>
        <template v-else>
          <div
            v-for="[clusterName, nodes] in groupedClusters"
            :key="clusterName"
          >
            <div
              class="flex items-center gap-1 px-3 py-1 hover:bg-ch-hover text-ch-text cursor-pointer"
              @click="toggleCluster(clusterName)"
            >
              <svg
                width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
                :class="{ 'rotate-90': expandedClusters.has(clusterName) }"
                class="transition-transform shrink-0"
              >
                <polyline points="9 18 15 12 9 6"/>
              </svg>
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="text-ch-muted shrink-0"><circle cx="12" cy="12" r="3"/><path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83"/></svg>
              <span class="truncate text-xs font-medium">{{ clusterName }}</span>
              <span class="text-ch-muted text-[10px] ml-auto">{{ nodes.length }}</span>
            </div>
            <div v-show="expandedClusters.has(clusterName)" class="pl-5">
              <div
                v-for="(node, ni) in nodes"
                :key="ni"
                class="px-2 py-1 text-[11px] border-b border-ch-border/30 last:border-b-0"
              >
                <div class="flex items-center gap-1 mb-0.5">
                  <span
                    v-if="node.is_local"
                    class="w-1.5 h-1.5 rounded-full bg-green-500 shrink-0"
                    :title="t('editor.local')"
                  ></span>
                  <span class="text-ch-text truncate font-mono">{{ node.host_address }}:{{ node.port }}</span>
                </div>
                <div class="text-ch-muted text-[10px] leading-relaxed">
                  <span class="mr-2">{{ t('editor.shard') }}: {{ node.shard_num }}</span>
                  <span class="mr-2">{{ t('editor.replica') }}: {{ node.replica_num }}</span>
                  <span class="text-ch-muted/70">{{ node.version }}</span>
                </div>
                <div
                  v-if="node.uuid"
                  class="mt-1 flex items-center gap-1 group"
                >
                  <code
                    class="text-[10px] font-mono text-blue-400 bg-ch-hover/60 px-1 py-0.5 rounded cursor-pointer truncate flex-1"
                    :title="node.uuid + ' (Ctrl+点击插入编辑器)'"
                    @click="emit('append-uuid', node.uuid)"
                  >{{ node.uuid.slice(0, 8) }}…{{ node.uuid.slice(-4) }}</code>
                  <button
                    class="shrink-0 opacity-0 group-hover:opacity-100 text-ch-muted hover:text-ch-accent transition-opacity"
                    :title="copiedUUID === node.uuid ? t('editor.uuidCopied') : t('editor.copyUUID')"
                    @click.stop="copyUUID(node.uuid)"
                  >
                    <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
                  </button>
                </div>
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>

    <Teleport to="body">
      <div
        v-if="dbMenu.visible"
        class="fixed z-[1000] bg-ch-panel border border-ch-border rounded-md shadow-xl py-1 min-w-[160px] select-none"
        :style="{ left: dbMenu.x + 'px', top: dbMenu.y + 'px' }"
        @click.stop
        @contextmenu.prevent
      >
        <div class="px-3 py-1 text-[10px] text-ch-muted uppercase tracking-wide border-b border-ch-border/50">
          {{ dbMenu.db }}
        </div>
        <button
          @click="openCreateTable(dbMenu.db)"
          class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-sm text-ch-text hover:bg-ch-hover"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
          {{ t('editor.createTableHere') }}
        </button>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="tableMenu.visible"
        class="fixed z-[1000] bg-ch-panel border border-ch-border rounded-md shadow-xl py-1 min-w-[160px] select-none"
        :style="{ left: tableMenu.x + 'px', top: tableMenu.y + 'px' }"
        @click.stop
        @contextmenu.prevent
      >
        <div class="px-3 py-1 text-[10px] text-ch-muted uppercase tracking-wide border-b border-ch-border/50">
          {{ tableMenu.db }}.{{ tableMenu.table }}
        </div>
        <button
          @click="emit('preview', tableMenu.db, tableMenu.table)"
          class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-sm text-ch-text hover:bg-ch-hover"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>
          SELECT * LIMIT 10
        </button>
        <button
          @click="openTableDDL(tableMenu.db, tableMenu.table)"
          class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-sm text-ch-text hover:bg-ch-hover"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
          {{ t('editor.viewDDL') }}
        </button>
        <button
          @click="openEditTable(tableMenu.db, tableMenu.table)"
          class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-sm text-ch-text hover:bg-ch-hover"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"/></svg>
          {{ t('editor.editTable') }}
        </button>
        <div class="border-t border-ch-border/50 my-1"></div>
        <button
          @click="confirmDropTable(tableMenu.db, tableMenu.table)"
          class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-sm text-red-400 hover:bg-red-900/20"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2m3 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/></svg>
          {{ t('editor.dropTable') }}
        </button>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="ddlModal.visible"
        class="fixed inset-0 z-[2000] flex items-center justify-center"
        @click.self="ddlModal.visible = false"
      >
        <div class="absolute inset-0 bg-black/50"></div>
        <div class="relative bg-ch-panel border border-ch-border rounded-lg shadow-2xl w-[720px] max-w-[90vw] max-h-[80vh] flex flex-col">
          <div class="flex items-center justify-between px-4 py-3 border-b border-ch-border shrink-0">
            <h3 class="text-sm font-semibold text-ch-text">{{ t('editor.viewDDL') }}: `{{ ddlModal.db }}`.`{{ ddlModal.table }}`</h3>
            <button
              @click="ddlModal.visible = false"
              class="p-1 rounded hover:bg-ch-hover text-ch-muted hover:text-ch-text"
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            </button>
          </div>

          <div class="flex-1 overflow-auto p-4 min-h-[200px]">
            <div v-if="ddlModal.loading" class="flex items-center justify-center h-full text-ch-muted text-sm gap-2">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="animate-spin"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
              {{ t('editor.loading') }}
            </div>
            <div v-else-if="ddlModal.error" class="bg-red-900/30 border border-red-700 rounded-md p-3 text-red-300 text-xs font-mono whitespace-pre-wrap">
              {{ ddlModal.error }}
            </div>
            <pre v-else class="text-xs font-mono text-ch-text whitespace-pre-wrap break-all">{{ ddlModal.sql }}</pre>
          </div>

          <div class="flex items-center justify-end gap-2 px-4 py-3 border-t border-ch-border shrink-0">
            <button
              v-if="!ddlModal.loading && !ddlModal.error"
              @click="copyDDL"
              class="btn-ghost btn-sm flex items-center gap-1"
            >
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
              {{ ddlModal.copied ? t('editor.ddlCopied') : 'Copy' }}
            </button>
            <button
              v-if="!ddlModal.loading && !ddlModal.error"
              @click="ddlToEditor"
              class="btn-primary btn-sm"
            >
              → Editor
            </button>
            <button
              @click="ddlModal.visible = false"
              class="btn-ghost btn-sm"
            >
              {{ t('common.close') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <CreateTableModal
      :visible="createModal.visible"
      :database="createModal.database"
      @close="createModal.visible = false"
      @created="refreshTables(createModal.database)"
    />

    <EditTableModal
      :visible="editModal.visible"
      :database="editModal.database"
      :table="editModal.table"
      @close="editModal.visible = false"
      @altered="refreshTables(editModal.database)"
    />

    <Teleport to="body">
      <div
        v-if="dropConfirm.visible"
        class="fixed inset-0 z-[3000] flex items-center justify-center"
        @click.self="dropConfirm.visible = false"
      >
        <div class="absolute inset-0 bg-black/50"></div>
        <div class="relative bg-ch-panel border border-red-700 rounded-lg shadow-2xl w-[420px] max-w-[90vw]">
          <div class="px-4 py-3 border-b border-ch-border">
            <h3 class="text-sm font-semibold text-red-400 flex items-center gap-2">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
              {{ t('editor.dropTable') }}
            </h3>
          </div>
          <div class="px-4 py-4 text-sm text-ch-text">
            <p class="whitespace-pre-line">{{ t('editor.confirmDrop').replace('{db}', dropConfirm.db).replace('{table}', dropConfirm.table) }}</p>
          </div>
          <div class="flex items-center justify-end gap-2 px-4 py-3 border-t border-ch-border">
            <button @click="dropConfirm.visible = false" class="btn-ghost btn-sm">{{ t('common.close') }}</button>
            <button @click="doDropTable" class="btn-primary btn-sm bg-red-600 hover:bg-red-700">
              {{ t('editor.dropTable') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>