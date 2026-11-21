import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { QueryResult } from '../types'
import {
  buildAlterUpdateSQL,
  buildRowKeys,
  detectKeyColumns,
  describeKeyStrategy,
  parseQuerySource,
  valueForEdit,
  type AutoKeyResult,
} from '../utils/cellEdit'

export interface PendingEdit {
  rowIndex: number
  colIndex: number
  value: string
  sql: string
}

export interface ApplyResult {
  ok: number
  error: string
  failedIndexes?: number[]
  skippedIndexes?: number[]
}

export interface UseCellEditingOptions {
  result: () => QueryResult | null
  availableSql: () => string
  executedSql: () => string | undefined
  applyResult: () => ApplyResult | null
  onError: (message: string) => void
  onApplyEdits: (statements: string[]) => void
  onEditSql: (sql: string) => void
}

export function useCellEditing(options: UseCellEditingOptions) {
  const { t } = useI18n()

  const editingCell = ref<{ rowIndex: number; colIndex: number } | null>(null)
  const editingValue = ref('')
  const pendingEdits = ref<PendingEdit[]>([])
  const applyConfirmVisible = ref(false)
  const userKeyIndex = ref(-1)
  const autoKey = ref<AutoKeyResult>({ indexes: [], reason: 'all' })
  let lastAppliedEdits: PendingEdit[] = []

  const editSource = computed(() => parseQuerySource(options.availableSql() || options.executedSql() || ''))
  const kcols = computed(() => options.result()?.columns ?? [])

  const keyIndexes = computed<number[]>(() =>
    userKeyIndex.value >= 0 ? [userKeyIndex.value] : autoKey.value.indexes,
  )

  const keyStrategy = computed(() =>
    describeKeyStrategy(
      keyIndexes.value,
      autoKey.value,
      kcols.value,
      userKeyIndex.value >= 0 ? kcols.value[userKeyIndex.value]?.name ?? null : null,
    ),
  )

  const editKeyHint = computed(() => {
    const s = keyStrategy.value
    if (s.mode === 'none') return t('editor.editAutoNoKey')
    if (s.mode === 'manual') return t('editor.editKeyManual', { column: s.columnNames.join(', ') })
    if (s.mode === 'primary') return t('editor.editKeyPrimary', { count: s.indexes.length })
    if (s.mode === 'unique') return t('editor.editKeyUnique', { column: s.columnNames[s.columnNames.length - 1] ?? '' })
    return t('editor.editKeyAll', { count: s.indexes.length })
  })

  const canEditResult = computed(() => editSource.value !== null)
  const pendingCount = computed(() => pendingEdits.value.length)
  const applyStatements = computed(() => pendingEdits.value.map((p) => p.sql))
  const applyTargetLabel = computed(() => {
    const src = editSource.value
    if (!src) return ''
    return src.database ? `${src.database}.${src.table}` : src.table
  })

  watch(
    () => options.result(),
    () => {
      const cols = options.result()?.columns ?? []
      const rows = options.result()?.rows ?? []
      autoKey.value = detectKeyColumns({ columns: cols, rows })
      userKeyIndex.value = -1
      editingCell.value = null
      pendingEdits.value = []
      applyConfirmVisible.value = false
    },
    { immediate: true },
  )

  watch(
    () => options.applyResult(),
    (res) => {
      if (!res) return
      if (res.error && lastAppliedEdits.length > 0) {
        const keep = new Set<number>([...(res.failedIndexes ?? []), ...(res.skippedIndexes ?? [])])
        if (keep.size > 0) {
          pendingEdits.value = lastAppliedEdits.filter((_, i) => keep.has(i))
        } else if (res.ok >= 0) {
          pendingEdits.value = lastAppliedEdits.slice(res.ok)
        }
      }
      lastAppliedEdits = []
    },
  )

  function startEditCell(rowIndex: number, colIndex: number) {
    const rows = options.result()?.rows ?? []
    if (rowIndex < 0 || rowIndex >= rows.length) return
    editingCell.value = { rowIndex, colIndex }
    editingValue.value = valueForEdit(rows[rowIndex][colIndex])
  }

  function cancelEditCell() {
    editingCell.value = null
    editingValue.value = ''
  }

  function setKeyColumn(colIndex: number) {
    userKeyIndex.value = userKeyIndex.value === colIndex ? -1 : colIndex
  }

  function isPendingEdited(rowIndex: number, colIndex: number): boolean {
    return pendingEdits.value.some((p) => p.rowIndex === rowIndex && p.colIndex === colIndex)
  }

  function pendingValue(rowIndex: number, colIndex: number): string {
    return pendingEdits.value.find((p) => p.rowIndex === rowIndex && p.colIndex === colIndex)?.value ?? ''
  }

  function commitEditCell() {
    const cell = editingCell.value
    const r = options.result()
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
      options.onError(t('editor.editNoKey'))
      return
    }
    const row = r.rows?.[cell.rowIndex] ?? []
    const keys = buildRowKeys(idxs, r.columns ?? [], row)
    if (keys.length === 0) {
      options.onError(t('editor.editNoKey'))
      return
    }
    if (keys.some((k) => k.value === null || k.value === undefined)) {
      options.onError(t('editor.editKeyNull'))
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
      options.onError(t('editor.editNoSource'))
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
    applyConfirmVisible.value = true
  }

  function cancelApplyEdits() {
    applyConfirmVisible.value = false
  }

  function confirmApplyEdits() {
    if (pendingEdits.value.length === 0) {
      applyConfirmVisible.value = false
      return
    }
    lastAppliedEdits = pendingEdits.value.slice()
    options.onApplyEdits(pendingEdits.value.map((p) => p.sql))
    pendingEdits.value = []
    applyConfirmVisible.value = false
  }

  async function copyApplyStatements(copyText: (text: string, hint: string) => Promise<void>) {
    const text = applyStatements.value.join('\n')
    if (!text) return
    await copyText(text, t('editor.copyEditsSql'))
  }

  function discardPendingEdits() {
    pendingEdits.value = []
  }

  function fillPendingEditsToEditor() {
    if (pendingEdits.value.length === 0) return
    options.onEditSql(pendingEdits.value.map((p) => p.sql).join('\n'))
    pendingEdits.value = []
  }

  return {
    editingCell,
    editingValue,
    pendingEdits,
    applyConfirmVisible,
    userKeyIndex,
    autoKey,
    editSource,
    keyIndexes,
    keyStrategy,
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
    copyApplyStatements,
    discardPendingEdits,
    fillPendingEditsToEditor,
  }
}