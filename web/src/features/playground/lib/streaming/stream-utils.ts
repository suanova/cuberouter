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
import { ERROR_MESSAGES } from '../../constants'
import type { ChatCompletionChunk, PluginEventPayload } from '../../types'

const STREAM_DONE_MESSAGE = '[DONE]'
const STREAM_CLOSED_READY_STATE = 2

export type StreamUpdateType = 'reasoning' | 'content' | 'plugin_event'

export type StreamMessageUpdate = {
  type: StreamUpdateType
  chunk: string
}

type StreamErrorPayload = {
  error?: {
    code?: string
    message?: string
  }
}

export type StreamErrorDetails = {
  errorCode?: string
  errorMessage: string
}

export function parseStreamErrorDetails(data?: string): StreamErrorDetails {
  const fallbackMessage = data || ERROR_MESSAGES.API_REQUEST_ERROR

  if (!data) {
    return { errorMessage: fallbackMessage }
  }

  try {
    const parsed = JSON.parse(data) as StreamErrorPayload

    if (!parsed?.error) {
      return { errorMessage: fallbackMessage }
    }

    return {
      errorCode: parsed.error.code || undefined,
      errorMessage: parsed.error.message || fallbackMessage,
    }
  } catch {
    return { errorMessage: fallbackMessage }
  }
}

// Validates an incoming plugin_event payload from the backend before it is
// persisted on a message. Returns null when the discriminator or a required
// variant field is missing or malformed, so bad events are dropped instead of
// being stored and rendered as broken hints.
export function parsePluginEventPayload(
  payload: unknown
): PluginEventPayload | null {
  if (typeof payload !== 'object' || payload === null) {
    return null
  }
  const event = payload as Record<string, unknown>

  if (event.type === 'interim') {
    if (typeof event.text !== 'string') {
      return null
    }
    return { type: 'interim', text: event.text }
  }

  if (event.type === 'tool_call') {
    if (typeof event.plugin !== 'string' || typeof event.tool !== 'string') {
      return null
    }
    return {
      type: 'tool_call',
      plugin: event.plugin,
      tool: event.tool,
      ...(typeof event.args === 'string' ? { args: event.args } : {}),
      ...(typeof event.durationMs === 'number'
        ? { durationMs: event.durationMs }
        : {}),
    }
  }

  return null
}

export function parseStreamMessageUpdates(data: string): StreamMessageUpdate[] {
  const chunk = JSON.parse(data) as ChatCompletionChunk
  const delta = chunk.choices?.[0]?.delta

  if (!delta) {
    return []
  }

  const updates: StreamMessageUpdate[] = []

  if (delta.reasoning_content) {
    updates.push({ type: 'reasoning', chunk: delta.reasoning_content })
  }

  if (delta.content) {
    updates.push({ type: 'content', chunk: delta.content })
  }

  if (delta.plugin_event) {
    updates.push({
      type: 'plugin_event',
      chunk: JSON.stringify(delta.plugin_event),
    })
  }

  return updates
}

export function isStreamDoneMessage(data: string): boolean {
  return data === STREAM_DONE_MESSAGE
}

export function isStreamClosedReadyState(readyState?: number): boolean {
  return readyState === STREAM_CLOSED_READY_STATE
}

export function getStreamReadyStateError(
  eventReadyState: number | undefined,
  source: unknown
): string | null {
  const status = (source as { status?: number }).status

  if (
    eventReadyState !== undefined &&
    eventReadyState >= STREAM_CLOSED_READY_STATE &&
    status !== undefined &&
    status !== 200
  ) {
    return `HTTP ${status}: ${ERROR_MESSAGES.CONNECTION_CLOSED}`
  }

  return null
}
