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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { getAffiliateCode } from '../api'
import { useAffiliate } from '../hooks/use-affiliate'

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    getAffiliateCode: vi.fn(),
    transferAffiliateQuota: vi.fn(),
  }
})

beforeEach(() => {
  vi.mocked(getAffiliateCode).mockResolvedValue({ success: true, data: 'AFF1' })
})

describe('useAffiliate', () => {
  test('推荐计划启用时拉取邀请码并生成邀请链接', async () => {
    const { result } = renderHook(() => useAffiliate(true))

    await vi.waitFor(() => {
      expect(result.current.affiliateLink).toMatch(/\/sign-up\?aff=AFF1$/)
    })
    expect(getAffiliateCode).toHaveBeenCalledTimes(1)
    expect(result.current.loading).toBe(false)
  })

  test('推荐计划未启用时不请求邀请码,避免被动生成 aff_code', () => {
    const { result } = renderHook(() => useAffiliate(false))

    expect(getAffiliateCode).not.toHaveBeenCalled()
    expect(result.current.loading).toBe(false)
    expect(result.current.affiliateLink).toBe('')
  })
})
