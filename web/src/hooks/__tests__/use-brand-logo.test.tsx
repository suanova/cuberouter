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
import { renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, test } from 'vitest'

import { DEFAULT_LOGO } from '@/lib/constants'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { DEFAULT_BRAND_LOGO, useBrandLogo } from '../use-brand-logo'

beforeEach(() => {
  useSystemConfigStore.getState().setConfig({ logo: DEFAULT_LOGO })
})

describe('useBrandLogo', () => {
  test('falls back to the bundled wordmark when no administrator logo is set', () => {
    const { result } = renderHook(() => useBrandLogo())

    expect(result.current.logo).toBe(DEFAULT_BRAND_LOGO)
    expect(result.current.recolorForDark).toBe(true)
  })

  test('shows a configured logo untouched so dark mode cannot invert it', () => {
    useSystemConfigStore
      .getState()
      .setConfig({ logo: 'https://cdn.example.com/brand.png' })

    const { result } = renderHook(() => useBrandLogo())

    expect(result.current.logo).toBe('https://cdn.example.com/brand.png')
    expect(result.current.recolorForDark).toBe(false)
  })
})
