<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { HistoryEntry } from '../types'

const props = defineProps<{
  visible: boolean
}>()

const emit = defineEmits<{
  (e: 'select', sql: string): void
  (e: 'close'): void
}>()

const { t } = useI18n()

const entries = ref<HistoryEntry[]>([])
const loading = ref(false)

async function reload() {
  loading.value = true
  try {
    entries.value = await api.GetQueryHistory()
  } catch {
    entries.value = []
  } finally {
    loading.value = false
  }
}

async function remove(entry: HistoryEntry, e: MouseEvent) {
  e.stopPropagation()
  try {
    await api.DeleteQueryHistory(entry.id)
    entries.value = entries.value.filter((h) => h.id !== entry.id)
  } catch {
  }
}

async function clearAll() {
  if (entries.value.length === 0) return
  if (!confirm(t('editor.clearHistoryConfirm'))) return
  try {
    await api.ClearQueryHistory()
    entries.value = []
  } catch {
  }
}

function select(entry: HistoryEntry) {
  emit('select', entry.sql)
}

function formatTime(ts: string): string {
  const d = new Date(ts)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
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
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="9"/><polyline points="12 7 12 12 15 15"/></svg>
      <span class="font-semibold">{{ t('editor.history') }}</span>
      <span v-if="!loading" class="text-ch-text">{{ entries.length }}</span>
      <div class="ml-auto flex items-center gap-1">
        <button class="px-2 py-0.5 rounded hover:bg-ch-hover hover:text-ch-text" :title="t('editor.refresh')" @click="reload">
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" :class="{ 'animate-spin': loading }"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
        </button>
        <button class="px-2 py-0.5 rounded hover:bg-ch-hover hover:text-ch-text disabled:opacity-40" :disabled="entries.length === 0" @click="clearAll">{{ t('editor.clearHistory') }}</button>
        <button class="px-2 py-0.5 rounded hover:bg-ch-hover hover:text-ch-text" :title="t('common.close')" @click="emit('close')">
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
        </button>
      </div>
    </div>
    <div class="flex-1 overflow-auto">
      <div v-if="loading" class="px-3 py-2 text-xs text-ch-muted italic">{{ t('editor.loading') }}</div>
      <div v-else-if="entries.length === 0" class="px-3 py-2 text-xs text-ch-muted italic">{{ t('editor.historyEmpty') }}</div>
      <div
        v-for="h in entries"
        :key="h.id"
        class="group flex items-start gap-2 px-3 py-1.5 border-b border-ch-border/50 cursor-pointer hover:bg-ch-hover"
        @click="select(h)"
      >
        <span class="mt-0.5 w-1.5 h-1.5 rounded-full shrink-0" :class="h.success ? 'bg-green-500' : 'bg-red-500'"></span>
        <div class="flex-1 min-w-0">
          <div class="text-xs text-ch-text font-mono truncate whitespace-pre" :title="h.sql">{{ h.sql.split('\n')[0] }}</div>
          <div class="flex items-center gap-2 mt-0.5 text-[10px] text-ch-muted">
            <span>{{ formatTime(h.timestamp) }}</span>
            <span>·</span>
            <span>{{ h.duration_ms?.toFixed(2) }} ms</span>
          </div>
        </div>
        <button
          class="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-ch-border text-ch-muted hover:text-ch-text shrink-0"
          :title="t('common.delete')"
          @click="remove(h, $event)"
        >
          <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
        </button>
      </div>
    </div>
  </div>
</template>