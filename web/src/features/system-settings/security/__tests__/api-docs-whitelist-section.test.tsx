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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { updateSystemOption } from '../../api'
import { SettingsPageProvider } from '../../components/settings-page-context'
import { ApiDocsWhitelistSection } from '../api-docs-whitelist-section'

vi.mock('../../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api')>()
  return { ...actual, updateSystemOption: vi.fn() }
})

const WHITELIST_OPTION_KEY = 'security_setting.swagger_api_ip_whitelist'

const queryClient = new QueryClient({
  defaultOptions: { mutations: { retry: false } },
})

function Harness(props: { whitelist: string[] }) {
  const [actionsContainer, setActionsContainer] =
    useState<HTMLDivElement | null>(null)

  return (
    <>
      <div ref={setActionsContainer} />
      <SettingsPageProvider actionsContainer={actionsContainer}>
        <ApiDocsWhitelistSection
          defaultValues={{ whitelist: props.whitelist }}
        />
      </SettingsPageProvider>
    </>
  )
}

function renderSection(whitelist: string[]) {
  return render(
    <QueryClientProvider client={queryClient}>
      <Harness whitelist={whitelist} />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  queryClient.clear()
  vi.mocked(updateSystemOption).mockReset()
  vi.mocked(updateSystemOption).mockResolvedValue({
    success: true,
    message: '',
  })
})

describe('ApiDocsWhitelistSection', () => {
  test('saves the whitelist as a JSON string array the backend []string can decode', async () => {
    const user = userEvent.setup()
    renderSection(['192.168.1.1'])

    const textarea = screen.getByLabelText('Allowed IPs / CIDRs')
    await user.clear(textarea)
    await user.type(textarea, '203.0.113.7{enter}10.0.0.0/8')
    await user.click(
      screen.getByRole('button', { name: 'Save API access whitelist' })
    )

    await waitFor(() => {
      expect(updateSystemOption).toHaveBeenCalledWith({
        key: WHITELIST_OPTION_KEY,
        value: '["203.0.113.7","10.0.0.0/8"]',
      })
    })
  })

  test('does not call the API when the list is unchanged', async () => {
    const user = userEvent.setup()
    renderSection(['203.0.113.7'])

    await user.click(
      screen.getByRole('button', { name: 'Save API access whitelist' })
    )

    expect(updateSystemOption).not.toHaveBeenCalled()
  })

  test('saving an empty list removes the restriction', async () => {
    const user = userEvent.setup()
    renderSection(['203.0.113.7'])

    await user.clear(screen.getByLabelText('Allowed IPs / CIDRs'))
    await user.click(
      screen.getByRole('button', { name: 'Save API access whitelist' })
    )

    await waitFor(() => {
      expect(updateSystemOption).toHaveBeenCalledWith({
        key: WHITELIST_OPTION_KEY,
        value: '[]',
      })
    })
  })

  test('rejects invalid entries before calling the API', async () => {
    const user = userEvent.setup()
    renderSection([])

    await user.type(screen.getByLabelText('Allowed IPs / CIDRs'), 'not-an-ip')
    await user.click(
      screen.getByRole('button', { name: 'Save API access whitelist' })
    )

    expect(
      await screen.findByText('Enter an IP address or CIDR range on each line')
    ).toBeInTheDocument()
    expect(updateSystemOption).not.toHaveBeenCalled()
  })

  test('a rejected save keeps the baseline so a retry is sent again', async () => {
    const user = userEvent.setup()
    vi.mocked(updateSystemOption).mockResolvedValueOnce({
      success: false,
      message: 'invalid IP or CIDR in whitelist: not-an-ip',
    })
    renderSection(['203.0.113.7'])

    const textarea = screen.getByLabelText('Allowed IPs / CIDRs')
    await user.clear(textarea)
    await user.type(textarea, '10.0.0.0/8')

    const saveButton = screen.getByRole('button', {
      name: 'Save API access whitelist',
    })
    await user.click(saveButton)
    await waitFor(() => expect(updateSystemOption).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(saveButton).toBeEnabled())
    await user.click(saveButton)

    await waitFor(() => expect(updateSystemOption).toHaveBeenCalledTimes(2))
    expect(updateSystemOption).toHaveBeenLastCalledWith({
      key: WHITELIST_OPTION_KEY,
      value: '["10.0.0.0/8"]',
    })
  })
})
