<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import type * as Monaco from 'monaco-editor/editor/editor.api.js'
import { useAppStore } from '../stores/app'
import { api } from '../api'
import type { QueryResult, ClusterInfo, ResultExportFormat, HistoryEntry } from '../types'
import { useVirtualizer } from '@tanstack/vue-virtual'
import { createTab, type QueryTab } from '../utils/tabs'
import { ROW_HEIGHT as RESULT_ROW_HEIGHT, measureColWidths, buildGridStyle, shouldVirtualize } from '../utils/resultTable'
import {
  rowToTSV,
  columnsToCSV,
  buildInsertStatement,
  buildInsertBatch,
  insertTarget,
  sqlLiteral,
} from '../utils/sqlLiteral'
import CreateTableModal from '../components/CreateTableModal.vue'
import EditTableModal from '../components/EditTableModal.vue'
import { buildDropTableSQL } from '../utils/ddl'

const { t } = useI18n()
const store = useAppStore()

const CANCELED_MESSAGE = '查询已取消'

const editorRef = ref<HTMLElement | null>(null)
let editor: Monaco.editor.IStandaloneCodeEditor | null = null

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

const selectedDB = ref<string | null>(null)
const selectedTable = ref<string | null>(null)
const selectedCluster = ref<string | null>(null)

const tabs = ref<QueryTab[]>([createTab(1)])
const activeTabId = ref<string>(tabs.value[0].id)

const activeTab = computed(() => tabs.value.find((t) => t.id === activeTabId.value) || tabs.value[0])
const result = computed(() => activeTab.value?.result ?? null)
const querying = computed(() => activeTab.value?.querying ?? false)
const error = computed(() => activeTab.value?.error ?? null)
const canceled = computed(() => activeTab.value?.canceled ?? false)

const resultExportFormats = ref<ResultExportFormat[]>([])
const exportingResult = ref(false)
const exportedPath = ref<string | null>(null)
const showHistory = ref(false)
const historyEntries = ref<HistoryEntry[]>([])
const historyLoading = ref(false)
const renamingTabId = ref<string | null>(null)
const renamingTitle = ref('')

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

const cellMenu = ref<{
  visible: boolean
  x: number
  y: number
  rowIndex: number
  colIndex: number
}>({ visible: false, x: 0, y: 0, rowIndex: 0, colIndex: 0 })

const headerMenu = ref<{ visible: boolean; x: number; y: number }>({ visible: false, x: 0, y: 0 })

const copiedHint = ref<string | null>(null)
let copiedHintTimer: number | undefined

function showCopiedHint(label: string) {
  copiedHint.value = label
  if (copiedHintTimer) clearTimeout(copiedHintTimer)
  copiedHintTimer = window.setTimeout(() => { copiedHint.value = null }, 1500)
}

function closeCellMenu() {
  cellMenu.value.visible = false
}

function closeHeaderMenu() {
  headerMenu.value.visible = false
}

function openCellMenu(e: MouseEvent, rowIndex: number, colIndex: number) {
  e.stopPropagation()
  closeAllMenus()
  cellMenu.value = { visible: true, x: e.clientX, y: e.clientY, rowIndex, colIndex }
}

function openHeaderMenu(e: MouseEvent) {
  e.stopPropagation()
  closeAllMenus()
  headerMenu.value = { visible: true, x: e.clientX, y: e.clientY }
}

function currentRow(): unknown[] | null {
  const r = result.value
  if (!r) return null
  const row = r.rows[cellMenu.value.rowIndex]
  return row ?? null
}

function insertPrefix(): string {
  return insertTarget(selectedDB.value, selectedTable.value)
}

async function copyText(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    showCopiedHint(label)
  } catch {
    showCopiedHint(t('editor.copyFailed'))
  }
}

async function copyCell() {
  const row = currentRow()
  if (!row) return
  const val = row[cellMenu.value.colIndex]
  const text = val === null || val === undefined ? '' : String(val)
  closeCellMenu()
  await copyText(text, t('editor.copyCell'))
}

async function copyRowTSV() {
  const row = currentRow()
  if (!row) return
  closeCellMenu()
  await copyText(rowToTSV(row), t('editor.copyRow'))
}

async function copyRowInsert() {
  const row = currentRow()
  const r = result.value
  if (!row || !r) return
  closeCellMenu()
  const cols = r.columns.map((c) => c.name)
  await copyText(buildInsertStatement(insertPrefix(), cols, row), t('editor.copyInsert'))
}

async function copyAllInsert() {
  const r = result.value
  if (!r || r.rows.length === 0) return
  closeCellMenu()
  const cols = r.columns.map((c) => c.name)
  await copyText(buildInsertBatch(insertPrefix(), cols, r.rows), t('editor.copyInsertAll'))
}

async function copyCellLiteral() {
  const row = currentRow()
  if (!row) return
  closeCellMenu()
  await copyText(sqlLiteral(row[cellMenu.value.colIndex]), t('editor.copyCellLiteral'))
}

async function copyColumns() {
  const r = result.value
  if (!r) return
  closeHeaderMenu()
  await copyText(columnsToCSV(r.columns.map((c) => c.name)), t('editor.copyColumns'))
}

function closeAllMenus() {
  tableMenu.value.visible = false
  dbMenu.value.visible = false
  cellMenu.value.visible = false
  headerMenu.value.visible = false
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

async function toggleDB(db: string) {
  selectedDB.value = db
  selectedTable.value = null
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
  selectedCluster.value = cluster
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
    } else if (r.rows.length > 0 && r.rows[0].length > 0) {
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
  if (!ddlModal.value.sql || !editor) return
  editor.setValue(ddlModal.value.sql)
  syncTabSQL()
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
      activeTab.value.error = r.error
    } else {
      dropConfirm.value.visible = false
      tablesByDB.value.delete(db)
      tablesByDB.value = new Map(tablesByDB.value)
    }
  } catch (e: any) {
    activeTab.value.error = e?.message || 'Drop table failed'
  }
}

function refreshTables(db: string) {
  tablesByDB.value.delete(db)
  tablesByDB.value = new Map(tablesByDB.value)
  viewsByDB.value.delete(db)
  viewsByDB.value = new Map(viewsByDB.value)
  toggleDB(db)
}

async function runQuery() {
  const tab = activeTab.value
  if (!store.activeConnectionId) {
    tab.error = t('editor.noConnection')
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

  tab.querying = true
  tab.result = null
  tab.error = null
  tab.canceled = false
  tab.resultScrollTop = 0
  tab.queryId = crypto.randomUUID()
  const queryId = tab.queryId
  try {
    const r: QueryResult = await api.Query(store.activeConnectionId, sql, queryId)
    if (tab.queryId !== queryId) return
    if (r.error === CANCELED_MESSAGE) {
      tab.canceled = true
    } else {
      tab.result = r
      if (r.error) {
        tab.error = r.error
      }
    }
  } catch (e: any) {
    if (tab.queryId !== queryId) return
    tab.error = e?.message || t('editor.queryFailed')
  } finally {
    if (tab.queryId === queryId) {
      tab.querying = false
      tab.queryId = null
    }
  }
}

// 取消当前标签正在执行的查询；后端未找到该查询时静默忽略
async function cancelQuery() {
  const tab = activeTab.value
  const queryId = tab.queryId
  if (!queryId) return
  try {
    await api.CancelQuery(queryId)
  } catch {
    // 取消失败不阻塞界面，查询结束后 finally 会恢复状态
  }
}

// 把当前结果集按指定格式导出，弹出系统「另存为」对话框
async function exportResult(format: ResultExportFormat) {
  if (!result.value || exportingResult.value) return
  const r = result.value
  exportingResult.value = true
  try {
    const path = await api.SaveQueryResultAs(format.id, r.columns.map((c) => c.name), r.rows)
    if (path) {
      exportedPath.value = path
      setTimeout(() => { exportedPath.value = null }, 4000)
    }
  } catch (e: any) {
    const msg = String(e?.message || e)
    if (!msg.includes('cancelled by user')) {
      activeTab.value.error = msg
    }
  } finally {
    exportingResult.value = false
  }
}

async function loadHistory() {
  historyLoading.value = true
  try {
    historyEntries.value = await api.GetQueryHistory()
  } catch {
    historyEntries.value = []
  } finally {
    historyLoading.value = false
  }
}

async function toggleHistory() {
  showHistory.value = !showHistory.value
  if (showHistory.value) await loadHistory()
}

function applyHistory(entry: HistoryEntry) {
  if (!editor) return
  editor.setValue(entry.sql)
  syncTabSQL()
  showHistory.value = false
}

async function deleteHistory(entry: HistoryEntry, e: MouseEvent) {
  e.stopPropagation()
  try {
    await api.DeleteQueryHistory(entry.id)
    historyEntries.value = historyEntries.value.filter((h) => h.id !== entry.id)
  } catch {
  }
}

async function clearHistory() {
  if (historyEntries.value.length === 0) return
  if (!confirm(t('editor.clearHistoryConfirm'))) return
  try {
    await api.ClearQueryHistory()
    historyEntries.value = []
  } catch {
  }
}

function formatHistoryTime(ts: string): string {
  const d = new Date(ts)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

// 启动时从 tabs.json 恢复上次打开的标签与活动标签。
// 首次启动（无快照）时保留默认标签，什么都不做。
async function restorePersistedTabs() {
  try {
    const snapshot = await api.GetTabs()
    if (!snapshot || !snapshot.tabs || snapshot.tabs.length === 0) return
    const restored: QueryTab[] = snapshot.tabs.map((t, i) => ({
      id: t.id || 'tab-' + i,
      title: t.title || `Query ${i + 1}`,
      sql: t.sql ?? '',
      result: null,
      error: null,
      querying: false,
      queryId: null,
      canceled: false,
      resultScrollTop: 0,
      viewState: null,
    }))
    tabs.value = restored
    const active = restored.find((t) => t.id === snapshot.active_tab_id)
    activeTabId.value = (active || restored[0]).id
    if (editor) restoreTab(active || restored[0])
  } catch {
    // 读取失败时保留默认标签，不打断用户
  }
}

let persistTimer: number | undefined

// 把当前标签列表与活动标签快照持久化，供下次启动恢复
function persistTabs() {
  window.clearTimeout(persistTimer)
  persistTimer = window.setTimeout(() => {
    const snapshot = {
      tabs: tabs.value.map((t) => ({ id: t.id, title: t.title, sql: t.sql })),
      active_tab_id: activeTabId.value,
    }
    api.SaveTabs(snapshot).catch(() => {})
  }, 500)
}

function saveActiveTabState() {
  const tab = activeTab.value
  if (!tab || !editor) return
  tab.sql = editor.getValue()
  tab.viewState = editor.saveViewState()
}

function restoreTab(tab: QueryTab) {
  if (!editor) return
  editor.setValue(tab.sql)
  if (tab.viewState) editor.restoreViewState(tab.viewState as Monaco.editor.ICodeEditorViewState)
  editor.focus()
  nextTick(() => {
    if (resultBodyRef.value) resultBodyRef.value.scrollTop = tab.resultScrollTop || 0
  })
}

function switchTab(id: string) {
  if (id === activeTabId.value) return
  saveActiveTabState()
  activeTabId.value = id
  nextTick(() => restoreTab(activeTab.value))
  persistTabs()
}

function addTab() {
  saveActiveTabState()
  const tab = createTab(tabs.value.length + 1)
  tabs.value.push(tab)
  activeTabId.value = tab.id
  nextTick(() => restoreTab(tab))
  persistTabs()
}

function closeTab(id: string) {
  if (tabs.value.length === 1) {
    const tab = tabs.value[0]
    if (tab.querying && tab.queryId) {
      api.CancelQuery(tab.queryId).catch(() => {})
    }
    tab.sql = '-- ClickHouse SQL\nSELECT version()'
    tab.result = null
    tab.error = null
    tab.canceled = false
    tab.querying = false
    tab.queryId = null
    tab.resultScrollTop = 0
    tab.viewState = null
    editor?.setValue(tab.sql)
    persistTabs()
    return
  }
  const idx = tabs.value.findIndex((t) => t.id === id)
  if (idx < 0) return
  const closing = tabs.value[idx]
  if (closing.querying && closing.queryId) {
    api.CancelQuery(closing.queryId).catch(() => {})
  }
  tabs.value.splice(idx, 1)
  if (activeTabId.value === id) {
    const next = tabs.value[Math.min(idx, tabs.value.length - 1)]
    activeTabId.value = next.id
    nextTick(() => restoreTab(next))
  }
  persistTabs()
}

function startRenameTab(tab: QueryTab) {
  renamingTabId.value = tab.id
  renamingTitle.value = tab.title
}

function commitRenameTab() {
  const id = renamingTabId.value
  if (!id) return
  const tab = tabs.value.find((t) => t.id === id)
  const title = renamingTitle.value.trim()
  if (tab && title) {
    tab.title = title
    persistTabs()
  }
  renamingTabId.value = null
  renamingTitle.value = ''
}

function syncTabSQL() {
  const tab = activeTab.value
  if (!tab || !editor) return
  tab.sql = editor.getValue()
}

function insertTable(db: string, table: string) {
  selectedDB.value = db
  selectedTable.value = table
  if (!editor) return
  const snippet = `SELECT * FROM ${db}.${table} LIMIT 10`
  editor.setValue(snippet)
  syncTabSQL()
  runQuery()
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

const ROW_HEIGHT = RESULT_ROW_HEIGHT
const resultBodyRef = ref<HTMLElement | null>(null)
const resultHeaderRef = ref<HTMLElement | null>(null)

let syncingScroll = false

function syncScroll(from: HTMLElement | null, to: HTMLElement | null) {
  if (syncingScroll || !from || !to) return
  syncingScroll = true
  to.scrollLeft = from.scrollLeft
  requestAnimationFrame(() => { syncingScroll = false })
}

function onResultScroll() {
  syncScroll(resultBodyRef.value, resultHeaderRef.value)
  if (activeTab.value && resultBodyRef.value) {
    activeTab.value.resultScrollTop = resultBodyRef.value.scrollTop
  }
}

function onHeaderScroll() {
  syncScroll(resultHeaderRef.value, resultBodyRef.value)
}

const isVirtual = computed(() => shouldVirtualize(result.value?.rows.length ?? 0))

const colWidths = ref<number[]>([])

const gridStyle = computed(() =>
  result.value ? buildGridStyle(colWidths.value, result.value.columns.length) : ''
)

const totalGridWidth = computed(() => {
  const n = result.value?.columns.length ?? 0
  if (colWidths.value.length !== n) return null
  return colWidths.value.reduce((a, b) => a + b, 0)
})

const virtualizer = useVirtualizer(
  computed(() => ({
    count: result.value?.rows.length ?? 0,
    getScrollElement: () => resultBodyRef.value,
    estimateSize: () => ROW_HEIGHT,
    overscan: 10,
  }))
)

const virtualRows = computed(() => (isVirtual.value ? virtualizer.value.getVirtualItems() : []))
const virtualTotalSize = computed(() => (isVirtual.value ? virtualizer.value.getTotalSize() : 0))

function visibleRows(): { ri: number; row: any[] }[] {
  const rows = result.value?.rows ?? []
  if (!isVirtual.value) {
    return rows.map((row, ri) => ({ ri, row }))
  }
  return virtualRows.value.map((v) => ({ ri: v.index, row: rows[v.index] }))
}

watch(
  () => [result.value?.rows.length, result.value?.columns.length],
  () => {
    const cols = result.value?.columns
    colWidths.value = cols && cols.length ? measureColWidths(cols, result.value?.rows ?? []) : []
    nextTick(() => {
      virtualizer.value.measure()
      if (resultBodyRef.value) resultBodyRef.value.scrollTop = 0
    })
  },
  { immediate: true }
)

const groupedClusters = computed(() => {
  const map = new Map<string, ClusterInfo[]>()
  for (const c of clusters.value) {
    if (!map.has(c.cluster)) map.set(c.cluster, [])
    map.get(c.cluster)!.push(c)
  }
  return map
})

onMounted(async () => {
  api.GetResultExportFormats().then((list) => { resultExportFormats.value = list }).catch(() => {})
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
  ;(self as any).MonacoEnvironment = {
    getWorkerUrl(_moduleId: string, _label: string) {
      return URL.createObjectURL(blob)
    },
  }

  window.addEventListener('click', closeAllMenus)
  window.addEventListener('contextmenu', closeAllMenus)

  const [monacoApi] = await Promise.all([
    import('monaco-editor/editor/editor.api.js'),
    import('monaco-editor/languages/definitions/sql/register.js'),
  ])
  const monaco = monacoApi as unknown as typeof Monaco
  if (!editorRef.value) return

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

  tabs.value[0].sql = editor.getValue()
  editor.onDidChangeModelContent(() => {
    if (activeTab.value) activeTab.value.sql = editor!.getValue()
    persistTabs()
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

  await restorePersistedTabs()

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
  window.clearTimeout(persistTimer)
  const snapshot = {
    tabs: tabs.value.map((t) => ({ id: t.id, title: t.title, sql: t.sql })),
    active_tab_id: activeTabId.value,
  }
  api.SaveTabs(snapshot).catch(() => {})
  window.removeEventListener('click', closeAllMenus)
  window.removeEventListener('contextmenu', closeAllMenus)
  editor?.dispose()
})
</script>

<template>
  <div class="h-full w-full flex flex-col">
    <div class="flex items-center gap-2 px-3 py-2 border-b border-ch-border h-11 bg-ch-panel shrink-0">
      <div v-if="store.activeConnection" class="flex items-center gap-2 text-sm">
        <span
          class="w-2 h-2 rounded-full shrink-0 cursor-pointer"
          :class="{
            'bg-yellow-500 animate-pulse': store.getConnectionStatus(store.activeConnection.id) === 'connecting',
            'bg-green-500': store.getConnectionStatus(store.activeConnection.id) === 'connected',
            'bg-red-500': store.getConnectionStatus(store.activeConnection.id) === 'failed',
            'bg-ch-muted/50': store.getConnectionStatus(store.activeConnection.id) === 'unknown',
          }"
          :title="
            store.getConnectionError(store.activeConnection.id)
              ? t('status.failed') + ': ' + store.getConnectionError(store.activeConnection.id)
              : store.getConnectionStatus(store.activeConnection.id) === 'connected' ? t('status.connected')
              : store.getConnectionStatus(store.activeConnection.id) === 'connecting' ? t('status.connecting')
              : store.getConnectionStatus(store.activeConnection.id) === 'failed' ? t('status.failed')
              : t('status.unknown')
          "
          @click="store.checkConnection(store.activeConnection!.id)"
        ></span>
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
        @click="toggleHistory"
        class="btn-ghost btn-sm flex items-center gap-1"
        :title="t('editor.history')"
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="9"/><polyline points="12 7 12 12 15 15"/></svg>
      </button>
      <button
        v-if="store.activeConnection"
        @click="querying ? cancelQuery() : runQuery()"
        class="btn-primary btn-sm flex items-center gap-1"
        :class="querying ? 'btn-danger' : ''"
      >
        <svg v-if="!querying" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>
        <svg v-else width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="5" y="5" width="14" height="14" rx="2"/></svg>
        {{ querying ? t('editor.cancel') : t('editor.run') }}
      </button>
    </div>

    <div class="flex items-center gap-1 px-2 border-b border-ch-border bg-ch-panel shrink-0 h-9 overflow-x-auto">
      <div
        v-for="tab in tabs"
        :key="tab.id"
        class="group flex items-center gap-1 px-2 h-7 rounded text-xs cursor-pointer border shrink-0 max-w-[200px]"
        :class="tab.id === activeTabId ? 'bg-ch-accent/15 border-ch-accent/60 text-ch-accent' : 'border-transparent text-ch-muted hover:bg-ch-hover'"
        @click="switchTab(tab.id)"
        @dblclick="startRenameTab(tab)"
      >
        <input
          v-if="renamingTabId === tab.id"
          v-model="renamingTitle"
          class="bg-transparent border border-ch-accent rounded px-1 w-24 text-ch-text outline-none"
          @click.stop
          @keydown.enter="commitRenameTab"
          @keydown.esc="renamingTabId = null"
          @blur="commitRenameTab"
          v-focus
        />
        <span v-else class="truncate">{{ tab.title }}</span>
        <svg v-if="tab.querying" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="animate-spin shrink-0"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
        <button
          class="shrink-0 p-0.5 rounded hover:bg-ch-border text-ch-muted hover:text-ch-text"
          :title="t('common.close')"
          @click.stop="closeTab(tab.id)"
        >
          <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
        </button>
      </div>
      <button
        class="flex items-center justify-center w-6 h-6 rounded text-ch-muted hover:bg-ch-hover hover:text-ch-text shrink-0"
        :title="t('editor.newTab')"
        @click="addTab"
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
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
              class="cursor-pointer group"
            >
              <div
                class="flex items-center gap-1 px-3 py-1 hover:bg-ch-hover border-l-2 border-l-transparent"
                :class="selectedDB === db ? 'bg-ch-accent/15 border-l-ch-accent text-ch-accent' : 'text-ch-text'"
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
                  :class="selectedDB === db && selectedTable === tb.name ? 'bg-ch-accent/15 border-l-ch-accent text-ch-accent' : 'text-ch-text'"
                  @click="insertTable(db, tb.name)"
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
                  :class="selectedDB === db && selectedTable === v.name ? 'bg-ch-accent/15 border-l-ch-accent text-ch-accent' : 'text-ch-text'"
                  @click="insertTable(db, v.name)"
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
        <div :class="showHistory ? 'h-[35%]' : 'h-1/2'" class="min-h-[120px]">
          <div ref="editorRef" class="w-full h-full"></div>
        </div>

        <div v-if="showHistory" class="h-[28%] min-h-[140px] border-y border-ch-border bg-ch-panel flex flex-col">
          <div class="flex items-center gap-2 px-3 py-1.5 border-b border-ch-border text-xs text-ch-muted shrink-0">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="9"/><polyline points="12 7 12 12 15 15"/></svg>
            <span class="font-semibold">{{ t('editor.history') }}</span>
            <span v-if="!historyLoading" class="text-ch-text">{{ historyEntries.length }}</span>
            <div class="ml-auto flex items-center gap-1">
              <button class="px-2 py-0.5 rounded hover:bg-ch-hover hover:text-ch-text" :title="t('editor.refresh')" @click="loadHistory">
                <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" :class="{ 'animate-spin': historyLoading }"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
              </button>
              <button class="px-2 py-0.5 rounded hover:bg-ch-hover hover:text-ch-text disabled:opacity-40" :disabled="historyEntries.length === 0" @click="clearHistory">{{ t('editor.clearHistory') }}</button>
              <button class="px-2 py-0.5 rounded hover:bg-ch-hover hover:text-ch-text" :title="t('common.close')" @click="showHistory = false">
                <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
              </button>
            </div>
          </div>
          <div class="flex-1 overflow-auto">
            <div v-if="historyLoading" class="px-3 py-2 text-xs text-ch-muted italic">{{ t('editor.loading') }}</div>
            <div v-else-if="historyEntries.length === 0" class="px-3 py-2 text-xs text-ch-muted italic">{{ t('editor.historyEmpty') }}</div>
            <div
              v-for="h in historyEntries"
              :key="h.id"
              class="group flex items-start gap-2 px-3 py-1.5 border-b border-ch-border/50 cursor-pointer hover:bg-ch-hover"
              @click="applyHistory(h)"
            >
              <span class="mt-0.5 w-1.5 h-1.5 rounded-full shrink-0" :class="h.success ? 'bg-green-500' : 'bg-red-500'"></span>
              <div class="flex-1 min-w-0">
                <div class="text-xs text-ch-text font-mono truncate whitespace-pre" :title="h.sql">{{ h.sql.split('\n')[0] }}</div>
                <div class="flex items-center gap-2 mt-0.5 text-[10px] text-ch-muted">
                  <span>{{ formatHistoryTime(h.timestamp) }}</span>
                  <span>·</span>
                  <span>{{ h.duration_ms?.toFixed(2) }} ms</span>
                </div>
              </div>
              <button
                class="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-ch-border text-ch-muted hover:text-ch-text shrink-0"
                :title="t('common.delete')"
                @click="deleteHistory(h, $event)"
              >
                <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
              </button>
            </div>
          </div>
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
            <div v-if="result && result.columns.length > 0" class="ml-auto flex items-center gap-1">
              <span v-if="exportedPath" class="text-ch-accent text-xs truncate max-w-[240px]" :title="exportedPath">{{ t('editor.exported') }}</span>
              <span v-if="copiedHint" class="text-ch-accent text-xs truncate max-w-[240px]">{{ copiedHint }}</span>
              <button
                v-for="f in resultExportFormats"
                :key="f.id"
                class="flex items-center gap-1 px-2 py-0.5 rounded hover:bg-ch-hover text-ch-muted hover:text-ch-text disabled:opacity-50"
                :disabled="exportingResult"
                :title="f.description"
                @click="exportResult(f)"
              >
                <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
                <span>{{ t('editor.exportAs') }} {{ f.name }}</span>
              </button>
            </div>
          </div>

          <div v-if="result && result.truncated" class="px-3 py-1.5 border-b border-ch-border bg-amber-900/25 text-amber-300 text-xs shrink-0">
            {{ t('editor.truncatedHint', { limit: result.row_limit.toLocaleString() }) }}
          </div>

          <div v-if="canceled" class="flex-1 flex items-start justify-center p-6">
            <div class="bg-ch-panel border border-ch-border rounded-md p-4 max-w-full w-full text-ch-muted text-sm whitespace-pre-wrap">
              {{ t('editor.queryCanceled') }}
            </div>
          </div>

          <div v-else-if="error" class="flex-1 flex items-start justify-center p-6">
            <div class="bg-red-900/30 border border-red-700 rounded-md p-4 max-w-full w-full text-red-300 text-sm font-mono whitespace-pre-wrap break-all">
              {{ error }}
            </div>
          </div>

          <div v-else-if="result && result.columns.length > 0" class="flex-1 flex flex-col min-h-0 overflow-hidden">
            <div ref="resultHeaderRef" class="overflow-x-auto overflow-y-hidden shrink-0 bg-ch-panel border-b border-ch-border" @scroll="onHeaderScroll">
              <div class="grid" :style="gridStyle" :class="totalGridWidth ? '' : 'min-w-full'">
                <div
                  v-for="(col, ci) in result.columns"
                  :key="'h-' + ci"
                  class="px-3 py-2 text-xs font-semibold text-ch-text bg-ch-panel border-r border-ch-border last:border-r-0 truncate flex items-center gap-1 select-none"
                  :title="col.type ? col.name + ' · ' + col.type : col.name"
                  @contextmenu.prevent="openHeaderMenu($event)"
                >
                  <span class="truncate">{{ col.name }}</span>
                  <span v-if="col.type" class="text-[10px] font-normal text-ch-muted truncate shrink-0">{{ col.type }}</span>
                </div>
              </div>
            </div>

            <div
              ref="resultBodyRef"
              class="flex-1 overflow-auto min-h-0"
              @scroll="onResultScroll"
            >
              <div class="relative" :style="{ height: isVirtual ? virtualTotalSize + 'px' : 'auto' }">
                <div
                  v-if="isVirtual"
                  class="absolute top-0 left-0 w-full"
                  :style="{ transform: `translateY(${virtualRows[0]?.start ?? 0}px)` }"
                  :class="totalGridWidth ? '' : 'min-w-full'"
                >
                  <div
                    v-for="item in visibleRows()"
                    :key="'r-' + item.ri"
                    class="grid border-b border-ch-border/40 hover:bg-ch-hover"
                    :class="totalGridWidth ? '' : 'min-w-full'"
                    :style="[{ height: ROW_HEIGHT + 'px' }, gridStyle]"
                  >
                    <div
                      v-for="(val, ci) in item.row"
                      :key="'c-' + ci"
                      class="px-3 text-xs text-ch-text border-r border-ch-border/40 last:border-r-0 truncate font-mono leading-none flex items-center"
                      :title="val !== null && val !== undefined ? String(val) : 'NULL'"
                      @contextmenu.prevent="openCellMenu($event, item.ri, ci)"
                    >
                      <span v-if="val === null || val === undefined" class="text-ch-muted italic">NULL</span>
                      <template v-else>
                        <span v-if="typeof val === 'string' && (val.startsWith('{') || val.startsWith('['))" class="text-blue-400">{{ String(val).slice(0, 100) }}</span>
                        <span v-else>{{ String(val) }}</span>
                      </template>
                    </div>
                  </div>
                </div>

                <template v-else>
                  <div
                    v-for="item in visibleRows()"
                    :key="'r-' + item.ri"
                    class="grid border-b border-ch-border/40 hover:bg-ch-hover"
                    :style="gridStyle"
                  >
                    <div
                      v-for="(val, ci) in item.row"
                      :key="'c-' + ci"
                      class="px-3 py-1.5 text-xs text-ch-text border-r border-ch-border/40 last:border-r-0 truncate font-mono"
                      :title="val !== null && val !== undefined ? String(val) : 'NULL'"
                      @contextmenu.prevent="openCellMenu($event, item.ri, ci)"
                    >
                      <span v-if="val === null || val === undefined" class="text-ch-muted italic">NULL</span>
                      <template v-else>
                        <span v-if="typeof val === 'string' && (val.startsWith('{') || val.startsWith('['))" class="text-blue-400">{{ String(val).slice(0, 100) }}</span>
                        <span v-else>{{ String(val) }}</span>
                      </template>
                    </div>
                  </div>
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

    <!-- DB Context Menu -->
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

    <!-- Result Cell Context Menu -->
    <Teleport to="body">
      <div
        v-if="cellMenu.visible"
        class="fixed z-[1000] bg-ch-panel border border-ch-border rounded-md shadow-xl py-1 min-w-[200px] text-xs select-none"
        :style="{ left: cellMenu.x + 'px', top: cellMenu.y + 'px' }"
        @click.stop
        @contextmenu.prevent
      >
        <button
          @click="copyCell"
          class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-ch-text hover:bg-ch-hover"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
          {{ t('editor.copyCell') }}
        </button>
        <button
          @click="copyCellLiteral"
          class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-ch-text hover:bg-ch-hover"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 7 4 4 20 4 20 7"/><line x1="9" y1="20" x2="15" y2="20"/><line x1="12" y1="4" x2="12" y2="20"/></svg>
          {{ t('editor.copyCellLiteral') }}
        </button>
        <button
          @click="copyRowTSV"
          class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-ch-text hover:bg-ch-hover"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="18" x2="21" y2="18"/></svg>
          {{ t('editor.copyRow') }}
        </button>
        <div class="border-t border-ch-border/50 my-1"></div>
        <button
          @click="copyRowInsert"
          class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-ch-text hover:bg-ch-hover"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>
          {{ t('editor.copyInsert') }}
        </button>
        <button
          @click="copyAllInsert"
          class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-ch-text hover:bg-ch-hover"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
          {{ t('editor.copyInsertAll', { rows: result?.rows.length ?? 0 }) }}
        </button>
      </div>
    </Teleport>

    <!-- Result Header Context Menu -->
    <Teleport to="body">
      <div
        v-if="headerMenu.visible"
        class="fixed z-[1000] bg-ch-panel border border-ch-border rounded-md shadow-xl py-1 min-w-[200px] text-xs select-none"
        :style="{ left: headerMenu.x + 'px', top: headerMenu.y + 'px' }"
        @click.stop
        @contextmenu.prevent
      >
        <button
          @click="copyColumns"
          class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-ch-text hover:bg-ch-hover"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
          {{ t('editor.copyColumns') }}
        </button>
      </div>
    </Teleport>

    <!-- Table Context Menu -->
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
          @click="insertTable(tableMenu.db, tableMenu.table)"
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

    <!-- DDL Modal -->
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

    <!-- Create Table Modal -->
    <CreateTableModal
      :visible="createModal.visible"
      :database="createModal.database"
      @close="createModal.visible = false"
      @created="refreshTables(createModal.database)"
    />

    <!-- Edit Table Modal -->
    <EditTableModal
      :visible="editModal.visible"
      :database="editModal.database"
      :table="editModal.table"
      @close="editModal.visible = false"
      @altered="refreshTables(editModal.database)"
    />

    <!-- Drop Table Confirm -->
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