import { describe, it, expect } from 'vitest'
import { createTab, restoreTabs } from './tabs'

describe('createTab', () => {
  it('生成的标签初始状态为空结果', () => {
    const tab = createTab(1)
    expect(tab.result).toBeNull()
    expect(tab.error).toBeNull()
    expect(tab.querying).toBe(false)
    expect(tab.queryId).toBeNull()
    expect(tab.pendingQueryIds).toEqual([])
    expect(tab.canceled).toBe(false)
    expect(tab.resultScrollTop).toBe(0)
    expect(tab.viewState).toBeNull()
  })

  it('标题包含序号', () => {
    expect(createTab(3).title).toBe('Query 3')
  })

  it('可注入初始 SQL', () => {
    expect(createTab(1, 'SELECT 1').sql).toBe('SELECT 1')
  })

  it('默认 SQL 为带注释的示例', () => {
    expect(createTab(1).sql).toContain('SELECT version()')
  })

  it('每次生成唯一 ID', () => {
    const ids = new Set([createTab(1).id, createTab(1).id, createTab(1).id])
    expect(ids.size).toBe(3)
  })
})

describe('restoreTabs', () => {
  it('正常快照按顺序恢复并选中活动标签', () => {
    const out = restoreTabs({
      tabs: [
        { id: 'a', title: 'Query 1', sql: 'SELECT 1' },
        { id: 'b', title: 'Query 2', sql: 'SELECT 2' },
      ],
      active_tab_id: 'b',
    })
    expect(out.tabs).toHaveLength(2)
    expect(out.activeId).toBe('b')
    expect(out.tabs[1].sql).toBe('SELECT 2')
  })

  it('活动标签不存在时回退到第一个', () => {
    const out = restoreTabs({
      tabs: [{ id: 'a', title: 'Query 1', sql: 'SELECT 1' }],
      active_tab_id: 'missing',
    })
    expect(out.activeId).toBe('a')
  })

  it('空标签列表回退到默认标签，不返回 undefined', () => {
    const out = restoreTabs({ tabs: [], active_tab_id: '' })
    expect(out.tabs).toHaveLength(1)
    expect(out.activeId).toBe(out.tabs[0].id)
  })

  it('tabs 为 null 时回退到默认标签', () => {
    const out = restoreTabs({ tabs: null as unknown as [], active_tab_id: '' })
    expect(out.tabs).toHaveLength(1)
  })

  it('快照条目为 null 时用占位标题补齐，不抛错', () => {
    const out = restoreTabs({ tabs: [null], active_tab_id: '' })
    expect(out.tabs).toHaveLength(1)
    expect(out.tabs[0].title).toBe('Query 1')
    expect(out.tabs[0].sql).toBe('')
  })

  it('缺字段的条目用默认值补齐', () => {
    const out = restoreTabs({ tabs: [{}], active_tab_id: '' })
    expect(out.tabs[0].result).toBeNull()
    expect(out.tabs[0].page).toBeNull()
    expect(out.tabs[0].lastExecutedSql).toBe('')
  })
})