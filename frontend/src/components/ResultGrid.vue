<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useVirtualizer } from '@tanstack/vue-virtual'
import type { QueryResult, ResultExportFormat } from '../types'
import { api } from '../api'
import ResultFilterPanel from './ResultFilterPanel.vue'
import ResultToolbarInfo from './ResultToolbarInfo.vue'
import ResultToolbarActions from './ResultToolbarActions.vue'
import ResultTableBody from './ResultTableBody.vue'
import ResultCellMenu from './ResultCellMenu.vue'
import ResultHeaderMenu from './ResultHeaderMenu.vue'
import ResultSqlPanel from './ResultSqlPanel.vue'
import ResultCellFilterPanel from './ResultCellFilterPanel.vue'
import ResultAutoRefreshPicker from './ResultAutoRefreshPicker.vue'
import ResultApplyConfirmModal from './ResultApplyConfirmModal.vue'
import ResultExportBar from './ResultExportBar.vue'
import ResultCellEditBar from './ResultCellEditBar.vue'
import {
  type FilterOp,
  FILTER_OP_ORDER,
  isConditionable,
  appendSort,
  appendWhere,
} from '../utils/resultFilter'
import {
  buildInsertBatch,
  buildInsertStatement,
  insertTarget,
  rowToTSV,
  sqlLiteral,
} from '../utils/sqlLiteral'
import {
  columnValuesText,
  distinctColumnValues,
  filterRowIndices,
} from '../utils/resultView'
import {
  buildGridStyle,
  visibleColumnIndexes,
  hiddenColumnIndexes,
  gridWidthsFor,
  pickCellIndexes,
  resizeColWidths,
  measureColWidths,
  ROW_HEIGHT,
  VIRTUAL_THRESHOLD,
} from '../utils/resultTable'
import {
  type PageState,
  buildPageSQL,
  currentPageNumber,
  DEFAULT_PAGE_SIZE,
  hasNextPage,
  hasPrevPage,
  nextPage,
  prevPage,
  resetPage,
} from '../utils/pagination'
import { buildExplainSQL, canExplain, EXPLAIN_MODES, type ExplainMode } from '../utils/explain'
import { useAutoRefresh } from '../composables/useAutoRefresh'
import { useResultFilterPanel } from '../composables/useResultFilterPanel'
import { useCellEditing, type ApplyResult } from '../composables/useCellEditing'
import { parseSqlError } from '../utils/sqlError'
import { isDisconnectedError } from '../utils/connectionError'
import { rewriteProjectionToColumns, stripPageTail } from '../utils/sqlProjection'
import { MENU_MARGIN, clampMenuPosition as clampMenuPositionAt } from '../utils/menuPosition'

const props = defineProps<{
  result: QueryResult | null
  error: string | null
  canceled: boolean
  querying: boolean
  availableSql: string
  executedSql?: string
  insertDb?: string | null
  insertTable?: string | null
  scrollTop?: number
  page?: PageState | null
  applyResult?: ApplyResult | null
}>()

const emit = defineEmits<{
  (e: 'explain', sql: string): void
  (e: 'error', message: string): void
  (e: 'scroll-top', value: number): void
  (e: 'page', page: PageState): void
  (e: 'refresh'): void
  (e: 'rerun', sql: string): void
  (e: 'edit-sql', sql: string): void
  (e: 'apply-edits', statements: string[]): void
}>()

const { t } = useI18n()

const CELL_MENU_WIDTH = 220
const HEADER_MENU_WIDTH = 240
const FILTER_PANEL_WIDTH = 380

const tableBody = ref<{ headerScrollEl: HTMLElement | null; scrollEl: HTMLElement | null } | null>(null)
const scrollEl = computed(() => tableBody.value?.scrollEl ?? null)
const headerScrollEl = computed(() => tableBody.value?.headerScrollEl ?? null)
let syncingScroll = false

function syncScroll(from: HTMLElement | null, to: HTMLElement | null) {
  if (syncingScroll || !from || !to) return
  syncingScroll = true
  to.scrollLeft = from.scrollLeft
  requestAnimationFrame(() => {
    syncingScroll = false
  })
}

function onBodyScroll() {
  syncScroll(scrollEl.value, headerScrollEl.value)
  if (scrollEl.value) emit('scroll-top', scrollEl.value.scrollTop)
}

function onHeaderScroll() {
  syncScroll(headerScrollEl.value, scrollEl.value)
}

const searchText = ref('')
const searchColumn = ref(-1)
const searchActive = computed(() => searchText.value.trim() !== '')

const colWidths = ref<number[]>([])
const hiddenColumns = ref<number[]>([])

function onResizeColumns(ci: number, width: number) {
  colWidths.value = resizeColWidths(colWidths.value, [ci], width, 0)
}

const columnIndexes = computed(() => {
  const total = props.result?.columns?.length ?? 0
  return visibleColumnIndexes(total, hiddenColumns.value)
})

const hiddenColumnsResolved = computed(() => {
  const total = props.result?.columns?.length ?? 0
  return hiddenColumnIndexes(total, hiddenColumns.value)
})

const gridStyle = computed(() => {
  if (!props.result?.columns) return ''
  return buildGridStyle(gridWidthsFor(columnIndexes.value, colWidths.value), columnIndexes.value.length)
})
const rowIndices = computed(() => {
  const rows = props.result?.rows ?? []
  return filterRowIndices(rows, searchText.value, searchColumn.value)
})

const virtualEnabled = computed(() => rowIndices.value.length > VIRTUAL_THRESHOLD)

const virtualizer = useVirtualizer({
  count: rowIndices.value.length,
  getScrollElement: () => scrollEl.value,
  estimateSize: () => ROW_HEIGHT,
  overscan: 10,
})

function vInstance(): { getVirtualItems: () => Array<{ index: number; start: number }>; getTotalSize: () => number; measure: () => void } {
  return virtualizer.value as unknown as {
    getVirtualItems: () => Array<{ index: number; start: number }>
    getTotalSize: () => number
    measure: () => void
  }
}

const virtualRows = computed(() => (virtualEnabled.value ? vInstance().getVirtualItems() : []))
const virtualHeight = computed(() => (virtualEnabled.value ? vInstance().getTotalSize() : 0))

function visibleRows(): Array<{ ri: number; row: unknown[] }> {
  const rows = props.result?.rows ?? []
  if (virtualEnabled.value) {
    return virtualRows.value.map((v) => {
      const ri = rowIndices.value[v.index]
      return { ri, row: rows[ri] }
    })
  }
  return rowIndices.value.map((ri) => ({ ri, row: rows[ri] }))
}

function virtualRowTop(ri: number): number {
  const found = virtualRows.value.find((v) => rowIndices.value[v.index] === ri)
  return found ? found.start : 0
}

watch(
  () => [props.result?.rows?.length, props.result?.columns?.length],
  () => {
    const cols = props.result?.columns
    hiddenColumns.value = []
    colWidths.value = cols && cols.length ? measureColWidths(cols, props.result?.rows ?? []) : []
    nextTick(() => {
      vInstance().measure()
      if (scrollEl.value) scrollEl.value.scrollTop = 0
    })
  },
  { immediate: true },
)

watch(
  () => props.result,
  () => {
    cellMenu.value.visible = false
  },
)

watch(
  () => [props.result, props.scrollTop],
  () => {
    nextTick(() => {
      if (scrollEl.value) scrollEl.value.scrollTop = props.scrollTop || 0
    })
  },
)

const parsedError = computed(() => (props.error ? parseSqlError(props.error) : null))
const errorIsDisconnected = computed(() => isDisconnectedError(props.error))
const canExplainCurrent = computed(() => canExplain(props.availableSql))
const pageState = computed(() => props.page ?? resetPage())
const pageNumber = computed(() => currentPageNumber(pageState.value))
const canPrev = computed(() => hasPrevPage(pageState.value) && !props.querying)
const canNext = computed(() => hasNextPage(pageState.value, props.result?.rows?.length ?? 0) && !props.querying)

function goPrev() {
  if (canPrev.value) emit('page', prevPage(pageState.value))
}
function goNext() {
  if (canNext.value) emit('page', nextPage(pageState.value))
}

const explainMode = ref<ExplainMode>('PLAN')
const explainMenuVisible = ref(false)
const headerMenu = ref<{ visible: boolean; x: number; y: number; colIndex: number }>({
  visible: false,
  x: 0,
  y: 0,
  colIndex: 0,
})
const cellMenu = ref<{ visible: boolean; x: number; y: number; colIndex: number; rowIndex: number }>({
  visible: false,
  x: 0,
  y: 0,
  colIndex: 0,
  rowIndex: 0,
})
const copiedHint = ref<string | null>(null)
let copiedTimer: number | undefined

function clampMenuPosition(x: number, y: number, width: number, height: number) {
  return clampMenuPositionAt(x, y, width, height, { width: window.innerWidth, height: window.innerHeight })
}

function openCellMenu(e: MouseEvent, rowIndex: number, colIndex: number) {
  e.stopPropagation()
  closeAllMenus()
  const pos = clampMenuPosition(e.clientX, e.clientY, CELL_MENU_WIDTH, 320)
  cellMenu.value = { visible: true, x: pos.x, y: pos.y, rowIndex, colIndex }
  headerMenu.value = { visible: false, x: 0, y: 0, colIndex }
}

function sortBy(dir: 'ASC' | 'DESC') {
  const col = props.result?.columns?.[cellMenu.value.colIndex]?.name
  if (!col) return
  closeCellMenu()
  const sql = appendSort(props.availableSql, [{ column: col, dir }])
  if (sql) emit('rerun', sql)
}

function filterByValueMerged() {
  const res = props.result
  const col = res?.columns?.[cellMenu.value.colIndex]?.name
  if (!res || !col) return
  const rowIndex = cellMenu.value.rowIndex
  const colIndex = cellMenu.value.colIndex
  const x = cellMenu.value.x
  const y = cellMenu.value.y
  closeCellMenu()
  if (!canExplainCurrent.value) return
  const value = cellValueAt(rowIndex, colIndex)
  const panelPos = clampMenuPosition(x, y, FILTER_PANEL_WIDTH, 420)
  filterPanel.value = { visible: true, x: panelPos.x, y: panelPos.y, colIndex, rowIndex }
  filterInput.value = String(value ?? '')
  filterOp.value = FILTER_OP_ORDER[0]
  filterError.value = null
}

function openHeaderMenu(e: MouseEvent, colIndex: number) {
  e.stopPropagation()
  closeAllMenus()
  const pos = clampMenuPosition(e.clientX, e.clientY, HEADER_MENU_WIDTH, 360)
  headerMenu.value = { visible: true, x: pos.x, y: pos.y, colIndex }
}

function openHeaderValueFilter(colIndex: number) {
  if (!props.result) return
  const x = cellMenu.value.x
  const y = cellMenu.value.y
  closeAllMenus()
  if (!isConditionable(props.availableSql)) return
  cellMenu.value = { visible: false, x: 0, y: 0, colIndex, rowIndex: -1 }
  const pos = clampMenuPosition(x, y, FILTER_PANEL_WIDTH, 520)
  filterPanel.value = { visible: true, x: pos.x, y: pos.y, colIndex, rowIndex: -1 }
  filterInput.value = ''
  filterOp.value = FILTER_OP_ORDER[0]
  filterError.value = null
  filterCandidateFilter.value = ''
}

function closeHeaderMenu() {
  headerMenu.value.visible = false
}

function hideColumn(ci: number) {
  const total = props.result?.columns?.length ?? 0
  const resolved = hiddenColumnIndexes(total, hiddenColumns.value)
  if (resolved.length >= total - 1) {
    closeHeaderMenu()
    return
  }
  closeHeaderMenu()
  hiddenColumns.value = [...resolved, ci].sort((a, b) => a - b)
}

function showAllColumns() {
  closeHeaderMenu()
  hiddenColumns.value = []
}

function hideColumnBySql(ci: number) {
  const name = props.result?.columns?.[ci]?.name ?? ''
  const allNames = (props.result?.columns ?? []).map((c) => c.name ?? '')
  closeHeaderMenu()
  if (!name) {
    emit('error', t('editor.hideColumnSqlNotFound'))
    return
  }
  let sawUnsupported = false
  for (const source of [props.availableSql, props.executedSql ?? '']) {
    if (!source.trim()) continue
    const base = stripPageTail(source)
    const res = rewriteProjectionToColumns(base, [name], allNames)
    if (res.ok) {
      emit('rerun', res.sql)
      return
    }
    if (res.reason === 'unsupported') sawUnsupported = true
  }
  emit('error', t(sawUnsupported ? 'editor.hideColumnSqlUnsupported' : 'editor.hideColumnSqlNotFound'))
}

function cellValueAt(rowIndex: number, colIndex: number): unknown {
  const r = props.result
  if (!r || rowIndex < 0) return null
  return r.rows?.[rowIndex]?.[colIndex] ?? null
}

function currentTarget(): string {
  return insertTarget(props.insertDb ?? null, props.insertTable ?? null)
}

async function notifyCopied(message: string) {
  copiedHint.value = message
  if (copiedTimer) clearTimeout(copiedTimer)
  copiedTimer = window.setTimeout(() => {
    copiedHint.value = null
  }, 1500)
}

async function copyText(text: string, message: string) {
  try {
    await navigator.clipboard.writeText(text)
    await notifyCopied(message)
  } catch {
    await notifyCopied(t('editor.copyFailed'))
  }
}

function currentRow(): unknown[] | null {
  const res = props.result
  if (!res) return null
  return res.rows?.[cellMenu.value.rowIndex] ?? null
}

async function copyCell() {
  const row = currentRow()
  if (!row) return
  const value = row[cellMenu.value.colIndex]
  const text = value == null ? '' : String(value)
  closeCellMenu()
  await copyText(text, t('editor.copyCell'))
}

async function copyRow() {
  const row = currentRow()
  if (row) {
    closeCellMenu()
    await copyText(rowToTSV(pickCellIndexes(row, columnIndexes.value)), t('editor.copyRow'))
  }
}

async function copyInsert() {
  const row = currentRow()
  const res = props.result
  if (!row || !res?.columns) return
  closeCellMenu()
  const names = columnIndexes.value.map((ci) => res.columns![ci]?.name ?? '')
  const values = pickCellIndexes(row, columnIndexes.value)
  await copyText(buildInsertStatement(currentTarget(), names, values), t('editor.copyInsert'))
}

async function copyInsertAll() {
  const res = props.result
  if (!res || !res.rows || res.rows.length === 0) return
  closeCellMenu()
  const visible = columnIndexes.value
  const cols = res.columns ?? []
  const names = visible.map((ci) => cols[ci]?.name ?? '')
  const rows = visible.length === cols.length
    ? res.rows
    : res.rows.map((r) => pickCellIndexes(r, visible))
  await copyText(buildInsertBatch(currentTarget(), names, rows), t('editor.copyInsertAll'))
}

async function copyCellLiteral() {
  const row = currentRow()
  if (row) {
    closeCellMenu()
    await copyText(sqlLiteral(row[cellMenu.value.colIndex]), t('editor.copyCellLiteral'))
  }
}

function headerColumnName(): string {
  return props.result?.columns?.[headerMenu.value.colIndex]?.name ?? ''
}

async function copyHeaderColumnName() {
  const name = headerColumnName()
  if (name) {
    closeHeaderMenu()
    await copyText(name, t('editor.copyColumnName'))
  }
}

async function copyHeaderAllColumns() {
  if (props.result?.columns) {
    const visible = columnIndexes.value
    const cols = props.result.columns
    const names = visible.map((ci) => cols[ci]?.name ?? '')
    closeHeaderMenu()
    await copyText(names.join(', '), t('editor.copyColumns'))
  }
}

function searchOnlyThisColumn() {
  if (props.result) {
    searchColumn.value = headerMenu.value.colIndex
    closeHeaderMenu()
  }
}

function sortByHeader(dir: 'ASC' | 'DESC') {
  const col = headerColumnName()
  if (!col) return
  closeHeaderMenu()
  const sql = appendSort(props.availableSql, [{ column: col, dir }])
  if (sql) emit('rerun', sql)
}

async function copyColumnValues(distinct: boolean) {
  const res = props.result
  if (!res) return
  const indices = rowIndices.value
  closeHeaderMenu()
  const rows = res.rows ?? []
  const text = distinct
    ? distinctColumnValues(rows, indices, headerMenu.value.colIndex).join('\n')
    : columnValuesText(rows, indices, headerMenu.value.colIndex)
  await copyText(text, t(distinct ? 'editor.copyColumnValuesDistinct' : 'editor.copyColumnValues'))
}

const filterPanel = ref<{ visible: boolean; x: number; y: number; colIndex: number; rowIndex: number }>({
  visible: false,
  x: 0,
  y: 0,
  colIndex: 0,
  rowIndex: -1,
})
const filterInput = ref('')
const filterOp = ref<FilterOp>(FILTER_OP_ORDER[0])
const filterError = ref<string | null>(null)
const filterCandidateFilter = ref('')
const filterPanelHeight = computed(() => {
  const available = window.innerHeight - filterPanel.value.y - MENU_MARGIN
  return Math.max(200, Math.min(available, 560)) + 'px'
})
const truncatedHintVisible = computed(() => {
  const r = props.result
  if (!r || !r.row_limit) return false
  return (r.row_count ?? 0) >= r.row_limit
})
function parseStyle(style: string): Record<string, string> {
  if (!style) return {}
  return style.split(';').reduce<Record<string, string>>((acc, part) => {
    const [k, v] = part.split(':')
    if (k && v) acc[k.trim()] = v.trim()
    return acc
  }, {})
}

const filterEnabled = computed(() => isConditionable(props.availableSql))
const hasColumns = computed(() => !!props.result?.columns && props.result.columns.length > 0)
const isEmptyResult = computed(() => !!props.result && props.result.row_count === 0)
const loadingNoResult = computed(() => props.querying && !props.result)
const loadingWithResult = computed(() => props.querying && !!props.result)
const searchNoMatch = computed(
  () => searchActive.value && !props.querying && hasColumns.value && !isEmptyResult.value && rowIndices.value.length === 0,
)

const sqlPanelVisible = ref(false)
const toolbarInfo = ref<{ sqlButtonWrap: HTMLElement | null } | null>(null)
const sqlPanelPos = ref({ x: 0, y: 0 })
const executedSqlModified = computed(() => {
  const executed = (props.executedSql ?? '').trim()
  const source = (props.availableSql ?? '').trim()
  return executed !== '' && executed !== source
})

function toggleSqlPanel() {
  if (!sqlPanelVisible.value) {
    const wrap = toolbarInfo.value?.sqlButtonWrap ?? null
    if (wrap) {
      const rect = wrap.getBoundingClientRect()
      const width = Math.min(520, window.innerWidth * 0.8)
      sqlPanelPos.value = {
        x: Math.max(MENU_MARGIN, Math.min(rect.left, window.innerWidth - width - MENU_MARGIN)),
        y: Math.max(MENU_MARGIN, rect.bottom + 4),
      }
    }
  }
  sqlPanelVisible.value = !sqlPanelVisible.value
}

function closeSqlPanel() {
  sqlPanelVisible.value = false
}

const {
  editingCell,
  editingValue,
  pendingEdits,
  applyConfirmVisible,
  userKeyIndex,
  editSource,
  keyIndexes,
  editKeyHint,
  canEditResult,
  pendingCount,
  applyStatements,
  applyTargetLabel,
  startEditCell,
  cancelEditCell,
  setKeyColumn,
  isPendingEdited,
  pendingValue,
  commitEditCell,
  applyPendingEdits,
  cancelApplyEdits,
  confirmApplyEdits,
  copyApplyStatements: copyApplyStatementsRaw,
  discardPendingEdits,
  fillPendingEditsToEditor,
} = useCellEditing({
  result: () => props.result,
  availableSql: () => props.availableSql,
  executedSql: () => props.executedSql,
  applyResult: () => props.applyResult ?? null,
  onError: (message) => emit('error', message),
  onApplyEdits: (statements) => emit('apply-edits', statements),
  onEditSql: (sql) => emit('edit-sql', sql),
})

const keyColumnIndex = computed(() => (keyIndexes.value.length === 1 ? keyIndexes.value[0] : -1))

function copyApplyStatements() {
  return copyApplyStatementsRaw(copyText)
}

function fillApplyStatementsToEditor() {
  applyConfirmVisible.value = false
  fillPendingEditsToEditor()
}

async function copyExecutedSql() {
  const sql = props.executedSql ?? ''
  if (!sql) return
  await copyText(sql, t('editor.copySql'))
}

const explainModes = EXPLAIN_MODES
const {
  filterOpen,
  filterConditions,
  filterBuildError,
  filterHasActive,
  activeFilterCount,
  filterCandidates,
  openFilterPanel,
  closeFilterPanel,
  clearFilters,
  applyFilters,
  filterModeOptions,
  currentFilterColumnName,
  applyFilterPanel,
  applyCandidate,
} = useResultFilterPanel({
  result: () => props.result,
  rowIndices: () => rowIndices.value,
  availableSql: () => props.availableSql,
  filterPanel: () => filterPanel.value,
  filterOp: () => filterOp.value,
  candidateFilter: () => filterCandidateFilter.value,
  cellValueAt,
  onRerun: (sql) => emit('rerun', sql),
  onClosePanel: () => {
    filterPanel.value.visible = false
  },
})

const filterRowValue = computed(() => cellValueAt(filterPanel.value.rowIndex, filterPanel.value.colIndex))

function closeCellMenu() {
  cellMenu.value.visible = false
}

function closeAllMenus() {
  closeHeaderMenu()
  closeCellMenu()
  closeFilterPanel()
  explainMenuVisible.value = false
}

function onGlobalKeydown(e: KeyboardEvent) {
  if (e.key !== 'Escape') return
  if (applyConfirmVisible.value) {
    cancelApplyEdits()
    return
  }
  if (filterOpen.value) {
    closeFilterPanel()
    return
  }
  closeAllMenus()
}

window.addEventListener('click', closeAllMenus)
window.addEventListener('keydown', onGlobalKeydown)

const {
  autoRefreshSeconds,
  autoRefreshMenuVisible,
  autoRefreshCustomValue,
  autoRefreshCustomError,
  autoRefreshRemaining,
  pickAutoRefreshOpen,
  autoRefreshMenuX,
  autoRefreshMenuY,
  refreshEnabled,
  refreshOptions,
  toggleAutoRefreshMenu,
  chooseAutoRefresh,
  openAutoRefreshCustom,
  closeAutoRefreshCustom,
  manualRefresh,
} = useAutoRefresh({
  querying: () => props.querying,
  onRefresh: () => emit('refresh'),
})

watch(
  () => props.result?.columns?.length ?? 0,
  (len) => {
    if (searchColumn.value >= len) searchColumn.value = -1
  },
)

onBeforeUnmount(() => {
  window.removeEventListener('click', closeAllMenus)
  window.removeEventListener('keydown', onGlobalKeydown)
  if (copiedTimer) clearTimeout(copiedTimer)
})

function explainWithCurrentMode() {
  if (!canExplainCurrent.value) return
  const sql = buildExplainSQL(props.availableSql, explainMode.value)
  if (sql) emit('explain', sql)
}

function clearSearch() {
  searchText.value = ''
  searchColumn.value = -1
}

const exportMenuOpen = ref(false)
const exporting = ref(false)
const exportedPath = ref<string | null>(null)
const exportFormats = ref<ResultExportFormat[]>([])

async function loadExportFormats() {
  if (exportFormats.value.length > 0) return
  try {
    exportFormats.value = await api.GetResultExportFormats()
  } catch {
    exportFormats.value = []
  }
}

async function doExport(format: ResultExportFormat) {
  const res = props.result
  if (!res || exporting.value) return
  const indices = rowIndices.value
  const allRows = res.rows ?? []
  if (indices.some((ri) => ri < 0 || ri >= allRows.length)) {
    emit('error', t('editor.exportRowsMismatch'))
    return
  }
  exporting.value = true
  exportedPath.value = null
  try {
    const rows = indices.map((ri) => allRows[ri] ?? [])
    const visible = columnIndexes.value
    const exportCols = visible.map((ci) => (res.columns ?? [])[ci]?.name ?? '')
    const exportRows = visible.length === (res.columns ?? []).length
      ? rows
      : rows.map((r) => visible.map((ci) => r[ci]))
    const path = await api.SaveQueryResultAs(format.id, exportCols, exportRows, currentTarget())
    if (path) exportedPath.value = path
    exportMenuOpen.value = false
  } catch (err) {
    const msg = String((err as any)?.message || err)
    if (!msg.includes('cancelled by user')) {
      emit('error', msg)
    }
  } finally {
    exporting.value = false
  }
}

watch(exportMenuOpen, (open) => {
  if (open) loadExportFormats()
})

function restoreScroll(value: number) {
  nextTick(() => {
    if (scrollEl.value) scrollEl.value.scrollTop = value || 0
  })
}

defineExpose({
  closeMenus: closeAllMenus,
  restoreScroll,
})
</script>

<template>
  <div class="flex flex-col h-full min-h-0">
    <div class="flex items-center gap-2 px-3 py-1.5 border-b border-ch-border text-xs text-ch-muted bg-ch-panel shrink-0 overflow-x-auto overflow-y-hidden">
      <ResultToolbarInfo
        ref="toolbarInfo"
        :result="result"
        :executed-sql="executedSql ?? null"
        :executed-sql-modified="executedSqlModified"
        :loading-with-result="loadingWithResult"
        :refresh-enabled="refreshEnabled"
        :refresh-remaining="autoRefreshRemaining"
        :copied-hint="copiedHint"
        :search-text="searchText"
        :search-active="searchActive"
        :filtered-shown="rowIndices.length"
        @toggle-sql="toggleSqlPanel"
        @update:search-text="searchText = $event"
        @clear-search="clearSearch"
      />

      <div class="flex-1 min-w-0"></div>


      <ResultToolbarActions
        :querying="querying"
        :refresh-enabled="refreshEnabled"
        :refresh-seconds="autoRefreshSeconds"
        :can-explain="canExplainCurrent"
        :explain-mode="explainMode"
        :explain-modes="explainModes"
        :has-columns="hasColumns"
        :exporting="exporting"
        :filter-enabled="filterEnabled"
        :filter-has-active="filterHasActive"
        :active-filter-count="activeFilterCount"
        @manual-refresh="manualRefresh"
        @toggle-refresh-menu="toggleAutoRefreshMenu"
        @open-refresh-custom="openAutoRefreshCustom"
        @explain="explainWithCurrentMode"
        @update:explain-mode="explainMode = $event as ExplainMode"
        @toggle-export="exportMenuOpen = !exportMenuOpen"
        @open-filter="openFilterPanel"
      />
    </div>

    <ResultExportBar
      v-if="exportMenuOpen"
      :formats="exportFormats"
      :exporting="exporting"
      :exported-path="exportedPath"
      :export-rows="rowIndices.length"
      @select="doExport"
    />

    <div v-if="loadingNoResult" class="flex-1 flex items-center justify-center text-xs text-ch-muted">
      {{ t('common.loading') }}
    </div>

    <div v-else-if="parsedError" class="flex-1 overflow-auto p-4">
      <div v-if="errorIsDisconnected" class="text-sm text-amber-400 font-semibold">{{ t('editor.connectionLost') }}</div>
      <div v-else class="text-sm text-red-400 font-semibold">{{ parsedError.summary || t('editor.queryFailed') }}</div>
      <div v-if="parsedError.code !== null" class="text-xs text-ch-muted mt-1">Code: {{ parsedError.code }}</div>
      <div v-if="errorIsDisconnected" class="text-xs text-ch-muted mt-1">{{ t('editor.connectionLostHint') }}</div>
      <button
        v-if="errorIsDisconnected"
        type="button"
        class="mt-2 px-3 py-1 text-xs rounded border border-ch-border text-ch-text hover:bg-ch-hover disabled:opacity-50"
        :disabled="querying"
        @click="emit('refresh')"
      >
        {{ t('editor.reconnect') }}
      </button>
      <pre class="text-xs text-ch-text mt-2 whitespace-pre-wrap font-mono">{{ parsedError.raw }}</pre>
    </div>

    <div v-else-if="isEmptyResult" class="flex-1 flex flex-col items-center justify-center gap-2 text-xs text-ch-muted">
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="opacity-50"><rect x="3" y="3" width="18" height="18" rx="2"/><line x1="8" y1="12" x2="16" y2="12"/></svg>
      <span>{{ t('editor.emptyResult') }}</span>
    </div>

    <div v-else-if="result" class="flex-1 min-h-0 flex flex-col">
      <ResultTableBody
        ref="tableBody"
        :columns="result.columns ?? []"
        :visible-column-indexes="columnIndexes"
        :grid-style="gridStyle"
        :key-indexes="keyIndexes"
        :search-column="searchColumn"
        :search-no-match="searchNoMatch"
        :search-text="searchText"
        :virtual-enabled="virtualEnabled"
        :virtual-height="virtualHeight"
        :row-height="ROW_HEIGHT"
        :visible-rows="visibleRows()"
        :virtual-row-top="virtualRowTop"
        :parse-style="parseStyle"
        :editing-row="editingCell?.rowIndex ?? -1"
        :editing-col="editingCell?.colIndex ?? -1"
        :editing-value="editingValue"
        :is-pending-edited="isPendingEdited"
        :pending-value="pendingValue"
        :col-widths="colWidths"
        @resize-columns="onResizeColumns"
        @scroll-header="onHeaderScroll"
        @scroll-body="onBodyScroll"
        @open-header-menu="openHeaderMenu"
        @open-header-value-filter="openHeaderValueFilter"
        @open-cell-menu="openCellMenu"
        @start-edit="startEditCell"
        @commit-edit="commitEditCell"
        @cancel-edit="cancelEditCell"
        @update:editing-value="editingValue = $event"
      />
      <ResultCellEditBar
        v-if="pendingCount > 0"
        :count="pendingCount"
        :querying="querying"
        @fill="fillPendingEditsToEditor"
        @discard="discardPendingEdits"
        @apply="applyPendingEdits"
      />

      <div class="flex items-center justify-between gap-2 px-3 py-1 border-t border-ch-border bg-ch-panel text-[11px] shrink-0">
        <span class="text-ch-muted">{{ t('editor.pageNumber', { page: pageNumber }) }}</span>
        <div class="flex items-center gap-1">
          <button
            @click="goPrev"
            :disabled="!canPrev"
            class="flex items-center gap-1 px-2 py-0.5 rounded border border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-ch-muted"
          >
            <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 18 9 12 15 6"/></svg>
            <span>{{ t('editor.prevPage') }}</span>
          </button>
          <button
            @click="goNext"
            :disabled="!canNext"
            class="flex items-center gap-1 px-2 py-0.5 rounded border border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-ch-muted"
          >
            <span>{{ t('editor.nextPage') }}</span>
            <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>
          </button>
        </div>
      </div>

      <div v-if="truncatedHintVisible" class="px-3 py-1 text-[11px] text-amber-400 border-t border-ch-border bg-ch-panel shrink-0">
        {{ t('editor.truncatedHint', { limit: result.row_limit }) }}
      </div>
    </div>

    <div v-else class="flex-1 flex items-center justify-center text-xs text-ch-muted">
      {{ t('editor.queryHint') }}
    </div>

    <ResultCellMenu
      :visible="cellMenu.visible"
      :x="cellMenu.x"
      :y="cellMenu.y"
      :column-name="result?.columns?.[cellMenu.colIndex]?.name ?? ''"
      :can-explain="canExplainCurrent"
      @copy-cell="copyCell"
      @copy-cell-literal="copyCellLiteral"
      @copy-row="copyRow"
      @copy-insert="copyInsert"
      @copy-insert-all="copyInsertAll"
      @sort="sortBy"
      @filter-by-value="filterByValueMerged"
    />

    <ResultHeaderMenu
      :visible="headerMenu.visible"
      :x="headerMenu.x"
      :y="headerMenu.y"
      :column-name="result?.columns?.[headerMenu.colIndex]?.name ?? ''"
      :can-explain="canExplainCurrent"
      :can-edit="canEditResult"
      :user-key-index="userKeyIndex"
      :is-key="keyIndexes.includes(headerMenu.colIndex)"
      :edit-key-hint="editKeyHint"
      :hidden-count="hiddenColumnsResolved.length"
      @copy-column-name="copyHeaderColumnName"
      @copy-all-columns="copyHeaderAllColumns"
      @copy-column-values="copyColumnValues"
      @search-only="searchOnlyThisColumn"
      @sort="sortByHeader"
      @filter-by-value="openHeaderValueFilter(headerMenu.colIndex)"
      @toggle-key="setKeyColumn(headerMenu.colIndex)"
      @hide-column="hideColumn(headerMenu.colIndex)"
      @hide-column-sql="hideColumnBySql(headerMenu.colIndex)"
      @show-all-columns="showAllColumns"
    />

    <ResultSqlPanel
      :visible="sqlPanelVisible"
      :x="sqlPanelPos.x"
      :y="sqlPanelPos.y"
      :sql="executedSql || availableSql"
      :modified="executedSqlModified"
      @close="closeSqlPanel"
      @copy="copyExecutedSql"
    />

    <ResultFilterPanel
      :conditions="filterConditions"
      @update:conditions="filterConditions = $event"
      :error="filterBuildError"
      @update:error="filterBuildError = $event"
      :visible="filterOpen"
      :columns="result?.columns ?? []"
      :apply-disabled="!filterEnabled"
      @close="closeFilterPanel"
      @apply="applyFilters"
      @clear="clearFilters"
    />

    <ResultCellFilterPanel
      :visible="filterPanel.visible"
      :x="filterPanel.x"
      :y="filterPanel.y"
      :height="filterPanelHeight"
      :column-name="currentFilterColumnName()"
      :row-index="filterPanel.rowIndex"
      :row-value="filterRowValue"
      :op="filterOp"
      :op-options="filterModeOptions()"
      :input="filterInput"
      :error="filterError"
      :candidates="filterCandidates"
      :candidate-filter="filterCandidateFilter"
      @close="closeFilterPanel"
      @apply="applyFilterPanel"
      @apply-candidate="applyCandidate"
      @update:op="filterOp = $event"
      @update:input="filterInput = $event"
      @update:candidate-filter="filterCandidateFilter = $event"
    />

    <ResultAutoRefreshPicker
      :options="refreshOptions"
      :current="autoRefreshSeconds"
      :enabled="refreshEnabled"
      :custom-visible="autoRefreshMenuVisible"
      :custom-error="autoRefreshCustomError"
      :initial-custom="autoRefreshSeconds"
      :picker-visible="pickAutoRefreshOpen"
      :picker-x="autoRefreshMenuX"
      :picker-y="autoRefreshMenuY"
      @choose="chooseAutoRefresh"
      @close-custom="closeAutoRefreshCustom"
      @update:custom-error="autoRefreshCustomError = $event"
      @update:custom-value="autoRefreshCustomValue = $event"
    />

    <ResultApplyConfirmModal
      :visible="applyConfirmVisible"
      :statements="applyStatements"
      :target-table="applyTargetLabel"
      :count="pendingCount"
      :applying="querying"
      @cancel="cancelApplyEdits"
      @confirm="confirmApplyEdits"
      @copy="copyApplyStatements"
      @fill="fillApplyStatementsToEditor"
    />
  </div>
</template>