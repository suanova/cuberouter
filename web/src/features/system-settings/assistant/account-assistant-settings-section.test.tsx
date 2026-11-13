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
import i18next from 'i18next'
import { useState } from 'react'
import { beforeAll, describe, expect, test, vi } from 'vitest'

import { SettingsPageProvider } from '../components/settings-page-context'
import { updateSystemOption } from '../api'
import type { AccountAssistantSettings } from '../types'
import { AccountAssistantSettingsSection } from './account-assistant-settings-section'

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return { ...actual, updateSystemOption: vi.fn() }
})

const KEYS: Record<string, string> = {
  'Account Assistant': 'Account Assistant',
  'Enable Account Assistant': 'Enable Account Assistant',
  'Assistant Model': 'Assistant Model',
  'Billing Account ID': 'Billing Account ID',
  Group: 'Group',
  'Max Tokens': 'Max Tokens',
  'Max Tool Calls': 'Max Tool Calls',
  'Max Rounds': 'Max Rounds',
  'Hourly Limit': 'Hourly Limit',
  'Daily Limit': 'Daily Limit',
  'Daily Token Budget': 'Daily Token Budget',
  'Help Docs (FAQ)': 'Help Docs (FAQ)',
  'Support URL': 'Support URL',
  'Save assistant settings': 'Save assistant settings',
  'FAQ must be a JSON array of { title, content } objects':
    'FAQ must be a JSON array of { title, content } objects',
}

const defaultValues: AccountAssistantSettings = {
  'account_assistant.enabled': true,
  'account_assistant.model': 'gpt-4o-mini',
  'account_assistant.billing_user_id': 0,
  'account_assistant.group': 'default',
  'account_assistant.max_tokens': 1024,
  'account_assistant.max_tool_calls': 3,
  'account_assistant.max_rounds': 2,
  'account_assistant.hourly_limit': 10,
  'account_assistant.daily_limit': 50,
  'account_assistant.daily_token_budget': 200000,
  'account_assistant.faq_json': '[]',
  'account_assistant.support_url': '',
}

function renderSection(overrides?: Partial<AccountAssistantSettings>) {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  const values = overrides
    ? { ...defaultValues, ...overrides }
    : defaultValues

  function SectionWithActions() {
    const [actionsContainer, setActionsContainer] = useState<HTMLDivElement | null>(null)
    return (
      <SettingsPageProvider
        actionsContainer={actionsContainer}
        suppressSectionHeader={false}
      >
        <AccountAssistantSettingsSection defaultValues={values} />
        {/* SettingsPageFormActions portals here; the container mounts with a
            callback ref so the Save button is present for save tests. */}
        <div ref={setActionsContainer} data-testid='actions-container' />
      </SettingsPageProvider>
    )
  }

  return render(
    <QueryClientProvider client={queryClient}>
      <SectionWithActions />
    </QueryClientProvider>
  )
}

describe('AccountAssistantSettingsSection', () => {
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', KEYS)
  })

  test('renders the enable switch and core model fields from defaults', () => {
    renderSection()

    expect(
      screen.getByRole('heading', { name: 'Account Assistant' })
    ).toBeInTheDocument()
    const enableSwitch = screen.getByRole('switch', {
      name: 'Enable Account Assistant',
    })
    expect(enableSwitch).toBeChecked()

    const modelInput = screen.getByRole('textbox', { name: 'Assistant Model' })
    expect(modelInput).toHaveValue('gpt-4o-mini')

    expect(
      screen.getByRole('spinbutton', { name: 'Billing Account ID' })
    ).toHaveValue(0)
    expect(screen.getByRole('spinbutton', { name: 'Max Tokens' })).toHaveValue(
      1024
    )
  })

  test('renders the FAQ entry editor with the default empty array', () => {
    renderSection()

    // JsonCodeEditor pulls the assistant FAQ into a code editor; assert the
    // group and the default value render rather than simulating typing into
    // the CodeMirror layer.
    expect(screen.getByText('Help Docs (FAQ)')).toBeInTheDocument()
    expect(screen.getByText('Copy')).toBeInTheDocument()
    expect(screen.getByText('Format JSON')).toBeInTheDocument()
  })

  test('toggles the enable switch off', () => {
    renderSection()

    const enableSwitch = screen.getByRole('switch', {
      name: 'Enable Account Assistant',
    })
    fireEvent.click(enableSwitch)
    expect(enableSwitch).not.toBeChecked()
  })

  test('saves a changed model by writing the dotted key, and persists it', async () => {
    vi.mocked(updateSystemOption).mockResolvedValue({ success: true, message: '' })
    renderSection()

    const modelInput = screen.getByRole('textbox', { name: 'Assistant Model' })
    fireEvent.change(modelInput, { target: { value: 'claude-opus-5-5' } })

    fireEvent.click(screen.getByRole('button', { name: 'Save assistant settings' }))

    await waitFor(() => {
      expect(updateSystemOption).toHaveBeenCalledWith(
        expect.objectContaining({
          key: 'account_assistant.model',
          value: 'claude-opus-5-5',
        })
      )
    })
  })

  test('saves a billing change as a number, not a string', async () => {
    vi.mocked(updateSystemOption).mockResolvedValue({ success: true, message: '' })
    renderSection()

    const billingInput = screen.getByRole('spinbutton', {
      name: 'Billing Account ID',
    })
    fireEvent.change(billingInput, { target: { value: '42' } })

    fireEvent.click(screen.getByRole('button', { name: 'Save assistant settings' }))

    await waitFor(() => {
      expect(updateSystemOption).toHaveBeenCalledWith(
        expect.objectContaining({ key: 'account_assistant.billing_user_id', value: 42 })
      )
    })
  })

  test('writes the enable flag after the other settings', async () => {
    vi.mocked(updateSystemOption).mockResolvedValue({ success: true, message: '' })
    renderSection({ 'account_assistant.enabled': false })

    fireEvent.click(
      screen.getByRole('switch', { name: 'Enable Account Assistant' })
    )
    fireEvent.change(screen.getByRole('textbox', { name: 'Assistant Model' }), {
      target: { value: 'gpt-5' },
    })
    fireEvent.click(
      screen.getByRole('button', { name: 'Save assistant settings' })
    )

    await waitFor(() => {
      expect(vi.mocked(updateSystemOption)).toHaveBeenCalledTimes(2)
    })
    const calls = vi.mocked(updateSystemOption).mock.calls.map(([arg]) => arg.key)
    const enabledIdx = calls.indexOf('account_assistant.enabled')
    const modelIdx = calls.indexOf('account_assistant.model')
    expect(enabledIdx).toBeGreaterThan(modelIdx)
  })

  test('stops saving when an option update returns success:false and keeps baseline intact', async () => {
    const update = vi.mocked(updateSystemOption)
    // First change (model) is rejected by the backend; the enabled flag — which
    // is written last — must NOT be attempted, and the rejected value must not
    // be absorbed into the baseline.
    update.mockResolvedValueOnce({
      success: false,
      message: 'model rejected',
    })
    renderSection({ 'account_assistant.enabled': false })

    fireEvent.click(
      screen.getByRole('switch', { name: 'Enable Account Assistant' })
    )
    fireEvent.change(screen.getByRole('textbox', { name: 'Assistant Model' }), {
      target: { value: 'gpt-5' },
    })
    fireEvent.click(
      screen.getByRole('button', { name: 'Save assistant settings' })
    )

    await waitFor(() => {
      expect(updateSystemOption).toHaveBeenCalled()
    })
    // The rejected model write stops the loop: only the model key was sent,
    // and the enable flag (written last) was not attempted.
    const sentKeys = vi
      .mocked(updateSystemOption)
      .mock.calls.map(([arg]) => arg.key)
    expect(sentKeys).toEqual(['account_assistant.model'])
    expect(sentKeys).not.toContain('account_assistant.enabled')

    // The baseline is unchanged: a follow-up save with a different model value
    // still submits the change instead of treating it as already saved.
    vi.mocked(updateSystemOption).mockClear()
    vi.mocked(updateSystemOption).mockResolvedValue({
      success: true,
      message: '',
    })
    fireEvent.change(screen.getByRole('textbox', { name: 'Assistant Model' }), {
      target: { value: 'gpt-5.2' },
    })
    fireEvent.click(
      screen.getByRole('button', { name: 'Save assistant settings' })
    )
    await waitFor(() => {
      expect(updateSystemOption).toHaveBeenCalledWith(
        expect.objectContaining({ key: 'account_assistant.model' })
      )
    })
  })

  test('rejects an invalid FAQ entry and shows the validation message', async () => {
    vi.mocked(updateSystemOption).mockResolvedValue({ success: true, message: '' })
    // The FAQ JSON is an object, not an array of { title, content } objects;
    // and the numeric content type is rejected before it ever reaches the
    // backend. Render it as the starting value so typing is not required.
    renderSection({
      'account_assistant.faq_json': '{"title":"x","content":"y"}',
    })

    fireEvent.click(screen.getByRole('button', { name: 'Save assistant settings' }))

    expect(
      await screen.findByText(
        'FAQ must be a JSON array of { title, content } objects'
      )
    ).toBeInTheDocument()
    expect(updateSystemOption).not.toHaveBeenCalled()
  })

  test('rejects an FAQ entry whose content is a number, matching the backend', async () => {
    vi.mocked(updateSystemOption).mockResolvedValue({ success: true, message: '' })
    // The help-docs tool decodes title/content as strings; a numeric content
    // would fail the whole document array on the server, so it must be
    // rejected here before it is ever saved.
    renderSection({
      'account_assistant.faq_json': '[{"title":"Renewal","content":123}]',
    })

    fireEvent.click(screen.getByRole('button', { name: 'Save assistant settings' }))

    expect(
      await screen.findByText(
        'FAQ must be a JSON array of { title, content } objects'
      )
    ).toBeInTheDocument()
    expect(updateSystemOption).not.toHaveBeenCalled()
  })
})
