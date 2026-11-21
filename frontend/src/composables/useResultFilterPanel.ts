import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  appendWhere,
  buildFilterWhere,
  conditionPreview,
  countActiveRules,
  FILTER_EQ,
  FILTER_JOIN_AND,
  FILTER_OP_ORDER,
  opSymbol,
  type FilterCondition,
  type FilterOp,
} from '../utils/resultFilter'
import { distinctColumnValues } from '../utils/resultView'
import { sqlLiteral } from '../utils/sqlLiteral'

const FILTER_CANDIDATE_MAX = 12
const FILTER_PANEL_COL_INDEX_DEFAULT = -1

export interface UseResultFilterPanelOptions {
  result: () => { columns: Array<{ name: string }>; rows: unknown[][] | null } | null
  rowIndices: () => number[]
  availableSql: () => string
  filterPanel: () => { rowIndex: number; colIndex: number }
  filterOp: () => FilterOp
  candidateFilter: () => string
  cellValueAt: (rowIndex: number, colIndex: number) => unknown
  onRerun: (sql: string) => void
  onClosePanel?: () => void
}

export function useResultFilterPanel(options: UseResultFilterPanelOptions) {
  const { t } = useI18n()

  const filterOpen = ref(false)
  const filterConditions = ref<FilterCondition[]>([])
  const filterBuildError = ref<string | null>(null)
  const filterHasActive = computed(() => filterConditions.value.some((c) => buildFilterWhere([c]) !== ''))
  const activeFilterCount = computed(() => countActiveRules(filterConditions.value))

  function newRule() {
    return { op: FILTER_EQ as FilterOp, value: '' }
  }

  function newCondition(): FilterCondition {
    const res = options.result()
    return {
      column: res?.columns?.[0]?.name ?? '',
      join: FILTER_JOIN_AND,
      rules: [newRule()],
    }
  }

  function openFilterPanel() {
    filterBuildError.value = null
    if (filterConditions.value.length === 0) {
      const res = options.result()
      if (!res || !res.columns || res.columns.length === 0) return
      filterConditions.value = [newCondition()]
    }
    filterOpen.value = true
  }

  function closeFilterPanel() {
    filterOpen.value = false
    filterBuildError.value = null
    options.onClosePanel?.()
  }

  function resetFilterPanel() {
    filterConditions.value = [newCondition()]
    filterBuildError.value = null
  }

  function clearFilters() {
    resetFilterPanel()
    filterOpen.value = false
    if (options.availableSql().trim()) options.onRerun(options.availableSql())
  }

  function applyFilters() {
    if (!options.result()) return
    const where = buildFilterWhere(filterConditions.value)
    if (!where) {
      filterBuildError.value = t('editor.filterRuleRequired')
      return
    }
    const sql = appendWhere(options.availableSql(), [where])
    if (!sql) {
      filterBuildError.value = t('editor.filterValueInvalid')
      return
    }
    closeFilterPanel()
    options.onRerun(sql)
  }

  const filterCandidates = computed(() => {
    const panel = options.filterPanel()
    if (panel.rowIndex >= 0) return []
    const res = options.result()
    if (!res) return []
    const colName = currentFilterColumnName()
    if (!colName) return []
    const values = distinctColumnValues(res.rows ?? [], options.rowIndices(), panel.colIndex)
      .map((v) => (v == null ? 'NULL' : String(v)))
      .filter((v) => v !== 'NULL')
    const keyword = options.candidateFilter().trim().toLowerCase()
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

  function filterModeOptions() {
    const col = currentFilterColumnName()
    return FILTER_OP_ORDER.map((op) => ({ op, label: conditionPreview(col, op, '..') }))
  }

  function currentFilterColumnName(): string {
    return options.result()?.columns?.[options.filterPanel().colIndex]?.name ?? ''
  }

  function buildConditionLiteral(column: string, op: FilterOp, value: unknown): string {
    return `${column} ${opSymbol(op)} ${sqlLiteral(value)}`
  }

  function applyFilterPanel() {
    const res = options.result()
    const col = currentFilterColumnName()
    if (!res || !col) {
      closeFilterPanel()
      return
    }
    const value = options.cellValueAt(options.filterPanel().rowIndex, options.filterPanel().colIndex)
    closeFilterPanel()
    const sql = appendWhere(options.availableSql(), [buildConditionLiteral(col, options.filterOp(), value)])
    if (sql) options.onRerun(sql)
  }

  function applyCandidate(picked: { value: string }) {
    const col = currentFilterColumnName()
    if (!col) return
    closeFilterPanel()
    const sql = appendWhere(options.availableSql(), [buildConditionLiteral(col, FILTER_EQ, picked.value)])
    if (sql) options.onRerun(sql)
  }

  return {
    filterOpen,
    filterConditions,
    filterBuildError,
    filterHasActive,
    activeFilterCount,
    filterCandidates,
    openFilterPanel,
    closeFilterPanel,
    resetFilterPanel,
    clearFilters,
    applyFilters,
    filterModeOptions,
    currentFilterColumnName,
    applyFilterPanel,
    applyCandidate,
    FILTER_PANEL_COL_INDEX_DEFAULT,
  }
}