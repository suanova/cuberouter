/*
Copyright (C) 2023-2026 QuantumNous

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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  parsePluginEventPayload,
  parseStreamMessageUpdates,
} from './stream-utils'

function chunkWith(delta: Record<string, unknown>): string {
  return JSON.stringify({ choices: [{ index: 0, delta }] })
}

describe('parseStreamMessageUpdates', () => {
  test('extracts a tool_call plugin_event with payload', () => {
    const updates = parseStreamMessageUpdates(
      chunkWith({
        plugin_event: {
          type: 'tool_call',
          plugin: 'search',
          tool: 'web',
          args: '{"query":"example"}',
          durationMs: 3200,
        },
      })
    )
    assert.equal(updates.length, 1)
    assert.equal(updates[0]?.type, 'plugin_event')
    const event = JSON.parse(updates[0]?.chunk ?? '') as Record<string, unknown>
    assert.equal(event.type, 'tool_call')
    assert.equal(event.plugin, 'search')
    assert.equal(event.tool, 'web')
    assert.equal(event.durationMs, 3200)
  })

  test('extracts an interim plugin_event and keeps content updates', () => {
    const updates = parseStreamMessageUpdates(
      chunkWith({
        content: 'answer',
        plugin_event: { type: 'interim', text: 'Let me search for that.' },
      })
    )
    assert.deepEqual(
      updates.map((update) => update.type),
      ['content', 'plugin_event']
    )
    const event = JSON.parse(updates[1]?.chunk ?? '') as Record<string, unknown>
    assert.equal(event.type, 'interim')
    assert.equal(event.text, 'Let me search for that.')
  })

  test('returns no updates for chunks without a delta', () => {
    assert.deepEqual(parseStreamMessageUpdates(JSON.stringify({})), [])
    assert.deepEqual(
      parseStreamMessageUpdates(JSON.stringify({ choices: [] })),
      []
    )
  })
})

describe('parsePluginEventPayload', () => {
  test('accepts a valid interim event', () => {
    assert.deepEqual(
      parsePluginEventPayload({ type: 'interim', text: 'searching' }),
      {
        type: 'interim',
        text: 'searching',
      }
    )
  })

  test('accepts a valid tool_call event with optional fields', () => {
    assert.deepEqual(
      parsePluginEventPayload({
        type: 'tool_call',
        plugin: 'search',
        tool: 'web',
        args: '{"query":"x"}',
        durationMs: 3200,
      }),
      {
        type: 'tool_call',
        plugin: 'search',
        tool: 'web',
        args: '{"query":"x"}',
        durationMs: 3200,
      }
    )
  })

  test('keeps a zero durationMs instead of dropping it', () => {
    const event = parsePluginEventPayload({
      type: 'tool_call',
      plugin: 'search',
      tool: 'web',
      durationMs: 0,
    })
    assert.deepEqual(event, {
      type: 'tool_call',
      plugin: 'search',
      tool: 'web',
      durationMs: 0,
    })
  })

  test('rejects payloads that are not objects or lack a valid type', () => {
    assert.equal(parsePluginEventPayload(null), null)
    assert.equal(parsePluginEventPayload('tool_call'), null)
    assert.equal(parsePluginEventPayload({ type: 'unknown' }), null)
    assert.equal(parsePluginEventPayload({}), null)
  })

  test('rejects variants missing required fields', () => {
    assert.equal(parsePluginEventPayload({ type: 'interim' }), null)
    assert.equal(parsePluginEventPayload({ type: 'interim', text: 42 }), null)
    assert.equal(
      parsePluginEventPayload({ type: 'tool_call', tool: 'web' }),
      null
    )
    assert.equal(
      parsePluginEventPayload({ type: 'tool_call', plugin: 'search' }),
      null
    )
  })

  test('drops malformed optional fields', () => {
    assert.deepEqual(
      parsePluginEventPayload({
        type: 'tool_call',
        plugin: 'search',
        tool: 'web',
        args: 7,
      }),
      { type: 'tool_call', plugin: 'search', tool: 'web' }
    )
    assert.deepEqual(
      parsePluginEventPayload({
        type: 'tool_call',
        plugin: 'search',
        tool: 'web',
        durationMs: 'fast',
      }),
      { type: 'tool_call', plugin: 'search', tool: 'web' }
    )
  })
})
