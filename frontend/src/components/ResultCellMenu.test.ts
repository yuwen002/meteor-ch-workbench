import { afterEach, describe, expect, it } from 'vitest'
import ResultCellMenu from './ResultCellMenu.vue'
import { bodyText, mountWithI18n } from './testUtils'

function baseProps(overrides: Record<string, unknown> = {}) {
  return {
    visible: true,
    x: 50,
    y: 60,
    columnName: 'amount',
    canExplain: true,
    ...overrides,
  }
}

function buttons() {
  return Array.from(document.body.querySelectorAll('button')) as HTMLButtonElement[]
}

function findButton(label: string): HTMLButtonElement | undefined {
  return buttons().find((b) => (b.textContent ?? '').replace(/\s+/g, ' ').includes(label))
}

describe('ResultCellMenu 组件', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('visible 为假时不渲染任何菜单内容', () => {
    mountWithI18n(ResultCellMenu, { props: baseProps({ visible: false }) })
    expect(bodyText()).toBe('')
  })

  it('visible 为真时渲染复制类菜单项', () => {
    mountWithI18n(ResultCellMenu, { props: baseProps() })
    const text = bodyText()
    expect(text).toContain('Copy cell')
    expect(text).toContain('Copy as SQL literal')
    expect(text).toContain('Copy row (TSV)')
    expect(text).toContain('Copy as INSERT')
    expect(text).toContain('Copy all as INSERT')
  })

  it('菜单按 x / y 定位', () => {
    mountWithI18n(ResultCellMenu, { props: baseProps({ x: 7, y: 9 }) })
    const panel = document.body.querySelector('div[style*="left"]') as HTMLElement
    expect(panel.style.left).toBe('7px')
    expect(panel.style.top).toBe('9px')
  })

  it('canExplain 为假时不渲染排序与按值过滤项', () => {
    mountWithI18n(ResultCellMenu, { props: baseProps({ canExplain: false }) })
    const text = bodyText()
    expect(text).not.toContain('ASC')
    expect(text).not.toContain('DESC')
    expect(text).not.toContain('Filter by this value')
  })

  it('canExplain 为真时排序项携带列名', () => {
    mountWithI18n(ResultCellMenu, { props: baseProps({ columnName: 'price' }) })
    const text = bodyText()
    expect(text).toContain('price ASC')
    expect(text).toContain('price DESC')
  })

  const emitCases: Array<{ name: string; label: string; event: string; payload?: unknown[] }> = [
    { name: '点击复制单元格发出 copy-cell', label: 'Copy cell', event: 'copy-cell' },
    { name: '点击复制为 SQL 字面量发出 copy-cell-literal', label: 'Copy as SQL literal', event: 'copy-cell-literal' },
    { name: '点击复制整行发出 copy-row', label: 'Copy row (TSV)', event: 'copy-row' },
    { name: '点击复制为 INSERT 发出 copy-insert', label: 'Copy as INSERT', event: 'copy-insert' },
    { name: '点击复制全部为 INSERT 发出 copy-insert-all', label: 'Copy all as INSERT', event: 'copy-insert-all' },
    { name: '点击升序发出 sort 且参数为 ASC', label: 'ASC', event: 'sort', payload: ['ASC'] },
    { name: '点击降序发出 sort 且参数为 DESC', label: 'DESC', event: 'sort', payload: ['DESC'] },
    { name: '点击按值过滤发出 filter-by-value', label: 'Filter by this value', event: 'filter-by-value' },
  ]

  for (const c of emitCases) {
    it(c.name, () => {
      const wrapper = mountWithI18n(ResultCellMenu, { props: baseProps({ columnName: 'k' }) })
      const btn = findButton(c.label)
      expect(btn).toBeDefined()
      btn!.click()
      const emitted = (wrapper.emitted(c.event) ?? []) as unknown[][]
      expect(emitted).toHaveLength(1)
      if (c.payload) expect(emitted[0]).toEqual(c.payload)
    })
  }
})