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
import { describe, expect, test } from 'vitest'

import { MaintenanceInfoSection } from '../maintenance-info-section'

describe('MaintenanceInfoSection', () => {
  test('shows version and uptime without an update check control', () => {
    render(
      <MaintenanceInfoSection
        currentVersion='v2.0.0-fafdd71'
        startTime={1_700_000_000}
      />
    )

    expect(screen.getByText('Current version')).toBeInTheDocument()
    expect(screen.getByText('v2.0.0-fafdd71')).toBeInTheDocument()
    expect(screen.getByText('Uptime since')).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Check for updates' })
    ).not.toBeInTheDocument()
  })

  test('falls back to Unknown when version and start time are unavailable', () => {
    render(<MaintenanceInfoSection currentVersion={null} startTime={null} />)

    expect(screen.getAllByText('Unknown')).toHaveLength(2)
  })
})
