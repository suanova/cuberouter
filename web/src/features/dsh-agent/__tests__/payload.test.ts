/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, it } from 'vitest'

import {
  isSessionExpiredError,
  readErrorMessage,
  readToolCalls,
  readTranscript,
  resolveTurnStatus,
  unwrapTaskPayload,
} from '../lib/payload'

describe('unwrapTaskPayload', () => {
  it('unwraps the polled dsh envelope', () => {
    expect(
      unwrapTaskPayload({
        ok: true,
        data: { status: 'completed', reply: 'hi' },
      })
    ).toEqual({ status: 'completed', reply: 'hi' })
  })

  it('accepts a flat payload such as the submit receipt', () => {
    expect(unwrapTaskPayload({ taskId: 't1', sessionId: 's1' })).toEqual({
      taskId: 't1',
      sessionId: 's1',
    })
  })

  it('returns undefined for missing or non-object values', () => {
    expect(unwrapTaskPayload(null)).toBeUndefined()
    expect(unwrapTaskPayload('running')).toBeUndefined()
  })
})

describe('resolveTurnStatus', () => {
  it('keeps polling while the gateway is not settled', () => {
    expect(resolveTurnStatus('PENDING', undefined)).toBe('running')
    expect(resolveTurnStatus(undefined, { status: 'running' })).toBe('running')
  })

  it('reports gateway failures as failed', () => {
    expect(resolveTurnStatus('FAILURE', { status: 'completed' })).toBe('failed')
  })

  it('separates max-tokens and canceled from a normal completion', () => {
    expect(resolveTurnStatus('SUCCESS', { status: 'max-tokens' })).toBe(
      'max-tokens'
    )
    expect(resolveTurnStatus('SUCCESS', { status: 'canceled' })).toBe(
      'canceled'
    )
    expect(resolveTurnStatus('SUCCESS', { status: 'completed' })).toBe(
      'completed'
    )
  })
})

describe('payload readers', () => {
  it('drops malformed tool calls and transcript entries', () => {
    expect(
      readToolCalls({
        toolCalls: [{ name: 'bash', callId: 'c1' }, { callId: 'c2' }] as never,
      })
    ).toEqual([{ name: 'bash', callId: 'c1' }])

    expect(
      readTranscript({
        transcript: [
          { role: 'user', text: 'hi' },
          { role: 'assistant' },
        ] as never,
      })
    ).toEqual([{ role: 'user', text: 'hi' }])
  })

  it('prefers the upstream error message over the gateway fail reason', () => {
    expect(readErrorMessage({ error: { message: 'boom' } }, 'failed')).toBe(
      'boom'
    )
    expect(readErrorMessage({ error: 'bad' }, 'failed')).toBe('bad')
    expect(readErrorMessage({}, 'failed')).toBe('failed')
  })
})

describe('isSessionExpiredError', () => {
  it('recognises the dsh continuation failures', () => {
    expect(isSessionExpiredError('unknown_session')).toBe(true)
    expect(isSessionExpiredError('Unknown Session: abc')).toBe(true)
    expect(isSessionExpiredError('session not found')).toBe(true)
  })

  it('ignores unrelated failures', () => {
    expect(isSessionExpiredError('rate limited')).toBe(false)
    expect(isSessionExpiredError(undefined)).toBe(false)
  })
})
