<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from './stores/app'
import ConnectionModal from './components/ConnectionModal.vue'
import FolderModal from './components/FolderModal.vue'
import MenuBar from './components/MenuBar.vue'
import { setLocale, getCurrentLocale, type Locale } from './i18n'
import type { Connection } from './types'

const { t } = useI18n()
const store = useAppStore()

const showConnectionModal = ref(false)
const editingConnection = ref<any>(null)
const showFolderModal = ref(false)
const editingFolder = ref<any>(null)
const showLangMenu = ref(false)

interface ContextMenuState {
  visible: boolean
  x: number
  y: number
  type: 'folder' | 'connection' | null
  target: any
}
const ctxMenu = ref<ContextMenuState>({ visible: false, x: 0, y: 0, type: null, target: null })

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

onMounted(async () => {
  loadSidebarWidth()
  window.addEventListener('mousemove', onResizeMove)
  window.addEventListener('mouseup', onResizeEnd)
  window.addEventListener('click', onGlobalClick)
  window.addEventListener('contextmenu', hideContextMenu)
  await store.loadAll()
  store.checkAllConnections()
})

onBeforeUnmount(() => {
  window.removeEventListener('mousemove', onResizeMove)
  window.removeEventListener('mouseup', onResizeEnd)
  window.removeEventListener('click', onGlobalClick)
  window.removeEventListener('contextmenu', hideContextMenu)
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

function showContextMenu(e: MouseEvent, type: 'folder' | 'connection', target: any) {
  e.preventDefault()
  e.stopPropagation()
  ctxMenu.value = { visible: true, x: e.clientX, y: e.clientY, type, target }
}

function hideContextMenu() {
  ctxMenu.value.visible = false
}

function onGlobalClick() {
  hideContextMenu()
  showLangMenu.value = false
}

function ctxEdit() {
  if (ctxMenu.value.type === 'folder') editFolder(ctxMenu.value.target)
  else if (ctxMenu.value.type === 'connection') editConnection(ctxMenu.value.target)
  hideContextMenu()
}

function ctxDelete() {
  if (ctxMenu.value.type === 'folder') handleDeleteFolder(ctxMenu.value.target.id)
  else if (ctxMenu.value.type === 'connection') handleDeleteConnection(ctxMenu.value.target.id)
  hideContextMenu()
}

function ctxOpen() {
  if (ctxMenu.value.type === 'connection') store.activeConnectionId = ctxMenu.value.target.id
  hideContextMenu()
}

function ctxRefresh() {
  if (ctxMenu.value.type === 'connection') store.checkConnection(ctxMenu.value.target.id)
  hideContextMenu()
}

async function ctxDisconnect() {
  if (ctxMenu.value.type === 'connection') {
    await store.disconnectConnection(ctxMenu.value.target.id)
  }
  hideContextMenu()
}

function statusDotClass(id: string) {
  const s = store.getConnectionStatus(id)
  if (s === 'connecting') return 'bg-yellow-500 animate-pulse'
  if (s === 'connected') return 'bg-green-500'
  if (s === 'failed') return 'bg-red-500'
  return 'bg-ch-muted/50'
}

function statusTitle(id: string) {
  const s = store.getConnectionStatus(id)
  const err = store.getConnectionError(id)
  if (s === 'connecting') return t('status.connecting')
  if (s === 'connected') return t('status.connected')
  if (s === 'failed') return t('status.failed') + (err ? ': ' + err : '')
  return t('status.unknown')
}

watch(
  () => store.activeConnectionId,
  (id) => {
    if (id) store.checkConnection(id)
  }
)

// 导入配置后重新拉取全部数据并刷新连接状态
async function onConfigImported() {
  await store.loadAll()
  store.activeConnectionId = null
  store.checkAllConnections()
}

// ---- 拖拽排序 ----
// 采用自绘指针拖拽（不用 HTML5 DnD，规避 webview 下 DnD 不稳定问题）：
// 按下超过阈值后进入拖拽态，实时根据指针命中元素计算目标分组与插入位置，
// release 后一次性落盘。支持连接组内排序 / 跨分组拖拽，以及分组排序。
interface DragItem {
  kind: 'folder' | 'connection'
  id: string
  folderId: string | null
  active: boolean
  moved: boolean
  startX: number
  startY: number
}
interface DropTarget {
  kind: 'folder' | 'connection'
  folderId: string | null
  index: number // -1 表示追加到分组末尾；否则为目标分组内第 index 个元素之前
}
const treeEl = ref<HTMLElement | null>(null)
const dragItem = ref<DragItem | null>(null)
const dropTarget = ref<DropTarget | null>(null)
const indicatorY = ref(12)
const indicatorVisible = ref(false)
const suppressNextClick = ref(false)

// 按下：记录候选拖拽项（移动超过阈值后才真正进入拖拽态）
// 注意：此处不调 preventDefault —— 在 webview 里它会阻断后续 mousemove。
// 文本选择靠 CSS user-select: none 兜底，mousemove 里再 preventDefault 双重保险。
function onDragStartHandle(e: MouseEvent, kind: DragItem['kind'], id: string, folderId: string | null) {
  if (e.button !== 0) return
  dragItem.value = { kind, id, folderId, active: true, moved: false, startX: e.clientX, startY: e.clientY }
  window.addEventListener('mousemove', onPointerMove)
  window.addEventListener('mouseup', onPointerUp)
}

function onPointerMove(e: MouseEvent) {
  const item = dragItem.value
  if (!item || !item.active) return
  if (!item.moved) {
    if (Math.hypot(e.clientX - item.startX, e.clientY - item.startY) < 4) return
    item.moved = true
    suppressNextClick.value = true
    document.body.style.cursor = 'grabbing'
  }
  if (e.cancelable) e.preventDefault()
  updateDropTarget(e)
}

function onPointerUp(e: MouseEvent) {
  const item = dragItem.value
  if (!item) return
  if (item.active && item.moved) {
    performDrop()
  }
  document.body.style.cursor = ''
  dragItem.value = null
  dropTarget.value = null
  indicatorVisible.value = false
  window.removeEventListener('mousemove', onPointerMove)
  window.removeEventListener('mouseup', onPointerUp)
}

// 拖拽结束后若误触发所在行点击，则抑制（避免拖拽时切换选中 / 折叠分组）
function onClickSelect(connId: string) {
  if (suppressNextClick.value) {
    suppressNextClick.value = false
    return
  }
  store.activeConnectionId = connId
}
function onClickToggleFolder(folderId: string) {
  if (suppressNextClick.value) {
    suppressNextClick.value = false
    return
  }
  store.toggleFolder(folderId)
}

function updateDropTarget(e: MouseEvent) {
  const el = document.elementFromPoint(e.clientX, e.clientY)
  const tree = treeEl.value
  if (!tree || !el || !tree.contains(el)) {
    dropTarget.value = null
    indicatorVisible.value = false
    return
  }
  const item = dragItem.value!
  const treeRect = tree.getBoundingClientRect()
  const connRow = el.closest('.connection-item') as HTMLElement | null
  const header = el.closest('.folder-header') as HTMLElement | null

  if (connRow) {
    if (item.kind !== 'connection') return
    const rect = connRow.getBoundingClientRect()
    const before = e.clientY < rect.top + rect.height / 2
    const folderId = connRow.getAttribute('data-folder') || null
    const container = connRow.parentElement as HTMLElement
    const rows = Array.from(container.querySelectorAll(':scope > .connection-item'))
    const rowIndex = rows.indexOf(connRow)
    dropTarget.value = { kind: 'connection', folderId, index: before ? rowIndex : rowIndex + 1 }
    indicatorY.value = (before ? rect.top : rect.bottom) - treeRect.top + tree.scrollTop
    indicatorVisible.value = true
    return
  }

  if (header) {
    const headerFolderId = header.getAttribute('data-folder') || null
    if (item.kind === 'connection') {
      // 拖到分组头 → 移动到该分组末尾
      dropTarget.value = { kind: 'connection', folderId: headerFolderId, index: -1 }
      indicatorY.value = header.getBoundingClientRect().top - treeRect.top + tree.scrollTop
      indicatorVisible.value = true
      return
    }
    const headers = Array.from(tree.querySelectorAll('.folder-header'))
    const headerIndex = headers.indexOf(header)
    const rect = header.getBoundingClientRect()
    const before = e.clientY < rect.top + rect.height / 2
    dropTarget.value = { kind: 'folder', folderId: null, index: before ? headerIndex : headerIndex + 1 }
    indicatorY.value = (before ? rect.top : rect.bottom) - treeRect.top + tree.scrollTop
    indicatorVisible.value = true
    return
  }

  // 悬停在树内空白区域：连接拖到未分组；分组拖拽取消
  if (item.kind === 'connection') {
    dropTarget.value = { kind: 'connection', folderId: null, index: 0 }
    indicatorVisible.value = false
  } else {
    dropTarget.value = null
    indicatorVisible.value = false
  }
}

async function performDrop() {
  const drag = dragItem.value
  const target = dropTarget.value
  if (!drag || !target || !drag.moved) return
  if (drag.kind === 'connection' && target.kind === 'connection') {
    const dragConn = store.connections.find((c) => c.id === drag.id)
    if (!dragConn) return
    const moved = { ...dragConn, folder_id: target.folderId ?? undefined }
    if (drag.folderId !== target.folderId) {
      await store.saveConnection(moved)
    }
    const order = buildNewConnOrder(moved, target)
    await store.reorderConnections(order.map((c) => c.id))
  } else if (drag.kind === 'folder' && target.kind === 'folder') {
    await store.reorderFolders(buildNewFolderOrder(drag.id, target.index))
  }
}

// 根据目标分组与目标插入位置重建连接扁平顺序（组内顺序=扁平顺序）
function buildNewConnOrder(moved: Connection, target: DropTarget): Connection[] {
  const groups = store.groupedConnections
  const result: Connection[] = []
  for (const group of groups) {
    const groupFolderId = group.folder ? group.folder.id : null
    let conns = group.conns.filter((c) => c.id !== moved.id)
    if (groupFolderId === target.folderId) {
      if (target.index === -1) {
        conns = [...conns, moved]
      } else {
        const dragDispIndex = group.conns.findIndex((c) => c.id === moved.id)
        let ins = target.index
        if (dragDispIndex >= 0 && dragDispIndex < ins) ins -= 1
        ins = Math.max(0, Math.min(ins, conns.length))
        conns = [...conns.slice(0, ins), moved, ...conns.slice(ins)]
      }
    }
    result.push(...conns)
  }
  // 拖到未分组但当前没有未分组连接时不显示该分组，最后补上
  if (!result.some((c) => c.id === moved.id) && target.folderId === null) {
    result.push(moved)
  }
  return result
}

function buildNewFolderOrder(dragId: string, index: number): string[] {
  const full = store.folders.map((f) => f.id)
  const dragIdx = full.indexOf(dragId)
  let ins = index
  if (dragIdx >= 0 && dragIdx < ins) ins -= 1
  const ids = full.filter((id) => id !== dragId)
  ins = Math.max(0, Math.min(ins, ids.length))
  return [...ids.slice(0, ins), dragId, ...ids.slice(ins)]
}
</script>

<template>
  <div class="flex flex-col h-full w-full">
    <MenuBar
      @new-connection="newConnection"
      @new-folder="newFolder"
      @toggle-sidebar="store.toggleSidebar()"
      @reset-sidebar="resetSidebarWidth"
      @config-imported="onConfigImported"
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
      <div ref="treeEl" class="flex-1 overflow-y-auto py-1 relative select-none" v-if="!store.sidebarCollapsed">
        <div v-show="indicatorVisible" class="drop-line" :style="{ top: indicatorY + 'px' }"></div>
        <template v-for="group in store.groupedConnections" :key="group.folder?.id || 'ungrouped'">
          <!-- Folder Header -->
          <div
            v-if="group.folder"
            class="flex items-center gap-1 px-2 py-1 text-xs font-semibold text-ch-muted uppercase tracking-wide cursor-pointer hover:bg-ch-hover select-none group-header folder-header"
            :data-folder="group.folder.id"
            @click="onClickToggleFolder(group.folder.id)"
            @mousedown.stop="onDragStartHandle($event, 'folder', group.folder.id, null)"
            @contextmenu.prevent="showContextMenu($event, 'folder', group.folder)"
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
              :data-folder="group.folder?.id || ''"
              :data-conn-id="conn.id"
              :class="[
                'flex items-center gap-2 px-3 py-1.5 cursor-pointer text-sm transition-colors connection-item select-none',
                store.activeConnectionId === conn.id
                  ? 'bg-ch-active text-ch-text'
                  : 'text-ch-text hover:bg-ch-hover',
              ]"
              @click="onClickSelect(conn.id)"
              @mousedown.stop="onDragStartHandle($event, 'connection', conn.id, group.folder?.id || null)"
              @contextmenu.prevent="showContextMenu($event, 'connection', conn)"
            >
              <span
                class="w-2 h-2 rounded-full shrink-0"
                :class="statusDotClass(conn.id)"
                :title="statusTitle(conn.id)"
                @click.stop="store.checkConnection(conn.id)"
              ></span>
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
    <main class="flex-1 overflow-hidden min-w-0">
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

    <!-- Context Menu -->
    <Teleport to="body">
      <div
        v-if="ctxMenu.visible"
        class="fixed z-[1000] bg-ch-panel border border-ch-border rounded-md shadow-xl py-1 min-w-[160px] select-none"
        :style="{ left: ctxMenu.x + 'px', top: ctxMenu.y + 'px' }"
        @click.stop
        @contextmenu.prevent
      >
        <template v-if="ctxMenu.type === 'connection'">
          <button
            @click="ctxOpen"
            class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-sm text-ch-text hover:bg-ch-hover"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>
            {{ t('sidebar.open') }}
          </button>
          <button
            @click="ctxRefresh"
            class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-sm text-ch-text hover:bg-ch-hover"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
            {{ t('sidebar.refresh') }}
          </button>
          <button
            @click="ctxDisconnect"
            class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-sm text-ch-text hover:bg-ch-hover"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            {{ t('sidebar.disconnect') }}
          </button>
          <div class="my-1 border-t border-ch-border"></div>
        </template>
        <button
          @click="ctxEdit"
          class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-sm text-ch-text hover:bg-ch-hover"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4z"/></svg>
          {{ t('sidebar.rename') }}
        </button>
        <button
          @click="ctxDelete"
          class="flex items-center gap-2 w-full text-left px-3 py-1.5 text-sm text-ch-accent hover:bg-ch-hover"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/></svg>
          {{ t('sidebar.delete') }}
        </button>
      </div>
    </Teleport>
  </div>
</template>

<style>
aside, aside * {
  user-select: none !important;
  -webkit-user-select: none !important;
}
.connection-item:hover .connection-actions {
  display: inline-block;
}
.drop-line {
  position: absolute;
  left: 0;
  right: 0;
  height: 2px;
  z-index: 20;
  background: #e74c3c;
  pointer-events: none;
}
</style>