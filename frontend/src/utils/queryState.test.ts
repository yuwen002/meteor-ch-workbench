import { describe, it, expect } from 'vitest'
import {
  shouldCancelPrevious,
  reconcileQueryIds,
  isStaleResult,
  addPending,
  removePending,
} from './queryState'

describe('shouldCancelPrevious', () => {
  const cases: { name: string; input: string | null | undefined; want: string | null }[] = [
    { name: '空字符串无需取消', input: '', want: null },
    { name: 'null 无需取消', input: null, want: null },
    { name: 'undefined 无需取消', input: undefined, want: null },
    { name: '纯空白视为空', input: '   ', want: null },
    { name: '有 queryId 需要取消', input: 'q-1', want: 'q-1' },
    { name: '两侧空白被裁剪', input: '  q-2  ', want: 'q-2' },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(shouldCancelPrevious(c.input)).toBe(c.want)
    })
  }
})

describe('reconcileQueryIds', () => {
  const cases: {
    name: string
    pending: string[] | null
    current: string | null
    cancel: string[]
    keep: string[]
  }[] = [
    {
      name: 'pending 为空时无需取消',
      pending: [],
      current: 'q-1',
      cancel: [],
      keep: [],
    },
    {
      name: 'pending 为 null 时无需取消',
      pending: null,
      current: 'q-1',
      cancel: [],
      keep: [],
    },
    {
      name: '当前 id 保留，其余全部取消',
      pending: ['q-1', 'q-2', 'q-3'],
      current: 'q-2',
      cancel: ['q-1', 'q-3'],
      keep: ['q-2'],
    },
    {
      name: '当前 id 为空时全部取消',
      pending: ['q-1', 'q-2'],
      current: null,
      cancel: ['q-1', 'q-2'],
      keep: [],
    },
    {
      name: '空白 id 被忽略',
      pending: ['q-1', '  ', ''],
      current: 'q-1',
      cancel: [],
      keep: ['q-1'],
    },
    {
      name: '两侧空白参与比对时被裁剪',
      pending: ['  q-1  '],
      current: 'q-1',
      cancel: [],
      keep: ['q-1'],
    },
  ]
  for (const c of cases) {
    it(c.name, () => {
      const out = reconcileQueryIds(c.pending, c.current)
      expect(out.cancel).toEqual(c.cancel)
      expect(out.keep).toEqual(c.keep)
    })
  }
})

describe('isStaleResult', () => {
  const cases: {
    name: string
    queryId: string | null
    current: string | null
    pending: string[] | null
    want: boolean
  }[] = [
    { name: '空 queryId 视为过期', queryId: null, current: 'q-1', pending: ['q-1'], want: true },
    { name: '当前标签无 queryId 视为过期', queryId: 'q-1', current: null, pending: ['q-1'], want: true },
    { name: '不是当前 queryId 视为过期', queryId: 'q-2', current: 'q-1', pending: ['q-1', 'q-2'], want: true },
    { name: '等于当前但不在 pending 视为过期', queryId: 'q-1', current: 'q-1', pending: [], want: true },
    { name: '等于当前且在 pending 为有效结果', queryId: 'q-1', current: 'q-1', pending: ['q-1'], want: false },
    {
      name: '多个在途中命中当前为有效结果',
      queryId: 'q-2',
      current: 'q-2',
      pending: ['q-1', 'q-2'],
      want: false,
    },
    { name: 'pending 为 null 时视为过期', queryId: 'q-1', current: 'q-1', pending: null, want: true },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(isStaleResult(c.queryId, c.current, c.pending)).toBe(c.want)
    })
  }
})

describe('addPending', () => {
  it('空列表追加新 id', () => {
    expect(addPending([], 'q-1')).toEqual(['q-1'])
  })
  it('null 视为空列表', () => {
    expect(addPending(null, 'q-1')).toEqual(['q-1'])
  })
  it('重复 id 不重复追加', () => {
    expect(addPending(['q-1'], 'q-1')).toEqual(['q-1'])
  })
  it('空 id 不追加', () => {
    expect(addPending(['q-1'], '')).toEqual(['q-1'])
    expect(addPending(['q-1'], '   ')).toEqual(['q-1'])
  })
  it('不修改原数组', () => {
    const src = ['q-1']
    const out = addPending(src, 'q-2')
    expect(src).toEqual(['q-1'])
    expect(out).toEqual(['q-1', 'q-2'])
  })
})

describe('removePending', () => {
  it('移除指定 id', () => {
    expect(removePending(['q-1', 'q-2'], 'q-1')).toEqual(['q-2'])
  })
  it('不存在时保持不变', () => {
    expect(removePending(['q-1'], 'q-9')).toEqual(['q-1'])
  })
  it('null 与 null 组合返回空数组', () => {
    expect(removePending(null, null)).toEqual([])
  })
  it('空白 id 只清掉空白项', () => {
    expect(removePending(['q-1', '  '], '')).toEqual(['q-1'])
  })
})