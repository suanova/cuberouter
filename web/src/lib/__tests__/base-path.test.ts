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
// each case has to re-import the module against its own window global.
async function loadBasePath(
  injected?: string
): Promise<typeof import('../base-path')> {
  vi.resetModules()
  if (injected === undefined) {
    delete window.__BASE_PATH__
  } else {
    window.__BASE_PATH__ = injected
  }
  return import('../base-path')
}

afterEach(() => {
  delete window.__BASE_PATH__
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

  test('is idempotent for a path that already carries the prefix', async () => {
    // window.location.pathname already includes the prefix, and several callers
    // feed it straight back into withBasePath.
    const { basePath, withBasePath } = await loadBasePath('/g/w1')

    expect(withBasePath(basePath)).toBe('/g/w1')
    expect(withBasePath('/g/w1/dashboard')).toBe('/g/w1/dashboard')
    expect(withBasePath('/g/w1')).toBe('/g/w1')
  })

  test('does not mistake a sibling prefix for its own', async () => {
    const { withBasePath } = await loadBasePath('/cube')

    expect(withBasePath('/cuberouter/dashboard')).toBe(
      '/cube/cuberouter/dashboard'
    )
  })

  test('returns an empty path unchanged', async () => {
    const { withBasePath } = await loadBasePath('/g/w1')

    expect(withBasePath('')).toBe('')
  })
})
