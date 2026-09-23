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
const serverErrorMessageKeys = {
  AUTH_SESSION_LIMIT:
    'Too many active login sessions. On a device where you are already signed in, open Login sessions and use “Sign out other sessions” to revoke them. If you cannot access a signed-in device, reset your password to sign out all sessions.',
  AUTH_SESSION_ISSUANCE_LIMIT:
    'Too many login sessions were created recently. Please wait for the rolling window to pass, then try again.',
  TELEGRAM_BIND_DISABLED: 'Telegram binding is disabled.',
  TELEGRAM_BIND_INVALID_REQUEST:
    'The Telegram authorization request is invalid or expired.',
  TELEGRAM_BIND_FLOW_INVALID:
    'This Telegram binding request has expired or has already been used.',
  TELEGRAM_BIND_SESSION_INVALID:
    'The login session that started this Telegram binding is no longer valid.',
  TELEGRAM_BIND_ALREADY_BOUND: 'This Telegram account is already bound.',
  TELEGRAM_BIND_USER_DELETED: 'This user account no longer exists.',
  TELEGRAM_BIND_USER_DISABLED: 'This user account is disabled.',
  TELEGRAM_BIND_INTERNAL_ERROR: 'Telegram binding failed. Please try again.',

  // 组织相关的错误码。后端刻意只回稳定 code 加英文 message（见 types/organization_error.go），
  // 界面文案由前端按 code 决定，这样多语言和措辞调整都不用动后端。
  // 注意这里的键必须和 types.ErrorCode 的取值逐字相同（小写蛇形），不能跟着上面的
  // 大写风格走：查找是按 payload.code 精确匹配的。
  organization_context_mismatch:
    'Your account context changed. Reload the page and try again.',
  organization_access_denied:
    'You do not have permission to perform this action in this organization.',
  organization_disabled:
    'This organization is disabled. Contact a platform administrator.',
  organization_dissolved:
    'This organization has been dissolved and can no longer be used.',
  organization_limit_exceeded:
    'You have reached the maximum number of organizations you can create or join.',
  organization_name_conflict:
    'An organization with this name already exists.',
  insufficient_organization_quota:
    'The organization does not have enough quota for this request.',
  organization_idempotency_conflict:
    'This request conflicts with an earlier one. Reload and try again.',
  organization_idempotency_key_required:
    'This action requires an idempotency key.',
  organization_confirmation_mismatch:
    'The confirmation text does not match. Enter the exact organization name.',
  organization_billing_session_conflict:
    'The organization is settling another request. Try again in a moment.',
  organization_member_operation_forbidden:
    'That member operation is not allowed for this organization.',
  organization_operation_blocked:
    'This operation is blocked. Resolve the listed blockers and try again.',
  organization_invite_unavailable:
    'This invitation is no longer available. Ask an administrator for a new one.',
  organization_invite_delivery_failed:
    'The invitation email could not be delivered.',
  organization_invite_delivery_in_progress:
    'The invitation is still being delivered. Try again shortly.',
  organization_invite_recipient_rejected:
    'The mail server rejected the invitation recipient.',
  organization_invite_already_sent:
    'An invitation for this email address is already pending.',
  organization_join_rule_invalid:
    'Some lines of the join rules are not usable. Fix the flagged lines and submit again.',
  organization_token_responsible_member_disabled:
    'The member responsible for this key is disabled.',
  organization_token_responsible_user_disabled:
    'The user responsible for this key is disabled.',
  organization_token_enable_forbidden:
    'This key cannot be enabled while a blocker is active.',
} as const

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object'
}

function serverErrorPayload(value: unknown): Record<string, unknown> | null {
  if (!isRecord(value)) return null

  const response = value.response
  if (isRecord(response) && isRecord(response.data)) {
    return response.data
  }
  return value
}

export function getServerErrorMessageKey(value: unknown): string | null {
  const payload = serverErrorPayload(value)
  if (!payload || typeof payload.code !== 'string') return null

  return (
    serverErrorMessageKeys[
      payload.code as keyof typeof serverErrorMessageKeys
    ] ?? null
  )
}

/**
 * 取出后端返回的稳定错误码。
 *
 * 和后端的约定是「永远只回稳定 code + 英文 message」（见 types/organization_error.go）：
 * 调用方按 code 做分支判断，message 只用于日志。文案表上面那份是给用户看的，
 * 这里只关心 code 本身，两者不要混用。
 */
export function getServerErrorCode(value: unknown): string | null {
  const payload = serverErrorPayload(value)
  if (!payload || typeof payload.code !== 'string' || !payload.code) return null
  return payload.code
}
