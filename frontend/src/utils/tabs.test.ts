import { describe, it, expect } from 'vitest'
import { createTab } from './tabs'

describe('createTab', () => {
  it('生成的标签初始状态为空结果', () => {
    const tab = createTab(1)
    expect(tab.result).toBeNull()
    expect(tab.error).toBeNull()
    expect(tab.querying).toBe(false)
    expect(tab.queryId).toBeNull()
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