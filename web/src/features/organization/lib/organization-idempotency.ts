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

/**
 * Retry keys for the organization operations that must not apply twice.
 *
 * The backend deduplicates on `Idempotency-Key`, so the client's job is to send
 * the *same* key again when the user retries the same intent and a *different*
 * one when the intent changes. The key is therefore remembered against a
 * fingerprint of the request payload: editing the reason and resubmitting is a
 * new intent, while clicking the button twice because the first attempt timed
 * out is the same one.
 *
 * The slot is persisted in `sessionStorage` so an accidental reload does not
 * turn a retry into a second application. The in-memory map in front of it is
 * not just a cache — it is what keeps the key alive when storage is unavailable
 * (privacy settings, private mode, tests).
 */

const STORAGE_PREFIX = 'cuberouter:organization-idempotency:'

const memoryRecords = new Map<string, IdempotencyRecord>()
const inFlight = new Map<string, Promise<unknown>>()

interface IdempotencyRecord {
  fingerprint: string
  key: string
}

/** Operations the backend accepts an `Idempotency-Key` for. */
export type OrganizationIdempotentOperation =
  | 'disable-member'
  | 'remove-member'
  | 'exit-member'
  | 'rotate-invite'
  | 'dissolve'
  | 'batch-create-token'
  | 'batch-delete-token'
  | 'transfer-owner'
  | 'quota-adjustment'

type JsonValue = unknown

/**
 * Stable serialization of the payload: keys are sorted and `undefined` members
 * are dropped, so two payloads that mean the same thing produce the same
 * fingerprint regardless of property order or which optional fields a caller
 * happened to spell out.
 */
function normalizePayload(value: JsonValue): JsonValue {
  if (Array.isArray(value)) return value.map(normalizePayload)
  if (value && typeof value === 'object') {
    const source = value as Record<string, JsonValue>
    const result: Record<string, JsonValue> = {}
    for (const key of Object.keys(source).sort()) {
      if (source[key] !== undefined) result[key] = normalizePayload(source[key])
    }
    return result
  }
  return value
}

export function fingerprintIdempotencyPayload(payload: JsonValue): string {
  return JSON.stringify(normalizePayload(payload ?? {}))
}

function storageSlot(
  operation: string,
  targetId: string | number,
  fingerprint: string
): string {
  // `encodeURIComponent` keeps the `:` separators unambiguous whatever the
  // caller passes as a target id.
  return `${STORAGE_PREFIX}${encodeURIComponent(operation)}:${encodeURIComponent(
    String(targetId)
  )}:${encodeURIComponent(fingerprint)}`
}

/** `sessionStorage` is absent under SSR and throws in some privacy modes. */
function idempotencyStorage(): Storage | null {
  try {
    return globalThis.sessionStorage ?? null
  } catch {
    return null
  }
}

function readRecord(slot: string): IdempotencyRecord | null {
  const memoryRecord = memoryRecords.get(slot)
  if (memoryRecord) return memoryRecord

  const storage = idempotencyStorage()
  if (!storage) return null
  try {
    const record = JSON.parse(storage.getItem(slot) ?? 'null')
    if (record?.key && typeof record.fingerprint === 'string') {
      memoryRecords.set(slot, record)
      return record
    }
  } catch {
    // An unreadable slot is a spent one: drop it so the next write can land.
    try {
      storage.removeItem(slot)
    } catch {
      // Keep using the in-memory fallback when storage is unavailable.
    }
  }
  return null
}

function writeRecord(slot: string, record: IdempotencyRecord): void {
  memoryRecords.set(slot, record)
  const storage = idempotencyStorage()
  if (!storage) return
  try {
    storage.setItem(slot, JSON.stringify(record))
  } catch {
    // The memory record still preserves retries for this page's lifetime.
  }
}

function generateKey(operation: string, targetId: string | number): string {
  const cryptoApi = globalThis.crypto
  const suffix =
    typeof cryptoApi?.randomUUID === 'function'
      ? cryptoApi.randomUUID()
      : `${Date.now()}:${Math.random().toString(36).slice(2)}`
  return `${operation}:${targetId}:${suffix}`
}

/** The key to send for this intent, reusing a previous one when it exists. */
export function getOrCreateOrganizationIdempotencyKey(
  operation: OrganizationIdempotentOperation,
  targetId: string | number,
  payload: JsonValue = {}
): string {
  const fingerprint = fingerprintIdempotencyPayload(payload)
  const slot = storageSlot(operation, targetId, fingerprint)
  const existing = readRecord(slot)
  if (existing?.fingerprint === fingerprint) return existing.key

  const key = generateKey(operation, targetId)
  writeRecord(slot, { fingerprint, key })
  return key
}

/**
 * Forgets the key for this intent. Called only after the request succeeded:
 * once it is committed, retrying is no longer the same intent and must get a
 * fresh key, or the backend would replay the first response and the second
 * change would silently not happen.
 */
export function clearOrganizationIdempotencyKey(
  operation: OrganizationIdempotentOperation,
  targetId: string | number,
  payload: JsonValue = {}
): void {
  const fingerprint = fingerprintIdempotencyPayload(payload)
  const slot = storageSlot(operation, targetId, fingerprint)
  const existing = readRecord(slot)
  if (existing?.fingerprint !== fingerprint) return

  memoryRecords.delete(slot)
  try {
    idempotencyStorage()?.removeItem(slot)
  } catch {
    // Storage cleanup is best effort; the completed request must still succeed.
  }
}

/**
 * Runs `request` with an idempotency key derived from the intent, and clears
 * the key once it resolves.
 *
 * A failure deliberately leaves the key in place — that is what makes the
 * user's next attempt a retry rather than a duplicate. Concurrent identical
 * calls share one promise so a double-click cannot open two requests with
 * different keys.
 */
export async function withOrganizationIdempotencyKey<T>(
  operation: OrganizationIdempotentOperation,
  targetId: string | number,
  payload: JsonValue,
  request: (idempotencyKey: string) => Promise<T>
): Promise<T> {
  const fingerprint = fingerprintIdempotencyPayload(payload)
  const slot = storageSlot(operation, targetId, fingerprint)

  const existing = inFlight.get(slot) as Promise<T> | undefined
  if (existing) return existing

  const pending = (async () => {
    const key = getOrCreateOrganizationIdempotencyKey(
      operation,
      targetId,
      payload
    )
    const result = await request(key)
    clearOrganizationIdempotencyKey(operation, targetId, payload)
    return result
  })()

  inFlight.set(slot, pending)
  try {
    return await pending
  } finally {
    if (inFlight.get(slot) === pending) inFlight.delete(slot)
  }
}
