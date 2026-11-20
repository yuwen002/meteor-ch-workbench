<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { SavedSQL } from '../types'

const props = defineProps<{
  visible: boolean
  connectionId?: string
}>()

const emit = defineEmits<{
  (e: 'select', sql: string): void
  (e: 'close'): void
  (e: 'error', message: string): void
}>()

const { t } = useI18n()

const entries = ref<SavedSQL[]>([])
const loading = ref(false)
const keyword = ref('')
const renamingId = ref<string | null>(null)
const renamingName = ref('')

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return entries.value
  return entries.value.filter(
    (e) => e.name.toLowerCase().includes(kw) || e.sql.toLowerCase().includes(kw)
  )
})

async function reload() {
  loading.value = true
  try {
    entries.value = await api.GetSavedSQL()
  } catch {
    entries.value = []
  } finally {
    loading.value = false
  }
}

async function remove(entry: SavedSQL, e: MouseEvent) {
  e.stopPropagation()
  if (!confirm(t('editor.savedDeleteConfirm').replace('{name}', entry.name))) return
  try {
    await api.DeleteSavedSQL(entry.id)
    entries.value = entries.value.filter((s) => s.id !== entry.id)
  } catch (err: any) {
    emit('error', String(err?.message || err))
  }
}

function startRename(entry: SavedSQL, e: MouseEvent) {
  e.stopPropagation()
  renamingId.value = entry.id
  renamingName.value = entry.name
}

async function commitRename() {
  const id = renamingId.value
  const name = renamingName.value.trim()
  renamingId.value = null
  if (!id || !name) return
  const entry = entries.value.find((s) => s.id === id)
  if (!entry || entry.name === name) return
  try {
    const updated = await api.UpdateSavedSQL(id, name, entry.sql)
    entries.value = entries.value.map((s) => (s.id === id ? updated : s))
  } catch (err: any) {
    emit('error', String(err?.message || err))
  }
}

function select(entry: SavedSQL) {
  emit('select', entry.sql)
}

function formatTime(ts: string): string {
  const d = new Date(ts)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

watch(
  () => props.visible,
  async (v) => {
    if (v) await reload()
  },
  { immediate: true }
)

defineExpose({ reload })
</script>

<template>
  <div v-if="visible" class="h-[28%] min-h-[140px] border-y border-ch-border bg-ch-panel flex flex-col">
    <div class="flex items-center gap-2 px-3 py-1.5 border-b border-ch-border text-xs text-ch-muted shrink-0">
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2z"/></svg>
      <span class="font-semibold">{{ t('editor.savedSQL') }}</span>
      <span v-if="!loading" class="text-ch-text">{{ entries.length }}</span>
      <input
        v-model="keyword"
        type="text"
        class="ml-2 bg-ch-bg border border-ch-border rounded px-1.5 py-0.5 text-[11px] text-ch-text outline-none focus:border-ch-accent w-40"
        :placeholder="t('editor.savedSearchPlaceholder')"
      />
      <div class="ml-auto flex items-center gap-1">
        <button class="px-2 py-0.5 rounded hover:bg-ch-hover hover:text-ch-text" :title="t('editor.refresh')" @click="reload">
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" :class="{ 'animate-spin': loading }"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
        </button>
        <button class="px-2 py-0.5 rounded hover:bg-ch-hover hover:text-ch-text" :title="t('common.close')" @click="emit('close')">
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
        </button>
      </div>
    </div>
    <div class="flex-1 overflow-auto">
      <div v-if="loading" class="px-3 py-2 text-xs text-ch-muted italic">{{ t('editor.loading') }}</div>
      <div v-else-if="filtered.length === 0" class="px-3 py-2 text-xs text-ch-muted italic">
        {{ entries.length === 0 ? t('editor.savedEmpty') : t('editor.savedNoMatch') }}
      </div>
      <div
        v-for="s in filtered"
        :key="s.id"
        class="group flex items-start gap-2 px-3 py-1.5 border-b border-ch-border/50 cursor-pointer hover:bg-ch-hover"
        @click="select(s)"
      >
        <svg class="mt-0.5 shrink-0 text-ch-accent" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2z"/></svg>
        <div class="flex-1 min-w-0">
          <input
            v-if="renamingId === s.id"
            v-model="renamingName"
            class="w-full bg-ch-bg border border-ch-accent rounded px-1 text-xs text-ch-text outline-none"
            @click.stop
            @keydown.enter="commitRename"
            @keydown.esc="renamingId = null"
            @blur="commitRename"
            v-focus
          />
          <div v-else class="text-xs text-ch-text truncate" :title="s.name">{{ s.name }}</div>
          <div class="text-[11px] text-ch-muted font-mono truncate whitespace-pre" :title="s.sql">{{ s.sql }}</div>
          <div class="text-[10px] text-ch-muted mt-0.5">{{ formatTime(s.updated_at) }}</div>
        </div>
        <button
          class="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-ch-border text-ch-muted hover:text-ch-text shrink-0"
          :title="t('editor.rename')"
          @click="startRename(s, $event)"
        >
          <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 20h9"/><path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z"/></svg>
        </button>
        <button
          class="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-ch-border text-ch-muted hover:text-ch-text shrink-0"
          :title="t('common.delete')"
          @click="remove(s, $event)"
        >
          <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
        </button>
      </div>
    </div>
  </div>
</template>