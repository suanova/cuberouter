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

// basePath is read once at module load, exactly as it is in the browser, so each
// case re-imports the module against its own window global.
async function loadResolveServerAddress(injected?: string) {
  vi.resetModules()
  if (injected === undefined) {
    delete window.__BASE_PATH__
  } else {
    window.__BASE_PATH__ = injected
  }
  const { resolveServerAddress } = await import('../server-address')
  return resolveServerAddress
}

afterEach(() => {
  delete window.__BASE_PATH__
})

describe('resolveServerAddress', () => {
  // The administrator types the base a client must call, so the value already
  // carries the deployment prefix. Appending basePath would serve
  // .../cuberouter/cuberouter/v1/... , which is what the code samples and the
  // CC Switch / chat-client presets used to hand out.
  test('takes a configured ServerAddress as the complete public base', async () => {
    const resolveServerAddress = await loadResolveServerAddress('/cuberouter')

    expect(resolveServerAddress('http://127.0.0.1:5000/cuberouter')).toBe(
      'http://127.0.0.1:5000/cuberouter'
    )
  })

  test('does not add the prefix to a configured value that omits it', async () => {
    const resolveServerAddress = await loadResolveServerAddress('/cuberouter')

    expect(resolveServerAddress('http://127.0.0.1:5000')).toBe(
      'http://127.0.0.1:5000'
    )
  })

  test('drops a trailing slash from the configured value', async () => {
    const resolveServerAddress = await loadResolveServerAddress('/cuberouter')

    expect(resolveServerAddress('http://127.0.0.1:5000/cuberouter/')).toBe(
      'http://127.0.0.1:5000/cuberouter'
    )
  })

  // Nothing configured means the only base we know is the one serving the page,
  // and window.location.origin carries no path, so the prefix has to be added.
  test('falls back to this site plus the prefix when nothing is configured', async () => {
    const resolveServerAddress = await loadResolveServerAddress('/cuberouter')

    expect(resolveServerAddress(undefined)).toBe(
      `${window.location.origin}/cuberouter`
    )
    expect(resolveServerAddress('')).toBe(`${window.location.origin}/cuberouter`)
    expect(resolveServerAddress('   ')).toBe(
      `${window.location.origin}/cuberouter`
    )
  })
})
