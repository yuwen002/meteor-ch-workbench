<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { normalizeSeconds } from '../utils/autoRefresh'

const props = defineProps<{
  options: Array<{ value: number; label: string }>
  current: number
  enabled: boolean
  customVisible: boolean
  customError: string | null
  initialCustom: number
  pickerVisible?: boolean
  pickerX?: number
  pickerY?: number
}>()

const emit = defineEmits<{
  (e: 'choose', seconds: number): void
  (e: 'close-custom'): void
  (e: 'update:custom-error', value: string | null): void
  (e: 'update:custom-value', value: string): void
}>()

const { t } = useI18n()
const customValue = ref(props.initialCustom > 0 ? String(props.initialCustom) : '')

watch(
  () => props.customVisible,
  (v) => {
    if (v) customValue.value = props.initialCustom > 0 ? String(props.initialCustom) : ''
  },
)

function confirmCustom() {
  const raw = customValue.value.trim()
  if (!raw) {
    emit('update:custom-error', t('editor.refreshCustomRequired'))
    return
  }
  const seconds = normalizeSeconds(raw)
  if (seconds <= 0) {
    emit('update:custom-error', t('editor.refreshCustomInvalid'))
    return
  }
  emit('update:custom-value', String(seconds))
  emit('choose', seconds)
  emit('close-custom')
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="props.customVisible"
      class="fixed inset-0 z-[3000] bg-black/50 flex items-center justify-center"
      @click.self="emit('close-custom')"
    >
      <div class="bg-ch-panel border border-ch-border rounded shadow-xl w-[340px]">
        <div class="flex items-center justify-between px-4 py-2.5 border-b border-ch-border">
          <span class="text-sm font-semibold text-ch-text">{{ t('editor.refreshCustomTitle') }}</span>
          <button @click="emit('close-custom')" class="text-ch-muted hover:text-ch-text" :title="t('common.cancel')">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </div>
        <div class="px-4 py-3 space-y-2">
          <label class="block text-xs text-ch-muted">{{ t('editor.refreshCustomLabel') }}</label>
          <input
            v-model="customValue"
            type="number"
            min="1"
            class="w-full bg-ch-bg border border-ch-border rounded px-2 py-1.5 text-sm text-ch-text outline-none focus:border-ch-accent"
            :placeholder="t('editor.refreshCustomPlaceholder')"
            @keydown.enter.prevent="confirmCustom"
          />
          <p class="text-[11px] text-ch-muted">{{ t('editor.refreshCustomHint') }}</p>
          <p v-if="props.customError" class="text-[11px] text-red-400">{{ props.customError }}</p>
        </div>
        <div class="flex items-center justify-end gap-2 px-4 py-3 border-t border-ch-border">
          <button @click="emit('close-custom')" class="px-3 py-1 rounded border border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover text-xs">
            {{ t('common.cancel') }}
          </button>
          <button @click="confirmCustom" class="px-3 py-1 rounded bg-ch-accent text-white text-xs">
            {{ t('editor.refreshCustomConfirm') }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>

  <Teleport to="body">
    <div
      v-if="props.customVisible === false && props.pickerVisible"
      class="fixed z-[3000] bg-ch-panel border border-ch-border rounded shadow-lg py-1 text-xs text-ch-text min-w-[140px]"
      :style="{ left: (props.pickerX ?? 0) + 'px', top: (props.pickerY ?? 0) + 'px' }"
      @click.stop
    >
      <button
        @click="emit('choose', 0)"
        class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover"
      >
        <span class="flex-1">{{ t('editor.refreshOff') }}</span>
        <svg v-if="!props.enabled" width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"/></svg>
      </button>
      <button
        v-for="o in props.options"
        :key="o.value"
        @click="emit('choose', o.value)"
        class="flex items-center gap-2 w-full text-left px-3 py-1.5 hover:bg-ch-hover"
      >
        <span class="flex-1">{{ o.label }}</span>
        <svg v-if="props.current === o.value" width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"/></svg>
      </button>
    </div>
  </Teleport>
</template>