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
import { beforeEach, describe, expect, test } from 'vitest'

import { isHeaderNavModuleEnabled } from '../nav-modules'

const cacheStatus = (modules: Record<string, unknown>) => {
  window.localStorage.setItem(
    'status',
    JSON.stringify({ HeaderNavModules: JSON.stringify(modules) })
  )
}

beforeEach(() => {
  window.localStorage.clear()
})

describe('isHeaderNavModuleEnabled', () => {
  test('没有缓存状态时按默认开启处理', () => {
    expect(isHeaderNavModuleEnabled('media_studio')).toBe(true)
  })

  test('缓存里关闭 Media Studio 后返回 false', () => {
    cacheStatus({ media_studio: false })

    expect(isHeaderNavModuleEnabled('media_studio')).toBe(false)
  })

  test('关闭 Media Studio 不影响其它顶部导航开关', () => {
    cacheStatus({ media_studio: false })

    expect(isHeaderNavModuleEnabled('console')).toBe(true)
  })
})
