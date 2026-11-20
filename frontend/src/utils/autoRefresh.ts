export const AUTO_REFRESH_DISABLED = 0
export const MIN_REFRESH_SECONDS = 1
export const MAX_REFRESH_SECONDS = 86400
export const AUTO_REFRESH_PRESETS = [5, 10, 30, 60, 300] as const
export const AUTO_REFRESH_STORAGE_KEY = 'meteor-ch.autoRefreshSeconds'

export function normalizeSeconds(input: number | string | null | undefined): number {
  if (input === null || input === undefined) return AUTO_REFRESH_DISABLED
  const raw = typeof input === 'number' ? input : Number(String(input).trim())
  if (!Number.isFinite(raw)) return AUTO_REFRESH_DISABLED
  if (raw < MIN_REFRESH_SECONDS) return AUTO_REFRESH_DISABLED
  return Math.min(MAX_REFRESH_SECONDS, Math.floor(raw))
}

export function isRefreshEnabled(seconds: number): boolean {
  return normalizeSeconds(seconds) > 0
}

export function intervalMs(seconds: number): number {
  return normalizeSeconds(seconds) * 1000
}

export function isCustomPreset(seconds: number): boolean {
  const n = normalizeSeconds(seconds)
  return n > 0 && !AUTO_REFRESH_PRESETS.includes(n as (typeof AUTO_REFRESH_PRESETS)[number])
}

export function formatSeconds(seconds: number): string {
  const n = normalizeSeconds(seconds)
  if (n <= 0) return '0'
  if (n < 60) return String(n)
  const m = Math.floor(n / 60)
  const s = n % 60
  return s === 0 ? String(m * 60) : `${m * 60 + s}`
}

export interface CountdownInput {
  enabled: boolean
  intervalSeconds: number
  lastRunAt: number
  now: number
}

export function remainingSeconds(input: CountdownInput): number {
  if (!input.enabled) return 0
  const total = normalizeSeconds(input.intervalSeconds)
  if (total <= 0) return 0
  const elapsed = Math.max(0, Math.floor((input.now - input.lastRunAt) / 1000))
  return Math.max(0, total - elapsed)
}

export function shouldFire(input: CountdownInput): boolean {
  if (!input.enabled) return false
  const total = normalizeSeconds(input.intervalSeconds)
  if (total <= 0) return false
  return input.now - input.lastRunAt >= total * 1000
}

export function parseStoredSeconds(raw: string | null): number {
  if (raw === null) return AUTO_REFRESH_DISABLED
  return normalizeSeconds(raw)
}