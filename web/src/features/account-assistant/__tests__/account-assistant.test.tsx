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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { getAccountAssistantConfig, streamAccountAssistantChat } from '../api'
import { AccountAssistant } from '../components/account-assistant'
import { ASSISTANT_REFUSAL_TEXT } from '../constants'

vi.mock('../api', () => ({
  getAccountAssistantConfig: vi.fn(),
  streamAccountAssistantChat: vi.fn(),
}))

const configMock = vi.mocked(getAccountAssistantConfig)
const streamMock = vi.mocked(streamAccountAssistantChat)

function renderAssistant() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <AccountAssistant />
    </QueryClientProvider>
  )
}

async function openAssistant(enableSupportLink = false) {
  configMock.mockResolvedValue({
    enabled: true,
    support_url: enableSupportLink ? 'https://support.example.com' : '',
  })
  renderAssistant()
  const button = await screen.findByRole('button', {
    name: 'Open account assistant',
  })
  await userEvent.click(button)
  return screen.getByRole('dialog', { name: 'Account Assistant' })
}

beforeEach(() => {
  configMock.mockReset()
  streamMock.mockReset()
})

describe('AccountAssistant entry', () => {
  test('given the server reports disabled, no floating entry renders', async () => {
    configMock.mockResolvedValue({ enabled: false, support_url: '' })

    renderAssistant()

    await waitFor(() => expect(configMock).toHaveBeenCalled())
    expect(
      screen.queryByRole('button', { name: 'Open account assistant' })
    ).toBeNull()
  })

  test('given the server reports enabled, the floating ball opens a focused chat panel', async () => {
    const dialog = await openAssistant()

    expect(dialog).toBeInTheDocument()
    const input = screen.getByRole('textbox', {
      name: 'Ask about your account…',
    })
    expect(input).toHaveFocus()
    expect(screen.getByRole('button', { expanded: true })).toHaveAttribute(
      'aria-label',
      'Close account assistant'
    )
  })

  test('given the panel is open, Escape closes it', async () => {
    const dialog = await openAssistant()

    fireEvent.keyDown(dialog, { key: 'Escape' })

    expect(
      screen.queryByRole('dialog', { name: 'Account Assistant' })
    ).toBeNull()
  })
})

describe('AccountAssistant conversation', () => {
  test('given a question, the final tool-backed answer renders and the request carries the raw question', async () => {
    streamMock.mockImplementation(async (params) => {
      params.onEvent({ type: 'tool', tool: 'get_my_account' })
      params.onEvent({
        type: 'final',
        content: 'Your balance is **10 USD**.',
      })
    })
    await openAssistant()
    const input = screen.getByRole('textbox', {
      name: 'Ask about your account…',
    })

    await userEvent.type(input, 'What is my balance?')
    await userEvent.click(screen.getByRole('button', { name: 'Send' }))

    expect(await screen.findByText('What is my balance?')).toBeInTheDocument()
    expect(await screen.findByText('10 USD')).toBeInTheDocument()
    expect(streamMock).toHaveBeenCalledWith(
      expect.objectContaining({
        message: 'What is my balance?',
        history: [],
      })
    )
  })

  test('given a refusal event, the fixed refusal and human-support action render', async () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
    streamMock.mockImplementation(async (params) => {
      params.onEvent({ type: 'refusal', code: 'tool_required' })
    })
    await openAssistant(true)

    await userEvent.type(
      screen.getByRole('textbox', { name: 'Ask about your account…' }),
      'Refund my subscription'
    )
    await userEvent.click(screen.getByRole('button', { name: 'Send' }))

    expect(await screen.findByText(ASSISTANT_REFUSAL_TEXT)).toBeInTheDocument()
    const contact = screen.getByRole('button', {
      name: 'Contact human support',
    })
    await userEvent.click(contact)
    expect(openSpy).toHaveBeenCalledWith(
      'https://support.example.com',
      '_blank',
      'noopener,noreferrer'
    )
    openSpy.mockRestore()
  })

  test('given a limit error, the localized message and contact fallback render', async () => {
    streamMock.mockImplementation(async (params) => {
      params.onEvent({ type: 'error', code: 'daily_limit' })
    })
    await openAssistant()

    await userEvent.type(
      screen.getByRole('textbox', { name: 'Ask about your account…' }),
      'Help'
    )
    await userEvent.click(screen.getByRole('button', { name: 'Send' }))

    expect(
      await screen.findByText(
        'Daily question limit reached. Please contact human support.'
      )
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Contact human support' })
    ).toBeInTheDocument()
  })

  test('given a stream that ends without an answer, an unavailable error renders', async () => {
    streamMock.mockResolvedValue(undefined)
    await openAssistant()

    await userEvent.type(
      screen.getByRole('textbox', { name: 'Ask about your account…' }),
      'Hello'
    )
    await userEvent.click(screen.getByRole('button', { name: 'Send' }))

    expect(
      await screen.findByText(
        'The assistant is temporarily unavailable. Please try again later or contact human support.'
      )
    ).toBeInTheDocument()
  })
})
