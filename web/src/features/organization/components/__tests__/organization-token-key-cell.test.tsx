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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, test, vi } from 'vitest'

import type { OrganizationTokenRow } from '../../types'
import { OrganizationTokenKeyCell } from '../organization-token-key-cell'

/**
 * What the cell hands to a client.
 *
 * The stored secret is the bare key body; every other surface in this feature —
 * the mask, the bulk copy, the create dialog — puts the `sk-` prefix back on,
 * because that is the string a relay expects. This cell used to hand over the
 * bare body while *displaying* the prefixed mask, so the value a caller pasted
 * was neither what they saw nor a usable key.
 */
const fixtures = vi.hoisted(() => ({
  copyToClipboard: vi.fn(async () => true),
}))

vi.mock('@/lib/copy-to-clipboard', () => ({
  copyToClipboard: fixtures.copyToClipboard,
}))

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

function token(
  overrides: Partial<OrganizationTokenRow> = {}
): OrganizationTokenRow {
  return {
    id: 1,
    user_id: 7,
    key: 'abcdefghijklmnop',
    status: 1,
    name: 'CI',
    created_time: 0,
    accessed_time: 0,
    expired_time: -1,
    remain_quota: 0,
    unlimited_quota: true,
    model_limits_enabled: false,
    model_limits: '',
    used_quota: 0,
    group: 'default',
    cross_group_retry: false,
    visibility: 'private',
    ...overrides,
  }
}

function renderCell(
  row: OrganizationTokenRow,
  canCopy = true
): ReturnType<typeof render> {
  return render(<OrganizationTokenKeyCell token={row} canCopy={canCopy} />)
}

afterEach(() => {
  fixtures.copyToClipboard.mockClear()
})

describe('organization token key cell', () => {
  test('copies the key with the sk- prefix a relay expects', async () => {
    renderCell(token())

    await userEvent.click(screen.getByRole('button', { name: 'Copy Key' }))

    expect(fixtures.copyToClipboard).toHaveBeenCalledWith('sk-abcdefghijklmnop')
  })

  test('reveals the same prefixed key it copies', async () => {
    renderCell(token())

    // Hidden it is the mask, which already carries the prefix, so the revealed
    // value has to match it rather than dropping the prefix on the way out.
    expect(screen.getByText('sk-abcd**********mnop')).toBeInTheDocument()

    await userEvent.click(
      screen.getByRole('button', { name: 'Show or hide key' })
    )

    expect(screen.getByText('sk-abcdefghijklmnop')).toBeInTheDocument()
  })

  test('leaves a key that already carries the prefix alone', async () => {
    renderCell(token({ key: 'sk-abcdefghijklmnop' }))

    await userEvent.click(screen.getByRole('button', { name: 'Copy Key' }))

    expect(fixtures.copyToClipboard).toHaveBeenCalledWith('sk-abcdefghijklmnop')
  })

  test('does not copy for a caller who may not take the key away', async () => {
    renderCell(token(), false)

    const copyButton = screen.getByRole('button', { name: 'Copy Key' })
    expect(copyButton).toBeDisabled()

    await userEvent.click(copyButton)

    expect(fixtures.copyToClipboard).not.toHaveBeenCalled()
  })
})
