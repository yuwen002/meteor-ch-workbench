import { describe, expect, it } from 'vitest'
import {
  AUTO_REFRESH_DISABLED,
  AUTO_REFRESH_PRESETS,
  MAX_REFRESH_SECONDS,
  formatSeconds,
  intervalMs,
  isCustomPreset,
  isRefreshEnabled,
  normalizeSeconds,
  parseStoredSeconds,
  remainingSeconds,
  shouldFire,
} from './autoRefresh'

describe('autoRefresh 秒数归一化', () => {
  const cases: { name: string; input: unknown; expected: number }[] = [
    { name: 'null 关闭', input: null, expected: 0 },
    { name: 'undefined 关闭', input: undefined, expected: 0 },
    { name: '0 关闭', input: 0, expected: AUTO_REFRESH_DISABLED },
    { name: '负数关闭', input: -5, expected: 0 },
    { name: '非数字关闭', input: 'abc', expected: 0 },
    { name: '空串关闭', input: '', expected: 0 },
    { name: '小数向下取整', input: 2.9, expected: 2 },
    { name: '字符串数字解析', input: '30', expected: 30 },
    { name: '不足 1 秒视为关闭', input: 0.4, expected: AUTO_REFRESH_DISABLED },
    { name: '超过上限截断', input: 999999, expected: MAX_REFRESH_SECONDS },
    { name: '预设值原样返回', input: 60, expected: 60 },
  ]

  it.each(cases)('$name', ({ input, expected }) => {
    expect(normalizeSeconds(input as any)).toBe(expected)
  })

  it('启用判定与归一化一致', () => {
    expect(isRefreshEnabled(10)).toBe(true)
    expect(isRefreshEnabled(0)).toBe(false)
    expect(isRefreshEnabled(-1)).toBe(false)
  })

  it('间隔换算毫秒', () => {
    expect(intervalMs(30)).toBe(30000)
    expect(intervalMs(0)).toBe(0)
  })

  it('自定义判定：预设值不算自定义', () => {
    for (const p of AUTO_REFRESH_PRESETS) {
      expect(isCustomPreset(p)).toBe(false)
    }
    expect(isCustomPreset(7)).toBe(true)
    expect(isCustomPreset(0)).toBe(false)
  })

  it('秒数格式化保持归一化语义', () => {
    expect(formatSeconds(5)).toBe('5')
    expect(formatSeconds(60)).toBe('60')
    expect(formatSeconds(0)).toBe('0')
    expect(formatSeconds(-1)).toBe('0')
  })

  it('解析本地存储值', () => {
    expect(parseStoredSeconds(null)).toBe(0)
    expect(parseStoredSeconds('15')).toBe(15)
    expect(parseStoredSeconds('bad')).toBe(0)
    expect(parseStoredSeconds('-3')).toBe(0)
  })
})

describe('autoRefresh 倒计时与触发', () => {
  const cases: { name: string; input: { enabled: boolean; intervalSeconds: number; lastRunAt: number; now: number }; remaining: number; fire: boolean }[] = [
    {
      name: '刚执行完剩余整周期',
      input: { enabled: true, intervalSeconds: 30, lastRunAt: 1000, now: 1000 },
      remaining: 30,
      fire: false,
    },
    {
      name: '已过 10 秒剩余 20',
      input: { enabled: true, intervalSeconds: 30, lastRunAt: 1000, now: 11000 },
      remaining: 20,
      fire: false,
    },
    {
      name: '刚好到点触发',
      input: { enabled: true, intervalSeconds: 30, lastRunAt: 1000, now: 31000 },
      remaining: 0,
      fire: true,
    },
    {
      name: '超过周期仍触发且剩余不小于 0',
      input: { enabled: true, intervalSeconds: 30, lastRunAt: 1000, now: 40000 },
      remaining: 0,
      fire: true,
    },
    {
      name: '未启用不触发',
      input: { enabled: false, intervalSeconds: 30, lastRunAt: 1000, now: 99000 },
      remaining: 0,
      fire: false,
    },
    {
      name: '间隔为 0 不触发',
      input: { enabled: true, intervalSeconds: 0, lastRunAt: 1000, now: 99000 },
      remaining: 0,
      fire: false,
    },
    {
      name: '时钟回拨不出现负剩余',
      input: { enabled: true, intervalSeconds: 30, lastRunAt: 5000, now: 1000 },
      remaining: 30,
      fire: false,
    },
  ]

  it.each(cases)('$name', ({ input, remaining, fire }) => {
    expect(remainingSeconds(input)).toBe(remaining)
    expect(shouldFire(input)).toBe(fire)
  })
})