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

import {
  buildDomainListUpdate,
  domainListToText,
  textToDomainList,
} from '../domain-list'

describe('domain list text helpers', () => {
  it('converts comma-separated domains to one per line', () => {
    expect(domainListToText(' mailinator.com, *.tempmail.com, ')).toBe(
      'mailinator.com\n*.tempmail.com'
    )
  })

  it('converts one domain per line to a comma-separated list', () => {
    expect(textToDomainList('mailinator.com\n\n *.tempmail.com ')).toBe(
      'mailinator.com,*.tempmail.com'
    )
  })

  it('round-trips a normalized list unchanged', () => {
    expect(textToDomainList(domainListToText('a.com,b.com'))).toBe(
      'a.com,b.com'
    )
  })
})

describe('buildDomainListUpdate', () => {
  it('returns null when the edited list equals the saved list', () => {
    expect(
      buildDomainListUpdate(
        'EmailDomainBlacklist',
        'a.com\nb.com',
        'a.com,b.com'
      )
    ).toBeNull()
  })

  it('returns the normalized list when the edit changes it', () => {
    expect(
      buildDomainListUpdate(
        'EmailDomainBlacklist',
        'a.com\nc.com',
        'a.com,b.com'
      )
    ).toEqual({ key: 'EmailDomainBlacklist', value: 'a.com,c.com' })
  })

  it('returns an empty value when the list is cleared', () => {
    expect(
      buildDomainListUpdate('EmailDomainBlacklist', '', 'a.com')
    ).toEqual({ key: 'EmailDomainBlacklist', value: '' })
  })
})
