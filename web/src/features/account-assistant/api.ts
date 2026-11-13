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
import { api, getFreshAuthHeaders } from '@/lib/api'
import { withBasePath } from '@/lib/base-path'

import type {
  AccountAssistantConfig,
  AccountAssistantSseEvent,
  StreamAccountAssistantParams,
} from './types'

export async function getAccountAssistantConfig(): Promise<AccountAssistantConfig> {
  const res = await api.get('/api/account/assistant/config', {
    skipErrorHandler: true,
  })
  const data = res.data?.data as
    | { enabled?: unknown; support_url?: unknown }
    | undefined
  return {
    enabled: data?.enabled === true,
    support_url: typeof data?.support_url === 'string' ? data.support_url : '',
  }
}

export async function streamAccountAssistantChat(
  params: StreamAccountAssistantParams
): Promise<void> {
  const headers = await getFreshAuthHeaders()
  const response = await fetch(withBasePath('/api/account/assistant/chat'), {
    method: 'POST',
    credentials: 'include',
    headers: {
      ...headers,
      Accept: 'text/event-stream',
    },
    body: JSON.stringify({
      message: params.message,
      history: params.history,
    }),
    signal: params.signal,
  })

  const contentType = response.headers.get('content-type') ?? ''
  if (!response.ok || !response.body) {
    throw new Error(`HTTP ${response.status}`)
  }
  // Business validation errors (empty/oversized question, disabled assistant)
  // come back as JSON with HTTP 200, so detect them before reading the stream.
  if (!contentType.includes('text/event-stream')) {
    const payload = (await response.json().catch(() => null)) as {
      message?: unknown
    } | null
    throw new Error(
      typeof payload?.message === 'string'
        ? payload.message
        : 'Account assistant request failed'
    )
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  try {
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() ?? ''
      for (const rawLine of lines) {
        const line = rawLine.trimEnd()
        if (!line.startsWith('data:')) continue
        const data = line.slice(5).trim()
        if (!data) continue
        if (data === '[DONE]') return
        try {
          params.onEvent(JSON.parse(data) as AccountAssistantSseEvent)
        } catch {
          // A malformed frame must not abort a valid stream.
        }
      }
    }
  } finally {
    reader.releaseLock()
  }
}
