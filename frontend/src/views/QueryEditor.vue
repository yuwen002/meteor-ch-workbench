<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import * as monaco from 'monaco-editor'
import { useAppStore } from '../stores/app'
import { api } from '../api'
import type { QueryResult, ClusterInfo } from '../types'

const { t } = useI18n()
const store = useAppStore()

const editorRef = ref<HTMLElement | null>(null)
let editor: monaco.editor.IStandaloneCodeEditor | null = null

const databases = ref<string[]>([])
const expandedDBs = ref<Set<string>>(new Set())
const tablesByDB = ref<Map<string, { name: string; engine?: string }[]>>(new Map())
const loadingTables = ref<string | null>(null)
const loadingMeta = ref(false)

const clusters = ref<ClusterInfo[]>([])
const expandedClusters = ref<Set<string>>(new Set())
const loadingClusters = ref(false)
const copiedUUID = ref<string | null>(null)

const result = ref<QueryResult | null>(null)
const querying = ref(false)
const error = ref<string | null>(null)

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

async function toggleDB(db: string) {
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
    // swallow
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

async function runQuery() {
  if (!store.activeConnectionId) {
    error.value = t('editor.noConnection')
    return
  }
  if (!editor) return
  const sel = editor.getSelection()
  let sql = editor.getValue()
  if (sel && !sel.isEmpty()) {
    const model = editor.getModel()
    if (model) {
      sql = model.getValueInRange(sel)
    }
  }
  sql = sql.trim()
  if (!sql) return

  querying.value = true
  result.value = null
  error.value = null
  try {
    const r: QueryResult = await api.Query(store.activeConnectionId, sql)
    result.value = r
    if (r.error) {
      error.value = r.error
    }
  } catch (e: any) {
    error.value = e?.message || t('editor.queryFailed')
  } finally {
    querying.value = false
  }
}

function insertTable(db: string, table: string) {
  if (!editor) return
  const snippet = `SELECT * FROM ${db}.${table} LIMIT 10`
  editor.setValue(snippet)
  runQuery()
}

function insertTableName(table: string) {
  if (!editor) return
  const model = editor.getModel()
  const sel = editor.getSelection()
  if (model && sel) {
    model.pushEditOperations([], [
      { range: sel, text: table },
    ], () => null)
    editor.focus()
  }
}

function insertClusterUUID(uuid: string) {
  if (!editor || !uuid) return
  const model = editor.getModel()
  const sel = editor.getSelection()
  if (model && sel) {
    model.pushEditOperations([], [
      { range: sel, text: uuid },
    ], () => null)
    editor.focus()
  }
}

const gridStyle = computed(() => {
  if (!result.value) return ''
  const cols = result.value.columns
  const widths = cols.map(() => 'minmax(120px, 1fr)').join(' ')
  return `grid-template-columns: ${widths};`
})

const isSelectQuery = computed(() => {
  if (!editor) return false
  const sel = editor.getSelection()
  let sql: string
  if (sel && !sel.isEmpty()) {
    const model = editor.getModel()
    sql = model?.getValueInRange(sel) || editor.getValue()
  } else {
    sql = editor.getValue()
  }
  return /^\s*(select|show|describe|explain|with)\b/i.test(sql)
})

const groupedClusters = computed(() => {
  const map = new Map<string, ClusterInfo[]>()
  for (const c of clusters.value) {
    if (!map.has(c.cluster)) map.set(c.cluster, [])
    map.get(c.cluster)!.push(c)
  }
  return map
})

onMounted(async () => {
  await nextTick()
  if (!editorRef.value) return

  const workerCode = `
    self.languages = self.languages || {};
    self.languages['sql'] = { mimetypes: ['text/x-sql', 'text/x-mysql'] };
    self.onmessage = function(e) {
      var msg = e.data;
      if (msg && msg.type === 'initialize') {
        self.postMessage({ type: 'initialized' });
      }
    };
  `
  const blob = new Blob([workerCode], { type: 'application/javascript' })
  self.MonacoEnvironment = {
    getWorkerUrl(_moduleId: string, label: string) {
      if (label === 'sql') {
        return URL.createObjectURL(blob)
      }
      return URL.createObjectURL(blob)
    },
  }

  editor = monaco.editor.create(editorRef.value, {
    value: '-- ClickHouse SQL\nSELECT version()',
    language: 'sql',
    theme: 'vs-dark',
    automaticLayout: true,
    minimap: { enabled: false },
    fontSize: 13,
    fontFamily: 'Consolas, "JetBrains Mono", Menlo, monospace',
    tabSize: 2,
    wordWrap: 'on',
    scrollBeyondLastLine: false,
    lineNumbers: 'on',
    renderLineHighlight: 'line',
    cursorBlinking: 'smooth',
    smoothScrolling: true,
  })

  editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter, () => {
    runQuery()
  })

  monaco.editor.defineTheme('ch-dark', {
    base: 'vs-dark',
    inherit: true,
    rules: [],
    colors: {
      'editor.background': '#1d1e22',
      'editor.foreground': '#e4e4e7',
      'editor.lineHighlightBackground': '#2a2d33',
      'editorCursor.foreground': '#e74c3c',
      'editor.selectionBackground': 'rgba(231, 76, 60, 0.3)',
      'editorLineNumber.foreground': '#5c6270',
      'editorLineNumber.activeForeground': '#e4e4e7',
    },
  })
  monaco.editor.setTheme('ch-dark')

  watch(
    () => store.activeConnectionId,
    async (id) => {
      if (id) {
        await loadDatabases()
        await loadClusters()
      }
    },
    { immediate: true }
  )
})

onBeforeUnmount(() => {
  editor?.dispose()
})
</script>

<template>
  <div class="h-full w-full flex flex-col">
    <div class="flex items-center gap-2 px-3 py-2 border-b border-ch-border h-11 bg-ch-panel shrink-0">
      <div v-if="store.activeConnection" class="flex items-center gap-2 text-sm">
        <span class="w-2 h-2 rounded-full bg-green-500"></span>
        <span class="text-ch-text font-medium">{{ store.activeConnection.name }}</span>
        <span class="text-ch-muted text-xs">{{ store.activeConnection.host }}:{{ store.activeConnection.port }}</span>
        <span v-if="store.activeConnection.database" class="text-ch-muted text-xs bg-ch-hover px-1.5 py-0.5 rounded">{{ store.activeConnection.database }}</span>
      </div>
      <div v-else class="text-ch-muted text-sm">{{ t('editor.selectConnection') }}</div>

      <div class="flex-1" />

      <button
        v-if="store.activeConnection"
        @click="loadDatabases(); loadClusters()"
        class="btn-ghost btn-sm flex items-center gap-1"
        :disabled="loadingMeta || loadingClusters"
        :title="t('editor.refresh')"
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" :class="{ 'animate-spin': loadingMeta || loadingClusters }"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
      </button>
      <button
        v-if="store.activeConnection"
        @click="runQuery"
        class="btn-primary btn-sm flex items-center gap-1"
        :disabled="querying"
      >
        <svg v-if="!querying" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>
        <svg v-else width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="animate-spin"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
        {{ querying ? t('editor.running') : t('editor.run') }}
      </button>
    </div>

    <div class="flex-1 flex min-h-0">
      <div v-if="store.activeConnection" class="w-56 border-r border-ch-border bg-ch-panel flex flex-col shrink-0">
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
              class="cursor-pointer"
            >
              <div
                class="flex items-center gap-1 px-3 py-1 hover:bg-ch-hover text-ch-text"
                @click="toggleDB(db)"
              >
                <svg
                  width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
                  :class="{ 'rotate-90': expandedDBs.has(db) }"
                  class="transition-transform shrink-0"
                >
                  <polyline points="9 18 15 12 9 6"/>
                </svg>
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="text-ch-muted shrink-0"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14a9 3 0 0 0 18 0V5"/><path d="M3 12a9 3 0 0 0 18 0"/></svg>
                <span class="truncate">{{ db }}</span>
              </div>
              <div v-show="expandedDBs.has(db)" class="pl-5">
                <div v-if="loadingTables === db" class="px-3 py-1 text-xs text-ch-muted">{{ t('editor.loading') }}</div>
                <div
                  v-for="t in (tablesByDB.get(db) || [])"
                  :key="t.name"
                  class="flex items-center gap-1 px-2 py-1 hover:bg-ch-hover cursor-pointer text-ch-text text-xs"
                  @dblclick="insertTable(db, t.name)"
                  @click="insertTableName(t.name)"
                  :title="t.engine || ''"
                >
                  <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="text-ch-muted"><rect x="3" y="3" width="18" height="18" rx="2"/><line x1="3" y1="9" x2="21" y2="9"/><line x1="3" y1="15" x2="21" y2="15"/><line x1="9" y1="3" x2="9" y2="21"/><line x1="15" y1="3" x2="15" y2="21"/></svg>
                  <span class="truncate">{{ t.name }}</span>
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
                        @click="insertClusterUUID(node.uuid)"
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
      </div>

      <div class="flex-1 flex flex-col min-w-0">
        <div class="h-1/2 min-h-[180px] border-b border-ch-border">
          <div ref="editorRef" class="w-full h-full"></div>
        </div>

        <div class="flex-1 flex flex-col min-h-[120px]">
          <div class="flex items-center gap-2 px-3 py-1.5 border-b border-ch-border text-xs text-ch-muted bg-ch-panel shrink-0">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2"/><line x1="3" y1="9" x2="21" y2="9"/><line x1="3" y1="15" x2="21" y2="15"/><line x1="9" y1="3" x2="9" y2="21"/></svg>
            <span class="font-semibold">{{ t('editor.results') }}</span>
            <template v-if="result">
              <span class="text-ch-text">{{ result.row_count.toLocaleString() }} {{ t('editor.rows') }}</span>
              <span>·</span>
              <span class="text-ch-text">{{ result.elapsed_ms?.toFixed(2) }} ms</span>
            </template>
          </div>

          <div v-if="error" class="flex-1 flex items-start justify-center p-6">
            <div class="bg-red-900/30 border border-red-700 rounded-md p-4 max-w-full w-full text-red-300 text-sm font-mono whitespace-pre-wrap break-all">
              {{ error }}
            </div>
          </div>

          <div v-else-if="result && result.columns.length > 0" class="flex-1 overflow-auto">
            <div
              class="grid border-b border-ch-border sticky top-0 bg-ch-panel z-10"
              :style="gridStyle"
            >
              <div
                v-for="col in result.columns"
                :key="'h-' + col"
                class="px-3 py-2 text-xs font-semibold text-ch-text bg-ch-panel border-r border-ch-border last:border-r-0 truncate sticky top-0"
              >
                {{ col }}
              </div>
            </div>
            <div
              v-for="(row, ri) in result.rows"
              :key="'r-' + ri"
              class="grid border-b border-ch-border/40 hover:bg-ch-hover"
              :style="gridStyle"
            >
              <div
                v-for="(val, ci) in row"
                :key="'c-' + ci"
                class="px-3 py-1.5 text-xs text-ch-text border-r border-ch-border/40 last:border-r-0 truncate font-mono"
                :title="val !== null && val !== undefined ? String(val) : 'NULL'"
              >
                <span v-if="val === null || val === undefined" class="text-ch-muted italic">NULL</span>
                <template v-else>
                  <span v-if="typeof val === 'string' && (val.startsWith('{') || val.startsWith('['))" class="text-blue-400">{{ String(val).slice(0, 100) }}</span>
                  <span v-else>{{ String(val) }}</span>
                </template>
              </div>
            </div>
          </div>

          <div v-else-if="!querying && !result && !error" class="flex-1 flex flex-col items-center justify-center text-ch-muted text-sm gap-2">
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1" class="text-ch-muted"><polygon points="5 3 19 12 5 21 5 3"/></svg>
            <div>{{ t('editor.queryHint') }}</div>
          </div>

          <div v-else-if="result && result.row_count === 0" class="flex-1 flex items-center justify-center text-ch-muted text-sm">
            {{ t('editor.noResults') }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>