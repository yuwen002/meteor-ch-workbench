<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { api } from '../api'
import { buildAlterTableSQL, CLICKHOUSE_TYPES, type EditTableChange } from '../utils/ddl'
import type { ColumnInfo } from '../types'

const props = defineProps<{
  visible: boolean
  database: string
  table: string
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'altered'): void
}>()

const { t } = useI18n()
const store = useAppStore()

const loading = ref(false)
const error = ref('')
const existingColumns = ref<ColumnInfo[]>([])
const addColumns = ref<NewColumnDef[]>([])

const typeOptions = computed(() => {
  const set = new Set<string>(CLICKHOUSE_TYPES)
  for (const c of existingColumns.value) {
    if (c.type && !set.has(c.type)) set.add(c.type)
  }
  return Array.from(set)
})

interface NewColumnDef {
  name: string
  type: string
  default?: string
  comment?: string
  position: 'first' | 'after'
  afterColumn: string
}

watch(() => props.visible, async (v) => {
  if (v) {
    await loadSchema()
  }
})

async function loadSchema() {
  if (!store.activeConnectionId) return
  loading.value = true
  error.value = ''
  addColumns.value = [{ name: '', type: 'String', default: '', comment: '', position: 'after', afterColumn: '' }]
  try {
    const schema: ColumnInfo[] = await api.GetTableSchema(store.activeConnectionId, props.database, props.table)
    existingColumns.value = schema.map((c) => ({ ...c }))
    if (schema.length > 0) {
      addColumns.value[0].afterColumn = schema[schema.length - 1].name
    }
  } catch (e: any) {
    error.value = e?.message || 'Failed to load table schema'
  } finally {
    loading.value = false
  }
}

function addNewColumn() {
  const lastCol = existingColumns.value[existingColumns.value.length - 1]
  addColumns.value.push({
    name: '',
    type: 'String',
    default: '',
    comment: '',
    position: 'after',
    afterColumn: lastCol?.name || '',
  })
}

function removeAddColumn(idx: number) {
  addColumns.value.splice(idx, 1)
}

const allChanges = computed(() => {
  const changes: EditTableChange[] = []
  for (const col of existingColumns.value) {
    if ((col as any)._drop) {
      changes.push({ type: 'drop', column: (col as any)._originalName || col.name })
    } else {
      const origName = (col as any)._originalName || col.name
      const renamed = col.name !== origName
      if (renamed) {
        changes.push({ type: 'rename', column: origName, newName: col.name })
      }
      if ((col as any)._modifiedType || (col as any)._modifiedDefault || (col as any)._modifiedComment) {
        changes.push({
          type: 'modify',
          column: col.name,
          newType: col.type,
          newDefault: col.default,
          newComment: col.comment,
        })
      }
    }
  }
  for (const nc of addColumns.value) {
    if (!nc.name || !nc.type) continue
    changes.push({
      type: 'add',
      column: nc.name,
      newType: nc.type,
      newDefault: nc.default,
      newComment: nc.comment,
      position: nc.position === 'first' ? { first: true } : { after: nc.afterColumn || undefined },
    })
  }
  return changes
})

const sqlPreview = computed(() => {
  const stmts = buildAlterTableSQL(props.database, props.table, allChanges.value)
  return stmts.join(';\n') + (stmts.length ? ';' : '')
})

const hasChanges = computed(() => allChanges.value.length > 0)

async function saveChanges() {
  if (!store.activeConnectionId) return
  loading.value = true
  error.value = ''
  try {
    const stmts = buildAlterTableSQL(props.database, props.table, allChanges.value)
    let failed = false
    for (const sql of stmts) {
      const r = await api.Execute(store.activeConnectionId, sql)
      if (r.error) {
        error.value = r.error + '\n\nSQL: ' + sql
        failed = true
        break
      }
    }
    if (!failed) {
      emit('altered')
      emit('close')
    }
  } catch (e: any) {
    error.value = e?.message || 'Alter table failed'
  } finally {
    loading.value = false
  }
}

function onColumnChange(col: ColumnInfo, field: 'name' | 'type' | 'default' | 'comment') {
  if (!(col as any)._originalName) {
    ;(col as any)._originalName = col.name
    ;(col as any)._originalType = col.type
    ;(col as any)._originalDefault = col.default
    ;(col as any)._originalComment = col.comment
  }
  const orig = (col as any)
  if (field === 'type') orig._modifiedType = col.type !== orig._originalType
  if (field === 'default') orig._modifiedDefault = (col.default || '') !== (orig._originalDefault || '')
  if (field === 'comment') orig._modifiedComment = (col.comment || '') !== (orig._originalComment || '')
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
      <div class="relative bg-ch-panel border border-ch-border rounded-lg shadow-2xl w-[1000px] max-w-[95vw] max-h-[90vh] flex flex-col">
        <div class="flex items-center justify-between px-4 py-3 border-b border-ch-border shrink-0">
          <h3 class="text-sm font-semibold text-ch-text flex items-center gap-2">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"/></svg>
            {{ t('editor.editTableTitle') }}: <span class="text-ch-accent">{{ database }}.{{ table }}</span>
          </h3>
          <button @click="emit('close')" class="p-1 rounded hover:bg-ch-hover text-ch-muted hover:text-ch-text">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </div>

        <div class="flex-1 overflow-auto p-4 space-y-4 min-h-0">
          <div v-if="loading && existingColumns.length === 0" class="flex items-center justify-center h-40 text-ch-muted text-sm gap-2">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="animate-spin"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
            {{ t('editor.loading') }}
          </div>

          <div v-else-if="existingColumns.length > 0">
            <div class="text-xs font-semibold text-ch-text mb-2">
              {{ t('editor.columns') }} ({{ existingColumns.length }})
            </div>

            <div class="bg-ch-hover/40 rounded border border-ch-border overflow-hidden">
              <div class="grid grid-cols-[40px_1fr_1fr_1fr_1fr_40px] gap-2 px-3 py-1.5 text-[10px] font-semibold text-ch-muted border-b border-ch-border bg-ch-panel">
                <div class="text-center">Drop</div>
                <div>{{ t('editor.columnName') }}</div>
                <div>{{ t('editor.columnType') }}</div>
                <div>{{ t('editor.columnDefault') }}</div>
                <div>{{ t('editor.columnComment') }}</div>
                <div></div>
              </div>
              <div
                v-for="(col, idx) in existingColumns"
                :key="idx"
                class="grid grid-cols-[40px_1fr_1fr_1fr_1fr_40px] gap-2 px-3 py-1.5 items-center border-b border-ch-border/30 last:border-b-0"
                :class="{ 'opacity-40 bg-red-900/10': (col as any)._drop }"
              >
                <div class="text-center">
                  <input type="checkbox" v-model="(col as any)._drop" class="w-3 h-3" />
                </div>
                <input v-model="col.name" class="text-xs px-2 py-1" :disabled="(col as any)._drop" @input="onColumnChange(col, 'name')" />
                <select v-model="col.type" class="text-xs px-2 py-1" :disabled="(col as any)._drop" @change="onColumnChange(col, 'type')">
                  <option v-for="t in typeOptions" :key="'e-'+t" :value="t">{{ t }}</option>
                </select>
                <input v-model="col.default" class="text-xs px-2 py-1" :disabled="(col as any)._drop" @input="onColumnChange(col, 'default')" />
                <input v-model="col.comment" class="text-xs px-2 py-1" :disabled="(col as any)._drop" @input="onColumnChange(col, 'comment')" />
                <div class="text-center">
                  <span v-if="(col as any)._drop" class="text-red-400 text-[10px] font-bold">DROP</span>
                  <span v-else-if="(col as any)._originalName && (col as any)._originalName !== col.name" class="text-yellow-400 text-[10px] font-bold">REN</span>
                  <span v-else-if="(col as any)._modifiedType || (col as any)._modifiedDefault || (col as any)._modifiedComment" class="text-blue-400 text-[10px] font-bold">MOD</span>
                </div>
              </div>
            </div>
          </div>

          <div>
            <div class="flex items-center justify-between mb-2">
              <label class="text-xs font-semibold text-ch-text">{{ t('editor.addColumn') }}</label>
              <button @click="addNewColumn" class="btn-ghost btn-sm flex items-center gap-1">
                <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
                {{ t('editor.addColumn') }}
              </button>
            </div>

            <div v-if="addColumns.length === 0" class="text-xs text-ch-muted italic">
              —
            </div>
            <div v-else class="space-y-2">
              <div
                v-for="(nc, idx) in addColumns"
                :key="idx"
                class="grid grid-cols-[1fr_1fr_1fr_1fr_80px_120px_28px] gap-2 bg-ch-hover/40 rounded border border-ch-border p-2"
              >
                <input v-model="nc.name" class="text-xs px-2 py-1" placeholder="column_name" />
                <select v-model="nc.type" class="text-xs px-2 py-1">
                  <option v-for="t in CLICKHOUSE_TYPES" :key="'n-'+t" :value="t">{{ t }}</option>
                </select>
                <input v-model="nc.default" class="text-xs px-2 py-1" placeholder="default" />
                <input v-model="nc.comment" class="text-xs px-2 py-1" placeholder="comment" />
                <select v-model="nc.position" class="text-xs px-2 py-1">
                  <option value="after">AFTER</option>
                  <option value="first">FIRST</option>
                </select>
                <input
                  v-if="nc.position === 'after'"
                  v-model="nc.afterColumn"
                  class="text-xs px-2 py-1"
                  placeholder="after column"
                  list="existing-cols"
                />
                <span v-else class="text-xs text-ch-muted px-2 py-1">—</span>
                <button @click="removeAddColumn(idx)" class="text-red-400 hover:text-red-300 px-1">
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
                </button>
              </div>
            </div>
            <datalist id="existing-cols">
              <option v-for="c in existingColumns" :key="c.name" :value="c.name" />
            </datalist>
          </div>

          <div v-if="hasChanges">
            <label class="text-xs font-semibold text-ch-text block mb-2">SQL Preview ({{ allChanges.length }})</label>
            <pre class="bg-[#1d1e22] border border-ch-border rounded p-3 text-xs font-mono text-ch-text whitespace-pre-wrap break-all max-h-[200px] overflow-auto">{{ sqlPreview || '-- no changes' }}</pre>
          </div>

          <div v-if="error" class="bg-red-900/30 border border-red-700 rounded p-3 text-red-300 text-xs font-mono whitespace-pre-wrap">{{ error }}</div>
        </div>

        <div class="flex items-center justify-end gap-2 px-4 py-3 border-t border-ch-border shrink-0">
          <button @click="emit('close')" class="btn-ghost btn-sm">{{ t('common.close') }}</button>
          <button @click="saveChanges" class="btn-primary btn-sm flex items-center gap-1" :disabled="loading || !hasChanges">
            <svg v-if="loading" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="animate-spin"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
            {{ loading ? t('editor.loading') : t('editor.saveChanges') }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>