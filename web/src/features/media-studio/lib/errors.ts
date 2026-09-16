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
import { isAxiosError } from 'axios'

export type GenerationErrorKind = 'server' | 'timeout' | 'network' | 'empty'

export interface GenerationErrorInfo {
  kind: GenerationErrorKind
  /** kind 为 server 时是上游原始错误文本；否则为 i18n 键。 */
  message: string
}

interface ServerErrorBody {
  error?: { message?: unknown }
  message?: unknown
  code?: unknown
}

function readServerMessage(data: unknown): string | null {
  if (!data || typeof data !== 'object') {
    return null
  }
  const body = data as ServerErrorBody
  const nested =
    body.error && typeof body.error === 'object' ? body.error : null
  const candidate = nested?.message ?? body.message
  if (typeof candidate === 'string' && candidate.trim() !== '') {
    return candidate
  }
  if (typeof body.code === 'string' && body.code !== '') {
    return body.code
  }
  return null
}

/** 从 axios 错误中提取展示用的错误信息。 */
export function extractGenerationError(err: unknown): GenerationErrorInfo {
  if (isAxiosError(err)) {
    if (err.code === 'ECONNABORTED' || /timeout/i.test(err.message)) {
      return { kind: 'timeout', message: 'Generation timed out' }
    }
    if (err.response) {
      const message = readServerMessage(err.response.data)
      if (message) {
        return { kind: 'server', message }
      }
      return {
        kind: 'server',
        message: `HTTP ${err.response.status}`,
      }
    }
    return { kind: 'network', message: 'Network error' }
  }
  if (err instanceof Error && err.message === 'empty_response') {
    return { kind: 'empty', message: 'Empty response' }
  }
  return { kind: 'network', message: 'Network error' }
}
