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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

import { DshThread } from '../components/dsh-thread'
import type { TurnDurationEstimate } from '../lib/turn-duration'
import type { DshTurn } from '../types'

function runningTurn(createdAt: number): DshTurn {
  return {
    id: 'turn-1',
    taskId: 'task-1',
    prompt: '做一份 PPT',
    status: 'running',
    reply: '',
    toolCalls: [],
    transcript: [],
    createdAt,
  }
}

function renderThread(
  expectedTurnDuration: TurnDurationEstimate,
  elapsedMs = 5_000
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <DshThread
        expectedTurnDuration={expectedTurnDuration}
        sessionExpired={false}
        turns={[runningTurn(Date.now() - elapsedMs)]}
      />
    </QueryClientProvider>
  )
}

describe('running turn expectation', () => {
  test('states how long a turn usually takes when history exists', () => {
    renderThread({ milliseconds: 4 * 60 * 1000, fromHistory: true })

    expect(screen.getByText('Agent is working…')).toBeInTheDocument()
    expect(screen.getByText('Usually takes about 4 min.')).toBeInTheDocument()
  })

  test('stays vague without history instead of inventing a precise number', () => {
    renderThread({ milliseconds: 5 * 60 * 1000, fromHistory: false })

    expect(screen.getByText('Usually takes a few minutes.')).toBeInTheDocument()
  })

  test('stops promising the usual duration once the turn runs long', () => {
    renderThread(
      { milliseconds: 4 * 60 * 1000, fromHistory: true },
      10 * 60 * 1000
    )

    expect(
      screen.getByText('Taking longer than usual, the agent is still working.')
    ).toBeInTheDocument()
    expect(
      screen.queryByText('Usually takes about 4 min.')
    ).not.toBeInTheDocument()
  })
})
