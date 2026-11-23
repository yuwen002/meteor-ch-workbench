import { describe, it, expect } from 'vitest'
import { isDisconnectedError } from './connectionError'

describe('isDisconnectedError', () => {
  const cases: { name: string; input: string | null | undefined; want: boolean }[] = [
    { name: '空输入不算断开', input: '', want: false },
    { name: 'null 不算断开', input: null, want: false },
    { name: 'undefined 不算断开', input: undefined, want: false },
    { name: '普通 SQL 错误不算断开', input: 'Code: 62. DB::Exception: Syntax error', want: false },
    { name: '表不存在不算断开', input: 'Code: 60. DB::Exception: Table default.t does not exist', want: false },
    { name: 'connection is closed', input: 'sql: database is closed', want: false },
    { name: 'driver: connection closed', input: 'clickhouse [query]: connection closed', want: true },
    { name: 'EOF', input: 'read tcp 10.0.0.1:5432: EOF', want: true },
    { name: 'broken pipe', input: 'write tcp: broken pipe', want: true },
    { name: 'connection refused', input: 'dial tcp 10.0.0.1:9000: connect: connection refused', want: true },
    { name: 'connection reset by peer', input: 'read tcp: connection reset by peer', want: true },
    { name: 'i/o timeout', input: 'dial tcp: i/o timeout', want: true },
    { name: '大小写不敏感', input: 'CONNECTION CLOSED', want: true },
    { name: '前后空白被忽略', input: '  EOF  ', want: true },
  ]
  for (const c of cases) {
    it(c.name, () => {
      expect(isDisconnectedError(c.input)).toBe(c.want)
    })
  }
})