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
import { describe, expect, it } from 'vitest'

import { getFlowStages } from '../flow'

describe('getFlowStages', () => {
  it('keeps only token and model columns for the user dashboard flow view', () => {
    expect(getFlowStages('user')).toEqual(['token', 'model'])
  })

  it('keeps the wider column set for admin and root roles', () => {
    expect(getFlowStages('admin')).toEqual([
      'user',
      'group',
      'model',
      'channel',
    ])
    expect(getFlowStages('root')).toEqual([
      'user',
      'node',
      'token',
      'group',
      'model',
      'channel',
    ])
  })
})
