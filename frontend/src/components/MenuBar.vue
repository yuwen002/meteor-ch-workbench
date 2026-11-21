<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { ExportFormat, ImportFormat, ImportResult } from '../types'
import { setLocale, getCurrentLocale, type Locale } from '../i18n'

const { t } = useI18n()

const emit = defineEmits<{
  (e: 'newConnection'): void
  (e: 'newFolder'): void
  (e: 'toggleSidebar'): void
  (e: 'resetSidebar'): void
  (e: 'configImported'): void
}>()

const openMenu = ref<string | null>(null)
const showShortcuts = ref(false)
const showAbout = ref(false)
const showSettings = ref(false)
const rowLimit = ref<number>(50000)
const rowLimitInput = ref('50000')
const savingSettings = ref(false)
const settingsError = ref('')
const unlimitedRowLimit = ref(false)
const appVersion = ref('')
const importing = ref(false)
const importError = ref('')
const pendingImport = ref<{ fileName: string; raw: string } | null>(null)
const importFormats = ref<ImportFormat[]>([])
const importFormatId = ref('')
const importMode = ref<'replace' | 'merge'>('replace')
const importSummary = ref<ImportResult | null>(null)

const showExport = ref(false)
const exportFormats = ref<ExportFormat[]>([])
const exportFormatId = ref('json')
const exportIncludePassword = ref(false)
const exporting = ref(false)
const copyDone = ref(false)
const savedPath = ref('')

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

// 打开导出面板：拉取支持的格式列表后弹窗
async function actionExportConfig() {
  closeMenu()
  copyDone.value = false
  showExport.value = true
  try {
    const list = await api.GetExportFormats()
    exportFormats.value = list
    if (list.length && !list.some((f) => f.id === exportFormatId.value)) {
      exportFormatId.value = list[0].id
    }
  } catch (e: any) {
    alert(t('menu.exportFailed') + ': ' + (e?.message || e))
    showExport.value = false
  }
}

async function generateExport(): Promise<[string, string]> {
  return api.ExportConfigByFormat(exportFormatId.value, exportIncludePassword.value)
}

// 弹系统「另存为」对话框，用户选择保存位置后由后端写入
async function saveExportAs() {
  exporting.value = true
  savedPath.value = ''
  try {
    const path = await api.SaveExportAs(exportFormatId.value, exportIncludePassword.value)
    if (path) {
      savedPath.value = path
      showExport.value = false
      setTimeout(() => { savedPath.value = '' }, 6000)
    }
  } catch (e: any) {
    const msg = String(e?.message || e)
    if (msg.includes('cancelled by user')) {
      return
    }
    alert(t('menu.exportFailed') + ': ' + msg)
  } finally {
    exporting.value = false
  }
}

// 回退方案：浏览器下载到默认下载目录
async function downloadExport() {
  exporting.value = true
  try {
    const [content, ext] = await generateExport()
    const fmt = exportFormats.value.find((f) => f.id === exportFormatId.value)
    const mime = fmt?.mime || 'text/plain'
    const blob = new Blob([content], { type: mime })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `meteor-ch-config.${ext || 'txt'}`
    a.click()
    URL.revokeObjectURL(url)
    showExport.value = false
  } catch (e: any) {
    alert(t('menu.exportFailed') + ': ' + (e?.message || e))
  } finally {
    exporting.value = false
  }
}

// 复制到剪贴板，不下载文件
async function copyExport() {
  exporting.value = true
  copyDone.value = false
  try {
    const [content] = await generateExport()
    await navigator.clipboard.writeText(content)
    copyDone.value = true
    setTimeout(() => { copyDone.value = false }, 2000)
  } catch (e: any) {
    alert(t('menu.exportFailed') + ': ' + (e?.message || e))
  } finally {
    exporting.value = false
  }
}

// 打开系统文件选择框，选择后读取文件内容并进入确认流程
async function actionImportConfig() {
  closeMenu()
  importing.value = true
  importError.value = ''
  try {
    const [raw, fileName] = await api.PickImportFile()
    if (!raw) return
    pendingImport.value = { fileName, raw }
    importMode.value = 'replace'
    importFormatId.value = ''
    if (importFormats.value.length === 0) {
      api.GetImportFormats().then((list) => { importFormats.value = list }).catch(() => {})
    }
  } catch (e: any) {
    const msg = String(e?.message || e)
    if (!msg.includes('cancelled by user')) {
      importError.value = msg
      alert(t('menu.importFailed') + ': ' + msg)
    }
  } finally {
    importing.value = false
  }
}

// 用户确认为改变当前配置后，真正执行导入
async function confirmImport() {
  const pending = pendingImport.value
  if (!pending) return
  importing.value = true
  importError.value = ''
  try {
    const result = await api.ImportConfigWithOptions(pending.raw, {
      format: importFormatId.value,
      mode: importMode.value,
    })
    pendingImport.value = null
    importSummary.value = result
    setTimeout(() => { importSummary.value = null }, 8000)
    emit('configImported')
  } catch (e: any) {
    importError.value = e?.message || String(e)
    alert(t('menu.importFailed') + ': ' + importError.value)
  } finally {
    importing.value = false
  }
}

function cancelImport() {
  pendingImport.value = null
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

async function openSettings() {
  settingsError.value = ''
  try {
    const sf = await api.GetSettings()
    rowLimit.value = sf.query_row_limit
    unlimitedRowLimit.value = sf.query_row_limit === 0
    rowLimitInput.value = sf.query_row_limit === 0 ? '' : String(sf.query_row_limit)
  } catch (e: any) {
    settingsError.value = e?.message || String(e)
  }
  showSettings.value = true
  closeMenu()
}

async function saveSettings() {
  if (savingSettings.value) return
  settingsError.value = ''
  const limit = unlimitedRowLimit.value ? 0 : Number(rowLimitInput.value)
  if (!unlimitedRowLimit.value && (!Number.isFinite(limit) || limit < 100)) {
    settingsError.value = t('menu.rowLimitInvalid')
    return
  }
  savingSettings.value = true
  try {
    const sf = await api.SaveSettings({ query_row_limit: limit })
    rowLimit.value = sf.query_row_limit
    unlimitedRowLimit.value = sf.query_row_limit === 0
    rowLimitInput.value = sf.query_row_limit === 0 ? '' : String(sf.query_row_limit)
    showSettings.value = false
  } catch (e: any) {
    settingsError.value = e?.message || String(e)
  } finally {
    savingSettings.value = false
  }
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
        <button @click.stop="actionImportConfig()" :disabled="importing" class="menu-item w-full text-left px-3 py-1.5 hover:bg-ch-hover flex items-center justify-between disabled:opacity-50">
          <span>{{ importing ? t('menu.importing') : t('menu.importConfig') }}</span>
        </button>
        <button @click.stop="actionExportConfig()" class="menu-item w-full text-left px-3 py-1.5 hover:bg-ch-hover flex items-center justify-between">
          <span>{{ t('menu.exportConfig') }}</span>
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
        <button @click.stop="openSettings()" class="menu-item w-full text-left px-3 py-1.5 hover:bg-ch-hover flex items-center justify-between">
          <span>{{ t('menu.querySettings') }}</span>
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

    <!-- Settings Modal -->
    <Teleport to="body">
      <div v-if="showSettings" class="fixed inset-0 bg-black/50 flex items-center justify-center z-[300]" @click="showSettings = false">
        <div class="bg-ch-panel border border-ch-border rounded-lg shadow-2xl w-[440px] max-w-[90vw]" @click.stop>
          <div class="flex items-center justify-between px-4 py-3 border-b border-ch-border">
            <h3 class="text-ch-text font-semibold">{{ t('menu.querySettings') }}</h3>
            <button class="text-ch-muted hover:text-ch-text" @click="showSettings = false">✕</button>
          </div>
          <div class="px-4 py-4 text-sm space-y-3">
            <div>
              <label class="block text-ch-text mb-1">{{ t('menu.rowLimitLabel') }}</label>
              <input
                v-model="rowLimitInput"
                :disabled="unlimitedRowLimit"
                type="number"
                min="100"
                class="w-full px-2 py-1 rounded border border-ch-border bg-ch-bg text-ch-text disabled:opacity-40"
              />
              <p class="text-ch-muted text-xs mt-1">{{ t('menu.rowLimitHint') }}</p>
            </div>
            <label class="flex items-center gap-2 text-ch-text">
              <input v-model="unlimitedRowLimit" type="checkbox" />
              <span>{{ t('menu.rowLimitUnlimited') }}</span>
            </label>
            <p v-if="settingsError" class="text-red-400 text-xs">{{ settingsError }}</p>
          </div>
          <div class="px-4 py-3 border-t border-ch-border flex justify-end gap-2">
            <button class="px-3 py-1 rounded border border-ch-border text-ch-muted hover:text-ch-text hover:bg-ch-hover" @click="showSettings = false">{{ t('common.cancel') }}</button>
            <button class="btn-primary btn-sm" :disabled="savingSettings" @click="saveSettings">{{ savingSettings ? t('common.saving') : t('common.save') }}</button>
          </div>
        </div>
      </div>
    </Teleport>
    <!-- Import Confirm Modal -->
    <Teleport to="body">
      <div v-if="pendingImport" class="fixed inset-0 bg-black/50 flex items-center justify-center z-[100]" @click="cancelImport">
        <div class="bg-ch-panel border border-ch-border rounded-lg shadow-2xl w-[440px] max-w-[90vw]" @click.stop>
          <div class="flex items-center justify-between px-4 py-3 border-b border-ch-border">
            <h3 class="text-ch-text font-semibold">{{ t('menu.importConfirmTitle') }}</h3>
            <button class="text-ch-muted hover:text-ch-text" @click="cancelImport">✕</button>
          </div>
          <div class="px-4 py-4 text-sm space-y-3">
            <p class="text-ch-muted text-xs break-all">{{ t('menu.importFile') }}: {{ pendingImport.fileName }}</p>

            <div>
              <label class="block text-ch-muted text-xs mb-1.5">{{ t('menu.importFormat') }}</label>
              <select
                v-model="importFormatId"
                class="w-full bg-ch-bg border border-ch-border rounded px-2 py-1.5 text-xs text-ch-text"
              >
                <option value="">{{ t('menu.importFormatAuto') }}</option>
                <option v-for="f in importFormats" :key="f.id" :value="f.id">{{ f.name }} - {{ f.description }}</option>
              </select>
            </div>

            <div>
              <label class="block text-ch-muted text-xs mb-1.5">{{ t('menu.importMode') }}</label>
              <div class="flex gap-2">
                <button
                  class="flex-1 px-2.5 py-2 rounded border text-xs text-left"
                  :class="importMode === 'replace' ? 'border-ch-accent bg-ch-hover text-ch-text' : 'border-ch-border text-ch-muted hover:bg-ch-hover'"
                  @click="importMode = 'replace'"
                >
                  <span class="font-medium block">{{ t('menu.modeReplace') }}</span>
                  <span class="text-[11px] text-ch-muted">{{ t('menu.modeReplaceDesc') }}</span>
                </button>
                <button
                  class="flex-1 px-2.5 py-2 rounded border text-xs text-left"
                  :class="importMode === 'merge' ? 'border-ch-accent bg-ch-hover text-ch-text' : 'border-ch-border text-ch-muted hover:bg-ch-hover'"
                  @click="importMode = 'merge'"
                >
                  <span class="font-medium block">{{ t('menu.modeMerge') }}</span>
                  <span class="text-[11px] text-ch-muted">{{ t('menu.modeMergeDesc') }}</span>
                </button>
              </div>
            </div>

            <p v-if="importMode === 'replace'" class="text-ch-accent text-xs">{{ t('menu.importConfirmWarn') }}</p>
            <p v-else class="text-ch-muted text-xs">{{ t('menu.importMergeHint') }}</p>
            <p v-if="importError" class="text-red-400 text-xs break-all">{{ importError }}</p>
          </div>
          <div class="px-4 py-3 border-t border-ch-border flex justify-end gap-2">
            <button class="btn-ghost btn-sm" @click="cancelImport">{{ t('common.cancel') }}</button>
            <button class="btn-primary btn-sm" :disabled="importing" @click="confirmImport">
              {{ importing ? t('menu.importing') : (importMode === 'merge' ? t('menu.modeMerge') : t('menu.importConfirmOk')) }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Export Format Modal -->
    <Teleport to="body">
      <div v-if="showExport" class="fixed inset-0 bg-black/50 flex items-center justify-center z-[100]" @click="showExport = false">
        <div class="bg-ch-panel border border-ch-border rounded-lg shadow-2xl w-[480px] max-w-[92vw]" @click.stop>
          <div class="flex items-center justify-between px-4 py-3 border-b border-ch-border">
            <h3 class="text-ch-text font-semibold">{{ t('menu.exportConfig') }}</h3>
            <button class="text-ch-muted hover:text-ch-text" @click="showExport = false">✕</button>
          </div>
          <div class="px-4 py-4 text-sm space-y-3">
            <div>
              <label class="block text-ch-muted text-xs mb-1.5">{{ t('menu.exportFormat') }}</label>
              <div v-if="exportFormats.length === 0" class="text-ch-muted text-xs">{{ t('menu.loading') }}</div>
              <div class="space-y-1.5">
                <label
                  v-for="f in exportFormats"
                  :key="f.id"
                  class="flex items-start gap-2 px-2.5 py-2 rounded border cursor-pointer"
                  :class="exportFormatId === f.id ? 'border-ch-accent bg-ch-hover' : 'border-ch-border hover:bg-ch-hover'"
                >
                  <input type="radio" name="export-format" :value="f.id" v-model="exportFormatId" class="mt-0.5" />
                  <span class="flex-1">
                    <span class="text-ch-text font-medium">{{ f.name }}</span>
                    <span class="block text-ch-muted text-xs">{{ f.description }}</span>
                  </span>
                </label>
              </div>
            </div>
            <label class="flex items-center gap-2 cursor-pointer select-none">
              <input type="checkbox" v-model="exportIncludePassword" />
              <span class="text-ch-text text-xs">{{ t('menu.exportIncludePassword') }}</span>
            </label>
            <p v-if="exportIncludePassword" class="text-ch-accent text-xs">{{ t('menu.exportPasswordWarn') }}</p>
          </div>
          <div class="px-4 py-3 border-t border-ch-border flex justify-between items-center gap-2">
            <button class="btn-ghost btn-sm" :disabled="exporting" @click="copyExport">
              {{ copyDone ? t('menu.copied') : t('menu.copyToClipboard') }}
            </button>
            <div class="flex gap-2">
              <button class="btn-ghost btn-sm" :disabled="exporting" @click="downloadExport">{{ t('menu.quickDownload') }}</button>
              <button class="btn-ghost btn-sm" @click="showExport = false">{{ t('common.cancel') }}</button>
              <button class="btn-primary btn-sm" :disabled="exporting" @click="saveExportAs">
                {{ exporting ? t('menu.exporting') : t('menu.saveAs') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
    <!-- Save Success Toast -->
    <Teleport to="body">
      <div
        v-if="savedPath"
        class="fixed bottom-6 right-6 z-[110] bg-ch-panel border border-ch-accent rounded-lg shadow-2xl px-4 py-3 max-w-[420px] flex items-start gap-3"
      >
        <span class="text-ch-accent">✓</span>
        <div class="flex-1 min-w-0">
          <p class="text-ch-text text-sm font-medium">{{ t('menu.savedTo') }}</p>
          <p class="text-ch-muted text-xs mt-1 break-all">{{ savedPath }}</p>
        </div>
        <button class="text-ch-muted hover:text-ch-text" @click="savedPath = ''">✕</button>
      </div>
    </Teleport>
    <!-- Import Result Toast -->
    <Teleport to="body">
      <div
        v-if="importSummary"
        class="fixed bottom-6 right-6 z-[110] bg-ch-panel border border-ch-accent rounded-lg shadow-2xl px-4 py-3 max-w-[420px] flex items-start gap-3"
      >
        <span class="text-ch-accent">✓</span>
        <div class="flex-1 min-w-0">
          <p class="text-ch-text text-sm font-medium">
            {{ importSummary.mode === 'merge' ? t('menu.modeMerge') : t('menu.importConfirmOk') }} {{ t('menu.importDone') }}
          </p>
          <p class="text-ch-muted text-xs mt-1">
            {{ t('menu.importResultDetail', {
              folders: importSummary.importedFolders,
              conns: importSummary.importedConns,
              totalFolders: importSummary.totalFolders,
              totalConns: importSummary.totalConns,
            }) }}
          </p>
        </div>
        <button class="text-ch-muted hover:text-ch-text" @click="importSummary = null">✕</button>
      </div>
    </Teleport>
  </div>
</template>

<style>
[data-menu-bar] { position: relative; }
.menu-item:focus { outline: none; }
</style>