<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import { setLocale, getCurrentLocale, type Locale } from '../i18n'

const { t } = useI18n()

const emit = defineEmits<{
  (e: 'newConnection'): void
  (e: 'newFolder'): void
  (e: 'toggleSidebar'): void
  (e: 'resetSidebar'): void
}>()

const openMenu = ref<string | null>(null)
const showShortcuts = ref(false)
const showAbout = ref(false)
const appVersion = ref('')

const isMac = navigator.platform.toLowerCase().includes('mac')
const mod = isMac ? '⌘' : 'Ctrl'
const modShift = isMac ? '⌘⇧' : 'Ctrl+Shift'

function closeMenu() {
  openMenu.value = null
}

function toggleMenu(name: string) {
  openMenu.value = openMenu.value === name ? null : name
}

function onGlobalClick(e: MouseEvent) {
  const target = e.target as HTMLElement
  if (!target.closest('[data-menu-bar]')) {
    closeMenu()
  }
}

function onGlobalKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    closeMenu()
    showShortcuts.value = false
    showAbout.value = false
  }

  const isMod = isMac ? e.metaKey : e.ctrlKey
  if (!isMod) return

  switch (e.key.toLowerCase()) {
    case 'n':
      if (e.shiftKey) {
        e.preventDefault()
        emit('newFolder')
      } else {
        e.preventDefault()
        emit('newConnection')
      }
      break
    case 'b':
      e.preventDefault()
      emit('toggleSidebar')
      break
    case '/':
      e.preventDefault()
      showShortcuts.value = true
      break
  }
}

function actionOpenConfig() {
  api.OpenConfigFolder().catch(() => {})
  closeMenu()
}

function actionResetSidebar() {
  emit('resetSidebar')
  closeMenu()
}

function actionToggleSidebar() {
  emit('toggleSidebar')
  closeMenu()
}

function changeLocale(lang: Locale) {
  setLocale(lang)
  closeMenu()
}

async function openAbout() {
  try {
    appVersion.value = await api.GetAppVersion()
  } catch {
    appVersion.value = '0.1.0'
  }
  showAbout.value = true
  closeMenu()
}

function openShortcuts() {
  showShortcuts.value = true
  closeMenu()
}

function quit() {
  window.close()
  closeMenu()
}

onMounted(() => {
  document.addEventListener('click', onGlobalClick)
  document.addEventListener('keydown', onGlobalKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', onGlobalClick)
  document.removeEventListener('keydown', onGlobalKeydown)
})
</script>

<template>
  <div data-menu-bar class="h-8 bg-ch-panel border-b border-ch-border flex items-center text-[13px] select-none shrink-0 relative">
    <div class="flex items-center h-full">
      <div class="w-6 h-6 rounded bg-ch-accent flex items-center justify-center text-white font-bold text-[10px] ml-2 mr-1">M</div>
    </div>

    <!-- File -->
    <div class="relative h-full">
      <button
        @click.stop="toggleMenu('file')"
        @mouseenter="openMenu && (openMenu = 'file')"
        :class="['h-full px-3 flex items-center text-ch-text hover:bg-ch-hover transition-colors', openMenu === 'file' ? 'bg-ch-hover' : '']"
      >{{ t('menu.file') }}</button>
      <div v-if="openMenu === 'file'" class="absolute top-full left-0 min-w-[220px] bg-ch-panel border border-ch-border shadow-xl rounded py-1 z-50">
        <button @click.stop="emit('newConnection'); closeMenu()" class="menu-item w-full text-left px-3 py-1.5 hover:bg-ch-hover flex items-center justify-between">
          <span>{{ t('menu.newConnection') }}</span><span class="text-ch-muted text-xs">{{ mod }}N</span>
        </button>
        <button @click.stop="emit('newFolder'); closeMenu()" class="menu-item w-full text-left px-3 py-1.5 hover:bg-ch-hover flex items-center justify-between">
          <span>{{ t('menu.newFolder') }}</span><span class="text-ch-muted text-xs">{{ modShift }}N</span>
        </button>
        <div class="h-px bg-ch-border my-1" />
        <button @click.stop="actionOpenConfig()" class="menu-item w-full text-left px-3 py-1.5 hover:bg-ch-hover flex items-center justify-between">
          <span>{{ t('menu.openConfigFolder') }}</span>
        </button>
        <div class="h-px bg-ch-border my-1" />
        <button @click.stop="quit()" class="menu-item w-full text-left px-3 py-1.5 hover:bg-ch-hover flex items-center justify-between">
          <span>{{ t('menu.quit') }}</span><span class="text-ch-muted text-xs">{{ mod }}Q</span>
        </button>
      </div>
    </div>

    <!-- Edit -->
    <div class="relative h-full">
      <button
        @click.stop="toggleMenu('edit')"
        @mouseenter="openMenu && (openMenu = 'edit')"
        :class="['h-full px-3 flex items-center text-ch-text hover:bg-ch-hover transition-colors', openMenu === 'edit' ? 'bg-ch-hover' : '']"
      >{{ t('menu.edit') }}</button>
      <div v-if="openMenu === 'edit'" class="absolute top-full left-0 min-w-[220px] bg-ch-panel border border-ch-border shadow-xl rounded py-1 z-50">
        <div class="px-3 py-1.5 text-ch-muted text-xs uppercase tracking-wider">{{ t('menu.edit') }}</div>
        <div class="h-px bg-ch-border my-1" />
        <div class="px-3 py-1.5 text-ch-text flex items-center justify-between opacity-80">
          <span>{{ t('menu.undo') }}</span><span class="text-ch-muted text-xs">{{ mod }}Z</span>
        </div>
        <div class="px-3 py-1.5 text-ch-text flex items-center justify-between opacity-80">
          <span>{{ t('menu.redo') }}</span><span class="text-ch-muted text-xs">{{ mod }}Y</span>
        </div>
        <div class="h-px bg-ch-border my-1" />
        <div class="px-3 py-1.5 text-ch-text flex items-center justify-between opacity-80">
          <span>{{ t('menu.selectAll') }}</span><span class="text-ch-muted text-xs">{{ mod }}A</span>
        </div>
      </div>
    </div>

    <!-- View -->
    <div class="relative h-full">
      <button
        @click.stop="toggleMenu('view')"
        @mouseenter="openMenu && (openMenu = 'view')"
        :class="['h-full px-3 flex items-center text-ch-text hover:bg-ch-hover transition-colors', openMenu === 'view' ? 'bg-ch-hover' : '']"
      >{{ t('menu.view') }}</button>
      <div v-if="openMenu === 'view'" class="absolute top-full left-0 min-w-[220px] bg-ch-panel border border-ch-border shadow-xl rounded py-1 z-50">
        <button @click.stop="actionToggleSidebar()" class="menu-item w-full text-left px-3 py-1.5 hover:bg-ch-hover flex items-center justify-between">
          <span>{{ t('menu.toggleSidebar') }}</span><span class="text-ch-muted text-xs">{{ mod }}B</span>
        </button>
        <button @click.stop="actionResetSidebar()" class="menu-item w-full text-left px-3 py-1.5 hover:bg-ch-hover flex items-center justify-between">
          <span>{{ t('menu.resetSidebar') }}</span>
        </button>
        <div class="h-px bg-ch-border my-1" />
        <div class="relative">
          <div class="px-3 py-1.5 text-ch-text flex items-center justify-between">
            <span>{{ t('menu.language') }}</span>
            <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>
          </div>
          <div class="absolute left-full top-0 min-w-[140px] bg-ch-panel border border-ch-border shadow-xl rounded py-1 z-50">
            <button @click.stop="changeLocale('en')" class="menu-item w-full text-left px-3 py-1.5 hover:bg-ch-hover flex items-center justify-between">
              <span>English</span>
              <svg v-if="getCurrentLocale() === 'en'" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>
            </button>
            <button @click.stop="changeLocale('zh')" class="menu-item w-full text-left px-3 py-1.5 hover:bg-ch-hover flex items-center justify-between">
              <span>中文</span>
              <svg v-if="getCurrentLocale() === 'zh'" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Help -->
    <div class="relative h-full">
      <button
        @click.stop="toggleMenu('help')"
        @mouseenter="openMenu && (openMenu = 'help')"
        :class="['h-full px-3 flex items-center text-ch-text hover:bg-ch-hover transition-colors', openMenu === 'help' ? 'bg-ch-hover' : '']"
      >{{ t('menu.help') }}</button>
      <div v-if="openMenu === 'help'" class="absolute top-full left-0 min-w-[220px] bg-ch-panel border border-ch-border shadow-xl rounded py-1 z-50">
        <button @click.stop="openShortcuts()" class="menu-item w-full text-left px-3 py-1.5 hover:bg-ch-hover flex items-center justify-between">
          <span>{{ t('menu.shortcuts') }}</span><span class="text-ch-muted text-xs">{{ mod }}/</span>
        </button>
        <div class="h-px bg-ch-border my-1" />
        <button @click.stop="openAbout()" class="menu-item w-full text-left px-3 py-1.5 hover:bg-ch-hover flex items-center justify-between">
          <span>{{ t('menu.about') }}</span>
        </button>
      </div>
    </div>

    <div class="flex-1" />
    <Teleport to="body">
      <div v-if="showShortcuts" class="fixed inset-0 bg-black/50 flex items-center justify-center z-[100]" @click="showShortcuts = false">
        <div class="bg-ch-panel border border-ch-border rounded-lg shadow-2xl w-[480px] max-w-[90vw]" @click.stop>
          <div class="flex items-center justify-between px-4 py-3 border-b border-ch-border">
            <h3 class="text-ch-text font-semibold">{{ t('shortcuts.title') }}</h3>
            <button class="text-ch-muted hover:text-ch-text" @click="showShortcuts = false">✕</button>
          </div>
          <div class="px-4 py-3 text-sm">
            <div class="grid grid-cols-[1fr_auto] gap-x-6 gap-y-2">
              <template v-for="(label, key) in {
                newConnection: t('shortcuts.newConnection'),
                newFolder: t('shortcuts.newFolder'),
                runQuery: t('shortcuts.runQuery'),
                toggleSidebar: t('shortcuts.toggleSidebar'),
                selectAll: t('shortcuts.selectAll'),
                undo: t('shortcuts.undo'),
                redo: t('shortcuts.redo'),
              }" :key="key">
                <span class="text-ch-text">{{ label }}</span>
                <kbd class="bg-ch-hover border border-ch-border px-1.5 py-0.5 rounded text-xs text-ch-muted font-mono">
                  <template v-if="key === 'newConnection'">{{ mod }}+N</template>
                  <template v-else-if="key === 'newFolder'">{{ modShift }}+N</template>
                  <template v-else-if="key === 'runQuery'">{{ mod }}+Enter</template>
                  <template v-else-if="key === 'toggleSidebar'">{{ mod }}+B</template>
                  <template v-else-if="key === 'selectAll'">{{ mod }}+A</template>
                  <template v-else-if="key === 'undo'">{{ mod }}+Z</template>
                  <template v-else-if="key === 'redo'">{{ mod }}+Y</template>
                </kbd>
              </template>
            </div>
          </div>
          <div class="px-4 py-3 border-t border-ch-border flex justify-end">
            <button class="btn-primary btn-sm" @click="showShortcuts = false">{{ t('common.close') }}</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- About Modal -->
    <Teleport to="body">
      <div v-if="showAbout" class="fixed inset-0 bg-black/50 flex items-center justify-center z-[100]" @click="showAbout = false">
        <div class="bg-ch-panel border border-ch-border rounded-lg shadow-2xl w-[400px] max-w-[90vw] text-center" @click.stop>
          <div class="py-8">
            <div class="w-14 h-14 mx-auto rounded-lg bg-ch-accent flex items-center justify-center text-white font-bold text-2xl mb-4">M</div>
            <h3 class="text-ch-text font-semibold text-lg mb-1">{{ t('about.title') }}</h3>
            <div class="text-ch-muted text-sm mb-4">
              {{ t('about.version') }}: <span class="text-ch-text font-mono">{{ appVersion || '...' }}</span>
            </div>
            <p class="text-ch-muted text-sm px-6 leading-relaxed">{{ t('about.desc') }}</p>
          </div>
          <div class="px-4 py-3 border-t border-ch-border flex justify-center">
            <button class="btn-primary btn-sm" @click="showAbout = false">{{ t('about.close') }}</button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style>
[data-menu-bar] { position: relative; }
.menu-item:focus { outline: none; }
</style>