<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { QueryResult } from '../types'

const props = defineProps<{
  result: QueryResult | null
  executedSql: string | null
  executedSqlModified: boolean
  loadingWithResult: boolean
  refreshEnabled: boolean
  refreshRemaining: number
  copiedHint: string | null
  searchText: string
  searchActive: boolean
  filteredShown: number
}>()

const emit = defineEmits<{
  (e: 'toggle-sql'): void
  (e: 'update:search-text', value: string): void
  (e: 'clear-search'): void
}>()

const { t } = useI18n()

const sqlButtonWrap = ref<HTMLElement | null>(null)

defineExpose({ sqlButtonWrap })

function onSearchInput(e: Event) {
  emit('update:search-text', (e.target as HTMLInputElement).value)
}
</script>

<template>
  <div class="flex items-center gap-2 shrink-0 whitespace-nowrap">
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2"/><line x1="3" y1="9" x2="21" y2="9"/><line x1="3" y1="15" x2="21" y2="15"/><line x1="9" y1="3" x2="9" y2="21"/></svg>
    <span class="font-semibold">{{ t('editor.results') }}</span>
    <template v-if="props.result">
      <span class="text-ch-text">{{ props.result.row_count.toLocaleString() }} {{ t('editor.rows') }}</span>
      <span>·</span>
      <span class="text-ch-text">{{ props.result.elapsed_ms?.toFixed(2) }} ms</span>
    </template>
    <div v-if="props.executedSql" ref="sqlButtonWrap" class="relative shrink-0">
      <button
        @click="emit('toggle-sql')"
        class="flex items-center gap-1 px-2 py-0.5 rounded border border-ch-border text-ch-muted hover:text-ch-accent hover:border-ch-accent"
      >
        <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>
        <span>{{ t('editor.viewSql') }}</span>
        <span v-if="props.executedSqlModified" class="w-1.5 h-1.5 rounded-full bg-ch-accent" :title="t('editor.executedSqlModified')"></span>
      </button>
    </div>
    <span v-if="props.loadingWithResult" class="text-ch-accent">{{ t('editor.refreshing') }}</span>
    <span v-else-if="props.refreshEnabled" class="text-ch-muted" :title="t('editor.refreshHint')">{{ t('editor.refreshCountdown', { seconds: props.refreshRemaining }) }}</span>
    <span v-if="props.copiedHint" class="text-ch-accent">{{ props.copiedHint }}</span>
    <div class="relative shrink-0">
      <input
        :value="props.searchText"
        @input="onSearchInput"
        type="text"
        :placeholder="t('editor.searchPlaceholder')"
        class="w-[220px] bg-ch-bg border border-ch-border rounded px-2 py-0.5 text-xs text-ch-text outline-none focus:border-ch-accent"
      />
      <button
        v-if="props.searchText"
        @click="emit('clear-search')"
        class="absolute right-1 top-1/2 -translate-y-1/2 text-ch-muted hover:text-ch-text"
        :title="t('editor.clearSearch')"
      >
        <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
      </button>
    </div>
    <span v-if="props.searchActive && props.result" class="text-ch-accent shrink-0 whitespace-nowrap">
      {{ t('editor.filteredCount', { shown: props.filteredShown, total: props.result.rows?.length ?? 0 }) }}
    </span>
  </div>
</template>