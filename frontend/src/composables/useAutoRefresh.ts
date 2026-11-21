import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  AUTO_REFRESH_PRESETS,
  AUTO_REFRESH_STORAGE_KEY,
  normalizeSeconds,
  parseStoredSeconds,
} from '../utils/autoRefresh'

export interface UseAutoRefreshOptions {
  querying: () => boolean
  onRefresh: () => void
}

export function useAutoRefresh(options: UseAutoRefreshOptions) {
  const { t } = useI18n()

  const autoRefreshSeconds = ref(
    normalizeSeconds(parseStoredSeconds(localStorage.getItem(AUTO_REFRESH_STORAGE_KEY))),
  )
  const autoRefreshMenuVisible = ref(false)
  const autoRefreshCustomValue = ref('')
  const autoRefreshCustomError = ref<string | null>(null)
  const autoRefreshRemaining = ref(0)
  const pickAutoRefreshOpen = ref(false)
  const autoRefreshMenuX = ref(0)
  const autoRefreshMenuY = ref(0)

  let refreshTimer: number | undefined
  let lastRefreshAt = 0

  const refreshEnabled = computed(() => autoRefreshSeconds.value > 0)
  const refreshOptions = computed(() =>
    AUTO_REFRESH_PRESETS.map((v) => ({ value: v, label: t('editor.refreshSeconds', { seconds: v }) })),
  )

  function stopTimer() {
    if (refreshTimer) {
      clearInterval(refreshTimer)
      refreshTimer = undefined
    }
    autoRefreshRemaining.value = 0
  }

  function tick() {
    const total = normalizeSeconds(autoRefreshSeconds.value)
    if (total <= 0) {
      stopTimer()
      return
    }
    const elapsed = Math.floor((Date.now() - lastRefreshAt) / 1000)
    const left = Math.max(0, total - elapsed)
    autoRefreshRemaining.value = left
    if (left <= 0 && !options.querying()) triggerRefresh()
  }

  function startTimer() {
    stopTimer()
    lastRefreshAt = Date.now()
    autoRefreshRemaining.value = normalizeSeconds(autoRefreshSeconds.value)
    refreshTimer = window.setInterval(tick, 1000)
  }

  function triggerRefresh() {
    lastRefreshAt = Date.now()
    autoRefreshRemaining.value = normalizeSeconds(autoRefreshSeconds.value)
    options.onRefresh()
  }

  function chooseAutoRefresh(v: number) {
    const seconds = normalizeSeconds(v)
    autoRefreshSeconds.value = seconds
    if (seconds > 0) {
      localStorage.setItem(AUTO_REFRESH_STORAGE_KEY, String(seconds))
      startTimer()
    } else {
      localStorage.removeItem(AUTO_REFRESH_STORAGE_KEY)
      stopTimer()
    }
  }

  function openAutoRefreshCustom() {
    autoRefreshCustomValue.value = autoRefreshSeconds.value > 0 ? String(autoRefreshSeconds.value) : ''
    autoRefreshCustomError.value = null
    autoRefreshMenuVisible.value = true
  }

  function closeAutoRefreshCustom() {
    autoRefreshMenuVisible.value = false
    autoRefreshCustomError.value = null
  }

  function manualRefresh() {
    lastRefreshAt = Date.now()
    autoRefreshRemaining.value = normalizeSeconds(autoRefreshSeconds.value)
    options.onRefresh()
  }

  function toggleAutoRefreshMenu(e: MouseEvent) {
    if (pickAutoRefreshOpen.value) {
      pickAutoRefreshOpen.value = false
      return
    }
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
    autoRefreshMenuX.value = rect.right - 140
    autoRefreshMenuY.value = rect.bottom + 4
    pickAutoRefreshOpen.value = true
  }

  watch(
    () => options.querying(),
    (now, before) => {
      if (before && !now && refreshEnabled.value) startTimer()
    },
  )

  watch(refreshEnabled, (on) => {
    if (on) startTimer()
    else stopTimer()
  })

  onBeforeUnmount(stopTimer)

  return {
    autoRefreshSeconds,
    autoRefreshMenuVisible,
    autoRefreshCustomValue,
    autoRefreshCustomError,
    autoRefreshRemaining,
    pickAutoRefreshOpen,
    autoRefreshMenuX,
    autoRefreshMenuY,
    refreshEnabled,
    refreshOptions,
    toggleAutoRefreshMenu,
    chooseAutoRefresh,
    openAutoRefreshCustom,
    closeAutoRefreshCustom,
    manualRefresh,
  }
}