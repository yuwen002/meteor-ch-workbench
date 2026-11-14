<script setup lang="ts">
import { reactive, ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { Connection, Folder } from '../types'

const { t } = useI18n()

const props = defineProps<{
  connection: Connection | null
  folders: Folder[]
}>()
const emit = defineEmits<{
  close: []
  save: [conn: Connection]
  delete: [id: string]
}>()

const form = reactive<Connection>({
  id: '',
  folder_id: '',
  name: '',
  host: 'localhost',
  port: 8123,
  protocol: 'http',
  user: 'default',
  password: '',
  database: '',
  secure: false,
  use_compress: true,
})

const testing = ref(false)
const testMsg = ref('')
const testOk = ref(false)
const showPassword = ref(false)

const defaultPortByProtocol = { native: 9000, http: 8123 }

watch(
  () => form.protocol,
  (val) => {
    const defaults = defaultPortByProtocol as Record<string, number>
    const prevNative = form.port === defaultPortByProtocol.native
    const prevHttp = form.port === defaultPortByProtocol.http
    if (prevNative || prevHttp) {
      form.port = defaults[val] ?? defaultPortByProtocol.http
    }
  }
)

watch(
  () => props.connection,
  (val) => {
    showPassword.value = false
    if (val) {
      Object.assign(form, val)
      if (!form.protocol) form.protocol = 'http'
    } else {
      Object.assign(form, {
        id: '', folder_id: '', name: '',
        host: 'localhost', port: 8123, protocol: 'http', user: 'default',
        password: '', database: '',
        secure: false, use_compress: true,
      })
    }
  },
  { immediate: true }
)

async function testConnection() {
  testing.value = true
  testMsg.value = ''
  try {
    await api.TestConnection({ ...form })
    testOk.value = true
    testMsg.value = t('connection.testSuccess')
  } catch (e: any) {
    testOk.value = false
    testMsg.value = e?.message || t('connection.testFailed')
  } finally {
    testing.value = false
  }
}

function submit() {
  if (!form.name.trim()) return
  emit('save', { ...form })
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.stopPropagation()
    emit('close')
  }
}

onMounted(() => {
  document.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50" @contextmenu.prevent>
    <div class="bg-ch-panel border border-ch-border rounded-lg w-full max-w-md shadow-xl" @click.stop>
      <div class="flex items-center justify-between px-4 py-3 border-b border-ch-border">
        <h3 class="font-semibold text-ch-text">
          {{ connection ? t('connection.titleEdit') : t('connection.titleNew') }}
        </h3>
        <button @click="emit('close')" class="text-ch-muted hover:text-ch-text">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
        </button>
      </div>

      <div class="px-4 py-3 space-y-3 max-h-[65vh] overflow-y-auto">
        <div>
          <label class="block text-xs text-ch-muted mb-1">{{ t('connection.name') }} *</label>
          <input v-model="form.name" class="w-full" placeholder="My ClickHouse" />
        </div>

        <div>
          <label class="block text-xs text-ch-muted mb-1">{{ t('connection.folder') }}</label>
          <select v-model="form.folder_id" class="w-full">
            <option value="">{{ t('connection.noFolder') }}</option>
            <option v-for="f in folders" :key="f.id" :value="f.id">{{ f.name }}</option>
          </select>
        </div>

        <div class="grid grid-cols-4 gap-2">
          <div>
            <label class="block text-xs text-ch-muted mb-1">{{ t('connection.protocol') }}</label>
            <select v-model="form.protocol" class="w-full">
              <option value="native">Native (TCP)</option>
              <option value="http">HTTP</option>
            </select>
          </div>
          <div class="col-span-2">
            <label class="block text-xs text-ch-muted mb-1">{{ t('connection.host') }}</label>
            <input v-model="form.host" class="w-full" placeholder="localhost" />
          </div>
          <div>
            <label class="block text-xs text-ch-muted mb-1">{{ t('connection.port') }}</label>
            <input v-model.number="form.port" type="number" class="w-full" />
          </div>
        </div>

        <div class="grid grid-cols-2 gap-2">
          <div>
            <label class="block text-xs text-ch-muted mb-1">{{ t('connection.user') }}</label>
            <input v-model="form.user" class="w-full" />
          </div>
          <div>
            <label class="block text-xs text-ch-muted mb-1">{{ t('connection.password') }}</label>
            <div class="relative">
              <input
                v-model="form.password"
                :type="showPassword ? 'text' : 'password'"
                class="w-full pr-8"
              />
              <button
                type="button"
                @click="showPassword = !showPassword"
                class="absolute right-1 top-1/2 -translate-y-1/2 p-1 text-ch-muted hover:text-ch-text rounded"
                :title="showPassword ? t('connection.hidePassword') : t('connection.showPassword')"
              >
                <svg v-if="showPassword" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/>
                  <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/>
                  <path d="M14.12 14.12a3 3 0 1 1-4.24-4.24"/>
                  <line x1="1" y1="1" x2="23" y2="23"/>
                </svg>
                <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/>
                  <circle cx="12" cy="12" r="3"/>
                </svg>
              </button>
            </div>
          </div>
        </div>

        <div>
          <label class="block text-xs text-ch-muted mb-1">{{ t('connection.database') }}</label>
          <input v-model="form.database" class="w-full" />
        </div>

        <div class="flex items-center gap-4 pt-1">
          <label class="flex items-center gap-1.5 text-sm text-ch-text">
            <input v-model="form.secure" type="checkbox" />
            {{ t('connection.secure') }}
          </label>
          <label class="flex items-center gap-1.5 text-sm text-ch-text">
            <input v-model="form.use_compress" type="checkbox" />
            {{ t('connection.compress') }}
          </label>
        </div>

        <div v-if="testMsg" :class="['text-xs px-2 py-1 rounded', testOk ? 'bg-green-900/40 text-green-400' : 'bg-red-900/40 text-red-400']">
          {{ testMsg }}
        </div>
      </div>

      <div class="flex items-center justify-between px-4 py-3 border-t border-ch-border">
        <button
          v-if="connection"
          @click="emit('delete', connection.id)"
          class="btn-ghost btn-sm text-ch-accent border-ch-accent/50 hover:bg-red-900/20"
        >
          {{ t('connection.deleteBtn') }}
        </button>
        <div v-else />
        <div class="flex gap-2">
          <button @click="testConnection" class="btn-ghost btn-sm" :disabled="testing">
            {{ testing ? t('connection.testing') : t('connection.testConnection') }}
          </button>
          <button @click="submit" class="btn-primary btn-sm">
            {{ connection ? t('connection.save') : t('connection.create') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>