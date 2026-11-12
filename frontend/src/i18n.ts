import { createI18n } from 'vue-i18n'
import en from './locales/en'
import zh from './locales/zh'

export type Locale = 'en' | 'zh'

function getInitialLocale(): Locale {
  const saved = localStorage.getItem('meteor-locale') as Locale | null
  if (saved === 'en' || saved === 'zh') return saved
  const nav = navigator.language.toLowerCase()
  if (nav.startsWith('zh')) return 'zh'
  return 'en'
}

export const i18n = createI18n({
  legacy: false,
  locale: getInitialLocale(),
  fallbackLocale: 'en',
  messages: { en, zh },
})

export function setLocale(locale: Locale) {
  i18n.global.locale.value = locale
  localStorage.setItem('meteor-locale', locale)
  document.documentElement.lang = locale
}

export function getCurrentLocale(): Locale {
  return (i18n.global.locale.value as Locale) || 'en'
}