import { describe, it, expect } from 'vitest'
import { extractErrorCode, describeErrorCode, firstLine, parseSqlError, isCanceledMessage } from './sqlError'

const SAMPLE = 'Code: 60. DB::Exception: Table db.t does not exist. (UNKNOWN_TABLE) (version 23.8.1)'

describe('extractErrorCode', () => {
  it('从标准错误中提取错误码', () => {
    expect(extractErrorCode(SAMPLE)).toBe(60)
  })
  it('小写 code 也能识别', () => {
    expect(extractErrorCode('code: 47. DB::Exception: ...')).toBe(47)
  })
  it('无错误码返回 null', () => {
    expect(extractErrorCode('plain error text')).toBeNull()
  })
  it('空字符串返回 null', () => {
    expect(extractErrorCode('')).toBeNull()
  })
})

describe('describeErrorCode', () => {
  const cases: { name: string; code: number | null; want: string | null }[] = [
    { name: '表不存在', code: 60, want: '表不存在' },
    { name: '未知标识符', code: 47, want: '字段或标识符不存在' },
    { name: '语法错误', code: 62, want: 'SQL 语法错误' },
    { name: '内存超限', code: 241, want: '内存超限，请减小查询规模或提高 max_memory_usage' },
    { name: '查询超时', code: 159, want: '查询超时' },
    { name: '未收录错误码返回 null', code: 1001, want: null },
    { name: 'null 返回 null', code: null, want: null },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(describeErrorCode(c.code)).toBe(c.want)
    })
  }
})

describe('firstLine', () => {
  it('取首个非空行', () => {
    expect(firstLine('\n  \nabc def\nsecond')).toBe('abc def')
  })
  it('全空白返回空字符串', () => {
    expect(firstLine('   \n  ')).toBe('')
  })
})

describe('parseSqlError', () => {
  it('解析已知错误码并给出摘要', () => {
    const r = parseSqlError(SAMPLE)
    expect(r.code).toBe(60)
    expect(r.summary).toContain('Table db.t does not exist')
    expect(r.summary).toContain('表不存在')
    expect(r.raw).toBe(SAMPLE)
  })

  it('未收录错误码仍保留 Exception 描述', () => {
    const r = parseSqlError('Code: 1146. DB::Exception: Something odd happened. (X)')
    expect(r.code).toBe(1146)
    expect(r.summary).toBe('Something odd happened')
  })

  it('无错误码时回退为首行', () => {
    const r = parseSqlError('connection refused\nstack...')
    expect(r.code).toBeNull()
    expect(r.summary).toBe('connection refused')
  })

  it('空输入不抛出且 summary 为 null', () => {
    const r = parseSqlError('')
    expect(r.code).toBeNull()
    expect(r.summary).toBeNull()
    expect(r.raw).toBe('')
  })

  it('原文完整保留', () => {
    const r = parseSqlError(SAMPLE)
    expect(r.raw).toContain('(version 23.8.1)')
  })
})

describe('isCanceledMessage', () => {
  it('完全相同视为取消', () => {
    expect(isCanceledMessage('查询已取消', '查询已取消')).toBe(true)
  })
  it('前后空白不影响判定', () => {
    expect(isCanceledMessage(' 查询已取消 ', '查询已取消')).toBe(true)
  })
  it('不同文本不视为取消', () => {
    expect(isCanceledMessage('Code: 60', '查询已取消')).toBe(false)
  })
})