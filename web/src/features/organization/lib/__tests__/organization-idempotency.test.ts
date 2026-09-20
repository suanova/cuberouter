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
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import {
  clearOrganizationIdempotencyKey,
  fingerprintIdempotencyPayload,
  getOrCreateOrganizationIdempotencyKey,
  withOrganizationIdempotencyKey,
} from '../organization-idempotency'

/**
 * The module keeps a process-wide memory map alongside `sessionStorage`, so
 * every case uses its own target id: clearing storage is not enough to get a
 * clean slot.
 */
let target = 0
function nextTarget(): number {
  target += 1
  return target
}

beforeEach(() => {
  sessionStorage.clear()
})

afterEach(() => {
  sessionStorage.clear()
  vi.restoreAllMocks()
})

describe('fingerprintIdempotencyPayload', () => {
  test('ignores property order so a rebuilt payload still matches', () => {
    expect(fingerprintIdempotencyPayload({ a: 1, b: 2 })).toBe(
      fingerprintIdempotencyPayload({ b: 2, a: 1 })
    )
  })

  test('drops undefined members but keeps null and empty values', () => {
    expect(fingerprintIdempotencyPayload({ a: 1, b: undefined })).toBe(
      fingerprintIdempotencyPayload({ a: 1 })
    )
    // An explicitly cleared field is part of the intent, unlike an absent one.
    expect(fingerprintIdempotencyPayload({ a: 1, b: null })).not.toBe(
      fingerprintIdempotencyPayload({ a: 1 })
    )
    expect(fingerprintIdempotencyPayload({ a: '' })).not.toBe(
      fingerprintIdempotencyPayload({ a: 1 })
    )
  })

  test('normalizes nested objects, not just the top level', () => {
    expect(fingerprintIdempotencyPayload({ a: { x: 1, y: 2 } })).toBe(
      fingerprintIdempotencyPayload({ a: { y: 2, x: 1 } })
    )
  })

  test('keeps array order significant', () => {
    expect(fingerprintIdempotencyPayload({ ids: [1, 2] })).not.toBe(
      fingerprintIdempotencyPayload({ ids: [2, 1] })
    )
  })
})

describe('getOrCreateOrganizationIdempotencyKey', () => {
  test('returns the same key for the same intent so a retry replays', () => {
    const id = nextTarget()
    const first = getOrCreateOrganizationIdempotencyKey('dissolve', id, {
      confirm_name: 'acme',
    })
    const second = getOrCreateOrganizationIdempotencyKey('dissolve', id, {
      confirm_name: 'acme',
    })
    expect(second).toBe(first)
  })

  test('returns a new key when the payload changes', () => {
    const id = nextTarget()
    const first = getOrCreateOrganizationIdempotencyKey('dissolve', id, {
      confirm_name: 'acme',
    })
    const second = getOrCreateOrganizationIdempotencyKey('dissolve', id, {
      confirm_name: 'acme',
      reason: 'cleanup',
    })
    expect(second).not.toBe(first)
  })

  test('keys different targets apart even for an identical payload', () => {
    const payload = { confirm_name: 'acme' }
    const first = getOrCreateOrganizationIdempotencyKey('dissolve', 9001, payload)
    const second = getOrCreateOrganizationIdempotencyKey(
      'dissolve',
      9002,
      payload
    )
    expect(second).not.toBe(first)
  })

  test('reuses the key after a reload', async () => {
    const id = nextTarget()
    const payload = { confirm_name: 'acme' }
    const before = getOrCreateOrganizationIdempotencyKey('dissolve', id, payload)

    // A reload drops the in-process map; only sessionStorage carries the slot
    // across, which is exactly what keeps a refreshed page on the same intent.
    vi.resetModules()
    const reloaded = await import('../organization-idempotency')
    const after = reloaded.getOrCreateOrganizationIdempotencyKey(
      'dissolve',
      id,
      payload
    )

    expect(after).toBe(before)
    expect(sessionStorage.length).toBeGreaterThan(0)
  })
})

describe('clearOrganizationIdempotencyKey', () => {
  test('lets the next attempt mint a fresh key', () => {
    const id = nextTarget()
    const payload = { confirm_name: 'acme' }
    const first = getOrCreateOrganizationIdempotencyKey('dissolve', id, payload)
    clearOrganizationIdempotencyKey('dissolve', id, payload)
    expect(getOrCreateOrganizationIdempotencyKey('dissolve', id, payload)).not.toBe(
      first
    )
  })

  test('leaves a different intent alone', () => {
    const id = nextTarget()
    const kept = getOrCreateOrganizationIdempotencyKey('dissolve', id, {
      confirm_name: 'acme',
    })
    clearOrganizationIdempotencyKey('dissolve', id, { confirm_name: 'other' })
    expect(
      getOrCreateOrganizationIdempotencyKey('dissolve', id, {
        confirm_name: 'acme',
      })
    ).toBe(kept)
  })
})

describe('withOrganizationIdempotencyKey', () => {
  test('clears the key after success so the next intent is a new request', async () => {
    const id = nextTarget()
    const payload = { confirm_name: 'acme' }
    const keys: string[] = []
    const request = async (key: string) => {
      keys.push(key)
      return 'ok'
    }

    await withOrganizationIdempotencyKey('dissolve', id, payload, request)
    await withOrganizationIdempotencyKey('dissolve', id, payload, request)

    expect(keys).toHaveLength(2)
    expect(keys[1]).not.toBe(keys[0])
  })

  test('keeps the key after a failure so the retry replays the same intent', async () => {
    const id = nextTarget()
    const payload = { confirm_name: 'acme' }
    const keys: string[] = []
    const request = async (key: string) => {
      keys.push(key)
      if (keys.length === 1) throw new Error('network')
      return 'ok'
    }

    await expect(
      withOrganizationIdempotencyKey('dissolve', id, payload, request)
    ).rejects.toThrow('network')
    await withOrganizationIdempotencyKey('dissolve', id, payload, request)

    expect(keys).toHaveLength(2)
    expect(keys[1]).toBe(keys[0])
  })

  test('shares one request between concurrent identical calls', async () => {
    const id = nextTarget()
    const payload = { confirm_name: 'acme' }
    const request = vi.fn(async () => {
      await Promise.resolve()
      return 'ok'
    })

    const [first, second] = await Promise.all([
      withOrganizationIdempotencyKey('dissolve', id, payload, request),
      withOrganizationIdempotencyKey('dissolve', id, payload, request),
    ])

    expect(request).toHaveBeenCalledTimes(1)
    expect(first).toBe('ok')
    expect(second).toBe('ok')
  })

  test('does not de-duplicate calls that differ in payload', async () => {
    const id = nextTarget()
    const request = vi.fn(async () => 'ok')

    await Promise.all([
      withOrganizationIdempotencyKey('dissolve', id, { confirm_name: 'a' }, request),
      withOrganizationIdempotencyKey('dissolve', id, { confirm_name: 'b' }, request),
    ])

    expect(request).toHaveBeenCalledTimes(2)
  })
})

describe('without usable sessionStorage', () => {
  test('still reuses the key within the page lifetime', () => {
    const id = nextTarget()
    const getItem = vi
      .spyOn(Storage.prototype, 'getItem')
      .mockImplementation(() => {
        throw new Error('storage blocked')
      })
    const setItem = vi
      .spyOn(Storage.prototype, 'setItem')
      .mockImplementation(() => {
        throw new Error('storage blocked')
      })

    try {
      const payload = { confirm_name: 'acme' }
      const first = getOrCreateOrganizationIdempotencyKey('dissolve', id, payload)
      const second = getOrCreateOrganizationIdempotencyKey(
        'dissolve',
        id,
        payload
      )
      expect(second).toBe(first)
    } finally {
      getItem.mockRestore()
      setItem.mockRestore()
    }
  })
})
