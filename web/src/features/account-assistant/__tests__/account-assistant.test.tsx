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
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { AccountAssistant } from '../components/account-assistant'
import { ASSISTANT_REFUSAL_TEXT } from '../constants'
import { api as httpApi } from '@/lib/http-client'

// Mock only the network boundary (HTTP client config GET + global fetch for the
// SSE stream) so getAccountAssistantConfig and streamAccountAssistantChat — and
// their response/SSE parsing — stay real.
vi.mock('@/lib/http-client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/http-client')>()
  return { ...actual, api: { ...actual.api, get: vi.fn() } }
})
vi.mock('@/lib/auth-session', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/auth-session')>()
  return { ...actual, getFreshAuthHeaders: vi.fn().mockResolvedValue({}) }
})

const configGet = vi.mocked(httpApi.get)
const fetchMock = vi.fn()
vi.stubGlobal('fetch', fetchMock)

function mockConfig(enabled: boolean, supportUrl = '') {
  configGet.mockResolvedValue({
    data: { data: { enabled, support_url: supportUrl } },
  })
}

function mockSseStream(...events: Array<Record<string, unknown>>) {
  const frames = events
    .map((event) => `data: ${JSON.stringify(event)}\n\n`)
    .join('')
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(new TextEncoder().encode(`${frames}data: [DONE]\n\n`))
      controller.close()
    },
  })
  fetchMock.mockResolvedValue({
    ok: true,
    status: 200,
    headers: new Headers({ 'content-type': 'text/event-stream' }),
    body: stream,
  } as Response)
}

function renderAssistant() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <AccountAssistant />
    </QueryClientProvider>
  )
  return queryClient
}

// Waits until the config query has reached a settled success state, so an
// assertion about what renders afterward cannot accidentally pass while the
// component is still loading (when it also renders nothing).
async function waitForConfig(queryClient: QueryClient) {
  await waitFor(() => {
    expect(
      queryClient.getQueryState(['account-assistant', 'config'])?.status
    ).toBe('success')
  })
}

async function openAssistant(enableSupportLink = false) {
  mockConfig(true, enableSupportLink ? 'https://support.example.com' : '')
  const queryClient = renderAssistant()
  await waitForConfig(queryClient)
  const button = await screen.findByRole('button', {
    name: 'Open account assistant',
  })
  await userEvent.click(button)
  return screen.getByRole('dialog', { name: 'Account Assistant' })
}

beforeEach(() => {
  configGet.mockReset()
  fetchMock.mockReset()
})

describe('AccountAssistant entry', () => {
  test('given the server reports disabled, no floating entry renders', async () => {
    mockConfig(false)

    const queryClient = renderAssistant()

    // Wait for the config query to settle as success before asserting absence;
    // until then the component renders nothing regardless of the enabled flag,
    // which could make this assertion pass vacuously.
    await waitForConfig(queryClient)
    await waitFor(() =>
      expect(
        screen.queryByRole('button', { name: 'Open account assistant' })
      ).toBeNull()
    )
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

  test('given the panel is open, Escape closes it and returns focus to the launcher', async () => {
    await openAssistant()
    const dialog = screen.getByRole('dialog', { name: 'Account Assistant' })
    const launcher = screen.getByRole('button', {
      expanded: true,
      name: 'Close account assistant',
    })

    fireEvent.keyDown(dialog, { key: 'Escape' })

    expect(dialog).toHaveAttribute('hidden')
    expect(launcher).toHaveAttribute('aria-label', 'Open account assistant')
    await waitFor(() => expect(launcher).toHaveFocus())
  })

  test('given the panel closes and reopens, the transcript is preserved', async () => {
    mockSseStream({ type: 'final', content: 'Your balance is **10 USD**.' })
    await openAssistant()
    const input = screen.getByRole('textbox', {
      name: 'Ask about your account…',
    })

    await userEvent.type(input, 'What is my balance?')
    await userEvent.click(screen.getByRole('button', { name: 'Send' }))
    expect(await screen.findByText('10 USD')).toBeInTheDocument()

    // Close and reopen; the answer must still be there instead of resetting.
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    await userEvent.click(
      screen.getByRole('button', { name: 'Open account assistant' })
    )

    expect(screen.getByText('What is my balance?')).toBeInTheDocument()
    expect(screen.getByText('10 USD')).toBeInTheDocument()
  })

  test('the conversation is a labeled polite live region', async () => {
    await openAssistant()

    const log = screen.getByRole('log')
    expect(log).toHaveAttribute('aria-live', 'polite')
    expect(log).toHaveAccessibleName('Account Assistant conversation')
  })

  test('given the panel reopens during a stream, focus lands inside the panel', async () => {
    // A stream that never sends an answer keeps streaming=true, so the composer
    // stays disabled and cannot take focus on reopen.
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'text/event-stream' }),
      body: new ReadableStream<Uint8Array>({
        start(sink) {
          sink.enqueue(new TextEncoder().encode(': start\n\n'))
        },
      }),
    } as Response)
    mockConfig(true)
    const queryClient = renderAssistant()
    await waitForConfig(queryClient)
    await userEvent.click(
      screen.getByRole('button', { name: 'Open account assistant' })
    )
    await userEvent.type(
      screen.getByRole('textbox', { name: 'Ask about your account…' }),
      'Where is my balance?'
    )
    await userEvent.click(screen.getByRole('button', { name: 'Send' }))

    // Close while the stream is still open, then reopen.
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    const launcher = screen.getByRole('button', {
      name: 'Open account assistant',
    })
    await waitFor(() => expect(launcher).toHaveFocus())
    await userEvent.click(
      screen.getByRole('button', { name: 'Open account assistant' })
    )

    // Focus must move to an enabled element inside the dialog (the close
    // button), not stay on the launcher, so Escape still closes the panel.
    const dialog = screen.getByRole('dialog', { name: 'Account Assistant' })
    const closeButton = within(dialog).getByRole('button', {
      name: 'Close account assistant',
    })
    expect(closeButton).toHaveFocus()
    fireEvent.keyDown(dialog, { key: 'Escape' })
    expect(dialog).toHaveAttribute('hidden')
  })
})

describe('AccountAssistant conversation', () => {
  test('given a question, the final tool-backed answer renders and the request carries the raw question', async () => {
    mockSseStream(
      { type: 'tool', tool: 'get_my_account' },
      { type: 'final', content: 'Your balance is **10 USD**.' }
    )
    await openAssistant()
    const input = screen.getByRole('textbox', {
      name: 'Ask about your account…',
    })

    await userEvent.type(input, 'What is my balance?')
    await userEvent.click(screen.getByRole('button', { name: 'Send' }))

    expect(await screen.findByText('What is my balance?')).toBeInTheDocument()
    expect(await screen.findByText('10 USD')).toBeInTheDocument()
    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string)
    expect(body).toMatchObject({
      message: 'What is my balance?',
      history: [],
    })
  })

  test('given a refusal event, the fixed refusal and human-support action render', async () => {
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
    mockSseStream({ type: 'refusal', code: 'tool_required' })
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
    mockSseStream({ type: 'error', code: 'daily_limit' })
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
    // Stream returns immediately with only the [DONE] frame; no answer event.
    mockSseStream()
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