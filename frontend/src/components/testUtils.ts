import { mount, type ComponentMountingOptions } from '@vue/test-utils'
import type { Component } from 'vue'
import { createI18n } from 'vue-i18n'
import en from '../locales/en'
import zh from '../locales/zh'

export function createTestI18n(locale: 'en' | 'zh' = 'en') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en',
    missingWarn: false,
    fallbackWarn: false,
    messages: { en, zh },
  })
}

export function mountWithI18n<T extends Component>(
  component: T,
  options: ComponentMountingOptions<T> = {} as ComponentMountingOptions<T>,
) {
  return mount(component, {
    ...options,
    global: {
      ...(options.global ?? {}),
      plugins: [createTestI18n(), ...((options.global?.plugins as any[]) ?? [])],
    },
  })
}

export function textOf(wrapper: { text: () => string }): string {
  return wrapper.text().replace(/\s+/g, ' ').trim()
}

export function bodyText(): string {
  return document.body.textContent?.replace(/\s+/g, ' ').trim() ?? ''
}