<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { api } from '../api'
import { buildCreateTableSQL, CLICKHOUSE_ENGINES, CLICKHOUSE_TYPES, type ColumnDef } from '../utils/ddl'

const props = defineProps<{
  visible: boolean
  database: string
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'created'): void
}>()

const { t } = useI18n()
const store = useAppStore()

const table = ref('')
const tableComment = ref('')
const engine = ref('MergeTree')
const orderBy = ref('')
const partitionBy = ref('')
const primaryKey = ref('')
const settings = ref('')
const columns = ref<ColumnDef[]>([
  { name: '', type: 'String', default: '', comment: '' },
])

const engineNeedsOrderBy = computed(() => {
  const mergetreeFamilies = [
    'MergeTree', 'ReplacingMergeTree', 'ReplacingMergeTree(version)',
    'CollapsingMergeTree(sign)', 'SummingMergeTree',
    'AggregatingMergeTree', 'VersionedCollapsingMergeTree(sign, version)',
  ]
  return mergetreeFamilies.includes(engine.value)
})

const sqlPreview = computed(() => {
  return buildCreateTableSQL({
    database: props.database,
    table: table.value,
    comment: tableComment.value,
    engine: engine.value,
    orderBy: orderBy.value,
    partitionBy: partitionBy.value,
    primaryKey: primaryKey.value,
    settings: settings.value,
    columns: columns.value,
  })
})

const error = ref('')
const loading = ref(false)

function addColumn() {
  columns.value.push({ name: '', type: 'String', default: '', comment: '' })
}

function removeColumn(idx: number) {
  if (columns.value.length > 1) {
    columns.value.splice(idx, 1)
  }
}

function moveColumnUp(idx: number) {
  if (idx > 0) {
    const arr = columns.value
    ;[arr[idx - 1], arr[idx]] = [arr[idx], arr[idx - 1]]
    columns.value = [...arr]
  }
}

function moveColumnDown(idx: number) {
  if (idx < columns.value.length - 1) {
    const arr = columns.value
    ;[arr[idx + 1], arr[idx]] = [arr[idx], arr[idx + 1]]
    columns.value = [...arr]
  }
}

function resetForm() {
  table.value = ''
  tableComment.value = ''
  engine.value = 'MergeTree'
  orderBy.value = ''
  partitionBy.value = ''
  primaryKey.value = ''
  settings.value = ''
  columns.value = [{ name: '', type: 'String', default: '', comment: '' }]
  error.value = ''
}

watch(() => props.visible, (v) => {
  if (v) {
    resetForm()
  }
})

async function createTable() {
  if (!store.activeConnectionId) return
  if (!table.value.trim()) {
    error.value = t('editor.tableName') + ' ' + t('common.unknown')
    return
  }
  if (columns.value.length === 0 || !columns.value[0].name.trim()) {
    error.value = t('editor.noColumns')
    return
  }

  loading.value = true
  error.value = ''
  try {
    const sql = sqlPreview.value
    const r = await api.Execute(store.activeConnectionId, sql)
    if (r.error) {
      error.value = r.error
    } else {
      emit('created')
      emit('close')
    }
  } catch (e: any) {
    error.value = e?.message || 'Create table failed'
  } finally {
    loading.value = false
  }
}

function toEditor() {
  emit('close')
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="fixed inset-0 z-[2000] flex items-center justify-center"
      @click.self="emit('close')"
    >
      <div class="absolute inset-0 bg-black/50"></div>
      <div class="relative bg-ch-panel border border-ch-border rounded-lg shadow-2xl w-[900px] max-w-[95vw] max-h-[90vh] flex flex-col">
        <div class="flex items-center justify-between px-4 py-3 border-b border-ch-border shrink-0">
          <h3 class="text-sm font-semibold text-ch-text flex items-center gap-2">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2"/><line x1="3" y1="9" x2="21" y2="9"/><line x1="3" y1="15" x2="21" y2="15"/></svg>
            {{ t('editor.createTableTitle') }}: <span class="text-ch-accent">{{ database }}</span>
          </h3>
          <button @click="emit('close')" class="p-1 rounded hover:bg-ch-hover text-ch-muted hover:text-ch-text">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </div>

        <div class="flex-1 overflow-auto p-4 space-y-4 min-h-0">
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="text-xs text-ch-muted block mb-1">{{ t('editor.tableName') }} *</label>
              <input v-model="table" class="w-full" placeholder="my_table" />
            </div>
            <div>
              <label class="text-xs text-ch-muted block mb-1">{{ t('editor.tableComment') }}</label>
              <input v-model="tableComment" class="w-full" placeholder="optional" />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="text-xs text-ch-muted block mb-1">{{ t('editor.engine') }} *</label>
              <select v-model="engine" class="w-full">
                <option v-for="e in CLICKHOUSE_ENGINES" :key="e" :value="e">{{ e }}</option>
              </select>
            </div>
            <div>
              <label class="text-xs text-ch-muted block mb-1">{{ t('editor.orderBy') }} <span v-if="engineNeedsOrderBy" class="text-red-400">*</span></label>
              <input v-model="orderBy" class="w-full" placeholder="col1, col2" />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="text-xs text-ch-muted block mb-1">{{ t('editor.partitionBy') }}</label>
              <input v-model="partitionBy" class="w-full" placeholder="toYYYYMM(date)" />
            </div>
            <div>
              <label class="text-xs text-ch-muted block mb-1">{{ t('editor.primaryKey') }}</label>
              <input v-model="primaryKey" class="w-full" placeholder="col1" />
            </div>
          </div>

          <div>
            <label class="text-xs text-ch-muted block mb-1">{{ t('editor.settings') }}</label>
            <input v-model="settings" class="w-full" placeholder="index_granularity = 8192" />
          </div>

          <div>
            <div class="flex items-center justify-between mb-2">
              <label class="text-xs font-semibold text-ch-text">{{ t('editor.columns') }} ({{ columns.length }})</label>
              <button @click="addColumn" class="btn-ghost btn-sm flex items-center gap-1">
                <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
                {{ t('editor.addColumn') }}
              </button>
            </div>

            <div class="bg-ch-hover/40 rounded border border-ch-border">
              <div class="grid grid-cols-[20px_1fr_1fr_1fr_1fr_80px] gap-2 px-3 py-1.5 text-[10px] font-semibold text-ch-muted border-b border-ch-border bg-ch-panel rounded-t">
                <div></div>
                <div>{{ t('editor.columnName') }}</div>
                <div>{{ t('editor.columnType') }}</div>
                <div>{{ t('editor.columnDefault') }}</div>
                <div>{{ t('editor.columnComment') }}</div>
                <div></div>
              </div>
              <div v-for="(col, idx) in columns" :key="idx" class="grid grid-cols-[20px_1fr_1fr_1fr_1fr_80px] gap-2 px-3 py-1.5 items-center border-b border-ch-border/30 last:border-b-0">
                <div class="flex flex-col gap-0.5">
                  <button @click="moveColumnUp(idx)" class="text-ch-muted hover:text-ch-accent p-0 leading-none" :disabled="idx === 0" :title="t('editor.moveUp')">
                    <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="18 15 12 9 6 15"/></svg>
                  </button>
                  <button @click="moveColumnDown(idx)" class="text-ch-muted hover:text-ch-accent p-0 leading-none" :disabled="idx === columns.length - 1" :title="t('editor.moveDown')">
                    <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"/></svg>
                  </button>
                </div>
                <input v-model="col.name" class="text-xs px-2 py-1" placeholder="id" />
                <select v-model="col.type" class="text-xs px-2 py-1">
                  <option v-for="t in CLICKHOUSE_TYPES" :key="t" :value="t">{{ t }}</option>
                </select>
                <input v-model="col.default" class="text-xs px-2 py-1" placeholder="0" />
                <input v-model="col.comment" class="text-xs px-2 py-1" placeholder="optional" />
                <button @click="removeColumn(idx)" class="text-red-400 hover:text-red-300 text-xs" :disabled="columns.length <= 1">
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2m3 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/></svg>
                </button>
              </div>
            </div>
          </div>

          <div>
            <label class="text-xs font-semibold text-ch-text block mb-2">SQL Preview</label>
            <pre class="bg-[#1d1e22] border border-ch-border rounded p-3 text-xs font-mono text-ch-text whitespace-pre-wrap break-all max-h-[160px] overflow-auto">{{ sqlPreview }}</pre>
          </div>

          <div v-if="error" class="bg-red-900/30 border border-red-700 rounded p-3 text-red-300 text-xs font-mono whitespace-pre-wrap">{{ error }}</div>
        </div>

        <div class="flex items-center justify-end gap-2 px-4 py-3 border-t border-ch-border shrink-0">
          <button @click="emit('close')" class="btn-ghost btn-sm">{{ t('common.close') }}</button>
          <button @click="createTable" class="btn-primary btn-sm flex items-center gap-1" :disabled="loading">
            <svg v-if="loading" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="animate-spin"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
            {{ loading ? t('editor.loading') : t('editor.create') }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>