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

import { parseStreamMessageUpdates } from './stream-utils'

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
    assert.deepEqual(parseStreamMessageUpdates(JSON.stringify({ choices: [] })), [])
  })
})
