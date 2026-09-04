/*
Copyright (C) 2023-2026 QuantumNous

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
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, beforeAll, describe, expect, test } from 'vitest'

import type { PerfModelSummary } from '@/features/performance-metrics/types'

import { PerformanceOverview } from '../performance-overview'

// 模型名按服务端 request_count 降序给出的 8 个模型（前 5 为 Top）。
const MODEL_NAMES = [
  'gpt-4o',
  'claude-3-5-sonnet',
  'gemini-2.0-flash',
  'deepseek-r1',
  'o3',
  'llama-3.1-405b',
  'qwen-max',
  'glm-4-plus',
]

function makeModels(names: string[]): PerfModelSummary[] {
  return names.map((model_name, index) => ({
    model_name,
    avg_latency_ms: 500 + index,
    success_rate: 99 - index,
    avg_tps: 10 + index,
    request_count: 1000 - index,
  }))
}

function renderOverview(models: PerfModelSummary[]): QueryClient {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClient.setQueryData(
    ['perf-metrics-summary', 24],
    { data: { models } },
    { updatedAt: Date.now() + 60_000 }
  )
  render(
    <QueryClientProvider client={queryClient}>
      <PerformanceOverview />
    </QueryClientProvider>
  )
  return queryClient
}

describe('performance overview model badges', () => {
  const queryClients: QueryClient[] = []

  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      '+{{count}} Models': '+{{count}} Models',
    })
  })

  afterEach(() => {
    for (const queryClient of queryClients) {
      queryClient.clear()
    }
    queryClients.length = 0
  })

  test('shows at most five badges plus a +N chip when more models exist', () => {
    const queryClient = renderOverview(makeModels(MODEL_NAMES))
    queryClients.push(queryClient)

    for (const name of MODEL_NAMES.slice(0, 5)) {
      expect(screen.getByText(name)).toBeInTheDocument()
    }
    for (const name of MODEL_NAMES.slice(5)) {
      expect(screen.queryByText(name)).not.toBeInTheDocument()
    }

    const chip = screen.getByRole('button', { name: '+3 Models' })
    expect(chip).toHaveAttribute('aria-expanded', 'false')
  })

  test('clicking the +N chip expands the remaining badges and collapses again', async () => {
    const user = userEvent.setup()
    const queryClient = renderOverview(makeModels(MODEL_NAMES))
    queryClients.push(queryClient)

    const chip = screen.getByRole('button', { name: '+3 Models' })
    await user.click(chip)
    expect(chip).toHaveAttribute('aria-expanded', 'true')
    for (const name of MODEL_NAMES.slice(5)) {
      expect(screen.getByText(name)).toBeInTheDocument()
    }

    await user.click(chip)
    expect(chip).toHaveAttribute('aria-expanded', 'false')
    for (const name of MODEL_NAMES.slice(5)) {
      expect(screen.queryByText(name)).not.toBeInTheDocument()
    }
  })

  test('renders no +N chip when the model list fits the top-five limit', () => {
    const queryClient = renderOverview(makeModels(MODEL_NAMES.slice(0, 5)))
    queryClients.push(queryClient)

    expect(screen.queryByRole('button', { name: /Models/ })).not.toBeInTheDocument()
    for (const name of MODEL_NAMES.slice(0, 5)) {
      expect(screen.getByText(name)).toBeInTheDocument()
    }
  })
})
