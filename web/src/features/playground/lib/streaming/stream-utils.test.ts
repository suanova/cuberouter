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
import { expect, describe, test } from 'vitest'

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
    expect(updates.length).toBe(1)
    expect(updates[0]?.type).toBe('plugin_event')
    const event = JSON.parse(updates[0]?.chunk ?? '') as Record<string, unknown>
    expect(event.type).toBe('tool_call')
    expect(event.plugin).toBe('search')
    expect(event.tool).toBe('web')
    expect(event.durationMs).toBe(3200)
  })

  test('extracts an interim plugin_event and keeps content updates', () => {
    const updates = parseStreamMessageUpdates(
      chunkWith({
        content: 'answer',
        plugin_event: { type: 'interim', text: 'Let me search for that.' },
      })
    )
    expect(updates.map((update) => update.type)).toEqual(['content', 'plugin_event'])
    const event = JSON.parse(updates[1]?.chunk ?? '') as Record<string, unknown>
    expect(event.type).toBe('interim')
    expect(event.text).toBe('Let me search for that.')
  })

  test('returns no updates for chunks without a delta', () => {
    expect(parseStreamMessageUpdates(JSON.stringify({}))).toEqual([])
    expect(parseStreamMessageUpdates(JSON.stringify({ choices: [] }))).toEqual([])
  })
})

describe('parsePluginEventPayload', () => {
  test('accepts a valid interim event', () => {
    expect(parsePluginEventPayload({ type: 'interim', text: 'searching' })).toEqual({
        type: 'interim',
        text: 'searching',
      })
  })

  test('accepts a valid tool_call event with optional fields', () => {
    expect(parsePluginEventPayload({
        type: 'tool_call',
        plugin: 'search',
        tool: 'web',
        args: '{"query":"x"}',
        durationMs: 3200,
      })).toEqual({
        type: 'tool_call',
        plugin: 'search',
        tool: 'web',
        args: '{"query":"x"}',
        durationMs: 3200,
      })
  })

  test('keeps a zero durationMs instead of dropping it', () => {
    const event = parsePluginEventPayload({
      type: 'tool_call',
      plugin: 'search',
      tool: 'web',
      durationMs: 0,
    })
    expect(event).toEqual({
      type: 'tool_call',
      plugin: 'search',
      tool: 'web',
      durationMs: 0,
    })
  })

  test('rejects payloads that are not objects or lack a valid type', () => {
    expect(parsePluginEventPayload(null)).toBe(null)
    expect(parsePluginEventPayload('tool_call')).toBe(null)
    expect(parsePluginEventPayload({ type: 'unknown' })).toBe(null)
    expect(parsePluginEventPayload({})).toBe(null)
  })

  test('rejects variants missing required fields', () => {
    expect(parsePluginEventPayload({ type: 'interim' })).toBe(null)
    expect(parsePluginEventPayload({ type: 'interim', text: 42 })).toBe(null)
    expect(parsePluginEventPayload({ type: 'tool_call', tool: 'web' })).toBe(null)
    expect(parsePluginEventPayload({ type: 'tool_call', plugin: 'search' })).toBe(null)
  })

  test('drops malformed optional fields', () => {
    expect(parsePluginEventPayload({
        type: 'tool_call',
        plugin: 'search',
        tool: 'web',
        args: 7,
      })).toEqual({ type: 'tool_call', plugin: 'search', tool: 'web' })
    expect(parsePluginEventPayload({
        type: 'tool_call',
        plugin: 'search',
        tool: 'web',
        durationMs: 'fast',
      })).toEqual({ type: 'tool_call', plugin: 'search', tool: 'web' })
  })
})
