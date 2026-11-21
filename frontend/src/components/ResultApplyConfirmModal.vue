<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  visible: boolean
  statements: string[]
  targetTable: string
  count: number
  applying: boolean
}>()

const emit = defineEmits<{
  (e: 'cancel'): void
  (e: 'confirm'): void
  (e: 'copy'): void
  (e: 'fill'): void
}>()

const { t } = useI18n()
</script>

<template>
  <Teleport to="body">
    <div
      v-if="props.visible"
      class="fixed inset-0 z-[3000] bg-black/50 flex items-center justify-center"
      @click.self="emit('cancel')"
    >
      <div class="bg-ch-panel border border-ch-border rounded shadow-xl w-[620px] max-w-[92vw] flex flex-col" @click.stop>
        <div class="flex items-center justify-between px-4 py-2.5 border-b border-ch-border shrink-0">
          <span class="text-sm font-semibold text-ch-text">{{ t('editor.applyConfirmTitle') }}</span>
          <button @click="emit('cancel')" class="text-ch-muted hover:text-ch-text" :title="t('common.cancel')">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </div>
        <div class="px-4 py-3 space-y-2 text-xs">
          <p class="text-amber-400">{{ t('editor.applyConfirmHint') }}</p>
          <p class="text-ch-muted">
            {{ t('editor.applyConfirmCount', { count: props.count, table: props.targetTable || t('editor.editSourceUnknown') }) }}
          </p>
          <pre class="bg-ch-bg border border-ch-border rounded px-3 py-2 text-[11px] leading-relaxed text-ch-text font-mono whitespace-pre-wrap max-h-[320px] overflow-auto">{{ props.statements.join('\n') }}</pre>
        </div>
        <div class="flex items-center justify-between gap-2 px-4 py-3 border-t border-ch-border shrink-0">
          <div class="flex items-center gap-2">
            <button @click="emit('copy')" class="px-2 py-1 rounded border border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover">
              {{ t('editor.copyEditsSql') }}
            </button>
            <button @click="emit('fill')" class="px-2 py-1 rounded border border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover">
              {{ t('editor.fillToEditor') }}
            </button>
          </div>
          <div class="flex items-center gap-2">
            <button @click="emit('cancel')" class="px-3 py-1 rounded border border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover">
              {{ t('common.cancel') }}
            </button>
            <button
              @click="emit('confirm')"
              :disabled="props.applying"
              class="px-3 py-1 rounded bg-ch-accent text-white hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {{ props.applying ? t('editor.applyingEdits') : t('editor.applyConfirmExec') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>