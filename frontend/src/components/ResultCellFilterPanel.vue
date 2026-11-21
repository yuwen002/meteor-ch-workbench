<script setup lang="ts">
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FilterOp } from '../utils/resultFilter'
import { usePanelDrag } from '../composables/usePanelDrag'

const props = defineProps<{
  visible: boolean
  x: number
  y: number
  height: string
  columnName: string
  rowIndex: number
  rowValue: unknown
  op: FilterOp
  opOptions: Array<{ op: FilterOp; label: string }>
  input: string
  error: string | null
  candidates: Array<{ value: string; label: string }>
  candidateFilter: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'apply'): void
  (e: 'apply-candidate', picked: { value: string }): void
  (e: 'update:op', value: FilterOp): void
  (e: 'update:input', value: string): void
  (e: 'update:candidate-filter', value: string): void
}>()

const { t } = useI18n()

const panelWidth = 380
const drag = usePanelDrag(() => ({ width: panelWidth, height: 360 }))
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

function onOpChange(e: Event) {
  emit('update:op', (e.target as HTMLSelectElement).value as FilterOp)
}

function onInputChange(e: Event) {
  emit('update:input', (e.target as HTMLInputElement).value)
}

function onCandidateFilterChange(e: Event) {
  emit('update:candidate-filter', (e.target as HTMLInputElement).value)
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="props.visible"
      class="fixed z-[3000] bg-ch-panel border border-ch-border rounded shadow-lg text-xs flex flex-col"
      :style="{ left: pos.x + 'px', top: pos.y + 'px', width: '380px', maxHeight: props.height }"
      @click.stop
      @pointermove="drag.moveDrag($event)"
      @pointerup="drag.endDrag($event)"
      @pointercancel="drag.endDrag($event)"
      @keydown.esc="emit('close')"
    >
      <div
        class="flex items-center justify-between px-3 py-1.5 border-b border-ch-border shrink-0 cursor-move select-none"
        @pointerdown="onHeaderPointerDown"
      >
        <span class="font-semibold text-ch-text truncate">{{ t('editor.filterByValue') }} · {{ props.columnName }}</span>
        <button @pointerdown.stop @click="emit('close')" class="text-ch-muted hover:text-ch-text cursor-pointer">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
        </button>
      </div>
      <div class="p-3 space-y-2 overflow-auto">
        <template v-if="props.rowIndex < 0">
          <input
            :value="props.candidateFilter"
            @input="onCandidateFilterChange"
            type="text"
            :placeholder="t('editor.filterCandidatePlaceholder')"
            class="w-full bg-ch-bg border border-ch-border rounded px-2 py-1 text-xs text-ch-text outline-none focus:border-ch-accent"
          />
          <div v-if="props.candidates.length" class="border border-ch-border rounded max-h-[180px] overflow-auto">
            <button
              v-for="c in props.candidates"
              :key="c.value"
              @click="emit('apply-candidate', c)"
              class="block w-full text-left px-2 py-1 text-ch-text hover:bg-ch-hover truncate"
            >
              {{ c.label }}
            </button>
          </div>
          <div v-else class="text-ch-muted px-1">{{ t('editor.filterCandidateEmpty') }}</div>
          <div class="border-t border-ch-border/50 pt-2">
            <div class="text-ch-muted mb-1">{{ t('editor.customFilter') }}</div>
          </div>
        </template>
        <div class="flex items-center gap-2">
          <select
            :value="props.op"
            @change="onOpChange"
            class="flex-1 bg-ch-bg border border-ch-border rounded px-2 py-1 text-xs text-ch-text outline-none focus:border-ch-accent"
          >
            <option v-for="o in props.opOptions" :key="o.op" :value="o.op">{{ o.label }}</option>
          </select>
        </div>
        <input
          :value="props.input"
          @input="onInputChange"
          type="text"
          :placeholder="props.rowIndex >= 0 ? String(props.rowValue ?? '') : t('editor.filterValuePlaceholder')"
          class="w-full bg-ch-bg border border-ch-border rounded px-2 py-1 text-xs text-ch-text outline-none focus:border-ch-accent"
          @keydown.enter.prevent="emit('apply')"
        />
        <div v-if="props.error" class="text-red-400">{{ props.error }}</div>
      </div>
      <div class="flex items-center justify-end gap-2 px-3 py-2 border-t border-ch-border shrink-0">
        <button @click="emit('close')" class="px-2 py-1 rounded border border-ch-border text-ch-muted hover:text-ch-text">
          {{ t('common.cancel') }}
        </button>
        <button
          @click="emit('apply')"
          class="px-2 py-1 rounded bg-ch-accent text-white disabled:opacity-50"
        >
          {{ t('editor.filterApply') }}
        </button>
      </div>
    </div>
  </Teleport>
</template>