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

// Run with: node --test docker/dsh/plugins/dsh-api/test/isolation.test.mjs
// The dsh-api core has no Node imports, so it loads directly in Node.

import assert from 'node:assert/strict'
import test from 'node:test'

import { ensureDirectory, ownerFrom, resolveWorkspace } from '../lib/core.js'

test('ownerFrom reads the header, the body, or both', () => {
  assert.deepEqual(ownerFrom({ headers: { 'x-dsh-user-id': '42' } }), { owner: '42' })
  assert.deepEqual(ownerFrom({ headers: {} }, { userId: '42' }), { owner: '42' })
  assert.deepEqual(ownerFrom({ headers: {} }, { userId: 42 }), { owner: '42' })
  assert.deepEqual(
    ownerFrom({ headers: { 'x-dsh-user-id': '42' } }, { userId: '42' }),
    { owner: '42' },
  )
  assert.deepEqual(ownerFrom({ headers: {} }), { owner: undefined })
  assert.deepEqual(ownerFrom(undefined, {}), { owner: undefined })
})

test('ownerFrom rejects malformed or conflicting identities', () => {
  assert.match(ownerFrom({ headers: { 'x-dsh-user-id': '0' } }).error, /positive integer/)
  assert.match(ownerFrom({ headers: { 'x-dsh-user-id': '01' } }).error, /positive integer/)
  assert.match(ownerFrom({ headers: { 'x-dsh-user-id': 'abc' } }).error, /positive integer/)
  assert.match(ownerFrom({ headers: {} }, { userId: 'abc' }).error, /positive integer/)
  assert.match(ownerFrom({ headers: {} }, { userId: 1.5 }).error, /positive integer/)
  assert.match(ownerFrom({ headers: {} }, { userId: {} }).error, /positive integer/)
  assert.match(
    ownerFrom({ headers: { 'x-dsh-user-id': '7' } }, { userId: '8' }).error,
    /disagree/,
  )
})

test('resolveWorkspace keeps legacy behavior without an owner', () => {
  assert.deepEqual(resolveWorkspace('/data/workspace', undefined, undefined, undefined, false), {
    cwd: '/data/workspace',
  })
  assert.deepEqual(
    resolveWorkspace('/data/workspace', undefined, '/tmp/legacy', undefined, false),
    { cwd: '/tmp/legacy' },
  )
})

test('resolveWorkspace places owned tasks under the user directory', () => {
  assert.deepEqual(resolveWorkspace('/data/workspace', '7', undefined, undefined, false), {
    cwd: '/data/workspace/7',
  })
  assert.deepEqual(
    resolveWorkspace('/data/workspace/', '7', '/data/workspace/7/deck', undefined, false),
    { cwd: '/data/workspace/7/deck' },
  )
  assert.deepEqual(resolveWorkspace('/', '7', undefined, undefined, false), { cwd: '/7' })
})

test('resolveWorkspace confines an explicit cwd to the user directory', () => {
  const root = '/data/workspace'
  for (const requested of [
    '/data/workspace/70',
    '/data/workspace/7/../8',
    '/data/workspace/7/../../etc',
    '/etc',
    'relative/path',
    '/data/workspace/7/../../data/workspace/7',
  ]) {
    assert.ok(
      resolveWorkspace(root, '7', requested, undefined, false).error,
      'expected rejection for ' + requested,
    )
  }
  assert.ok(!resolveWorkspace(root, '7', '/data/workspace/7/sub', undefined, false).error)
  assert.ok(!resolveWorkspace(root, '7', '/data/workspace/7/', undefined, false).error)
})

test('resolveWorkspace always inherits the previous cwd on continuations', () => {
  assert.deepEqual(
    resolveWorkspace('/data/workspace', '7', undefined, '/data/workspace/7', true),
    { cwd: '/data/workspace/7' },
  )
  assert.deepEqual(
    resolveWorkspace('/data/workspace', '7', '/data/workspace/7/', '/data/workspace/7', true),
    { cwd: '/data/workspace/7' },
  )
  assert.match(
    resolveWorkspace('/data/workspace', '7', '/data/workspace/8', '/data/workspace/7', true).error,
    /cannot change/,
  )
  assert.match(
    resolveWorkspace('/data/workspace', '7', undefined, undefined, true).error,
    /missing/,
  )
})

test('ensureDirectory short-circuits when the directory exists', async () => {
  const calls = []
  const fs = {
    resolve: async (path) => ({ path }),
    stat: async () => ({ type: 'directory' }),
    writeText: async () => calls.push('write'),
    deleteFile: async () => calls.push('delete'),
  }
  assert.equal(await ensureDirectory(fs, '/data/workspace/7', 'task-1'), true)
  assert.deepEqual(calls, [])
})

test('ensureDirectory creates missing parents through writeText and cleans up', async () => {
  const calls = []
  const fs = {
    resolve: async (path) => ({ path }),
    stat: async () => undefined,
    writeText: async (target, content) => {
      calls.push(['write', target.path, content])
    },
    deleteFile: async (target) => {
      calls.push(['delete', target.path])
    },
  }
  assert.equal(await ensureDirectory(fs, '/data/workspace/7/', 'task-1'), true)
  assert.deepEqual(calls, [
    ['write', '/data/workspace/7/.dsh-api-bootstrap-task-1', ''],
    ['delete', '/data/workspace/7/.dsh-api-bootstrap-task-1'],
  ])
})

test('ensureDirectory reports failure instead of throwing', async () => {
  const denied = {
    resolve: async (path) => ({ path }),
    stat: async () => undefined,
    writeText: async () => {
      throw new Error('FS_SANDBOX_DENIED')
    },
  }
  assert.equal(await ensureDirectory(denied, '/data/workspace/7', 'task-1'), false)
  assert.equal(await ensureDirectory(undefined, '/data/workspace/7', 'task-1'), false)
})
