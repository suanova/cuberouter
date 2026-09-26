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
import {
  getServerErrorMessageKey,
  getServerErrorPayload,
} from '@/lib/server-error-message'

function serverErrorText(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function serverErrorBlockers(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value.filter(
    (blocker): blocker is string =>
      typeof blocker === 'string' && blocker.length > 0
  )
}

/**
 * 后端的响应体。拦截器抛出的 Error 本身不是响应体：它自带的 message 是
 * "Network Error" 之类的运行时文本，没有请求真的到过服务端，不能当成拒绝原因
 * 展示给管理员。
 */
function responseBody(error: unknown): Record<string, unknown> | null {
  const payload = getServerErrorPayload(error)
  if (!payload) return null
  if (payload === error && error instanceof Error) return null
  return payload
}

/**
 * 硬删账号被拒绝时给管理员看的话。
 *
 * 组织子系统的拒绝只回稳定 code 加英文 message，而"转让所有权 / 移交 key"这类
 * 处置动作只有 service 的 message 说得清（它点名了具体组织），所以后端 message
 * 原样展示；通用说明取 code 对应的既有文案，blocker token 另起一段列出。管理员
 * 拿到的是一份能照着处理的清单，而不是一句"删除失败"。
 *
 * 没有可展示的内容时返回空串，由调用方决定兜底文案。
 */
export function userDeleteFailureMessage(
  error: unknown,
  t: (key: string) => string
): string {
  const payload = responseBody(error)
  const message = serverErrorText(payload?.message)
  const messageKey = getServerErrorMessageKey(error)
  const blockers = serverErrorBlockers(payload?.blockers)

  const parts = [message]
  if (messageKey) {
    parts.push(t(messageKey))
  }
  if (blockers.length > 0) {
    parts.push(`${t('Blockers')}: ${blockers.join(', ')}`)
  }
  return parts
    .map((part) => part.trim())
    .filter((part) => part.length > 0)
    .join(' ')
}
