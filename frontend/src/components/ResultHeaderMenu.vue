<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  visible: boolean
  x: number
  y: number
  columnName: string
  canExplain: boolean
  canEdit: boolean
  userKeyIndex: number
  isKey: boolean
  editKeyHint: string
  hiddenCount: number
}>()

const emit = defineEmits<{
  (e: 'copy-column-name'): void
  (e: 'copy-all-columns'): void
  (e: 'copy-column-values', distinct: boolean): void
  (e: 'search-only'): void
  (e: 'sort', dir: 'ASC' | 'DESC'): void
  (e: 'filter-by-value'): void
  (e: 'toggle-key'): void
  (e: 'hide-column'): void
  (e: 'hide-column-sql'): void
  (e: 'show-all-columns'): void
}>()

const { t } = useI18n()
</script>

<template>
  <Teleport to="body">
    <div
      v-if="props.visible"
      class="fixed z-[3000] bg-ch-panel border border-ch-border rounded shadow-lg py-1 text-xs text-ch-text min-w-[200px]"
      :style="{ left: props.x + 'px', top: props.y + 'px' }"
      @click.stop
    >
      <button @click="emit('copy-column-name')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
        <span>{{ t('editor.copyColumnName') }}</span>
      </button>
      <button @click="emit('copy-all-columns')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
        <span>{{ t('editor.copyColumns') }}</span>
      </button>
      <button @click="emit('copy-column-values', false)" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="3" y1="8" x2="21" y2="8"/><line x1="3" y1="16" x2="21" y2="16"/></svg>
        <span>{{ t('editor.copyColumnValues') }}</span>
      </button>
      <button @click="emit('copy-column-values', true)" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="3" y1="8" x2="21" y2="8"/><line x1="3" y1="16" x2="21" y2="16"/></svg>
        <span>{{ t('editor.copyColumnValuesDistinct') }}</span>
      </button>
      <div class="border-t border-ch-border/50 my-1"></div>
      <button @click="emit('search-only')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
        <span>{{ t('editor.searchOnlyThisColumn') }}</span>
      </button>
      <button v-if="props.canExplain" @click="emit('sort', 'ASC')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="19" x2="12" y2="5"/><polyline points="5 12 12 5 19 12"/></svg>
        <span>{{ t('editor.orderByAsc', { column: props.columnName }) }}</span>
      </button>
      <button v-if="props.canExplain" @click="emit('sort', 'DESC')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><polyline points="19 12 12 19 5 12"/></svg>
        <span>{{ t('editor.orderByDesc', { column: props.columnName }) }}</span>
      </button>
      <template v-if="props.canExplain">
        <div class="border-t border-ch-border/50 my-1"></div>
        <button @click="emit('filter-by-value')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/></svg>
          <span>{{ t('editor.filterByValue') }}</span>
        </button>
        <div class="border-t border-ch-border/50 my-1"></div>
        <div class="px-3 py-1 text-[11px] text-ch-muted leading-snug">{{ props.editKeyHint }}</div>
        <div class="border-t border-ch-border/50 my-1"></div>
        <button
          :disabled="!props.canEdit"
          @click="emit('toggle-key')"
          class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-ch-text hover:bg-ch-hover disabled:opacity-40 disabled:hover:bg-transparent"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="7.5" cy="15.5" r="3.5"/><path d="M10 12L20 2"/><line x1="16" y1="6" x2="18" y2="8"/><line x1="18" y1="8" x2="20" y2="6"/></svg>
          <span class="flex-1">{{ props.isKey && props.userKeyIndex >= 0 ? t('editor.useAsEditKeyCancel') : t('editor.useAsEditKey') }}</span>
          <svg v-if="props.isKey" width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"/></svg>
        </button>
      </template>
      <div class="border-t border-ch-border/50 my-1"></div>
      <button @click="emit('hide-column')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
        <span>{{ t('editor.hideColumn') }}</span>
      </button>
      <button
        @click="emit('hide-column-sql')"
        class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-ch-muted hover:bg-ch-hover"
        :title="t('editor.hideColumnSqlHint')"
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>
        <span>{{ t('editor.hideColumnSql') }}</span>
      </button>
      <button
        :disabled="props.hiddenCount === 0"
        @click="emit('show-all-columns')"
        class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-ch-text hover:bg-ch-hover disabled:opacity-40 disabled:hover:bg-transparent"
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
        <span class="flex-1">{{ t('editor.showAllColumns') }}</span>
        <span v-if="props.hiddenCount > 0" class="text-[11px] text-ch-muted">{{ props.hiddenCount }}</span>
      </button>
    </div>
  </Teleport>
</template>