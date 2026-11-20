import { describe, expect, it } from 'vitest'
import { cellText, collectColumnValues, columnListText, distinctColumnValues, filterRowIndices, isEmptyCell, rowMatches } from './resultView'
import type { ColumnInfo } from '../types'

const cols: ColumnInfo[] = [
  { name: 'name', type: 'String' },
  { name: 'size', type: 'UInt64' },
  { name: 'note', type: 'Nullable(String)' },
]

const rows: unknown[][] = [
  ['b', 2, null],
  ['a', 10, 'x'],
  ['a', 1, undefined],
]

describe('resultView 空值判定', () => {
  const cases: { name: string; value: unknown; empty: boolean }[] = [
    { name: 'null 视为空', value: null, empty: true },
    { name: 'undefined 视为空', value: undefined, empty: true },
    { name: '空字符串不算空', value: '', empty: false },
    { name: '数字 0 不算空', value: 0, empty: false },
    { name: '布尔 false 不算空', value: false, empty: false },
  ]

  it.each(cases)('$name', ({ value, empty }) => {
    expect(isEmptyCell(value)).toBe(empty)
  })
})

describe('resultView 列值收集', () => {
  it('按索引顺序收集并跳过空值', () => {
    expect(collectColumnValues(rows, [0, 1, 2], 0)).toEqual(['b', 'a', 'a'])
  })

  it('列全是空值时返回空数组', () => {
    expect(collectColumnValues(rows, [0, 1, 2], 2)).toEqual(['x'])
  })

  it('空索引集合返回空数组', () => {
    expect(collectColumnValues(rows, [], 0)).toEqual([])
  })

  it('只取指定索引', () => {
    expect(collectColumnValues(rows, [1], 1)).toEqual(['10'])
  })

  it('越界索引安全返回空数组', () => {
    expect(collectColumnValues(rows, [99], 0)).toEqual([])
  })

  it('去重保留首次出现顺序', () => {
    expect(distinctColumnValues(rows, [0, 1, 2], 0)).toEqual(['b', 'a'])
  })

  it('去重空集合返回空数组', () => {
    expect(distinctColumnValues(rows, [], 0)).toEqual([])
  })
})

describe('resultView 列名', () => {
  it('返回对应列名', () => {
    expect(columnListText(cols, 0)).toBe('name')
  })

  it('越界返回空字符串', () => {
    expect(columnListText(cols, 9)).toBe('')
  })

  it('只返回该列列名而非全部列名', () => {
    expect(columnListText(cols, 1)).toBe('size')
    expect(columnListText(cols, 1)).not.toContain('name')
  })
})

describe('resultView 单元格文本', () => {
  const cases: { name: string; value: unknown; want: string }[] = [
    { name: 'null 转空串', value: null, want: '' },
    { name: 'undefined 转空串', value: undefined, want: '' },
    { name: '字符串原样', value: 'abc', want: 'abc' },
    { name: '数字转字符串', value: 0, want: '0' },
    { name: '布尔转字符串', value: false, want: 'false' },
  ]
  it.each(cases)('$name', ({ value, want }) => {
    expect(cellText(value)).toBe(want)
  })
})

describe('resultView 行匹配', () => {
  const searchRows: unknown[][] = [
    ['Apple', 100, 'fruit'],
    ['banana', 20, null],
    ['Cherry', 5, 'red'],
    [null, null, null],
  ]

  it('空关键字匹配所有行', () => {
    expect(rowMatches(searchRows[0], '')).toBe(true)
  })

  it('全列忽略大小写匹配', () => {
    expect(rowMatches(searchRows[0], 'apple')).toBe(true)
    expect(rowMatches(searchRows[1], 'BAN')).toBe(true)
  })

  it('数字列参与匹配', () => {
    expect(rowMatches(searchRows[2], '5')).toBe(true)
  })

  it('无命中返回 false', () => {
    expect(rowMatches(searchRows[1], 'zzz')).toBe(false)
  })

  it('全 null 行不匹配任何关键字', () => {
    expect(rowMatches(searchRows[3], 'x')).toBe(false)
  })

  it('指定列时只匹配该列', () => {
    expect(rowMatches(searchRows[0], 'apple', 0)).toBe(true)
    expect(rowMatches(searchRows[0], 'fruit', 0)).toBe(false)
    expect(rowMatches(searchRows[0], 'fruit', 2)).toBe(true)
  })
})

describe('resultView 行索引过滤', () => {
  const searchRows: unknown[][] = [
    ['Apple', 100, 'fruit'],
    ['banana', 20, null],
    ['Cherry', 5, 'red'],
  ]

  it('空关键字返回全部索引', () => {
    expect(filterRowIndices(searchRows, '')).toEqual([0, 1, 2])
  })

  it('空白关键字视为空', () => {
    expect(filterRowIndices(searchRows, '   ')).toEqual([0, 1, 2])
  })

  it('全列过滤返回命中索引', () => {
    expect(filterRowIndices(searchRows, 'e')).toEqual([0, 2])
    expect(filterRowIndices(searchRows, '90')).toEqual([])
  })

  it('指定列过滤', () => {
    expect(filterRowIndices(searchRows, 'fruit', 2)).toEqual([0])
  })

  it('无命中返回空数组', () => {
    expect(filterRowIndices(searchRows, 'nope')).toEqual([])
  })

  it('空结果集返回空数组', () => {
    expect(filterRowIndices([], 'x')).toEqual([])
  })
})