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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { DshAgent } from '..'

vi.mock('../api', () => ({
  getUserModels: vi.fn(async () => []),
  submitAgentTask: vi.fn(),
  fetchAgentTask: vi.fn(),
  getRequestErrorMessage: vi.fn((error: unknown) => String(error)),
}))

function renderAgent() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <DshAgent />
    </QueryClientProvider>
  )
}

describe('DshAgent layout', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  test('the message log sits in a constrained flex column so it can scroll', () => {
    renderAgent()

    const log = screen.getByRole('log')
    const viewport = log.parentElement

    expect(viewport).not.toBeNull()
    expect(viewport).toHaveClass(
      'flex',
      'min-h-0',
      'flex-1',
      'flex-col',
      'overflow-hidden'
    )
  })

  test('the message log and composer share one centered column', () => {
    renderAgent()

    const log = screen.getByRole('log')
    // 左右留白来自居中列的宽度限制，而不是滚动容器的内边距：
    // 两者必须同宽，输入框才和消息左右对齐。
    const content = log.querySelector('.max-w-4xl')
    const composer = screen.getByRole('textbox').closest('.max-w-4xl')

    expect(content).toHaveClass('mx-auto', 'w-full', 'px-4')
    expect(composer).toHaveClass('mx-auto', 'w-full', 'px-4')
  })
})
