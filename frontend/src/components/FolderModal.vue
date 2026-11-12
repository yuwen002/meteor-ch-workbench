<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Folder } from '../types'

const { t } = useI18n()

const props = defineProps<{ folder: Folder | null }>()
const emit = defineEmits<{ close: []; save: [f: Folder] }>()

const form = reactive<Folder>({ id: '', name: '' })

watch(
  () => props.folder,
  (val) => {
    if (val) Object.assign(form, val)
    else Object.assign(form, { id: '', name: '' })
  },
  { immediate: true }
)

function submit() {
  if (!form.name.trim()) return
  emit('save', { ...form })
}
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50" @click.self="emit('close')">
    <div class="bg-ch-panel border border-ch-border rounded-lg w-full max-w-xs shadow-xl">
      <div class="flex items-center justify-between px-4 py-3 border-b border-ch-border">
        <h3 class="font-semibold text-ch-text text-sm">
          {{ folder ? t('folder.titleEdit') : t('folder.titleNew') }}
        </h3>
        <button @click="emit('close')" class="text-ch-muted hover:text-ch-text">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
        </button>
      </div>
      <div class="px-4 py-3">
        <input v-model="form.name" class="w-full" :placeholder="t('folder.name')" @keyup.enter="submit" />
      </div>
      <div class="flex justify-end gap-2 px-4 py-3 border-t border-ch-border">
        <button @click="emit('close')" class="btn-ghost btn-sm">{{ t('folder.cancel') }}</button>
        <button @click="submit" class="btn-primary btn-sm">{{ folder ? t('folder.rename') : t('folder.create') }}</button>
      </div>
    </div>
  </div>
</template>