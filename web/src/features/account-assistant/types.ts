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
export type AccountAssistantConfig = {
  enabled: boolean
  support_url: string
}

export type AccountAssistantHistoryMessage = {
  role: 'user' | 'assistant'
  content: string
}

export type AccountAssistantSseEvent =
  | {
      type: 'tool'
      tool: string
      success?: boolean
      duration_ms?: number
    }
  | { type: 'final'; content: string }
  | { type: 'refusal'; code?: string }
  | { type: 'error'; code?: string; message?: string }

export type AccountAssistantChatMessage = {
  id: string
  role: 'user' | 'assistant'
  content: string
  kind?: 'text' | 'refusal' | 'error'
  code?: string
  tools?: string[]
}

export type StreamAccountAssistantParams = {
  message: string
  history: AccountAssistantHistoryMessage[]
  signal?: AbortSignal
  onEvent: (event: AccountAssistantSseEvent) => void
}
