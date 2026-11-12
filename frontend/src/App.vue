<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from './stores/app'
import ConnectionModal from './components/ConnectionModal.vue'
import FolderModal from './components/FolderModal.vue'
import MenuBar from './components/MenuBar.vue'
import { setLocale, getCurrentLocale, type Locale } from './i18n'

const { t } = useI18n()
const store = useAppStore()

const showConnectionModal = ref(false)
const editingConnection = ref<any>(null)
const showFolderModal = ref(false)
const editingFolder = ref<any>(null)
const showLangMenu = ref(false)

const SIDEBAR_MIN = 160
const SIDEBAR_MAX = 480
const SIDEBAR_DEFAULT = 256

const sidebarWidth = ref(SIDEBAR_DEFAULT)
const resizing = ref(false)
const appEl = ref<HTMLElement | null>(null)

function loadSidebarWidth() {
  const saved = localStorage.getItem('meteor-ch.sidebarWidth')
  if (saved) {
    const w = parseInt(saved, 10)
    if (!isNaN(w) && w >= SIDEBAR_MIN && w <= SIDEBAR_MAX) {
      sidebarWidth.value = w
    }
  }
}

function onResizeStart(e: MouseEvent) {
  resizing.value = true
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
  e.preventDefault()
}

function onResizeMove(e: MouseEvent) {
  if (!resizing.value || !appEl.value) return
  const rect = appEl.value.getBoundingClientRect()
  let w = e.clientX - rect.left
  w = Math.max(SIDEBAR_MIN, Math.min(SIDEBAR_MAX, w))
  sidebarWidth.value = w
}

function onResizeEnd() {
  if (!resizing.value) return
  resizing.value = false
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
  localStorage.setItem('meteor-ch.sidebarWidth', String(sidebarWidth.value))
}

onMounted(() => {
  loadSidebarWidth()
  window.addEventListener('mousemove', onResizeMove)
  window.addEventListener('mouseup', onResizeEnd)
})

onBeforeUnmount(() => {
  window.removeEventListener('mousemove', onResizeMove)
  window.removeEventListener('mouseup', onResizeEnd)
})

function newConnection() {
  editingConnection.value = null
  showConnectionModal.value = true
}

function editConnection(conn: any) {
  editingConnection.value = { ...conn }
  showConnectionModal.value = true
}

async function handleDeleteConnection(id: string) {
  if (confirm(t('connection.confirmDelete'))) {
    await store.deleteConnection(id)
  }
}

function newFolder() {
  editingFolder.value = null
  showFolderModal.value = true
}

function editFolder(f: any) {
  editingFolder.value = { ...f }
  showFolderModal.value = true
}

async function handleDeleteFolder(id: string) {
  if (confirm(t('folder.confirmDelete'))) {
    await store.deleteFolder(id)
  }
}

function changeLocale(lang: Locale) {
  setLocale(lang)
  showLangMenu.value = false
}

function resetSidebarWidth() {
  sidebarWidth.value = SIDEBAR_DEFAULT
  localStorage.setItem('meteor-ch.sidebarWidth', String(SIDEBAR_DEFAULT))
}
</script>

<template>
  <div class="flex flex-col h-full w-full">
    <MenuBar
      @new-connection="newConnection"
      @new-folder="newFolder"
      @toggle-sidebar="store.toggleSidebar()"
      @reset-sidebar="resetSidebarWidth"
    />
    <div ref="appEl" class="flex flex-1 min-h-0">
    <!-- Sidebar -->
    <aside
      :class="[
        'flex flex-col bg-ch-panel border-r border-ch-border',
        store.sidebarCollapsed ? 'w-12 transition-all duration-200' : '',
      ]"
      :style="store.sidebarCollapsed ? {} : { width: sidebarWidth + 'px', flex: '0 0 ' + sidebarWidth + 'px' }"
    >
      <!-- Header -->
      <div class="flex items-center gap-1 px-2 border-b border-ch-border h-11">
        <div v-if="!store.sidebarCollapsed" class="flex items-center gap-1.5 min-w-0 flex-1">
          <div class="w-6 h-6 rounded bg-ch-accent flex items-center justify-center text-white font-bold text-xs shrink-0">M</div>
          <span class="font-semibold text-ch-text text-sm truncate">{{ t('app.name') }}</span>
        </div>
        <div v-else class="w-6 h-6 rounded bg-ch-accent flex items-center justify-center text-white font-bold text-xs mx-auto">M</div>
        <div class="flex items-center gap-1 shrink-0">
          <!-- Language Switch -->
          <div class="relative" v-if="!store.sidebarCollapsed">
            <button
              @click="showLangMenu = !showLangMenu"
              class="p-1 rounded hover:bg-ch-hover text-ch-muted hover:text-ch-text text-xs"
              title="Language"
            >
              {{ getCurrentLocale() === 'zh' ? '中文' : 'EN' }}
            </button>
            <div
              v-if="showLangMenu"
              class="absolute right-0 top-8 z-50 bg-ch-panel border border-ch-border rounded shadow-lg py-1 min-w-[80px]"
            >
              <button
                @click="changeLocale('en')"
                :class="[
                  'block w-full text-left px-3 py-1 text-xs hover:bg-ch-hover',
                  getCurrentLocale() === 'en' ? 'text-ch-accent' : 'text-ch-text',
                ]"
              >
                English
              </button>
              <button
                @click="changeLocale('zh')"
                :class="[
                  'block w-full text-left px-3 py-1 text-xs hover:bg-ch-hover',
                  getCurrentLocale() === 'zh' ? 'text-ch-accent' : 'text-ch-text',
                ]"
              >
                中文
              </button>
            </div>
          </div>
          <button
            @click="store.toggleSidebar()"
            class="p-1 rounded hover:bg-ch-hover text-ch-muted hover:text-ch-text"
            :title="store.sidebarCollapsed ? t('sidebar.expand') : t('sidebar.collapse')"
          >
            <svg v-if="store.sidebarCollapsed" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>
            <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 18 9 12 15 6"/></svg>
          </button>
        </div>
      </div>

      <!-- Actions -->
      <div v-if="!store.sidebarCollapsed" class="flex items-center gap-1 px-2 py-2 border-b border-ch-border">
        <button @click="newConnection()" class="btn-primary btn-sm flex-1 flex items-center justify-center gap-1">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
          {{ t('sidebar.new') }}
        </button>
        <button @click="newFolder()" class="btn-ghost btn-sm flex items-center justify-center px-2" :title="t('sidebar.newFolder')">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>
        </button>
      </div>

      <!-- Connection Tree -->
      <div class="flex-1 overflow-y-auto py-1" v-if="!store.sidebarCollapsed">
        <template v-for="group in store.groupedConnections" :key="group.folder?.id || 'ungrouped'">
          <!-- Folder Header -->
          <div
            v-if="group.folder"
            class="flex items-center gap-1 px-2 py-1 text-xs font-semibold text-ch-muted uppercase tracking-wide cursor-pointer hover:bg-ch-hover select-none group-header"
            @click="store.toggleFolder(group.folder!.id)"
          >
            <svg
              width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
              :class="{ 'rotate-90': !store.collapsedFolders.has(group.folder.id) }"
              class="transition-transform shrink-0"
            >
              <polyline points="9 18 15 12 9 6"/>
            </svg>
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="shrink-0"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>
            <span class="flex-1 truncate">{{ group.folder.name }}</span>
            <button
              @click.stop="editFolder(group.folder!)"
              class="opacity-0 group-hover:opacity-100 hover:text-ch-text px-0.5"
              :title="t('sidebar.rename')"
            >
              <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4z"/></svg>
            </button>
            <button
              @click.stop="handleDeleteFolder(group.folder.id)"
              class="opacity-0 group-hover:opacity-100 hover:text-ch-accent px-0.5"
              :title="t('sidebar.delete')"
            >
              <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/></svg>
            </button>
          </div>

          <!-- Connections in Folder -->
          <div v-show="!store.collapsedFolders.has(group.folder?.id || '')">
            <div
              v-for="conn in group.conns"
              :key="conn.id"
              :class="[
                'flex items-center gap-2 px-3 py-1.5 cursor-pointer text-sm transition-colors connection-item',
                store.activeConnectionId === conn.id
                  ? 'bg-ch-active text-ch-text'
                  : 'text-ch-text hover:bg-ch-hover',
              ]"
              @click="store.activeConnectionId = conn.id"
            >
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="text-ch-muted shrink-0"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14a9 3 0 0 0 18 0V5"/><path d="M3 12a9 3 0 0 0 18 0"/></svg>
              <span class="flex-1 truncate">{{ conn.name }}</span>
              <span class="text-xs text-ch-muted opacity-60 group-hover:opacity-100" v-if="!group.folder">
                {{ conn.host }}:{{ conn.port }}
              </span>
              <button
                @click.stop="editConnection(conn)"
                class="hidden hover:text-ch-text px-0.5 connection-actions"
                :title="t('sidebar.rename')"
              >
                <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4z"/></svg>
              </button>
            </div>
            <div v-if="group.conns.length === 0" class="px-5 py-1 text-xs text-ch-muted italic">{{ t('sidebar.emptyFolder') }}</div>
          </div>

          <!-- Ungrouped header -->
          <div
            v-if="!group.folder && store.folders.length > 0"
            class="flex items-center gap-1 px-2 py-1 text-xs font-semibold text-ch-muted uppercase tracking-wide mt-1"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="shrink-0"><circle cx="12" cy="12" r="10"/></svg>
            <span>{{ t('sidebar.ungrouped') }}</span>
          </div>
        </template>

        <div v-if="store.connections.length === 0" class="px-3 py-4 text-center text-xs text-ch-muted">
          {{ t('sidebar.noConnections') }}<br>{{ t('sidebar.clickNewToAdd', { tag: '<span class="text-ch-accent">' + t('sidebar.new') + '</span>' }) }}
        </div>
      </div>

      <!-- Sidebar collapsed mini -->
      <div v-else class="flex-1 overflow-y-auto py-1">
        <div
          v-for="group in store.groupedConnections"
          :key="group.folder?.id || 'ungrouped'"
        >
          <div
            v-for="conn in group.conns"
            :key="conn.id"
            :class="[
              'flex justify-center py-1.5 cursor-pointer text-sm',
              store.activeConnectionId === conn.id ? 'bg-ch-active' : 'hover:bg-ch-hover',
            ]"
            :title="conn.name"
            @click="store.activeConnectionId = conn.id"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="text-ch-text"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14a9 3 0 0 0 18 0V5"/><path d="M3 12a9 3 0 0 0 18 0"/></svg>
          </div>
        </div>
      </div>
    </aside>

    <!-- Resize Handle -->
    <div
      v-if="!store.sidebarCollapsed"
      class="w-1 shrink-0 cursor-col-resize group relative"
      @mousedown="onResizeStart"
    >
      <div
        class="absolute inset-y-0 -left-0.5 -right-0.5 transition-colors"
        :class="resizing ? 'bg-ch-accent/40' : 'bg-transparent group-hover:bg-ch-accent/30'"
      ></div>
      <div class="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-1 h-10 rounded-full bg-ch-muted/30 group-hover:bg-ch-accent/60 transition-colors"></div>
    </div>

    <!-- Main Content -->
    <main class="flex-1 overflow-hidden min-w-0" @click="showLangMenu = false">
      <router-view />
    </main>
    </div>

    <!-- Modals -->
    <ConnectionModal
      v-if="showConnectionModal"
      :connection="editingConnection"
      :folders="store.folders"
      @close="showConnectionModal = false"
      @save="async (conn) => { await store.saveConnection(conn); showConnectionModal = false }"
      @delete="handleDeleteConnection"
    />
    <FolderModal
      v-if="showFolderModal"
      :folder="editingFolder"
      @close="showFolderModal = false"
      @save="async (f) => { await store.saveFolder(f); showFolderModal = false }"
    />
  </div>
</template>

<style>
.connection-item:hover .connection-actions {
  display: inline-block;
}
</style>