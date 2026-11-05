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
import { describe, expect, test, vi } from 'vitest'

import { UsageExportMenu } from '../usage-export-menu'

describe('UsageExportMenu', () => {
  test('offers daily first and hourly second', async () => {
    const onExport = vi.fn()
    render(<UsageExportMenu exporting={null} onExport={onExport} />)

    await userEvent.click(screen.getByRole('button', { name: /Export/ }))
    await userEvent.click(
      await screen.findByRole('menuitem', { name: 'Daily' })
    )
    expect(onExport).toHaveBeenLastCalledWith('day')

    await userEvent.click(screen.getByRole('button', { name: /Export/ }))
    await userEvent.click(
      await screen.findByRole('menuitem', { name: 'Hourly' })
    )
    expect(onExport).toHaveBeenLastCalledWith('hour')
  })

  test('disables the trigger while exporting or without rows', () => {
    const onExport = vi.fn()
    const { rerender } = render(
      <UsageExportMenu exporting={null} onExport={onExport} />
    )
    expect(screen.getByRole('button', { name: /Export/ })).toBeEnabled()

    rerender(<UsageExportMenu exporting='hour' onExport={onExport} />)
    expect(screen.getByRole('button', { name: /Exporting/ })).toBeDisabled()

    rerender(<UsageExportMenu exporting={null} disabled onExport={onExport} />)
    expect(screen.getByRole('button', { name: /Export/ })).toBeDisabled()
  })
})
