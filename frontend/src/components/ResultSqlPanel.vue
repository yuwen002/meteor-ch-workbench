<script setup lang="ts">
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePanelDrag } from '../composables/usePanelDrag'

const props = defineProps<{
  visible: boolean
  x: number
  y: number
  sql: string
  modified: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'copy'): void
}>()

const { t } = useI18n()

const drag = usePanelDrag(() => ({ width: 520, height: 460 }))
const pos = computed(() => drag.currentPosition({ x: props.x, y: props.y }))

watch(
  () => props.visible,
  (v) => {
    if (!v) drag.resetDrag()
  },
)

function onHeaderPointerDown(e: PointerEvent) {
  drag.startDrag(e, { x: props.x, y: props.y })
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="props.visible"
      class="fixed z-[3000] bg-ch-panel border border-ch-border rounded shadow-lg text-xs"
      :style="{ left: pos.x + 'px', top: pos.y + 'px', width: '520px', maxWidth: '80vw' }"
      @click.stop
      @pointermove="drag.moveDrag($event)"
      @pointerup="drag.endDrag($event)"
      @pointercancel="drag.endDrag($event)"
    >
      <div
        class="flex items-center justify-between px-3 py-1.5 border-b border-ch-border cursor-move select-none"
        @pointerdown="onHeaderPointerDown"
      >
        <span class="font-semibold text-ch-text">{{ t('editor.viewSql') }}</span>
        <div class="flex items-center gap-2">
          <button @pointerdown.stop @click="emit('copy')" class="text-ch-muted hover:text-ch-accent">{{ t('editor.copySql') }}</button>
          <button @pointerdown.stop @click="emit('close')" class="text-ch-muted hover:text-ch-text cursor-pointer">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </div>
      </div>
      <div class="px-3 py-1 text-[11px] text-ch-muted border-b border-ch-border/50">
        <span v-if="props.modified">{{ t('editor.executedSqlModified') }}</span>
        <span v-else>{{ t('editor.executedSqlOriginal') }}</span>
      </div>
      <pre class="px-3 py-2 text-xs text-ch-text font-mono whitespace-pre-wrap max-h-[420px] overflow-auto">{{ props.sql }}</pre>
    </div>
  </Teleport>
</template>