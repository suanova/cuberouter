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
  platformOrganizationAuditOrganizationName,
  platformOrganizationAuditQuery,
  platformOrganizationAuditSearchSchema,
} from '../organization-audit-log'

describe('platformOrganizationAuditSearchSchema', () => {
  it('accepts a view with every filter set', () => {
    const parsed = platformOrganizationAuditSearchSchema.parse({
      page: 2,
      pageSize: 50,
      organization: 'acme',
      action: ['organization.dissolve'],
      targetType: ['member'],
    })

    expect(parsed).toEqual({
      page: 2,
      pageSize: 50,
      organization: 'acme',
      action: ['organization.dissolve'],
      targetType: ['member'],
    })
  })

  it('reads a link with nothing in it as an unfiltered first page', () => {
    const parsed = platformOrganizationAuditSearchSchema.parse({})

    expect(parsed.page).toBeUndefined()
    expect(parsed.organization).toBeUndefined()
    expect(parsed.action).toBeUndefined()
  })

  it('falls back rather than failing the route on a hand-edited URL', () => {
    const parsed = platformOrganizationAuditSearchSchema.parse({
      page: 'two',
      organization: 7,
      action: 'organization.dissolve',
      targetType: 'member',
    })

    expect(parsed.page).toBe(1)
    expect(parsed.organization).toBe('')
    expect(parsed.action).toEqual([])
    expect(parsed.targetType).toEqual([])
  })
})

describe('platformOrganizationAuditQuery', () => {
  it('sends the page, and nothing it was not given', () => {
    const params = platformOrganizationAuditQuery({
      columnFilters: [],
      page: 1,
      pageSize: 20,
    })

    expect(params).toEqual({
      p: 1,
      page_size: 20,
      organization_slug: undefined,
      action_type: undefined,
      target_type: undefined,
    })
  })

  it('reduces each filter to the one value the read compares', () => {
    // The table holds a filter as an array, the backend compares one value
    // exactly, so a filter with several values asks about its first.
    const params = platformOrganizationAuditQuery({
      columnFilters: [
        { id: 'action_type', value: ['organization.dissolve'] },
        { id: 'target_type', value: ['member', 'token'] },
      ],
      page: 3,
      pageSize: 50,
    })

    expect(params.action_type).toBe('organization.dissolve')
    expect(params.target_type).toBe('member')
    expect(params.p).toBe(3)
    expect(params.page_size).toBe(50)
  })

  it('carries the slug, which is what the organization filter is', () => {
    const params = platformOrganizationAuditQuery({
      columnFilters: [{ id: 'organization_slug', value: 'acme' }],
      page: 1,
      pageSize: 20,
    })

    expect(params.organization_slug).toBe('acme')
  })

  it('drops a cleared filter instead of sending an empty one', () => {
    // An empty parameter is not an absent one: the backend would compare the
    // slug against nothing and find no records.
    const params = platformOrganizationAuditQuery({
      columnFilters: [
        { id: 'organization_slug', value: '' },
        { id: 'action_type', value: [] },
        { id: 'target_type', value: '   ' },
      ],
      page: 1,
      pageSize: 20,
    })

    expect(params.organization_slug).toBeUndefined()
    expect(params.action_type).toBeUndefined()
    expect(params.target_type).toBeUndefined()
  })
})

describe('platformOrganizationAuditOrganizationName', () => {
  it('names the organization the record was written about', () => {
    expect(
      platformOrganizationAuditOrganizationName({
        organization_id: 5,
        organization_name: 'Acme',
        organization_slug: 'acme',
      })
    ).toBe('Acme')
  })

  it('falls back to the slug when the name is missing', () => {
    // A record written before the name was captured still has the slug.
    expect(
      platformOrganizationAuditOrganizationName({
        organization_id: 5,
        organization_name: '',
        organization_slug: 'acme',
      })
    ).toBe('acme')
  })

  it('falls back to the id when the record names nothing', () => {
    expect(
      platformOrganizationAuditOrganizationName({
        organization_id: 5,
        organization_name: '  ',
        organization_slug: '',
      })
    ).toBe('#5')
  })

  it('says nothing rather than something false about a nameless row', () => {
    expect(
      platformOrganizationAuditOrganizationName({
        organization_id: 0,
        organization_name: '',
        organization_slug: '',
      })
    ).toBe('-')
  })
})
