<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  visible: boolean
  x: number
  y: number
  columnName: string
  canExplain: boolean
}>()

const emit = defineEmits<{
  (e: 'copy-cell'): void
  (e: 'copy-cell-literal'): void
  (e: 'copy-row'): void
  (e: 'copy-insert'): void
  (e: 'copy-insert-all'): void
  (e: 'sort', dir: 'ASC' | 'DESC'): void
  (e: 'filter-by-value'): void
}>()

const { t } = useI18n()
</script>

<template>
  <Teleport to="body">
    <div
      v-if="props.visible"
      class="fixed z-[3000] bg-ch-panel border border-ch-border rounded shadow-lg py-1 text-xs text-ch-text min-w-[180px]"
      :style="{ left: props.x + 'px', top: props.y + 'px' }"
      @click.stop
    >
      <button @click="emit('copy-cell')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
        <span>{{ t('editor.copyCell') }}</span>
      </button>
      <button @click="emit('copy-cell-literal')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 7 4 4 20 4 20 7"/><line x1="9" y1="20" x2="15" y2="20"/><line x1="12" y1="4" x2="12" y2="20"/></svg>
        <span>{{ t('editor.copyCellLiteral') }}</span>
      </button>
      <div class="border-t border-ch-border/50 my-1"></div>
      <button @click="emit('copy-row')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="3" y1="8" x2="21" y2="8"/><line x1="3" y1="16" x2="21" y2="16"/></svg>
        <span>{{ t('editor.copyRow') }}</span>
      </button>
      <button @click="emit('copy-insert')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
        <span>{{ t('editor.copyInsert') }}</span>
      </button>
      <button @click="emit('copy-insert-all')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
        <span>{{ t('editor.copyInsertAll') }}</span>
      </button>
      <div v-if="props.canExplain" class="border-t border-ch-border/50 my-1"></div>
      <button v-if="props.canExplain" @click="emit('sort', 'ASC')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="19" x2="12" y2="5"/><polyline points="5 12 12 5 19 12"/></svg>
        <span>{{ t('editor.orderByAsc', { column: props.columnName }) }}</span>
      </button>
      <button v-if="props.canExplain" @click="emit('sort', 'DESC')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><polyline points="19 12 12 19 5 12"/></svg>
        <span>{{ t('editor.orderByDesc', { column: props.columnName }) }}</span>
      </button>
      <button v-if="props.canExplain" @click="emit('filter-by-value')" class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/></svg>
        <span>{{ t('editor.filterByValue') }}</span>
      </button>
    </div>
  </Teleport>
</template>