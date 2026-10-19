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
import { describe, expect, test } from 'vitest'

import { isValidCounterpartUrl } from '../counterpart-url'

describe('isValidCounterpartUrl', () => {
  // 配置值会原样进入页头的 <a href>，所以协议是安全边界而不是格式偏好：
  // React 只把 javascript: 换成无害的 throw，data: 会照常渲染成可点击链接。
  test.each([
    ['https://other.example.com', true],
    ['http://other.example.com', true],
    ['https://other.example.com/base?utm_source=cn#top', true],
    ['data:text/html,<script>alert(1)</script>', false],
    ['javascript:alert(1)', false],
    ['vbscript:msgbox(1)', false],
  ])(
    'accepts HTTP(S) and rejects every other scheme: %s -> %s',
    (value, expected) => {
      expect(isValidCounterpartUrl(value)).toBe(expected)
    }
  )

  test.each([
    ['other.example.com'],
    ['/pricing'],
    ['//other.example.com'],
    [''],
    ['   '],
    [undefined],
  ])('treats %s as not configured', (value) => {
    expect(isValidCounterpartUrl(value)).toBe(false)
  })
})
