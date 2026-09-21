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
import { expect, test, vi } from 'vitest'

import { TemplateGallery } from '../components/template-gallery'

test('template cards keep quarter-size columns on desktop and two-up on phones', () => {
  render(<TemplateGallery disabled={false} onApply={vi.fn()} />)
  const grid = screen
    .getByRole('region', { name: 'Template gallery' })
    .querySelector('div.grid')
  expect(grid?.className.split(' ')).toEqual(
    expect.arrayContaining(['grid-cols-2', 'md:grid-cols-4', 'xl:grid-cols-6'])
  )
})
