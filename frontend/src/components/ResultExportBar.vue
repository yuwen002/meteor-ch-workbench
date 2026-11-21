<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ResultExportFormat } from '../types'

const props = defineProps<{
  formats: ResultExportFormat[]
  exporting: boolean
  exportedPath: string | null
  exportRows?: number
}>()

const emit = defineEmits<{
  (e: 'select', format: ResultExportFormat): void
}>()

const { t } = useI18n()
</script>

<template>
  <div class="flex items-center gap-2 px-3 py-1 border-b border-ch-border bg-ch-panel text-xs shrink-0">
    <span class="text-ch-muted">{{ t('editor.exportFormat') }}</span>
    <button
      v-for="f in props.formats"
      :key="f.id"
      @click="emit('select', f)"
      :disabled="props.exporting"
      class="px-2 py-0.5 rounded border border-ch-border text-ch-muted hover:text-ch-accent hover:border-ch-accent disabled:opacity-40"
    >
      {{ f.name }}
    </button>
    <span v-if="props.exporting" class="text-ch-muted">{{ t('editor.exportingRows', { rows: props.exportRows || 0 }) }}</span>
    <span v-else-if="props.exportedPath" class="text-ch-accent truncate">{{ t('editor.exported') }} {{ props.exportedPath }}</span>
  </div>
</template>