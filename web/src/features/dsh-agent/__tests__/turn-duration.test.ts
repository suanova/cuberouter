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

import { approximateMinutes, estimateTurnDuration } from '../lib/turn-duration'
import type { DshConversation, DshTurn } from '../types'

const START = 1_700_000_000_000

function turn(
  id: string,
  status: DshTurn['status'],
  durationMs: number | null
): DshTurn {
  return {
    id,
    taskId: `task-${id}`,
    prompt: 'prompt',
    status,
    reply: '',
    toolCalls: [],
    transcript: [],
    createdAt: START,
    finishedAt: durationMs === null ? undefined : START + durationMs,
  }
}

function conversation(turns: DshTurn[]): DshConversation {
  return {
    id: 'conversation-1',
    title: 'title',
    model: 'dsh',
    preset: 'standard',
    turns,
    updatedAt: START,
  }
}

describe('estimateTurnDuration', () => {
  it('falls back to a vague default without finished turns', () => {
    expect(estimateTurnDuration([])).toEqual({
      milliseconds: 5 * 60 * 1000,
      fromHistory: false,
    })
    expect(
      estimateTurnDuration([conversation([turn('a', 'running', null)])])
    ).toEqual({ milliseconds: 5 * 60 * 1000, fromHistory: false })
  })

  it('uses the median of finished turns, ignoring the running one', () => {
    const estimate = estimateTurnDuration([
      conversation([
        turn('a', 'completed', 2 * 60 * 1000),
        turn('b', 'completed', 4 * 60 * 1000),
        turn('c', 'completed', 12 * 60 * 1000),
        turn('d', 'running', null),
      ]),
    ])

    expect(estimate).toEqual({ milliseconds: 4 * 60 * 1000, fromHistory: true })
  })

  it('clamps outliers so an immediate or stuck turn cannot skew the estimate', () => {
    expect(
      estimateTurnDuration([conversation([turn('a', 'completed', 1000)])])
        .milliseconds
    ).toBe(30 * 1000)
    expect(
      estimateTurnDuration([
        conversation([turn('a', 'completed', 45 * 60 * 1000)]),
      ]).milliseconds
    ).toBe(30 * 60 * 1000)
  })

  it('ignores turns that produced no result', () => {
    // 卡死 12 分钟后被用户停止的轮次不能抬高之后所有轮次的预期。
    const estimate = estimateTurnDuration([
      conversation([
        turn('a', 'canceled', 12 * 60 * 1000),
        turn('b', 'failed', 8 * 60 * 1000),
        turn('c', 'completed', 3 * 60 * 1000),
      ]),
    ])

    expect(estimate).toEqual({ milliseconds: 3 * 60 * 1000, fromHistory: true })
  })

  it('counts a truncated turn as a finished turn', () => {
    const estimate = estimateTurnDuration([
      conversation([turn('a', 'max-tokens', 2 * 60 * 1000)]),
    ])

    expect(estimate).toEqual({ milliseconds: 2 * 60 * 1000, fromHistory: true })
  })
})

describe('approximateMinutes', () => {
  it('rounds to whole minutes and never shows zero', () => {
    expect(approximateMinutes(4 * 60 * 1000)).toBe(4)
    expect(approximateMinutes(90_000)).toBe(2)
    expect(approximateMinutes(1000)).toBe(1)
  })
})
