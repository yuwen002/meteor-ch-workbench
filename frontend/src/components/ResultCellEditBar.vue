<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  count: number
  querying: boolean
}>()

const emit = defineEmits<{
  (e: 'fill'): void
  (e: 'discard'): void
  (e: 'apply'): void
}>()

const { t } = useI18n()
</script>

<template>
  <div v-if="count > 0" class="flex items-center gap-2 px-3 py-1 border-t border-ch-border bg-amber-500/10 text-[11px] shrink-0">
    <span class="text-amber-300">{{ t('editor.pendingEdits', { count }) }}</span>
    <div class="flex-1"></div>
    <button
      @click="emit('fill')"
      :disabled="querying"
      class="px-2 py-0.5 rounded border border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover disabled:opacity-50 disabled:cursor-not-allowed"
    >
      {{ t('editor.fillToEditor') }}
    </button>
    <button
      @click="emit('discard')"
      :disabled="querying"
      class="px-2 py-0.5 rounded border border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover disabled:opacity-50 disabled:cursor-not-allowed"
    >
      {{ t('editor.discardEdits') }}
    </button>
    <button
      @click="emit('apply')"
      :disabled="querying"
      class="px-2 py-0.5 rounded bg-ch-accent text-white hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
    >
      {{ querying ? t('editor.applyingEdits') : t('editor.applyEdits') }}
    </button>
  </div>
</template>