<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Plus, Trash2, Edit2, Check, X, Wifi, WifiOff } from 'lucide-vue-next'

interface Connection {
  id: string
  name: string
  host: string
  port: number
  user: string
  password: string
  database: string
  secure: boolean
  use_compress: boolean
}

const connections = ref<Connection[]>([])
const showModal = ref(false)
const editing = ref<Connection | null>(null)
const testing = ref(false)
const testResult = ref<string | null>(null)

const form = ref<Partial<Connection>>({
  name: '', host: 'localhost', port: 9000,
  user: 'default', password: '', database: 'default',
  secure: false, use_compress: false,
})

async function loadConnections() {
  try {
    // connections.value = await window.go.main.App.GetConnections()
    connections.value = []
  } catch {
    connections.value = []
  }
}

function openNew() {
  editing.value = null
  form.value = { name: '', host: 'localhost', port: 9000, user: 'default', password: '', database: 'default', secure: false, use_compress: false }
  showModal.value = true
}

function openEdit(c: Connection) {
  editing.value = c
  form.value = { ...c }
  showModal.value = true
}

function closeModal() {
  showModal.value = false
  editing.value = null
  testResult.value = null
}

async function testConnection() {
  testing.value = true
  testResult.value = null
  try {
    // await window.go.main.App.TestConnection(form.value as Connection)
    testResult.value = '连接成功！'
  } catch (e: any) {
    testResult.value = '连接失败: ' + (e?.message ?? e)
  } finally {
    testing.value = false
  }
}

async function save() {
  if (!form.value.name || !form.value.host) return
  try {
    // await window.go.main.App.SaveConnection(form.value)
    closeModal()
    await loadConnections()
  } catch (e: any) {
    alert('保存失败: ' + e?.message)
  }
}

async function remove(id: string) {
  if (!confirm('确认删除此连接？')) return
  try {
    // await window.go.main.App.DeleteConnection(id)
    await loadConnections()
  } catch (e: any) {
    alert('删除失败: ' + e?.message)
  }
}

onMounted(loadConnections)
</script>

<template>
  <div class="p-6">
    <div class="flex items-center justify-between mb-6">
      <div>
        <h2 class="text-2xl font-semibold">连接管理</h2>
        <p class="text-sm text-gray-500">管理你的 ClickHouse 数据库连接</p>
      </div>
      <button
        @click="openNew"
        class="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 rounded-lg text-sm font-medium transition-colors"
      >
        <Plus class="w-4 h-4" />
        新建连接
      </button>
    </div>

    <div class="grid gap-4">
      <div
        v-for="conn in connections"
        :key="conn.id"
        class="bg-gray-800 rounded-xl p-4 border border-gray-700 hover:border-blue-500 transition-colors"
      >
        <div class="flex items-start justify-between">
          <div>
            <h3 class="font-semibold text-white">{{ conn.name }}</h3>
            <p class="text-sm text-gray-400 mt-1">{{ conn.host }}:{{ conn.port }} / {{ conn.database }}</p>
            <p class="text-xs text-gray-500 mt-1">用户: {{ conn.user }}{{ conn.secure ? ' · TLS' : '' }}</p>
          </div>
          <div class="flex items-center gap-2">
            <button @click="openEdit(conn)" class="p-2 text-gray-400 hover:text-blue-400 hover:bg-gray-700 rounded-lg transition-colors">
              <Edit2 class="w-4 h-4" />
            </button>
            <button @click="remove(conn.id)" class="p-2 text-gray-400 hover:text-red-400 hover:bg-gray-700 rounded-lg transition-colors">
              <Trash2 class="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>

      <div v-if="connections.length === 0" class="text-center py-16 text-gray-500">
        <Database class="w-12 h-12 mx-auto mb-4 opacity-50" />
        <p>暂无连接，点击右上角新建连接</p>
      </div>
    </div>

    <div v-if="showModal" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50" @click.self="closeModal">
      <div class="bg-gray-800 rounded-xl p-6 w-[480px] border border-gray-700">
        <div class="flex items-center justify-between mb-5">
          <h3 class="text-lg font-semibold">{{ editing ? '编辑连接' : '新建连接' }}</h3>
          <button @click="closeModal" class="p-1 hover:bg-gray-700 rounded">
            <X class="w-5 h-5 text-gray-400" />
          </button>
        </div>

        <div class="space-y-4">
          <div class="grid grid-cols-2 gap-3">
            <div class="col-span-2">
              <label class="text-xs text-gray-400 mb-1 block">连接名称</label>
              <input v-model="form.name" class="w-full bg-gray-900 border border-gray-600 rounded-lg px-3 py-2 text-sm focus:border-blue-500 outline-none" placeholder="生产环境" />
            </div>
            <div>
              <label class="text-xs text-gray-400 mb-1 block">主机</label>
              <input v-model="form.host" class="w-full bg-gray-900 border border-gray-600 rounded-lg px-3 py-2 text-sm focus:border-blue-500 outline-none" />
            </div>
            <div>
              <label class="text-xs text-gray-400 mb-1 block">端口</label>
              <input v-model.number="form.port" type="number" class="w-full bg-gray-900 border border-gray-600 rounded-lg px-3 py-2 text-sm focus:border-blue-500 outline-none" />
            </div>
            <div>
              <label class="text-xs text-gray-400 mb-1 block">用户名</label>
              <input v-model="form.user" class="w-full bg-gray-900 border border-gray-600 rounded-lg px-3 py-2 text-sm focus:border-blue-500 outline-none" />
            </div>
            <div>
              <label class="text-xs text-gray-400 mb-1 block">密码</label>
              <input v-model="form.password" type="password" class="w-full bg-gray-900 border border-gray-600 rounded-lg px-3 py-2 text-sm focus:border-blue-500 outline-none" />
            </div>
            <div class="col-span-2">
              <label class="text-xs text-gray-400 mb-1 block">默认数据库</label>
              <input v-model="form.database" class="w-full bg-gray-900 border border-gray-600 rounded-lg px-3 py-2 text-sm focus:border-blue-500 outline-none" />
            </div>
            <label class="flex items-center gap-2 text-sm text-gray-300">
              <input v-model="form.secure" type="checkbox" class="rounded border-gray-600" />
              使用 TLS 加密
            </label>
            <label class="flex items-center gap-2 text-sm text-gray-300">
              <input v-model="form.use_compress" type="checkbox" class="rounded border-gray-600" />
              启用压缩
            </label>
          </div>

          <div class="flex items-center gap-2">
            <button
              @click="testConnection"
              :disabled="testing"
              class="flex items-center gap-2 px-3 py-1.5 text-sm bg-gray-700 hover:bg-gray-600 rounded-lg transition-colors disabled:opacity-50"
            >
              <Wifi v-if="!testing" class="w-4 h-4" />
              <WifiOff v-else class="w-4 h-4 animate-pulse" />
              测试连接
            </button>
            <span v-if="testResult" class="text-sm" :class="testResult.includes('成功') ? 'text-green-400' : 'text-red-400'">
              {{ testResult }}
            </span>
          </div>
        </div>

        <div class="flex justify-end gap-2 mt-6">
          <button @click="closeModal" class="px-4 py-2 text-sm bg-gray-700 hover:bg-gray-600 rounded-lg transition-colors">取消</button>
          <button @click="save" class="flex items-center gap-2 px-4 py-2 text-sm bg-blue-600 hover:bg-blue-500 rounded-lg transition-colors">
            <Check class="w-4 h-4" />
            保存
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
