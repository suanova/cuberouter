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
import { afterEach, describe, expect, test, vi } from 'vitest'

// The prefix is read once at module load, exactly as it is in the browser, so
// each case has to re-import the module against its own window global and its
// own compiled-in import.meta.env.BASE_URL.
async function loadBasePath(
  injected?: string,
  compiled?: string
): Promise<typeof import('../base-path')> {
  vi.resetModules()
  vi.stubEnv('BASE_URL', compiled ?? '')
  if (injected === undefined) {
    delete window.__BASE_PATH__
  } else {
    window.__BASE_PATH__ = injected
  }
  return import('../base-path')
}

afterEach(() => {
  delete window.__BASE_PATH__
  vi.unstubAllEnvs()
})

describe('deployment prefix resolution', () => {
  test('serves from the site root when the server injects nothing', async () => {
    const { basePath, withBasePath } = await loadBasePath()

    expect(basePath).toBe('')
    expect(withBasePath('/api/status')).toBe('/api/status')
  })

  test('treats a bare slash as the site root', async () => {
    const { basePath } = await loadBasePath('/')

    expect(basePath).toBe('')
  })

  test.each([
    ['cuberouter', '/cuberouter'],
    ['/cuberouter', '/cuberouter'],
    ['/cuberouter/', '/cuberouter'],
    ['/g/w1', '/g/w1'],
    ['/g/w1/', '/g/w1'],
  ])('normalizes %o to %o', async (injected, expected) => {
    const { basePath } = await loadBasePath(injected)

    expect(basePath).toBe(expected)
  })
})

// A build run with BASE_PATH set pins the prefix into the bundle via
// import.meta.env.BASE_URL, so the page still reaches its own API when no server
// injected a value -- dist on a static host, or a server without the injection.
describe('compiled-in prefix', () => {
  test('falls back to the compiled prefix when the server injects nothing', async () => {
    const { basePath, withBasePath } = await loadBasePath(
      undefined,
      '/cuberouter/'
    )

    expect(basePath).toBe('/cuberouter')
    expect(withBasePath('/api/channel')).toBe('/cuberouter/api/channel')
  })

  test('prefers the server-injected prefix over the compiled one', async () => {
    const { basePath, withBasePath } = await loadBasePath(
      '/g/w1',
      '/cuberouter/'
    )

    expect(basePath).toBe('/g/w1')
    expect(withBasePath('/api/channel')).toBe('/g/w1/api/channel')
  })

  // The server is authoritative about which paths reach the API, so a server
  // reporting the site root has to win over a prefix pinned at build time.
  test('follows a server that reports the site root over the compiled prefix', async () => {
    const { basePath, withBasePath } = await loadBasePath('', '/cuberouter/')

    expect(basePath).toBe('')
    expect(withBasePath('/api/channel')).toBe('/api/channel')
  })

  test('serves from the site root when the build ran without a prefix', async () => {
    const { basePath } = await loadBasePath(undefined, '/')

    expect(basePath).toBe('')
  })
})

describe('withBasePath', () => {
  test('prefixes an app-owned absolute path', async () => {
    const { withBasePath } = await loadBasePath('/g/w1')

    expect(withBasePath('/api/status')).toBe('/g/w1/api/status')
  })

  test('adds the separator when the caller omits the leading slash', async () => {
    const { withBasePath } = await loadBasePath('/g/w1')

    expect(withBasePath('static/js/index.js')).toBe('/g/w1/static/js/index.js')
  })

  test('leaves absolute URLs alone', async () => {
    const { withBasePath } = await loadBasePath('/g/w1')

    // An administrator may point a docs link at another origin; prefixing it
    // would turn a working external link into a 404 on this host.
    for (const url of [
      'https://docs.example.com/guide',
      'http://docs.example.com/guide',
      '//cdn.example.com/logo.png',
      'data:image/png;base64,AAAA',
      'blob:https://example.com/1234',
      'mailto:support@example.com',
    ]) {
      expect(withBasePath(url)).toBe(url)
    }
  })

  test('returns an empty path unchanged', async () => {
    const { withBasePath } = await loadBasePath('/g/w1')

    expect(withBasePath('')).toBe('')
  })

  // The function is additive, so it must not try to recognise a path that
  // already carries the prefix: that check cannot tell an already-prefixed path
  // from an app-owned one that merely starts with the same segment.
  test.each([
    ['/api/status', '/api/api/status'],
    ['/v1/chat/completions', '/api/v1/chat/completions'],
    ['/static/js/index.js', '/api/static/js/index.js'],
    ['/docs/user/', '/api/docs/user/'],
  ])(
    'prefixes %o to %o even though it starts with the prefix segment',
    async (path, expected) => {
      const { withBasePath } = await loadBasePath('/api')

      expect(withBasePath(path)).toBe(expected)
    }
  )

  test('does not mistake a sibling prefix for its own', async () => {
    const { withBasePath } = await loadBasePath('/cube')

    expect(withBasePath('/cuberouter/dashboard')).toBe(
      '/cube/cuberouter/dashboard'
    )
  })
})

describe('favicon resolution', () => {
  afterEach(() => {
    document.head
      .querySelectorAll('link[rel~="icon"]')
      .forEach((link) => link.remove())
  })

  // Mirrors main.tsx: the raw status.logo is a server path, so it is resolved
  // against the deployment prefix on the way in and applied exactly once.
  test('resolves a raw server logo path once', async () => {
    const { withBasePath } = await loadBasePath('/g/w1')
    const { applyFaviconToDom } = await import('../dom-utils')

    applyFaviconToDom(withBasePath('/logo.png'))

    const icons =
      document.head.querySelectorAll<HTMLLinkElement>('link[rel~="icon"]')
    expect(icons).toHaveLength(1)
    expect(icons[0].href).toBe(`${window.location.origin}/g/w1/logo.png`)
  })

  // Mirrors use-system-config.ts, which hands over config.logo -- a value that
  // has already been through withBasePath.
  test('does not prefix an already-resolved path a second time', async () => {
    await loadBasePath('/g/w1')
    const { applyFaviconToDom } = await import('../dom-utils')

    applyFaviconToDom('/g/w1/logo.png')

    const icons =
      document.head.querySelectorAll<HTMLLinkElement>('link[rel~="icon"]')
    expect(icons).toHaveLength(1)
    expect(icons[0].href).toBe(`${window.location.origin}/g/w1/logo.png`)
    expect(icons[0].href).not.toContain('/g/w1/g/w1/')
  })
})
