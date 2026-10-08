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
import { beforeEach, describe, expect, it, vi } from 'vitest'

// The deployment prefix is read once at module load, exactly as it is in the
// browser, so every case has to re-import the module after setting the global.
async function loadModule(basePath: string | undefined) {
  vi.resetModules()
  if (basePath === undefined) {
    delete (window as { __BASE_PATH__?: string }).__BASE_PATH__
  } else {
    ;(window as { __BASE_PATH__?: string }).__BASE_PATH__ = basePath
  }
  return import('../oauth-callback-url')
}

const SERVER_ADDRESS = 'https://api.example.com'

describe('buildOAuthCallbackUrl under a URL prefix', () => {
  beforeEach(() => {
    delete (window as { __BASE_PATH__?: string }).__BASE_PATH__
  })

  it('is unchanged at the site root', async () => {
    const { buildOAuthCallbackUrl } = await loadModule(undefined)
    expect(buildOAuthCallbackUrl(SERVER_ADDRESS, '/callback/github', '')).toBe(
      'https://api.example.com/oauth/callback/github'
    )
  })

  // The provider is given the URL the server will build, and the server composes
  // it from ServerAddress alone -- it has no notion of BASE_PATH. So the prefix
  // has to be part of the configured value; adding it here again would produce
  // .../cuberouter/cuberouter/oauth/... and a redirect_uri that never matches.
  it('does not add the prefix to the configured ServerAddress', async () => {
    const { buildOAuthCallbackUrl } = await loadModule('/cuberouter')
    expect(buildOAuthCallbackUrl(SERVER_ADDRESS, '/callback/github', '')).toBe(
      'https://api.example.com/oauth/callback/github'
    )
  })

  it('keeps a prefix that the configured ServerAddress carries', async () => {
    const { buildOAuthCallbackUrl } = await loadModule('/cuberouter')
    expect(
      buildOAuthCallbackUrl(
        `${SERVER_ADDRESS}/cuberouter`,
        '/callback/github',
        ''
      )
    ).toBe('https://api.example.com/cuberouter/oauth/callback/github')
  })

  it('normalizes a trailing slash on ServerAddress', async () => {
    const { buildOAuthCallbackUrl } = await loadModule('/cuberouter')
    expect(
      buildOAuthCallbackUrl(`${SERVER_ADDRESS}/`, '/callback/github', '')
    ).toBe('https://api.example.com/oauth/callback/github')
  })

  it('trims the leading slashes off the callback path', async () => {
    const { buildOAuthCallbackUrl } = await loadModule(undefined)
    expect(buildOAuthCallbackUrl(SERVER_ADDRESS, 'callback/github', '')).toBe(
      'https://api.example.com/oauth/callback/github'
    )
  })

  it('falls back to the provided default when ServerAddress is empty', async () => {
    const { buildOAuthCallbackUrl } = await loadModule('/cuberouter')
    expect(buildOAuthCallbackUrl('', '/callback/github', SERVER_ADDRESS)).toBe(
      'https://api.example.com/cuberouter/oauth/callback/github'
    )
  })
})
