import { describe, expect, it } from 'vitest'
import ResultToolbarInfo from './ResultToolbarInfo.vue'
import { mountWithI18n, textOf } from './testUtils'
import type { QueryResult } from '../types'

function baseProps(overrides: Record<string, unknown> = {}) {
  return {
    result: null,
    executedSql: null,
    executedSqlModified: false,
    loadingWithResult: false,
    refreshEnabled: false,
    refreshRemaining: 0,
    copiedHint: null,
    searchText: '',
    searchActive: false,
    filteredShown: 0,
    ...overrides,
  }
}

function makeResult(rowCount: number, elapsedMs: number, rows: unknown[][] = []): QueryResult {
  return { columns: [], rows, row_count: rowCount, elapsed_ms: elapsedMs, error: null } as unknown as QueryResult
}

describe('ResultToolbarInfo 组件', () => {
  const cases: Array<{ name: string; props: Record<string, unknown>; expect: string[]; absent?: string[] }> = [
    {
      name: '无结果时只显示标题，不显示行数与耗时',
      props: { result: null },
      expect: ['Results'],
      absent: ['rows', 'ms'],
    },
    {
      name: '有结果时显示带千分位的行数与两位小数耗时',
      props: { result: makeResult(1234, 12.345) },
      expect: ['1,234', '12.35 ms'],
    },
    {
      name: '零行结果仍显示 0 行与 0.00 ms',
      props: { result: makeResult(0, 0) },
      expect: ['0', '0.00 ms'],
    },
    {
      name: '刷新中且保留旧结果时提示 refreshing 而非倒计时',
      props: { result: makeResult(1, 1), loadingWithResult: true },
      expect: ['Refreshing'],
      absent: ['Refresh in'],
    },
    {
      name: '开启自动刷新且不在刷新中时显示倒计时文案',
      props: { refreshEnabled: true, refreshRemaining: 7 },
      expect: ['Refresh in 7 s'],
    },
    {
      name: '复制提示存在时显示复制文案',
      props: { copiedHint: 'Copied 3 rows' },
      expect: ['Copied 3 rows'],
    },
    {
      name: '搜索生效且有结果时显示过滤统计',
      props: { result: makeResult(10, 2, [[1], [2]]), searchActive: true, filteredShown: 2 },
      expect: ['2 / 2'],
    },
  ]

  for (const c of cases) {
    it(c.name, () => {
      const wrapper = mountWithI18n(ResultToolbarInfo, { props: baseProps(c.props) })
      const text = textOf(wrapper)
      for (const s of c.expect) expect(text).toContain(s)
      for (const s of c.absent ?? []) expect(text).not.toContain(s)
    })
  }

  it('搜索框为空时不渲染按钮，有值时渲染清除按钮', () => {
    const empty = mountWithI18n(ResultToolbarInfo, { props: baseProps({ searchText: '' }) })
    const filled = mountWithI18n(ResultToolbarInfo, { props: baseProps({ searchText: 'abc' }) })
    expect(empty.findAll('button')).toHaveLength(0)
    expect(filled.findAll('button')).toHaveLength(1)
  })

  it('executedSql 为空时不渲染查看 SQL 按钮', () => {
    const wrapper = mountWithI18n(ResultToolbarInfo, { props: baseProps({ executedSql: null }) })
    expect(textOf(wrapper)).not.toContain('View SQL')
  })

  it('executedSql 非空时渲染查看 SQL 按钮并点击发出 toggle-sql', async () => {
    const wrapper = mountWithI18n(ResultToolbarInfo, { props: baseProps({ executedSql: 'SELECT 1' }) })
    expect(textOf(wrapper)).toContain('View SQL')
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('toggle-sql')).toHaveLength(1)
  })

  it('搜索框输入时发出 update:search-text 事件并携带输入值', async () => {
    const wrapper = mountWithI18n(ResultToolbarInfo, { props: baseProps() })
    await wrapper.find('input').setValue('hello')
    expect(wrapper.emitted('update:search-text')).toEqual([['hello']])
  })

  it('清除按钮点击后发出 clear-search 事件', async () => {
    const wrapper = mountWithI18n(ResultToolbarInfo, { props: baseProps({ searchText: 'x' }) })
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('clear-search')).toHaveLength(1)
  })

  it('executedSqlModified 为真时在查看 SQL 按钮内渲染改动圆点', () => {
    const wrapper = mountWithI18n(ResultToolbarInfo, {
      props: baseProps({ executedSql: 'SELECT 1', executedSqlModified: true }),
    })
    expect(wrapper.findAll('.bg-ch-accent').length).toBeGreaterThan(0)
  })
})