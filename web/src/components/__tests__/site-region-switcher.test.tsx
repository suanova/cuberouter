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
import { afterEach, describe, expect, test } from 'vitest'

import { useSystemConfigStore } from '@/stores/system-config-store'

import { SiteRegionSwitcher } from '../site-region-switcher'

const INITIAL_CONFIG = useSystemConfigStore.getState().config

function configure(overrides: {
  enabled: boolean
  url?: string
  label?: string
}) {
  useSystemConfigStore.setState({
    config: {
      ...INITIAL_CONFIG,
      siteCounterpartEnabled: overrides.enabled,
      siteCounterpartUrl: overrides.url ?? '',
      siteCounterpartLabel: overrides.label ?? '',
    },
  })
}

function visit(path: string) {
  window.history.pushState({}, '', path)
}

afterEach(() => {
  useSystemConfigStore.setState({ config: INITIAL_CONFIG })
  window.history.pushState({}, '', '/')
})

describe('SiteRegionSwitcher', () => {
  test('keeps the current path and hash when jumping to the counterpart site', () => {
    configure({ enabled: true, url: 'https://other.example.com' })
    visit('/dashboard/overview#usage')

    render(<SiteRegionSwitcher />)

    expect(screen.getByRole('link')).toHaveAttribute(
      'href',
      'https://other.example.com/dashboard/overview#usage'
    )
  })

  // 两端是独立部署、独立数据库，查询串里的凭据（重置口令的 token、
  // OAuth 的 code）到了对端本就无效；把当前页面的查询串照搬过去，
  // 只会把本机的令牌泄露到另一个域上，所以这里刻意不带 query。
  test('never forwards the current query string to the counterpart site', () => {
    configure({ enabled: true, url: 'https://other.example.com' })
    visit('/reset?email=a%40b.com&token=secret-token')

    render(<SiteRegionSwitcher />)

    const href = screen.getByRole('link').getAttribute('href')
    expect(href).toBe('https://other.example.com/reset')
    expect(href).not.toContain('secret-token')
  })

  test('renders the configured label instead of the fallback', () => {
    configure({
      enabled: true,
      url: 'https://other.example.com',
      label: 'Hong Kong · International',
    })

    render(<SiteRegionSwitcher />)

    expect(screen.getByText('Hong Kong · International')).toBeInTheDocument()
  })

  test('renders nothing while the master switch is off', () => {
    configure({ enabled: false, url: 'https://other.example.com' })

    render(<SiteRegionSwitcher />)

    expect(screen.queryByRole('link')).toBeNull()
  })

  test('renders nothing when the URL is not an absolute HTTP(S) address', () => {
    configure({ enabled: true, url: 'javascript:alert(1)' })

    render(<SiteRegionSwitcher />)

    expect(screen.queryByRole('link')).toBeNull()
  })
})
