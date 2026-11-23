import { describe, expect, it } from 'vitest'
import ResultToolbarActions from './ResultToolbarActions.vue'
import { mountWithI18n, textOf } from './testUtils'

function baseProps(overrides: Record<string, unknown> = {}) {
  return {
    querying: false,
    refreshEnabled: false,
    refreshSeconds: 0,
    canExplain: true,
    explainMode: 'plan',
    explainModes: [
      { id: 'plan', labelKey: 'editor.explainPlan' },
      { id: 'ast', labelKey: 'editor.explainAst' },
    ],
    hasColumns: true,
    exporting: false,
    filterEnabled: true,
    filterHasActive: false,
    activeFilterCount: 0,
    ...overrides,
  }
}

describe('ResultToolbarActions 组件', () => {
  const labelCases: Array<{ name: string; props: Record<string, unknown>; expect: string[] }> = [
    {
      name: '自动刷新关闭时显示 No auto refresh',
      props: { refreshEnabled: false },
      expect: ['No auto refresh'],
    },
    {
      name: '自动刷新开启时显示间隔秒数',
      props: { refreshEnabled: true, refreshSeconds: 15 },
      expect: ['15'],
    },
    {
      name: '有激活过滤条件时显示数量角标',
      props: { filterHasActive: true, activeFilterCount: 3 },
      expect: ['3'],
    },
    {
      name: '无激活过滤条件时不显示角标',
      props: { filterHasActive: false, activeFilterCount: 0 },
      expect: ['Filter'],
    },
  ]

  for (const c of labelCases) {
    it(c.name, () => {
      const wrapper = mountWithI18n(ResultToolbarActions, { props: baseProps(c.props) })
      const text = textOf(wrapper)
      for (const s of c.expect) expect(text).toContain(s)
    })
  }

  const disabledCases: Array<{ name: string; props: Record<string, unknown>; title: string; expectDisabled: boolean }> = [
    { name: '查询进行中时刷新按钮禁用', props: { querying: true }, title: 'Run the current SQL again', expectDisabled: true },
    { name: '空闲时刷新按钮可用', props: {}, title: 'Run the current SQL again', expectDisabled: false },
    { name: 'canExplain 为假时 Explain 按钮禁用', props: { canExplain: false }, title: 'Run EXPLAIN on the selected SQL (or the whole editor)', expectDisabled: true },
    { name: 'canExplain 为真时 Explain 按钮可用', props: {}, title: 'Run EXPLAIN on the selected SQL (or the whole editor)', expectDisabled: false },
    { name: '无结果列时导出按钮禁用', props: { hasColumns: false }, title: 'Export the current result set', expectDisabled: true },
    { name: '导出中导出按钮禁用', props: { exporting: true }, title: 'Export the current result set', expectDisabled: true },
    { name: '有列且未导出时导出按钮可用', props: {}, title: 'Export the current result set', expectDisabled: false },
    { name: '过滤功能关闭时过滤按钮禁用', props: { filterEnabled: false }, title: 'Only a single SELECT / WITH statement is supported', expectDisabled: true },
    { name: '查询进行中过滤按钮禁用', props: { querying: true }, title: 'Server-side filter: builds a WHERE clause and re-runs the query; combines with local search and local sort', expectDisabled: true },
  ]

  for (const c of disabledCases) {
    it(c.name, () => {
      const wrapper = mountWithI18n(ResultToolbarActions, { props: baseProps(c.props) })
      const btn = wrapper.findAll('button').find((b) => b.attributes('title') === c.title)
      expect(btn).toBeDefined()
      expect(btn!.element.hasAttribute('disabled')).toBe(c.expectDisabled)
    })
  }

  it('Explain 模式下拉框列出全部模式并按 change 发出 update:explain-mode', async () => {
    const wrapper = mountWithI18n(ResultToolbarActions, { props: baseProps() })
    const select = wrapper.find('select')
    expect(select.findAll('option')).toHaveLength(2)
    await select.setValue('ast')
    expect(wrapper.emitted('update:explain-mode')).toEqual([['ast']])
  })

  it('canExplain 为假时下拉框禁用', () => {
    const wrapper = mountWithI18n(ResultToolbarActions, { props: baseProps({ canExplain: false }) })
    expect(wrapper.find('select').element.hasAttribute('disabled')).toBe(true)
  })

  it('查询进行中时下拉框禁用', () => {
    const wrapper = mountWithI18n(ResultToolbarActions, { props: baseProps({ querying: true }) })
    expect(wrapper.find('select').element.hasAttribute('disabled')).toBe(true)
  })

  it('点击刷新按钮发出 manual-refresh', async () => {
    const wrapper = mountWithI18n(ResultToolbarActions, { props: baseProps() })
    const refreshBtn = wrapper.findAll('button').find((b) => textOf(b).includes('Refresh'))
    await refreshBtn!.trigger('click')
    expect(wrapper.emitted('manual-refresh')).toHaveLength(1)
  })

  it('点击间隔按钮发出 toggle-refresh-menu 并携带事件对象', async () => {
    const wrapper = mountWithI18n(ResultToolbarActions, { props: baseProps() })
    const intervalBtn = wrapper.findAll('button').find((b) => b.attributes('title') === 'Auto refresh interval')
    await intervalBtn!.trigger('click')
    const ev = wrapper.emitted('toggle-refresh-menu')
    expect(ev).toHaveLength(1)
    expect(ev![0][0]).toBeInstanceOf(Event)
  })

  it('点击 Explain 按钮发出 explain', async () => {
    const wrapper = mountWithI18n(ResultToolbarActions, { props: baseProps() })
    const explainBtn = wrapper.findAll('button').find((b) => b.attributes('title') === 'Run EXPLAIN on the selected SQL (or the whole editor)')
    await explainBtn!.trigger('click')
    expect(wrapper.emitted('explain')).toHaveLength(1)
  })

  it('Explain 按钮禁用时不发出 explain', async () => {
    const wrapper = mountWithI18n(ResultToolbarActions, { props: baseProps({ canExplain: false }) })
    const explainBtn = wrapper.findAll('button').find((b) => b.attributes('title') === 'Run EXPLAIN on the selected SQL (or the whole editor)')
    await explainBtn!.trigger('click')
    expect(wrapper.emitted('explain')).toBeUndefined()
  })

  it('点击导出按钮发出 toggle-export', async () => {
    const wrapper = mountWithI18n(ResultToolbarActions, { props: baseProps() })
    const exportBtn = wrapper.findAll('button').find((b) => b.attributes('title') === 'Export the current result set')
    await exportBtn!.trigger('click')
    expect(wrapper.emitted('toggle-export')).toHaveLength(1)
  })

  it('点击过滤按钮发出 open-filter', async () => {
    const wrapper = mountWithI18n(ResultToolbarActions, { props: baseProps() })
    const filterBtn = wrapper
      .findAll('button')
      .find(
        (b) =>
          b.attributes('title') ===
          'Server-side filter: builds a WHERE clause and re-runs the query; combines with local search and local sort',
      )
    await filterBtn!.trigger('click')
    expect(wrapper.emitted('open-filter')).toHaveLength(1)
  })
})