<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import type * as Monaco from 'monaco-editor/editor/editor.api.js'
import { useAppStore } from '../stores/app'
import { api } from '../api'
import type { QueryResult, ExecBatchResult } from '../types'
import { createTab, restoreTabs, type QueryTab } from '../utils/tabs'
import { buildPageSQL, type PageState } from '../utils/pagination'
import ResultGrid from '../components/ResultGrid.vue'
import DatabaseTree from '../components/DatabaseTree.vue'
import HistoryPanel from '../components/HistoryPanel.vue'
import SavedSQLPanel from '../components/SavedSQLPanel.vue'
import { useMonaco } from '../composables/useMonaco'
import type { ApplyResult } from '../composables/useCellEditing'
import { clampSplitRatio, splitRatioFromY } from '../utils/splitPane'

const { t } = useI18n()
const store = useAppStore()

const CANCELED_MESSAGE = '查询已取消'

const {
  editorRef,
  initEditor,
  getEditor,
  invalidateCompletion,
} = useMonaco({
  onContentChange: () => {
    if (activeTab.value) activeTab.value.sql = getEditor()!.getValue()
    persistTabs()
  },
  onSelectionChange: () => {
    selectedSQL.value = currentEditorSelection()
  },
  onRun: () => {
    runQuery()
  },
  getConnectionId: () => store.activeConnectionId,
})

const selectedDB = ref<string | null>(null)
const selectedTable = ref<string | null>(null)

const tabs = ref<QueryTab[]>([createTab(1)])
const activeTabId = ref<string>(tabs.value[0].id)

const activeTab = computed(() => tabs.value.find((t) => t.id === activeTabId.value) || tabs.value[0])
const result = computed(() => activeTab.value?.result ?? null)
const querying = computed(() => activeTab.value?.querying ?? false)
const error = computed(() => activeTab.value?.error ?? null)
const canceled = computed(() => activeTab.value?.canceled ?? false)

const showHistory = ref(false)

const SPLIT_STORAGE_KEY = 'meteor-ch.splitRatio'
const splitRatio = ref(0.5)
const splitDragging = ref(false)
const splitContainerEl = ref<HTMLElement | null>(null)
const splitStart = ref<{ y: number; ratio: number; height: number } | null>(null)

function loadSplitRatio() {
  const saved = localStorage.getItem(SPLIT_STORAGE_KEY)
  const ratio = saved ? parseFloat(saved) : Number.NaN
  if (!Number.isNaN(ratio)) splitRatio.value = clampSplitRatio(ratio, 1000)
}

function onSplitStart(ev: MouseEvent) {
  if (!splitContainerEl.value) return
  ev.preventDefault()
  splitStart.value = {
    y: ev.clientY,
    ratio: splitRatio.value,
    height: splitContainerEl.value.clientHeight,
  }
  splitDragging.value = true
  document.body.style.cursor = 'row-resize'
  document.body.style.userSelect = 'none'
}

function onSplitMove(ev: MouseEvent) {
  const state = splitStart.value
  if (!state) return
  splitRatio.value = splitRatioFromY(state.y, state.ratio, ev.clientY - state.y, state.height)
}

function onSplitEnd() {
  if (!splitStart.value) return
  splitStart.value = null
  splitDragging.value = false
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
  localStorage.setItem(SPLIT_STORAGE_KEY, String(splitRatio.value))
}
const showSavedSQL = ref(false)
const savedSQLPanelRef = ref<{ reload: () => Promise<void> } | null>(null)
const saveDialogVisible = ref(false)
const saveDialogName = ref('')
const saveDialogSQL = ref('')
const saveDialogError = ref<string | null>(null)
const renamingTabId = ref<string | null>(null)
const renamingTitle = ref('')

const selectedSQL = ref('')
const availableSQL = computed(() => (selectedSQL.value.trim() ? selectedSQL.value : activeTab.value?.sql ?? ''))
const resultGridRef = ref<{ closeMenus: () => void; restoreScroll: (value: number) => void } | null>(null)
const databaseTreeRef = ref<{ closeMenus: () => void; reload: () => Promise<void> } | null>(null)
const applyResult = ref<ApplyResult | null>(null)

function closeAllMenus() {
  databaseTreeRef.value?.closeMenus()
  resultGridRef.value?.closeMenus()
}

function onResultError(msg: string) {
  activeTab.value.error = msg
}

function onResultScrollTop(value: number) {
  const tab = activeTab.value
  if (tab) tab.resultScrollTop = value
}

function onTreePreview(db: string, table: string) {
  insertTable(db, table)
}

function onTreeSelectDB(db: string) {
  selectedDB.value = db
  selectedTable.value = null
}

function onTreeInsertDDL(sql: string) {
  const ed = getEditor()
  if (!ed) return
  ed.setValue(sql)
  syncTabSQL()
}

function onTreeError(msg: string) {
  activeTab.value.error = msg
}

function onTreeRefresh() {
  databaseTreeRef.value?.reload()
}

function currentEditorSelection(): string {
  const ed = getEditor()
  if (!ed) return ''
  const sel = ed.getSelection()
  let sql = ed.getValue()
  if (sel && !sel.isEmpty()) {
    const model = ed.getModel()
    if (model) {
      sql = model.getValueInRange(sel)
    }
  }
  return sql.trim()
}

async function runQuery() {
  const sql = currentEditorSelection()
  if (!sql) return
  await executeSelectedSQL(sql)
}

async function executeSelectedSQL(rawSQL: string, page: PageState | null = null, keepResult = false) {
  const tab = activeTab.value
  if (!store.activeConnectionId) {
    tab.error = t('editor.noConnection')
    return
  }
  const sql = rawSQL.trim()
  if (!sql) return
  const execSQL = page ? buildPageSQL(sql, page.offset, page.pageSize) : sql
  if (!execSQL) return

  tab.querying = true
  if (!keepResult) {
    tab.result = null
    tab.error = null
    tab.canceled = false
    tab.resultScrollTop = 0
    tab.page = page
  }
  tab.queryId = crypto.randomUUID()
  const queryId = tab.queryId
  tab.lastExecutedSql = execSQL
  try {
    const r: QueryResult = await api.Query(store.activeConnectionId, execSQL, queryId)
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

async function onResultPageChange(page: PageState) {
  const sql = availableSQL.value
  if (!sql) return
  await executeSelectedSQL(sql, page)
}

async function onResultRefresh() {
  const tab = activeTab.value
  if (!tab || tab.querying) return
  const sql = availableSQL.value
  if (!sql) return
  await executeSelectedSQL(sql, tab.page, true)
}

async function onResultRerun(sql: string) {
  const tab = activeTab.value
  if (!tab || tab.querying) return
  if (!sql.trim()) return
  tab.page = null
  await executeSelectedSQL(sql, null, true)
}

function onResultEditSql(sql: string) {
  const ed = getEditor()
  if (!ed) return
  ed.setValue(sql)
  syncTabSQL()
  ed.focus()
}

async function onResultApplyEdits(statements: string[]) {
  const tab = activeTab.value
  if (!tab || !statements.length) return
  if (!store.activeConnectionId) {
    tab.error = t('editor.noConnection')
    return
  }
  tab.querying = true
  applyResult.value = null
  let failed = ''
  let ok = 0
  let failedIndexes: number[] = []
  let skippedIndexes: number[] = []
  try {
    const r: ExecBatchResult = await api.ExecuteBatch(store.activeConnectionId, statements)
    ok = r.ok
    const all = r.statements ?? []
    for (const st of all) {
      if (st.error) failedIndexes.push(st.index)
    }
    const lastFailed = failedIndexes.length ? Math.max(...failedIndexes) : -1
    for (let i = 0; i < statements.length; i++) {
      if (failedIndexes.includes(i)) continue
      const st = all[i]
      if (!st) skippedIndexes.push(i)
      else if (!st.error && lastFailed >= 0 && i > lastFailed) skippedIndexes.push(i)
    }
    const first = all.find((st) => st.error)
    if (first) failed = first.error || t('editor.queryFailed')
  } catch (e: any) {
    failed = e?.message || t('editor.queryFailed')
    skippedIndexes = statements.map((_, i) => i)
  } finally {
    tab.querying = false
  }
  if (failed) {
    applyResult.value = { ok, error: failed, failedIndexes, skippedIndexes }
    tab.error = t('editor.applyEditsPartial', {
      ok,
      failed: failedIndexes.length || 1,
      error: failed,
      remaining: skippedIndexes.length,
    })
    return
  }
  applyResult.value = { ok, error: '' }
  const sql = availableSQL.value
  if (!sql) return
  await executeSelectedSQL(sql, tab.page, true)
}

async function cancelQuery() {
  const tab = activeTab.value
  const queryId = tab.queryId
  if (!queryId) return
  try {
    await api.CancelQuery(queryId)
  } catch {
  }
}

function toggleHistory() {
  showHistory.value = !showHistory.value
  if (showHistory.value) showSavedSQL.value = false
}

function toggleSavedSQL() {
  showSavedSQL.value = !showSavedSQL.value
  if (showSavedSQL.value) showHistory.value = false
}

function saveCurrentSQL() {
  const ed = getEditor()
  const sql = (ed ? ed.getValue() : activeTab.value?.sql ?? '').trim()
  if (!sql) {
    onSavedSQLError(t('editor.savedEmptySQL'))
    return
  }
  saveDialogSQL.value = sql
  saveDialogName.value = sql.split('\n').find((l) => l.trim())?.trim().slice(0, 80) || ''
  saveDialogError.value = null
  saveDialogVisible.value = true
}

function closeSaveDialog() {
  saveDialogVisible.value = false
  saveDialogError.value = null
}

async function confirmSaveSQL() {
  const name = saveDialogName.value.trim()
  const sql = saveDialogSQL.value
  if (!sql) {
    saveDialogError.value = t('editor.savedEmptySQL')
    return
  }
  try {
    await api.SaveSQL(name, sql, store.activeConnection?.id ?? '')
    saveDialogVisible.value = false
    showSavedSQL.value = true
    showHistory.value = false
    await savedSQLPanelRef.value?.reload()
  } catch (e: any) {
    const msg = String(e?.message || e)
    saveDialogError.value = msg.includes('already exists') ? t('editor.savedDuplicate') : msg
  }
}

function onSavedSQLError(message: string) {
  onResultError(message)
}

function applySavedSQL(sql: string) {
  const ed = getEditor()
  if (!ed) return
  ed.setValue(sql)
  syncTabSQL()
  showSavedSQL.value = false
}

function applyHistory(sql: string) {
  const ed = getEditor()
  if (!ed) return
  ed.setValue(sql)
  syncTabSQL()
  showHistory.value = false
}

// 启动时从 tabs.json 恢复上次打开的标签与活动标签。
// 首次启动（无快照）时保留默认标签，什么都不做。
async function restorePersistedTabs() {
  try {
    const snapshot = await api.GetTabs()
    if (!snapshot || !snapshot.tabs || snapshot.tabs.length === 0) return
    const restored = restoreTabs(snapshot)
    tabs.value = restored.tabs
    activeTabId.value = restored.activeId
    const active = restored.tabs.find((t) => t.id === restored.activeId)!
    if (getEditor()) restoreTab(active)
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
  const ed = getEditor()
  const tab = activeTab.value
  if (!tab || !ed) return
  tab.sql = ed.getValue()
  tab.viewState = ed.saveViewState()
}

function restoreTab(tab: QueryTab) {
  const ed = getEditor()
  if (!ed) return
  ed.setValue(tab.sql)
  if (tab.viewState) ed.restoreViewState(tab.viewState as Monaco.editor.ICodeEditorViewState)
  ed.focus()
  nextTick(() => {
    resultGridRef.value?.restoreScroll(tab.resultScrollTop || 0)
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
    tab.page = null
    tab.lastExecutedSql = ''
    getEditor()?.setValue(tab.sql)
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
  const ed = getEditor()
  const tab = activeTab.value
  if (!tab || !ed) return
  tab.sql = ed.getValue()
}

function insertTable(db: string, table: string) {
  selectedDB.value = db
  selectedTable.value = table
  const ed = getEditor()
  if (!ed) return
  const snippet = `SELECT * FROM ${db}.${table} LIMIT 10`
  ed.setValue(snippet)
  syncTabSQL()
  runQuery()
}

function insertClusterUUID(uuid: string) {
  const ed = getEditor()
  if (!ed || !uuid) return
  const model = ed.getModel()
  const sel = ed.getSelection()
  if (model && sel) {
    model.pushEditOperations([], [
      { range: sel, text: uuid },
    ], () => null)
    ed.focus()
  }
}

onMounted(async () => {
  window.addEventListener('click', closeAllMenus)
  window.addEventListener('contextmenu', closeAllMenus)
  window.addEventListener('mousemove', onSplitMove)
  window.addEventListener('mouseup', onSplitEnd)
  loadSplitRatio()

  await initEditor('-- ClickHouse SQL\nSELECT version()')

  const ed = getEditor()
  if (ed) {
    tabs.value[0].sql = ed.getValue()
  }

  await restorePersistedTabs()

  watch(
    () => store.activeConnectionId,
    async (id) => {
      invalidateCompletion()
      if (id) {
        await nextTick()
        await databaseTreeRef.value?.reload()
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
  window.removeEventListener('mousemove', onSplitMove)
  window.removeEventListener('mouseup', onSplitEnd)
})
</script>

<template>
  <div class="h-full w-full flex flex-col">
    <div class="flex items-center gap-2 px-3 py-2 border-b border-ch-border h-11 bg-ch-panel shrink-0">
      <div v-if="store.activeConnection" class="flex items-center gap-2 text-sm min-w-0 shrink">
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
        <span class="text-ch-text font-medium truncate">{{ store.activeConnection.name }}</span>
        <span class="text-ch-muted text-xs shrink-0">{{ store.activeConnection.host }}:{{ store.activeConnection.port }}</span>
        <span v-if="store.activeConnection.database" class="text-ch-muted text-xs bg-ch-hover px-1.5 py-0.5 rounded shrink-0">{{ store.activeConnection.database }}</span>
      </div>
      <div v-else class="text-ch-muted text-sm">{{ t('editor.selectConnection') }}</div>

      <div class="flex-1 min-w-0" />

      <div class="flex items-center gap-2 shrink-0">
      <button
        v-if="store.activeConnection"
        @click="onTreeRefresh"
        class="btn-ghost btn-sm flex items-center gap-1 shrink-0"
        :title="t('editor.refresh')"
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
      </button>
      <button
        v-if="store.activeConnection"
        @click="toggleHistory"
        class="btn-ghost btn-sm flex items-center gap-1 shrink-0"
        :class="showHistory ? 'text-ch-accent' : ''"
        :title="t('editor.history')"
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="9"/><polyline points="12 7 12 12 15 15"/></svg>
      </button>
      <button
        v-if="store.activeConnection"
        @click="toggleSavedSQL"
        class="btn-ghost btn-sm flex items-center gap-1 shrink-0"
        :class="showSavedSQL ? 'text-ch-accent' : ''"
        :title="t('editor.savedSQL')"
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2z"/></svg>
      </button>
      <button
        v-if="store.activeConnection"
        @click="saveCurrentSQL"
        class="btn-ghost btn-sm flex items-center gap-1 shrink-0"
        :title="t('editor.saveCurrentSQL')"
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 5v14"/><path d="M5 12h14"/></svg>
      </button>
      <button
        v-if="store.activeConnection"
        @click="querying ? cancelQuery() : runQuery()"
        class="btn-primary btn-sm flex items-center gap-1 shrink-0 whitespace-nowrap"
        :class="querying ? 'btn-danger' : ''"
      >
        <svg v-if="!querying" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>
        <svg v-else width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="5" y="5" width="14" height="14" rx="2"/></svg>
        {{ querying ? t('editor.cancel') : t('editor.run') }}
      </button>
      </div>
    </div>

    <div class="flex items-center gap-1 px-2 border-b border-ch-border bg-ch-panel shrink-0 h-9 overflow-x-auto overflow-y-hidden">
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
        class="flex items-center gap-1 h-7 px-2 rounded shrink-0 border border-ch-border bg-ch-hover/50 text-ch-text hover:bg-ch-accent/15 hover:border-ch-accent hover:text-ch-accent transition-colors"
        :title="t('editor.newTab')"
        @click="addTab"
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
        <span class="text-xs whitespace-nowrap">{{ t('editor.newTabShort') }}</span>
      </button>
    </div>

    <div class="flex-1 flex min-h-0">
      <DatabaseTree
        v-if="store.activeConnection"
        ref="databaseTreeRef"
        :active="store.activeConnection !== null"
        :selected-db="selectedDB"
        :selected-table="selectedTable"
        @preview="onTreePreview"
        @select-db="onTreeSelectDB"
        @append-uuid="insertClusterUUID"
        @insert-ddl="onTreeInsertDDL"
        @error="onTreeError"
      />
      <div ref="splitContainerEl" class="flex-1 flex flex-col min-w-0 min-h-0">
        <div
          class="min-h-[120px] shrink-0"
          :style="{ height: (splitRatio * 100).toFixed(3) + '%' }"
        >
          <div ref="editorRef" class="w-full h-full"></div>
        </div>

        <div
          class="h-1.5 shrink-0 cursor-row-resize group relative select-none"
          @mousedown="onSplitStart"
        >
          <div
            class="absolute inset-x-0 -top-0.5 -bottom-0.5 transition-colors"
            :class="splitDragging ? 'bg-ch-accent/40' : 'bg-transparent group-hover:bg-ch-accent/30'"
          ></div>
          <div class="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 h-1 w-10 rounded-full bg-ch-muted/30 group-hover:bg-ch-accent/60 transition-colors"></div>
        </div>

        <HistoryPanel
          :visible="showHistory"
          @select="applyHistory"
          @close="showHistory = false"
        />

        <SavedSQLPanel
          ref="savedSQLPanelRef"
          :visible="showSavedSQL"
          :connection-id="store.activeConnection?.id"
          @select="applySavedSQL"
          @error="onSavedSQLError"
          @close="showSavedSQL = false"
        />

        <ResultGrid
          ref="resultGridRef"
          class="flex-1"
          :result="result"
          :error="error"
          :canceled="canceled"
          :querying="querying"
          :available-sql="availableSQL"
          :executed-sql="activeTab?.lastExecutedSql ?? ''"
          :insert-db="selectedDB"
          :insert-table="selectedTable"
          :scroll-top="activeTab?.resultScrollTop || 0"
          :apply-result="applyResult"
          :page="activeTab?.page ?? null"
          @explain="executeSelectedSQL"
          @error="onResultError"
          @scroll-top="onResultScrollTop"
          @page="onResultPageChange"
          @refresh="onResultRefresh"
          @rerun="onResultRerun"
          @edit-sql="onResultEditSql"
          @apply-edits="onResultApplyEdits"
        />
      </div>
    </div>

    <Teleport to="body">
      <div
        v-if="saveDialogVisible"
        class="fixed inset-0 z-[2000] bg-black/50 flex items-center justify-center"
        @click.self="closeSaveDialog"
      >
        <div class="bg-ch-panel border border-ch-border rounded-lg shadow-2xl w-[520px] max-w-[92vw] overflow-hidden">
          <div class="flex items-center justify-between px-4 py-3 border-b border-ch-border">
            <span class="text-ch-text text-sm font-medium">{{ t('editor.saveSQLTitle') }}</span>
            <button class="text-ch-muted hover:text-ch-text" :title="t('common.close')" @click="closeSaveDialog">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            </button>
          </div>
          <div class="px-4 py-3 space-y-2">
            <label class="block text-xs text-ch-muted">{{ t('editor.saveSQLName') }}</label>
            <input
              v-model="saveDialogName"
              type="text"
              class="w-full bg-ch-bg border border-ch-border rounded px-2 py-1.5 text-sm text-ch-text outline-none focus:border-ch-accent"
              :placeholder="t('editor.saveSQLNamePlaceholder')"
              @keydown.enter.prevent="confirmSaveSQL"
            />
            <pre class="max-h-48 overflow-auto bg-ch-bg border border-ch-border rounded p-2 text-[11px] text-ch-muted font-mono whitespace-pre-wrap">{{ saveDialogSQL }}</pre>
            <p v-if="saveDialogError" class="text-[11px] text-red-400">{{ saveDialogError }}</p>
          </div>
          <div class="flex items-center justify-end gap-2 px-4 py-3 border-t border-ch-border">
            <button
              class="px-3 py-1 rounded border border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover text-xs"
              @click="closeSaveDialog"
            >
              {{ t('common.cancel') }}
            </button>
            <button class="px-3 py-1 rounded bg-ch-accent text-white text-xs" @click="confirmSaveSQL">
              {{ t('editor.saveSQLConfirm') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>