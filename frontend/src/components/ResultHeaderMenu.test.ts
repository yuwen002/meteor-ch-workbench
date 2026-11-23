import { afterEach, describe, expect, it } from 'vitest'
import ResultHeaderMenu from './ResultHeaderMenu.vue'
import { bodyText, mountWithI18n } from './testUtils'

function baseProps(overrides: Record<string, unknown> = {}) {
  return {
    visible: true,
    x: 100,
    y: 200,
    columnName: 'user_id',
    canExplain: true,
    canEdit: true,
    userKeyIndex: -1,
    isKey: false,
    editKeyHint: 'Pick a unique column to locate the edited row',
    hiddenCount: 0,
    ...overrides,
  }
}

function buttons() {
  return Array.from(document.body.querySelectorAll('button')) as HTMLButtonElement[]
}

function findButton(label: string): HTMLButtonElement | undefined {
  return buttons().find((b) => (b.textContent ?? '').replace(/\s+/g, ' ').includes(label))
}

describe('ResultHeaderMenu 组件', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('visible 为假时不渲染任何菜单内容', () => {
    mountWithI18n(ResultHeaderMenu, { props: baseProps({ visible: false }) })
    expect(bodyText()).toBe('')
  })

  it('visible 为真时渲染复制与隐藏类菜单项', () => {
    mountWithI18n(ResultHeaderMenu, { props: baseProps() })
    const text = bodyText()
    expect(text).toContain('Copy column name')
    expect(text).toContain('Copy column names')
    expect(text).toContain('Copy all values of this field')
    expect(text).toContain('Copy distinct values of this field')
    expect(text).toContain('Search only in')
    expect(text).toContain('Hide this column')
    expect(text).toContain('Show all hidden columns')
  })

  it('菜单按 x / y 定位', () => {
    mountWithI18n(ResultHeaderMenu, { props: baseProps({ x: 12, y: 34 }) })
    const panel = document.body.querySelector('div[style*="left"]') as HTMLElement
    expect(panel.style.left).toBe('12px')
    expect(panel.style.top).toBe('34px')
  })

  it('canExplain 为假时不渲染排序与按值过滤项', () => {
    mountWithI18n(ResultHeaderMenu, { props: baseProps({ canExplain: false }) })
    const text = bodyText()
    expect(text).not.toContain('ASC')
    expect(text).not.toContain('DESC')
    expect(text).not.toContain('Filter by this value')
  })

  it('canExplain 为真时排序项携带列名', () => {
    mountWithI18n(ResultHeaderMenu, { props: baseProps({ columnName: 'city' }) })
    const text = bodyText()
    expect(text).toContain('city ASC')
    expect(text).toContain('city DESC')
  })

  it('isKey 为假时显示「设为编辑键」，为真时显示「取消编辑键」', () => {
    const off = mountWithI18n(ResultHeaderMenu, { props: baseProps({ isKey: false }) })
    const offText = bodyText()
    off.unmount()
    document.body.innerHTML = ''
    mountWithI18n(ResultHeaderMenu, { props: { ...baseProps({ isKey: true, userKeyIndex: 2 }) } })
    const onText = bodyText()
    expect(offText).toContain('Use as edit key column')
    expect(offText).not.toContain('Cancel')
    expect(onText).toContain('Cancel as edit key column')
  })

  it('canEdit 为假时设为编辑键按钮禁用', () => {
    mountWithI18n(ResultHeaderMenu, { props: baseProps({ canEdit: false }) })
    expect(findButton('Use as edit key column')!.disabled).toBe(true)
  })

  it('hiddenCount 为 0 时显示全部列按钮禁用，大于 0 时可用并显示数量', () => {
    const zero = mountWithI18n(ResultHeaderMenu, { props: baseProps({ hiddenCount: 0 }) })
    const zeroBtn = findButton('Show all hidden columns')!
    expect(zeroBtn.disabled).toBe(true)
    zero.unmount()
    document.body.innerHTML = ''
    mountWithI18n(ResultHeaderMenu, { props: baseProps({ hiddenCount: 3 }) })
    const someBtn = findButton('Show all hidden columns')!
    expect(someBtn.disabled).toBe(false)
    expect((someBtn.textContent ?? '').replace(/\s+/g, ' ')).toContain('3')
  })

  it('editKeyHint 原样渲染', () => {
    mountWithI18n(ResultHeaderMenu, { props: baseProps({ editKeyHint: 'CUSTOM HINT' }) })
    expect(bodyText()).toContain('CUSTOM HINT')
  })

  const emitCases: Array<{ name: string; label: string; event: string; payload?: unknown[] }> = [
    { name: '点击复制列名发出 copy-column-name', label: 'Copy column name', event: 'copy-column-name' },
    { name: '点击复制全部列名发出 copy-all-columns', label: 'Copy column names', event: 'copy-all-columns' },
    { name: '点击复制列值发出 copy-column-values 且 distinct 为 false', label: 'Copy all values of this field', event: 'copy-column-values', payload: [false] },
    { name: '点击复制去重列值发出 copy-column-values 且 distinct 为 true', label: 'Copy distinct values of this field', event: 'copy-column-values', payload: [true] },
    { name: '点击仅搜索本列发出 search-only', label: 'Search only in', event: 'search-only' },
    { name: '点击升序发出 sort 且参数为 ASC', label: 'ASC', event: 'sort', payload: ['ASC'] },
    { name: '点击降序发出 sort 且参数为 DESC', label: 'DESC', event: 'sort', payload: ['DESC'] },
    { name: '点击按值过滤发出 filter-by-value', label: 'Filter by this value', event: 'filter-by-value' },
    { name: '点击隐藏列发出 hide-column', label: 'Hide this column', event: 'hide-column' },
    { name: '点击隐藏列并改 SQL 发出 hide-column-sql', label: 'Hide this column (rewrite SQL)', event: 'hide-column-sql' },
    { name: '点击设为编辑键发出 toggle-key', label: 'Use as edit key column', event: 'toggle-key' },
    { name: '点击显示全部列发出 show-all-columns', label: 'Show all hidden columns', event: 'show-all-columns' },
  ]

  for (const c of emitCases) {
    it(c.name, async () => {
      const wrapper = mountWithI18n(ResultHeaderMenu, {
        props: baseProps({ hiddenCount: 1, columnName: 'k' }),
      })
      const btn = findButton(c.label)
      expect(btn).toBeDefined()
      btn!.click()
      const emitted = (wrapper.emitted(c.event) ?? []) as unknown[][]
      expect(emitted).toHaveLength(1)
      if (c.payload) expect(emitted[0]).toEqual(c.payload)
    })
  }
})