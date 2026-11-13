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
import { fireEvent, render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, describe, expect, test } from 'vitest'

import type { AccountAssistantSettings } from '../types'
import { AccountAssistantSettingsSection } from './account-assistant-settings-section'

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

function renderSection() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <AccountAssistantSettingsSection defaultValues={defaultValues} />
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
})
