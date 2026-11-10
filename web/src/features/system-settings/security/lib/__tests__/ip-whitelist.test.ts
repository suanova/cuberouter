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

import { isIPOrCIDR, splitWhitelist, whitelistSchema } from '../ip-whitelist'

describe('isIPOrCIDR', () => {
  test.each([
    '203.0.113.7',
    '10.0.0.0/8',
    '0.0.0.0/0',
    '255.255.255.255',
    '::1',
    '2001:db8::1/128',
  ])('accepts %s', (value) => {
    expect(isIPOrCIDR(value)).toBe(true)
  })

  test.each([
    'example.com',
    '203.0.113.256',
    '10.0.0.0/33',
    '2001:db8::1/129',
    '::1]#',
    '::1]?x',
    '203.0.113.7/8/8',
  ])('rejects %s', (value) => {
    expect(isIPOrCIDR(value)).toBe(false)
  })
})

describe('splitWhitelist', () => {
  test('trims entries and drops empty lines', () => {
    expect(splitWhitelist(' 203.0.113.7 \n\n 10.0.0.0/8\n')).toEqual([
      '203.0.113.7',
      '10.0.0.0/8',
    ])
  })
})

describe('whitelistSchema', () => {
  test('accepts an empty list', () => {
    expect(whitelistSchema.safeParse({ whitelist: '' }).success).toBe(true)
  })

  test('accepts multiple valid entries', () => {
    expect(
      whitelistSchema.safeParse({ whitelist: '203.0.113.7\n10.0.0.0/8' })
        .success
    ).toBe(true)
  })

  test('reports the i18n message for an invalid entry', () => {
    const result = whitelistSchema.safeParse({
      whitelist: '203.0.113.7\nnot-an-ip',
    })
    expect(result.success).toBe(false)
    expect(result.error?.issues[0].message).toBe(
      'Enter an IP address or CIDR range on each line'
    )
  })
})
