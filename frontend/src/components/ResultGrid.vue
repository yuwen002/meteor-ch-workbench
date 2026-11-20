<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useVirtualizer } from '@tanstack/vue-virtual'
import type { QueryResult, ResultExportFormat } from '../types'
import { api } from '../api'
import ResultFilterPanel from './ResultFilterPanel.vue'
import {
  type FilterCondition,
  type FilterJoin,
  type FilterOp,
  FILTER_EQ,
  FILTER_JOIN_AND,
  FILTER_OP_ORDER,
  buildFilterWhere,
  conditionPreview,
  countActiveRules,
  isConditionable,
  opSymbol,
  appendSort,
  appendWhere,
} from '../utils/resultFilter'
import {
  buildInsertBatch,
  buildInsertStatement,
  insertTarget,
  rowToTSV,
  sqlLiteral,
  PENDING_TABLE_PLACEHOLDER,
} from '../utils/sqlLiteral'
import {
  columnListText,
  columnValuesText,
  distinctColumnValues,
  filterRowIndices,
} from '../utils/resultView'
import {
  buildGridStyle,
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
import {
  buildAlterUpdateSQL,
  buildRowKeys,
  detectKeyColumns,
  parseQuerySource,
  valueForEdit,
  type AutoKeyResult,
} from '../utils/cellEdit'
import {
  AUTO_REFRESH_DISABLED,
  AUTO_REFRESH_PRESETS,
  AUTO_REFRESH_STORAGE_KEY,
  formatSeconds,
  normalizeSeconds,
  parseStoredSeconds,
} from '../utils/autoRefresh'
import { parseSqlError } from '../utils/sqlError'

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

const MENU_MARGIN = 8
const CELL_MENU_WIDTH = 220
const HEADER_MENU_WIDTH = 240
const FILTER_PANEL_WIDTH = 380

const scrollEl = ref<HTMLElement | null>(null)
const headerScrollEl = ref<HTMLElement | null>(null)
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
const gridStyle = computed(() => {
  if (!props.result?.columns) return ''
  return buildGridStyle(colWidths.value, props.result.columns.length)
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
  const right = Math.max(MENU_MARGIN, window.innerWidth - width - MENU_MARGIN)
  const bottom = Math.max(MENU_MARGIN, window.innerHeight - height - MENU_MARGIN)
  return {
    x: Math.min(Math.max(x, MENU_MARGIN), right),
    y: Math.min(Math.max(y, MENU_MARGIN), bottom),
  }
}

function openCellMenu(e: MouseEvent, rowIndex: number, colIndex: number) {
  e.stopPropagation()
  closeAllMenus()
  const pos = clampMenuPosition(e.clientX, e.clientY, CELL_MENU_WIDTH, 320)
  cellMenu.value = { visible: true, x: pos.x, y: pos.y, rowIndex, colIndex }
  headerMenu.value = { visible: false, x: 0, y: 0, colIndex }
}

function sortBy(dir: 'ASC' | 'DESC') {
  const col = props.result?.columns[cellMenu.value.colIndex]?.name
  if (!col) return
  closeCellMenu()
  const sql = appendSort(props.availableSql, [{ column: col, dir }])
  if (sql) emit('rerun', sql)
}

function filterByValueMerged() {
  const res = props.result
  const col = res?.columns[cellMenu.value.colIndex]?.name
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

function openHeaderValueFilter(e: MouseEvent, colIndex: number) {
  e.stopPropagation()
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
    await copyText(rowToTSV(row), t('editor.copyRow'))
  }
}

async function copyInsert() {
  const row = currentRow()
  const res = props.result
  if (!row || !res) return
  closeCellMenu()
  const names = res.columns.map((c) => c.name)
  await copyText(buildInsertStatement(currentTarget(), names, row), t('editor.copyInsert'))
}

async function copyInsertAll() {
  const res = props.result
  if (!res || !res.rows || res.rows.length === 0) return
  closeCellMenu()
  const cols = res.columns ?? []
  await copyText(buildInsertBatch(currentTarget(), cols.map((c) => c.name), res.rows), t('editor.copyInsertAll'))
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
  if (props.result) {
    closeHeaderMenu()
    await copyText(columnListText(props.result.columns, 0), t('editor.copyColumns'))
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
const sqlButtonWrap = ref<HTMLElement | null>(null)
const sqlPanelPos = ref({ x: 0, y: 0 })
const executedSqlModified = computed(() => {
  const executed = (props.executedSql ?? '').trim()
  const source = (props.availableSql ?? '').trim()
  return executed !== '' && executed !== source
})

function toggleSqlPanel() {
  if (!sqlPanelVisible.value) {
    const wrap = sqlButtonWrap.value
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

const editingCell = ref<{ rowIndex: number; colIndex: number } | null>(null)
const editingValue = ref('')

let editInputEl: HTMLInputElement | null = null

const pendingEdits = ref<Array<{ rowIndex: number; colIndex: number; value: string; sql: string }>>([])

const editSource = computed(() => parseQuerySource(props.availableSql || props.executedSql || ''))
const userKeyIndex = ref(-1)
const autoKey = ref<AutoKeyResult>({ indexes: [], reason: 'all' })
const kcols = computed(() => props.result?.columns ?? [])

const keyIndexes = computed<number[]>(() =>
  userKeyIndex.value >= 0 ? [userKeyIndex.value] : autoKey.value.indexes,
)

const keyColumnIndex = computed(() => (keyIndexes.value.length === 1 ? keyIndexes.value[0] : -1))

const editKeyHint = computed(() => {
  const n = keyIndexes.value.length
  if (n === 0) return t('editor.editAutoNoKey')
  if (userKeyIndex.value >= 0) return t('editor.editKeyManual', { column: kcols.value[userKeyIndex.value]?.name ?? '' })
  if (autoKey.value.reason === 'primary') return t('editor.editKeyPrimary', { count: n })
  if (autoKey.value.reason === 'unique') return t('editor.editKeyUnique', { column: kcols.value[n - 1]?.name ?? '' })
  return t('editor.editKeyAll', { count: n })
})

watch(
  () => props.result,
  () => {
    const cols = props.result?.columns ?? []
    const rows = props.result?.rows ?? []
    autoKey.value = detectKeyColumns({ columns: cols, rows })
    userKeyIndex.value = -1
    editingCell.value = null
    pendingEdits.value = []
  },
  { immediate: true },
)

const canEditResult = computed(() => editSource.value !== null)

function bindEditInput(el: any) {
  if (!el) return
  editInputEl = el as HTMLInputElement
}

function startEditCell(rowIndex: number, colIndex: number) {
  const rows = props.result?.rows ?? []
  if (rowIndex < 0 || rowIndex >= rows.length) return
  editingCell.value = { rowIndex, colIndex }
  editingValue.value = valueForEdit(rows[rowIndex][colIndex])
  nextTick(() => {
    if (editInputEl) {
      editInputEl.focus()
      editInputEl.select()
    }
  })
}

function cancelEditCell() {
  editingCell.value = null
  editingValue.value = ''
}

function setKeyColumn(colIndex: number) {
  userKeyIndex.value = userKeyIndex.value === colIndex ? -1 : colIndex
  closeHeaderMenu()
}

const pendingCount = computed(() => pendingEdits.value.length)

function isPendingEdited(rowIndex: number, colIndex: number): boolean {
  return pendingEdits.value.some((p) => p.rowIndex === rowIndex && p.colIndex === colIndex)
}

function pendingValue(rowIndex: number, colIndex: number): string {
  return pendingEdits.value.find((p) => p.rowIndex === rowIndex && p.colIndex === colIndex)?.value ?? ''
}

function commitEditCell() {
  const cell = editingCell.value
  const r = props.result
  if (!cell || !r) {
    cancelEditCell()
    return
  }
  const before = valueForEdit(r.rows?.[cell.rowIndex]?.[cell.colIndex])
  const after = editingValue.value
  const colName = r.columns?.[cell.colIndex]?.name
  cancelEditCell()
  if (!colName || before === after) {
    if (colName) {
      const i = pendingEdits.value.findIndex((p) => p.rowIndex === cell.rowIndex && p.colIndex === cell.colIndex)
      if (i >= 0) pendingEdits.value.splice(i, 1)
    }
    return
  }

  const idxs = keyIndexes.value
  if (idxs.length === 0) {
    emit('error', t('editor.editNoKey'))
    return
  }
  const row = r.rows?.[cell.rowIndex] ?? []
  const keys = buildRowKeys(idxs, r.columns ?? [], row)
  if (keys.length === 0) {
    emit('error', t('editor.editNoKey'))
    return
  }
  if (keys.some((k) => k.value === null || k.value === undefined)) {
    emit('error', t('editor.editKeyNull'))
    return
  }
  const columnType = (name: string) => r.columns?.find((c) => c.name === name)?.type ?? null
  const sql = buildAlterUpdateSQL(
    editSource.value,
    { column: colName, oldValue: row?.[cell.colIndex], newValue: after },
    keys,
    columnType,
  )
  if (!sql) {
    emit('error', t('editor.editNoSource'))
    return
  }
  const existing = pendingEdits.value.find((p) => p.rowIndex === cell.rowIndex && p.colIndex === cell.colIndex)
  if (existing) {
    existing.value = after
    existing.sql = sql
  } else {
    pendingEdits.value.push({ rowIndex: cell.rowIndex, colIndex: cell.colIndex, value: after, sql })
  }
}

function applyPendingEdits() {
  if (pendingEdits.value.length === 0) return
  emit('apply-edits', pendingEdits.value.map((p) => p.sql))
  pendingEdits.value = []
}

function discardPendingEdits() {
  pendingEdits.value = []
}

function fillPendingEditsToEditor() {
  if (pendingEdits.value.length === 0) return
  emit('edit-sql', pendingEdits.value.map((p) => p.sql).join('\n'))
  pendingEdits.value = []
}

async function copyExecutedSql() {
  const sql = props.executedSql ?? ''
  if (!sql) return
  await copyText(sql, t('editor.copySql'))
}

const explainModes = EXPLAIN_MODES

const filterOpen = ref(false)
const filterConditions = ref<FilterCondition[]>([])
const filterBuildError = ref<string | null>(null)
const filterHasActive = computed(() => filterConditions.value.some((c) => buildFilterWhere([c]) !== ''))
const activeFilterCount = computed(() => countActiveRules(filterConditions.value))

function newRule() {
  return { op: FILTER_EQ as FilterOp, value: '' }
}

function newCondition(): FilterCondition {
  const res = props.result
  return {
    column: res?.columns[0]?.name ?? '',
    join: FILTER_JOIN_AND,
    rules: [newRule()],
  }
}

function openFilterPanel() {
  filterBuildError.value = null
  if (filterConditions.value.length === 0) {
    const res = props.result
    if (!res || !res.columns || res.columns.length === 0) return
    filterConditions.value = [newCondition()]
  }
  filterOpen.value = true
}

function closeFilterPanel() {
  filterOpen.value = false
  filterBuildError.value = null
}

function resetFilterPanel() {
  filterConditions.value = [newCondition()]
  filterBuildError.value = null
}

function clearFilters() {
  resetFilterPanel()
  filterOpen.value = false
  if (props.availableSql.trim()) emit('rerun', props.availableSql)
}

function applyFilters() {
  if (!props.result) return
  const where = buildFilterWhere(filterConditions.value)
  if (!where) {
    filterBuildError.value = t('editor.filterRuleRequired')
    return
  }
  const sql = appendWhere(props.availableSql, [where])
  if (!sql) {
    filterBuildError.value = t('editor.filterValueInvalid')
    return
  }
  closeFilterPanel()
  emit('rerun', sql)
}

const filterCandidates = computed(() => {
  const panel = filterPanel.value
  if (panel.rowIndex >= 0) return []
  const res = props.result
  if (!res) return []
  const colName = currentFilterColumnName()
  if (!colName) return []
  const values = distinctColumnValues(res.rows ?? [], rowIndices.value, panel.colIndex)
    .map((v) => (v == null ? 'NULL' : String(v)))
    .filter((v) => v !== 'NULL')
  const keyword = filterCandidateFilter.value.trim().toLowerCase()
  const seen = new Set<string>()
  const out: Array<{ label: string; op: FilterOp; value: string }> = []
  for (const v of values) {
    if (seen.has(v)) continue
    seen.add(v)
    if (keyword && !v.toLowerCase().includes(keyword)) continue
    out.push({ label: conditionPreview(colName, FILTER_EQ, v), op: FILTER_EQ, value: v })
    if (out.length >= FILTER_CANDIDATE_MAX) break
  }
  return out
})

const FILTER_CANDIDATE_MAX = 12

function filterModeOptions() {
  const col = currentFilterColumnName()
  return FILTER_OP_ORDER.map((op) => ({ op, label: conditionPreview(col, op, '..') }))
}

function currentFilterColumnName(): string {
  return props.result?.columns?.[filterPanel.value.colIndex]?.name ?? ''
}

const filterRowValue = computed(() => cellValueAt(filterPanel.value.rowIndex, filterPanel.value.colIndex))

function buildConditionLiteral(column: string, op: FilterOp, value: unknown): string {
  return `${column} ${opSymbol(op)} ${sqlLiteral(value)}`
}

function applyFilterPanel() {
  const res = props.result
  const col = currentFilterColumnName()
  if (!res || !col) {
    closeFilterPanel()
    return
  }
  const value = filterRowValue.value
  closeFilterPanel()
  const sql = appendWhere(props.availableSql, [buildConditionLiteral(col, filterOp.value, value)])
  if (sql) emit('rerun', sql)
}

function applyCandidate(picked: { value: string }) {
  const col = currentFilterColumnName()
  if (!col) return
  closeFilterPanel()
  const sql = appendWhere(props.availableSql, [buildConditionLiteral(col, FILTER_EQ, picked.value)])
  if (sql) emit('rerun', sql)
}


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
  if (filterOpen.value) {
    closeFilterPanel()
    return
  }
  closeAllMenus()
}

window.addEventListener('click', closeAllMenus)
window.addEventListener('keydown', onGlobalKeydown)

const autoRefreshSeconds = ref(normalizeSeconds(parseStoredSeconds(localStorage.getItem(AUTO_REFRESH_STORAGE_KEY))))
const autoRefreshMenuVisible = ref(false)
const autoRefreshCustomValue = ref('')
const autoRefreshCustomError = ref<string | null>(null)
const autoRefreshRemaining = ref(0)
let refreshTimer: number | undefined
let lastRefreshAt = 0

const refreshEnabled = computed(() => autoRefreshSeconds.value > 0)
const refreshOptions = computed(() =>
  AUTO_REFRESH_PRESETS.map((v) => ({ value: v, label: t('editor.refreshSeconds', { seconds: v }) })),
)

const pickAutoRefreshOpen = ref(false)
const autoRefreshMenuX = ref(0)
const autoRefreshMenuY = ref(0)

function toggleAutoRefreshMenu(e: MouseEvent) {
  if (pickAutoRefreshOpen.value) {
    pickAutoRefreshOpen.value = false
    return
  }
  closeAllMenus()
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const pos = clampMenuPosition(rect.right - 140, rect.bottom + 4, 140, 260)
  autoRefreshMenuX.value = pos.x
  autoRefreshMenuY.value = pos.y
  pickAutoRefreshOpen.value = true
}

function stopTimer() {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = undefined
  }
  autoRefreshRemaining.value = 0
}

function tick() {
  const total = normalizeSeconds(autoRefreshSeconds.value)
  if (total <= 0) {
    stopTimer()
    return
  }
  const elapsed = Math.floor((Date.now() - lastRefreshAt) / 1000)
  const left = Math.max(0, total - elapsed)
  autoRefreshRemaining.value = left
  if (left <= 0 && !props.querying) triggerRefresh()
}

function startTimer() {
  stopTimer()
  lastRefreshAt = Date.now()
  autoRefreshRemaining.value = normalizeSeconds(autoRefreshSeconds.value)
  refreshTimer = window.setInterval(tick, 1000)
}

function triggerRefresh() {
  lastRefreshAt = Date.now()
  autoRefreshRemaining.value = normalizeSeconds(autoRefreshSeconds.value)
  emit('refresh')
}

function chooseAutoRefresh(v: number) {
  const seconds = normalizeSeconds(v)
  autoRefreshSeconds.value = seconds
  if (seconds > 0) {
    localStorage.setItem(AUTO_REFRESH_STORAGE_KEY, String(seconds))
    startTimer()
  } else {
    localStorage.removeItem(AUTO_REFRESH_STORAGE_KEY)
    stopTimer()
  }
}

function openAutoRefreshCustom() {
  autoRefreshCustomValue.value = autoRefreshSeconds.value > 0 ? String(autoRefreshSeconds.value) : ''
  autoRefreshCustomError.value = null
  autoRefreshMenuVisible.value = true
}

function closeAutoRefreshCustom() {
  autoRefreshMenuVisible.value = false
  autoRefreshCustomError.value = null
}

function confirmAutoRefreshCustom() {
  const raw = autoRefreshCustomValue.value.trim()
  if (!raw) {
    autoRefreshCustomError.value = t('editor.refreshCustomRequired')
    return
  }
  const seconds = normalizeSeconds(raw)
  if (seconds <= 0) {
    autoRefreshCustomError.value = t('editor.refreshCustomInvalid')
    return
  }
  closeAutoRefreshCustom()
  chooseAutoRefresh(seconds)
}

function manualRefresh() {
  lastRefreshAt = Date.now()
  autoRefreshRemaining.value = normalizeSeconds(autoRefreshSeconds.value)
  emit('refresh')
}

watch(
  () => props.querying,
  (now, before) => {
    if (before && !now && refreshEnabled.value) startTimer()
  },
)

watch(
  () => props.result?.columns?.length ?? 0,
  (len) => {
    if (searchColumn.value >= len) searchColumn.value = -1
  },
)

watch(refreshEnabled, (on) => {
  if (on) startTimer()
  else stopTimer()
})

onBeforeUnmount(() => {
  window.removeEventListener('click', closeAllMenus)
  window.removeEventListener('keydown', onGlobalKeydown)
  if (copiedTimer) clearTimeout(copiedTimer)
  stopTimer()
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
  exporting.value = true
  exportedPath.value = null
  try {
    const indices = rowIndices.value
    const rows = indices.map((ri) => res.rows?.[ri] ?? [])
    const path = await api.SaveQueryResultAs(format.id, res.columns.map((c) => c.name), rows, currentTarget())
    if (path) exportedPath.value = path
    exportMenuOpen.value = false
  } catch (err) {
    emit('error', String(err))
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
      <div class="flex items-center gap-2 shrink-0 whitespace-nowrap">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2"/><line x1="3" y1="9" x2="21" y2="9"/><line x1="3" y1="15" x2="21" y2="15"/><line x1="9" y1="3" x2="9" y2="21"/></svg>
        <span class="font-semibold">{{ t('editor.results') }}</span>
        <template v-if="result">
          <span class="text-ch-text">{{ result.row_count.toLocaleString() }} {{ t('editor.rows') }}</span>
          <span>·</span>
          <span class="text-ch-text">{{ result.elapsed_ms?.toFixed(2) }} ms</span>
        </template>
        <div v-if="executedSql" ref="sqlButtonWrap" class="relative shrink-0">
          <button
            @click="toggleSqlPanel"
            class="flex items-center gap-1 px-2 py-0.5 rounded border border-ch-border text-ch-muted hover:text-ch-accent hover:border-ch-accent"
          >
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>
            <span>{{ t('editor.viewSql') }}</span>
            <span v-if="executedSqlModified" class="w-1.5 h-1.5 rounded-full bg-ch-accent" :title="t('editor.executedSqlModified')"></span>
          </button>
        </div>
        <span v-if="loadingWithResult" class="text-ch-accent">{{ t('editor.refreshing') }}</span>
        <span v-else-if="refreshEnabled" class="text-ch-muted" :title="t('editor.refreshHint')">{{ t('editor.refreshCountdown', { seconds: autoRefreshRemaining }) }}</span>
        <span v-if="copiedHint" class="text-ch-accent">{{ copiedHint }}</span>
        <div class="relative shrink-0">
          <input
            v-model="searchText"
            type="text"
            :placeholder="t('editor.searchPlaceholder')"
            class="w-[220px] bg-ch-bg border border-ch-border rounded px-2 py-0.5 text-xs text-ch-text outline-none focus:border-ch-accent"
          />
          <button
            v-if="searchText"
            @click="clearSearch"
            class="absolute right-1 top-1/2 -translate-y-1/2 text-ch-muted hover:text-ch-text"
            :title="t('editor.clearSearch')"
          >
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </div>
        <span v-if="searchActive && result" class="text-ch-accent shrink-0 whitespace-nowrap">
          {{ t('editor.filteredCount', { shown: rowIndices.length, total: result.rows?.length ?? 0 }) }}
        </span>
      </div>

      <div class="flex-1 min-w-0"></div>

      <div class="flex items-center gap-2 shrink-0">
        <div class="flex items-center rounded border border-ch-border overflow-hidden">
          <button
            @click="manualRefresh"
            :disabled="querying"
            class="flex items-center gap-1 px-2 py-0.5 text-ch-muted hover:text-ch-text hover:bg-ch-hover disabled:opacity-40 disabled:hover:bg-transparent"
            :title="t('editor.refreshHint')"
          >
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
            <span>{{ t('editor.refresh') }}</span>
          </button>
          <div class="relative">
            <button
              @click.stop="toggleAutoRefreshMenu"
              class="flex items-center gap-1 px-2 py-0.5 border-l border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover"
              :title="t('editor.refreshInterval')"
            >
              <span>{{ refreshEnabled ? t('editor.refreshSeconds', { seconds: autoRefreshSeconds }) : t('editor.refreshOff') }}</span>
              <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"/></svg>
            </button>
          </div>
          <button
            @click.stop="openAutoRefreshCustom"
            class="px-1.5 py-0.5 border-l border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover"
            :title="t('editor.refreshCustom')"
          >
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
          </button>
        </div>

        <div class="flex items-center rounded border border-ch-border overflow-hidden">
          <button
            @click="explainWithCurrentMode"
            :disabled="!canExplainCurrent || querying"
            class="flex items-center gap-1 px-2 py-0.5 text-ch-muted hover:text-ch-text hover:bg-ch-hover disabled:opacity-40 disabled:hover:bg-transparent"
            :title="t('editor.explainHint')"
          >
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
            <span>{{ t('editor.explain') }}</span>
          </button>
          <select
            v-model="explainMode"
            :disabled="!canExplainCurrent || querying"
            class="border-l border-ch-border bg-transparent text-ch-muted text-xs px-1 py-0.5 outline-none disabled:opacity-40"
          >
            <option v-for="m in explainModes" :key="m.id" :value="m.id">{{ t(m.labelKey) }}</option>
          </select>
        </div>

        <button
          @click.stop="exportMenuOpen = !exportMenuOpen"
          :disabled="!hasColumns || exporting"
          class="flex items-center gap-1 px-2 py-0.5 rounded border border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover disabled:opacity-40 disabled:hover:bg-transparent"
          :title="t('editor.exportHint')"
        >
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
          <span>{{ t('editor.export') }}</span>
          <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"/></svg>
        </button>

        <button
          @click.stop="openFilterPanel"
          :disabled="!filterEnabled || querying"
          class="flex items-center gap-1 px-2 py-0.5 rounded border disabled:opacity-40 disabled:hover:bg-transparent"
          :class="filterHasActive ? 'border-ch-accent text-ch-accent' : 'border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover'"
          :title="filterEnabled ? t('editor.filterHint') : t('editor.filterDisabled')"
        >
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/></svg>
          <span>{{ t('editor.filter') }}</span>
          <span v-if="activeFilterCount > 0" class="text-[10px] opacity-80">{{ activeFilterCount }}</span>
        </button>
      </div>
    </div>

    <div v-if="exportMenuOpen" class="flex items-center gap-2 px-3 py-1 border-b border-ch-border bg-ch-panel text-xs shrink-0">
      <span class="text-ch-muted">{{ t('editor.exportFormat') }}</span>
      <button
        v-for="f in exportFormats"
        :key="f.id"
        @click="doExport(f)"
        :disabled="exporting"
        class="px-2 py-0.5 rounded border border-ch-border text-ch-muted hover:text-ch-accent hover:border-ch-accent disabled:opacity-40"
      >
        {{ f.name }}
      </button>
      <span v-if="exportedPath" class="text-ch-accent truncate">{{ t('editor.exported') }} {{ exportedPath }}</span>
    </div>

    <div v-if="loadingNoResult" class="flex-1 flex items-center justify-center text-xs text-ch-muted">
      {{ t('common.loading') }}
    </div>

    <div v-else-if="parsedError" class="flex-1 overflow-auto p-4">
      <div class="text-sm text-red-400 font-semibold">{{ parsedError.summary || t('editor.queryFailed') }}</div>
      <div v-if="parsedError.code !== null" class="text-xs text-ch-muted mt-1">Code: {{ parsedError.code }}</div>
      <pre class="text-xs text-ch-text mt-2 whitespace-pre-wrap font-mono">{{ parsedError.raw }}</pre>
    </div>

    <div v-else-if="isEmptyResult" class="flex-1 flex flex-col items-center justify-center gap-2 text-xs text-ch-muted">
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="opacity-50"><rect x="3" y="3" width="18" height="18" rx="2"/><line x1="8" y1="12" x2="16" y2="12"/></svg>
      <span>{{ t('editor.emptyResult') }}</span>
    </div>

    <div v-else-if="result" class="flex-1 min-h-0 flex flex-col">
      <div
        ref="headerScrollEl"
        @scroll="onHeaderScroll"
        class="overflow-hidden shrink-0 border-b border-ch-border bg-ch-panel"
      >
        <div class="grid" :style="gridStyle">
          <div
            v-for="(col, ci) in result.columns"
            :key="'h-' + ci"
            class="group relative px-3 py-1.5 text-xs font-semibold text-ch-text border-r border-ch-border/40 last:border-r-0 truncate select-none"
            :title="col.name"
            @contextmenu.prevent="openHeaderMenu($event, ci)"
          >
            <div class="flex items-center gap-1.5">
              <span class="truncate">{{ col.name }}</span>
              <span v-if="keyIndexes.includes(ci)" class="text-[10px] text-ch-accent shrink-0" :title="t('editor.useAsEditKey')">KEY</span>
              <span v-if="searchColumn === ci" class="text-[10px] text-ch-accent shrink-0">🔍</span>
            </div>
            <div class="text-[10px] text-ch-muted font-normal truncate">{{ col.type }}</div>
            <button
              @click="openHeaderValueFilter($event, ci)"
              class="absolute right-1 top-1 opacity-0 group-hover:opacity-100 text-ch-muted hover:text-ch-accent bg-ch-panel rounded p-0.5"
              :title="t('editor.filterByValue')"
            >
              <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/></svg>
            </button>
          </div>
        </div>
      </div>

      <div ref="scrollEl" @scroll="onBodyScroll" class="flex-1 min-h-0 overflow-auto">
        <div v-if="searchNoMatch" class="flex items-center justify-center h-full text-xs text-ch-muted">
          {{ t('editor.searchNoMatch', { query: searchText }) }}
        </div>
        <div v-else-if="virtualEnabled" class="relative" :style="{ height: virtualHeight + 'px' }">
          <div
            v-for="item in visibleRows()"
            :key="'r-' + item.ri"
            class="grid absolute left-0 w-full border-b border-ch-border/40 hover:bg-ch-hover"
            :style="{ ...parseStyle(gridStyle), transform: `translateY(${virtualRowTop(item.ri)}px)`, height: ROW_HEIGHT + 'px' }"
          >
            <div
              v-for="(val, ci) in item.row"
              :key="'c-' + ci"
              class="px-3 text-xs text-ch-text border-r border-ch-border/40 last:border-r-0 truncate font-mono leading-none flex items-center"
              :title="val !== null && val !== undefined ? String(val) : 'NULL'"
              @contextmenu.prevent="openCellMenu($event, item.ri, ci)"
              @dblclick="startEditCell(item.ri, ci)"
            >
              <input
                v-if="editingCell && editingCell.rowIndex === item.ri && editingCell.colIndex === ci"
                :ref="bindEditInput"
                v-model="editingValue"
                class="w-full h-5 box-border bg-ch-bg border border-ch-accent rounded px-1 text-xs leading-5 text-ch-text font-mono focus:outline-none"
                @keydown.enter.prevent="commitEditCell"
                @keydown.esc.prevent="cancelEditCell"
                @blur="commitEditCell"
              />
              <template v-else>
                <span v-if="isPendingEdited(item.ri, ci)" class="text-amber-300">{{ String(pendingValue(item.ri, ci)) }}</span>
                <span v-else-if="val === null || val === undefined" class="text-ch-muted italic">NULL</span>
                <template v-else>
                  <span v-if="typeof val === 'string' && (val.startsWith('{') || val.startsWith('['))" class="text-blue-400">{{ String(val).slice(0, 100) }}</span>
                  <span v-else>{{ String(val) }}</span>
                </template>
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
              @dblclick="startEditCell(item.ri, ci)"
            >
              <input
                v-if="editingCell && editingCell.rowIndex === item.ri && editingCell.colIndex === ci"
                :ref="bindEditInput"
                v-model="editingValue"
                class="w-full h-5 box-border bg-ch-bg border border-ch-accent rounded px-1 text-xs leading-5 text-ch-text font-mono focus:outline-none"
                @keydown.enter.prevent="commitEditCell"
                @keydown.esc.prevent="cancelEditCell"
                @blur="commitEditCell"
              />
              <template v-else>
                <span v-if="isPendingEdited(item.ri, ci)" class="text-amber-300">{{ String(pendingValue(item.ri, ci)) }}</span>
                <span v-else-if="val === null || val === undefined" class="text-ch-muted italic">NULL</span>
                <template v-else>
                  <span v-if="typeof val === 'string' && (val.startsWith('{') || val.startsWith('['))" class="text-blue-400">{{ String(val).slice(0, 100) }}</span>
                  <span v-else>{{ String(val) }}</span>
                </template>
              </template>
            </div>
          </div>
        </template>
      </div>

      <div v-if="pendingCount > 0" class="flex items-center gap-2 px-3 py-1 border-t border-ch-border bg-amber-500/10 text-[11px] shrink-0">
        <span class="text-amber-300">{{ t('editor.pendingEdits', { count: pendingCount }) }}</span>
        <div class="flex-1"></div>
        <button
          @click="fillPendingEditsToEditor"
          :disabled="querying"
          class="px-2 py-0.5 rounded border border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {{ t('editor.fillToEditor') }}
        </button>
        <button
          @click="discardPendingEdits"
          :disabled="querying"
          class="px-2 py-0.5 rounded border border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {{ t('editor.discardEdits') }}
        </button>
        <button
          @click="applyPendingEdits"
          :disabled="querying"
          class="px-2 py-0.5 rounded bg-ch-accent text-white hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {{ querying ? t('editor.applyingEdits') : t('editor.applyEdits') }}
        </button>
      </div>

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

    <Teleport to="body">
      <div
        v-if="cellMenu.visible"
        class="fixed z-[3000] bg-ch-panel border border-ch-border rounded shadow-lg py-1 text-xs text-ch-text min-w-[180px]"
        :style="{ left: cellMenu.x + 'px', top: cellMenu.y + 'px' }"
        @click.stop
      >
        <button @click="copyCell" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
          <span>{{ t('editor.copyCell') }}</span>
        </button>
        <button @click="copyCellLiteral" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 7 4 4 20 4 20 7"/><line x1="9" y1="20" x2="15" y2="20"/><line x1="12" y1="4" x2="12" y2="20"/></svg>
          <span>{{ t('editor.copyCellLiteral') }}</span>
        </button>
        <div class="border-t border-ch-border/50 my-1"></div>
        <button @click="copyRow" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="3" y1="8" x2="21" y2="8"/><line x1="3" y1="16" x2="21" y2="16"/></svg>
          <span>{{ t('editor.copyRow') }}</span>
        </button>
        <button @click="copyInsert" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
          <span>{{ t('editor.copyInsert') }}</span>
        </button>
        <button @click="copyInsertAll" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
          <span>{{ t('editor.copyInsertAll') }}</span>
        </button>
        <div v-if="canExplainCurrent" class="border-t border-ch-border/50 my-1"></div>
        <button v-if="canExplainCurrent" @click="sortBy('ASC')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="19" x2="12" y2="5"/><polyline points="5 12 12 5 19 12"/></svg>
          <span>{{ t('editor.orderByAsc', { column: result?.columns[cellMenu.colIndex]?.name }) }}</span>
        </button>
        <button v-if="canExplainCurrent" @click="sortBy('DESC')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><polyline points="19 12 12 19 5 12"/></svg>
          <span>{{ t('editor.orderByDesc', { column: result?.columns[cellMenu.colIndex]?.name }) }}</span>
        </button>
        <button v-if="canExplainCurrent" @click="filterByValueMerged" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/></svg>
          <span>{{ t('editor.filterByValue') }}</span>
        </button>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="headerMenu.visible"
        class="fixed z-[3000] bg-ch-panel border border-ch-border rounded shadow-lg py-1 text-xs text-ch-text min-w-[200px]"
        :style="{ left: headerMenu.x + 'px', top: headerMenu.y + 'px' }"
        @click.stop
      >
        <button @click="copyHeaderColumnName" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
          <span>{{ t('editor.copyColumnName') }}</span>
        </button>
        <button @click="copyHeaderAllColumns" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
          <span>{{ t('editor.copyColumns') }}</span>
        </button>
        <button @click="copyColumnValues(false)" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="3" y1="8" x2="21" y2="8"/><line x1="3" y1="16" x2="21" y2="16"/></svg>
          <span>{{ t('editor.copyColumnValues') }}</span>
        </button>
        <button @click="copyColumnValues(true)" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="3" y1="8" x2="21" y2="8"/><line x1="3" y1="16" x2="21" y2="16"/></svg>
          <span>{{ t('editor.copyColumnValuesDistinct') }}</span>
        </button>
        <div class="border-t border-ch-border/50 my-1"></div>
        <button @click="searchOnlyThisColumn" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
          <span>{{ t('editor.searchOnlyThisColumn') }}</span>
        </button>
        <button v-if="canExplainCurrent" @click="sortByHeader('ASC')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="19" x2="12" y2="5"/><polyline points="5 12 12 5 19 12"/></svg>
          <span>{{ t('editor.orderByAsc', { column: headerColumnName() }) }}</span>
        </button>
        <button v-if="canExplainCurrent" @click="sortByHeader('DESC')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><polyline points="19 12 12 19 5 12"/></svg>
          <span>{{ t('editor.orderByDesc', { column: headerColumnName() }) }}</span>
        </button>
        <template v-if="canExplainCurrent">
          <div class="border-t border-ch-border/50 my-1"></div>
          <button @click="openHeaderValueFilter($event, headerMenu.colIndex)" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/></svg>
            <span>{{ t('editor.filterByValue') }}</span>
          </button>
          <div class="border-t border-ch-border/50 my-1"></div>
          <div class="px-3 py-1 text-[11px] text-ch-muted leading-snug">{{ editKeyHint }}</div>
          <div class="border-t border-ch-border/50 my-1"></div>
          <button
            :disabled="!canEditResult"
            @click="setKeyColumn(headerMenu.colIndex)"
            class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-ch-text hover:bg-ch-hover disabled:opacity-40 disabled:hover:bg-transparent"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="7.5" cy="15.5" r="3.5"/><path d="M10 12L20 2"/><line x1="16" y1="6" x2="18" y2="8"/><line x1="18" y1="8" x2="20" y2="6"/></svg>
            <span class="flex-1">{{ userKeyIndex === headerMenu.colIndex ? t('editor.useAsEditKeyCancel') : t('editor.useAsEditKey') }}</span>
            <svg v-if="keyColumnIndex === headerMenu.colIndex" width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"/></svg>
          </button>
        </template>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="sqlPanelVisible"
        class="fixed z-[3000] bg-ch-panel border border-ch-border rounded shadow-lg text-xs"
        :style="{ left: sqlPanelPos.x + 'px', top: sqlPanelPos.y + 'px', width: '520px', maxWidth: '80vw' }"
        @click.stop
      >
        <div class="flex items-center justify-between px-3 py-1.5 border-b border-ch-border">
          <span class="font-semibold text-ch-text">{{ t('editor.viewSql') }}</span>
          <div class="flex items-center gap-2">
            <button @click="copyExecutedSql" class="text-ch-muted hover:text-ch-accent">{{ t('editor.copySql') }}</button>
            <button @click="closeSqlPanel" class="text-ch-muted hover:text-ch-text">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            </button>
          </div>
        </div>
        <div class="px-3 py-1 text-[11px] text-ch-muted border-b border-ch-border/50">
          <span v-if="executedSqlModified">{{ t('editor.executedSqlModified') }}</span>
          <span v-else>{{ t('editor.executedSqlOriginal') }}</span>
        </div>
        <pre class="px-3 py-2 text-xs text-ch-text font-mono whitespace-pre-wrap max-h-[420px] overflow-auto">{{ executedSql || availableSql }}</pre>
      </div>
    </Teleport>

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

    <Teleport to="body">
      <div
        v-if="filterPanel.visible"
        class="fixed z-[3000] bg-ch-panel border border-ch-border rounded shadow-lg text-xs flex flex-col"
        :style="{ left: filterPanel.x + 'px', top: filterPanel.y + 'px', width: '380px', maxHeight: filterPanelHeight }"
        @click.stop
        @keydown.esc="closeFilterPanel"
      >
        <div class="flex items-center justify-between px-3 py-1.5 border-b border-ch-border shrink-0">
          <span class="font-semibold text-ch-text truncate">{{ t('editor.filterByValue') }} · {{ currentFilterColumnName() }}</span>
          <button @click="closeFilterPanel" class="text-ch-muted hover:text-ch-text">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </div>
        <div class="p-3 space-y-2 overflow-auto">
          <template v-if="filterPanel.rowIndex < 0">
            <input
              v-model="filterCandidateFilter"
              type="text"
              :placeholder="t('editor.filterCandidatePlaceholder')"
              class="w-full bg-ch-bg border border-ch-border rounded px-2 py-1 text-xs text-ch-text outline-none focus:border-ch-accent"
            />
            <div v-if="filterCandidates.length" class="border border-ch-border rounded max-h-[180px] overflow-auto">
              <button
                v-for="c in filterCandidates"
                :key="c.value"
                @click="applyCandidate(c)"
                class="block w-full text-left px-2 py-1 text-ch-text hover:bg-ch-hover truncate"
              >
                {{ c.label }}
              </button>
            </div>
            <div v-else class="text-ch-muted px-1">{{ t('editor.filterCandidateEmpty') }}</div>
            <div class="border-t border-ch-border/50 pt-2">
              <div class="text-ch-muted mb-1">{{ t('editor.customFilter') }}</div>
            </div>
          </template>
          <div class="flex items-center gap-2">
            <select
              v-model="filterOp"
              class="flex-1 bg-ch-bg border border-ch-border rounded px-2 py-1 text-xs text-ch-text outline-none focus:border-ch-accent"
            >
              <option v-for="o in filterModeOptions()" :key="o.op" :value="o.op">{{ o.label }}</option>
            </select>
          </div>
          <input
            v-model="filterInput"
            type="text"
            :placeholder="filterPanel.rowIndex >= 0 ? String(filterRowValue ?? '') : t('editor.filterValuePlaceholder')"
            class="w-full bg-ch-bg border border-ch-border rounded px-2 py-1 text-xs text-ch-text outline-none focus:border-ch-accent"
            @keydown.enter.prevent="applyFilterPanel"
          />
          <div v-if="filterError" class="text-red-400">{{ filterError }}</div>
        </div>
        <div class="flex items-center justify-end gap-2 px-3 py-2 border-t border-ch-border shrink-0">
          <button @click="closeFilterPanel" class="px-2 py-1 rounded border border-ch-border text-ch-muted hover:text-ch-text">
            {{ t('common.cancel') }}
          </button>
          <button
            @click="applyFilterPanel"
            class="px-2 py-1 rounded bg-ch-accent text-white disabled:opacity-50"
          >
            {{ t('editor.filterApply') }}
          </button>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="autoRefreshMenuVisible"
        class="fixed inset-0 z-[3000] bg-black/50 flex items-center justify-center"
        @click.self="closeAutoRefreshCustom"
      >
        <div class="bg-ch-panel border border-ch-border rounded shadow-xl w-[340px]">
          <div class="flex items-center justify-between px-4 py-2.5 border-b border-ch-border">
            <span class="text-sm font-semibold text-ch-text">{{ t('editor.refreshCustomTitle') }}</span>
            <button @click="closeAutoRefreshCustom" class="text-ch-muted hover:text-ch-text" :title="t('common.cancel')">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            </button>
          </div>
          <div class="px-4 py-3 space-y-2">
            <label class="block text-xs text-ch-muted">{{ t('editor.refreshCustomLabel') }}</label>
            <input
              v-model="autoRefreshCustomValue"
              type="number"
              min="1"
              class="w-full bg-ch-bg border border-ch-border rounded px-2 py-1.5 text-sm text-ch-text outline-none focus:border-ch-accent"
              :placeholder="t('editor.refreshCustomPlaceholder')"
              @keydown.enter.prevent="confirmAutoRefreshCustom"
            />
            <p class="text-[11px] text-ch-muted">{{ t('editor.refreshCustomHint') }}</p>
            <p v-if="autoRefreshCustomError" class="text-[11px] text-red-400">{{ autoRefreshCustomError }}</p>
          </div>
          <div class="flex items-center justify-end gap-2 px-4 py-3 border-t border-ch-border">
            <button @click="closeAutoRefreshCustom" class="px-3 py-1 rounded border border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover text-xs">
              {{ t('common.cancel') }}
            </button>
            <button @click="confirmAutoRefreshCustom" class="px-3 py-1 rounded bg-ch-accent text-white text-xs">
              {{ t('editor.refreshCustomConfirm') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="pickAutoRefreshOpen"
        class="fixed z-[3000] bg-ch-panel border border-ch-border rounded shadow-lg py-1 text-xs text-ch-text min-w-[140px]"
        :style="{ left: autoRefreshMenuX + 'px', top: autoRefreshMenuY + 'px' }"
        @click.stop
      >
        <button
          @click="chooseAutoRefresh(0)"
          class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover"
        >
          <span class="flex-1">{{ t('editor.refreshOff') }}</span>
          <svg v-if="!refreshEnabled" width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"/></svg>
        </button>
        <button
          v-for="o in refreshOptions"
          :key="o.value"
          @click="chooseAutoRefresh(o.value)"
          class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover"
        >
          <span class="flex-1">{{ o.label }}</span>
          <svg v-if="autoRefreshSeconds === o.value" width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"/></svg>
        </button>
      </div>
    </Teleport>
  </div>
</template>