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
import {
  render,
  screen,
  within,
  waitFor,
  cleanup,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, expect, it, vi } from 'vitest'

import zh from '@/i18n/locales/zh.json'

import type { AuditLog } from '../api'
import { AuditLogDetailsDialog } from '../components/audit-log-details-dialog'

const userAgent =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/152.0.0.0 Safari/537.36'
const entry: AuditLog = {
  event_id: 'audit-event-1',
  user_id: 1,
  username: 'root',
  actor_role: 100,
  created_at: 1788600600,
  category: 'operation',
  action: 'channel.update',
  token_ref: '',
  ip: '::1',
  user_agent: userAgent,
  method: 'PUT',
  route: '/api/channel/',
  status: 200,
  success: true,
  request_id: '20260905092951096890008268d9d6b7oeNCj3',
  content: 'Updated channel batch (ID: 42)',
  other: {
    admin_info: {
      admin_id: 1,
      admin_username: 'root',
      admin_role: 100,
      auth_method: 'session',
    },
    op: {
      action: 'channel.update',
      params: { id: 42, name: 'batch', changed_fields: [] },
    },
  },
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

it.each([
  [100, 'root'],
  [10, 'admin'],
  [1, 'user'],
] as const)('keeps role %i as %s in Chinese', async (role, label) => {
  const i18n = createInstance()
  await i18n.init({ lng: 'zh', resources: { zh } })
  const log = {
    ...entry,
    actor_role: role,
    other: {
      admin_info: { admin_username: 'literal-name', admin_role: role },
      op: { action: 'channel.update', params: { id: 42, name: 'batch' } },
    },
  }
  render(
    <I18nextProvider i18n={i18n}>
      <AuditLogDetailsDialog entry={log} />
    </I18nextProvider>
  )
  await userEvent.click(screen.getByRole('button', { name: '详情' }))
  const dialog = await screen.findByRole('dialog', { name: '日志详情' })
  expect(within(dialog).getByText(label)).toBeVisible()
  expect(within(dialog).getByText('literal-name (ID: 1)')).toBeVisible()
})

it('uses the returned authentication method for personal access records', async () => {
  const { dialog } = await openDetails({
    ...entry,
    auth_method: 'access_token',
  })
  expect(within(dialog).getByText('Access Token')).toBeVisible()
  expect(within(dialog).queryByText('Session')).not.toBeInTheDocument()
})

async function openDetails(log: AuditLog = entry) {
  render(<AuditLogDetailsDialog entry={log} />)
  const trigger = screen.getByRole('button', { name: 'Details' })
  trigger.focus()
  await userEvent.keyboard('{Enter}')
  return {
    trigger,
    dialog: await screen.findByRole('dialog', { name: 'Log Details' }),
  }
}

it('renders the channel update as a readable summary and compact operation rows without JSON or empty token fields', async () => {
  const { dialog } = await openDetails()
  expect(
    within(dialog).getByText('Updated channel batch (ID: 42)')
  ).toBeVisible()
  expect(
    within(dialog).getByText('Field change details were not recorded')
  ).toBeVisible()
  expect(within(dialog).getByText('root')).toBeVisible()
  expect(within(dialog).getByText('Session')).toBeVisible()
  expect(within(dialog).getByText(userAgent)).toBeVisible()
  expect(within(dialog).queryByText('Token identifier')).not.toBeInTheDocument()
  expect(
    within(dialog).queryByRole('button', { name: /Expand|Collapse/ })
  ).not.toBeInTheDocument()
  expect(dialog.querySelector('pre')).not.toBeInTheDocument()
  expect(dialog).not.toHaveTextContent('changed_fields')
})

it('aligns every request field in the same label and value columns', async () => {
  const { dialog } = await openDetails({
    ...entry,
    token_ref: 'token-fingerprint',
  })
  for (const label of [
    'Method',
    'HTTP',
    'IP',
    'Client',
    'Route',
    'Request ID',
    'Token identifier',
  ]) {
    const row = within(dialog).getByText(label, { exact: true }).parentElement
    expect(row).toHaveClass(
      'grid',
      'grid-cols-[5.25rem_minmax(0,1fr)]',
      'sm:grid-cols-[7rem_minmax(0,1fr)]'
    )
  }
})

it('shows long values in full, copies the complete identifier, and restores focus when dismissed', async () => {
  const user = userEvent.setup()
  const writeText = vi
    .spyOn(navigator.clipboard, 'writeText')
    .mockResolvedValue()
  const requestId = `request-${'x'.repeat(100)}`
  const { trigger, dialog } = await openDetails({
    ...entry,
    request_id: requestId,
  })
  expect(within(dialog).getByText(requestId)).toBeVisible()
  expect(within(dialog).getByText(requestId)).not.toHaveClass('truncate')
  expect(
    within(dialog).queryByRole('button', { name: /Expand|Collapse/ })
  ).not.toBeInTheDocument()
  await user.click(
    within(dialog).getByRole('button', { name: 'Copy Request ID' })
  )
  expect(writeText).toHaveBeenCalledWith(requestId)
  expect(within(dialog).getByText(userAgent)).toBeVisible()
  await user.keyboard('{Escape}')
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  await waitFor(() => expect(trigger).toHaveFocus())
})

it.each([
  [
    'channel.status_update',
    { id: 42, status: 2, changed: false },
    ['Disabled', 'No'],
  ],
  [
    'channel.status_update_batch',
    { count: 0, total: 3, status: 1 },
    ['Enabled', '0 / 3'],
  ],
  ['user.create', { username: 'alice', role: 10 }, ['admin', 'alice']],
])(
  'formats known parameters for %s without losing zero or false',
  async (action, params, values) => {
    const { dialog } = await openDetails({
      ...entry,
      action,
      other: { op: { action, params } },
    })
    for (const value of values) {
      expect(
        within(dialog)
          .getAllByText(value)
          .some((node) => node.textContent === value)
      ).toBe(true)
    }
  }
)

it('translates changed field names and shows unknown nested metadata as readable fields', async () => {
  const { dialog } = await openDetails({
    ...entry,
    other: {
      op: {
        action: 'channel.update',
        params: {
          id: 42,
          name: 'batch',
          changed_fields: ['models', 'group'],
          custom: { attempts: 0, permitted: false },
        },
      },
    },
  })
  expect(within(dialog).getByText('Models, Group')).toBeVisible()
  expect(within(dialog).getByText('custom')).toBeVisible()
  expect(
    within(dialog).queryByRole('button', { name: 'custom' })
  ).not.toBeInTheDocument()
  expect(within(dialog).getByText('attempts')).toBeVisible()
  expect(within(dialog).getByText('0')).toBeVisible()
  expect(within(dialog).getByText('No')).toBeVisible()
  expect(dialog).not.toHaveTextContent('[object Object]')
  expect(dialog.querySelector('pre')).not.toBeInTheDocument()
})

it.each([null, undefined, '', '{broken', [], 42])(
  'preserves request details when metadata is missing or invalid (%s)',
  async (other) => {
    const { dialog } = await openDetails({
      ...entry,
      other: other as unknown as AuditLog['other'],
    })
    expect(within(dialog).getByText(entry.request_id)).toBeVisible()
    expect(within(dialog).getByText(entry.ip)).toBeVisible()
    expect(dialog).not.toHaveTextContent('{broken')
    expect(dialog.querySelector('pre')).not.toBeInTheDocument()
  }
)

it.each([
  [
    'login',
    'login',
    { method: 'password' },
    'Logged in successfully via Password',
  ],
  ['security', 'user.2fa_enable', {}, 'Enabled two-factor authentication'],
  ['access_token', 'access_token.request', {}, 'Access Token'],
])(
  'shows summary and result for %s records',
  async (category, action, params, summary) => {
    const { dialog } = await openDetails({
      ...entry,
      category,
      action,
      actor_role: 1,
      content: '',
      success: false,
      status: 403,
      other: { op: { action, params } },
    })
    expect(within(dialog).getAllByText(summary).length).toBeGreaterThan(0)
    expect(within(dialog).getByText('Failed')).toBeVisible()
    expect(within(dialog).getByText('403')).toBeVisible()
    expect(within(dialog).queryByText('root')).not.toBeInTheDocument()
    expect(within(dialog).getByText('user')).toBeVisible()
  }
)
