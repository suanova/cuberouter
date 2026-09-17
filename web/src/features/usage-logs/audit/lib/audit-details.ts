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
import type { TFunction } from 'i18next'

import { loginMethodLabel } from '@/features/security/components/login-session-utils'
import { ROLE } from '@/lib/roles'

import { renderAuditContent } from '../../lib/format'
import type { LogOtherData } from '../../types'
import type { AuditLog } from '../api'

const AUDIT_ROLE_NAMES: Record<number, string> = {
  [ROLE.GUEST]: 'guest',
  [ROLE.USER]: 'user',
  [ROLE.ADMIN]: 'admin',
  [ROLE.SUPER_ADMIN]: 'root',
}

export function isAuditDetailObject(
  value: unknown
): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

export function auditFieldLabel(key: string, t: TFunction): string {
  switch (key) {
    case 'status':
      return t('Status')
    case 'role':
      return t('Role')
    case 'changed_fields':
      return t('Changed Fields')
    case 'changed':
      return t('State changed')
    case 'count':
      return t('Count')
    case 'total':
      return t('Total')
    case 'sourceId':
      return t('Source ID')
    case 'id':
      return 'ID'
    case 'name':
      return t('Name')
    case 'username':
      return t('Username')
    case 'target_user_id':
      return t('User ID')
    case 'plan_id':
      return t('Plan ID')
    case 'plan_title':
      return t('Plan Title')
    case 'reset_count':
      return t('Reset Count')
    case 'user_count':
      return t('User Count')
    case 'advance_reset_time':
      return t('Advance next reset time')
    case 'bindingType':
      return t('Binding Type')
    case 'from':
      return t('Previous value')
    case 'to':
      return t('New value')
    case 'action':
      return t('Operation')
    case 'method':
      return t('Authentication Method')
    case 'key':
      return t('Key')
    case 'tag':
      return t('Tag')
    case 'group':
      return t('Group')
    case 'models':
      return t('Models')
    case 'type':
      return t('Type')
    case 'base_url':
      return t('Base URL')
    case 'quota':
      return t('Quota')
    case 'admin_info':
      return t('Operator Admin')
    case 'audit_info':
      return t('Request')
    default:
      return key
  }
}

export function buildAuditDetails(entry: AuditLog, t: TFunction) {
  const metadata = isAuditDetailObject(entry.other) ? entry.other : {}
  const metadataUnavailable =
    entry.other != null && !isAuditDetailObject(entry.other)
  const op = isAuditDetailObject(metadata.op) ? metadata.op : {}
  const action = typeof op.action === 'string' ? op.action : entry.action
  const params = isAuditDetailObject(op.params) ? { ...op.params } : {}
  const summaryParams: NonNullable<NonNullable<LogOtherData['op']>['params']> =
    {}
  for (const [key, value] of Object.entries(params)) {
    if (
      typeof value === 'string' ||
      typeof value === 'number' ||
      typeof value === 'boolean'
    ) {
      summaryParams[key] = value
    } else if (
      Array.isArray(value) &&
      value.every((item) => typeof item === 'string')
    ) {
      summaryParams[key] = value
    }
  }
  if (typeof summaryParams.method === 'string') {
    summaryParams.method = loginMethodLabel(summaryParams.method, t)
  }
  if (
    typeof summaryParams.role === 'number' &&
    [0, 1, 10, 100].includes(summaryParams.role)
  ) {
    summaryParams.role = AUDIT_ROLE_NAMES[summaryParams.role]
  }

  let fallback = t('Operation audit')
  if (entry.category === 'login') fallback = t('Login')
  if (entry.category === 'security') fallback = t('Account security')
  if (entry.category === 'access_token') fallback = t('Access Token')
  const summary =
    renderAuditContent({ op: { action, params: summaryParams } }, t) ||
    (entry.content && entry.content !== action ? entry.content : fallback)
  const admin = isAuditDetailObject(metadata.admin_info)
    ? metadata.admin_info
    : {}
  const actorName =
    typeof admin.admin_username === 'string'
      ? admin.admin_username
      : entry.username
  const actorId =
    typeof admin.admin_id === 'number' || typeof admin.admin_id === 'string'
      ? admin.admin_id
      : entry.user_id
  let actor = actorName
  if (actorId) {
    actor = actorName ? `${actorName} (ID: ${actorId})` : `ID: ${actorId}`
  }
  let actorRole = ''
  if ([1, 10, 100].includes(entry.actor_role)) {
    actorRole = AUDIT_ROLE_NAMES[entry.actor_role]
  }
  const authMethod =
    entry.auth_method ||
    (typeof admin.auth_method === 'string' ? admin.auth_method : '')
  let authentication = authMethod
  if (authMethod === 'session') authentication = t('Session')
  else if (authMethod === 'access_token') authentication = t('Access Token')
  else if (authMethod) authentication = loginMethodLabel(authMethod, t)

  let targetName = ''
  if (typeof params.name === 'string') targetName = params.name
  else if (typeof params.username === 'string') targetName = params.username
  const targetId =
    typeof params.id === 'number' || typeof params.id === 'string'
      ? params.id
      : undefined
  let target = targetName
  if (targetId !== undefined) {
    target = targetName ? `${targetName} (ID: ${targetId})` : `ID: ${targetId}`
  }
  if (target) {
    delete params.id
    delete params.name
    delete params.username
  }

  const fields: { label: string; value: unknown }[] = []
  if (
    Array.isArray(params.changed_fields) &&
    params.changed_fields.every((field) => typeof field === 'string')
  ) {
    fields.push({
      label: t('Changed Fields'),
      value: params.changed_fields.length
        ? params.changed_fields
            .map((field) => auditFieldLabel(String(field), t))
            .join(', ')
        : t('Field change details were not recorded'),
    })
    delete params.changed_fields
  }
  if (action.startsWith('channel.') && typeof params.status === 'number') {
    if (params.status === 1) params.status = t('Enabled')
    else if (params.status === 2) params.status = t('Disabled')
    else if (params.status === 3) params.status = t('Auto Disabled')
  }
  if (
    typeof params.role === 'number' &&
    [0, 1, 10, 100].includes(params.role)
  ) {
    params.role = AUDIT_ROLE_NAMES[params.role]
  }
  if (typeof params.method === 'string') {
    params.method = loginMethodLabel(params.method, t)
  }
  if (
    action === 'channel.status_update_batch' &&
    typeof params.count === 'number' &&
    typeof params.total === 'number'
  ) {
    fields.push({
      label: t('Changed / Total'),
      value: `${params.count} / ${params.total}`,
    })
    delete params.count
    delete params.total
  }
  for (const [key, value] of Object.entries(params)) {
    fields.push({ label: auditFieldLabel(key, t), value })
  }
  if (
    typeof metadata.login_method === 'string' &&
    params.method === undefined
  ) {
    fields.push({
      label: t('Login Method'),
      value: loginMethodLabel(metadata.login_method, t),
    })
  }

  const extra = { ...metadata }
  delete extra.op
  delete extra.admin_info
  delete extra.audit_info
  delete extra.login_method
  delete extra.user_agent
  const adminExtra = { ...admin }
  for (const key of [
    'admin_id',
    'admin_username',
    'admin_role',
    'auth_method',
  ]) {
    delete adminExtra[key]
  }
  if (Object.keys(adminExtra).length) extra.admin_info = adminExtra
  if (isAuditDetailObject(metadata.audit_info)) {
    const auditExtra = { ...metadata.audit_info }
    for (const key of ['method', 'route', 'path', 'status', 'success']) {
      delete auditExtra[key]
    }
    if (Object.keys(auditExtra).length) extra.audit_info = auditExtra
  }
  return {
    summary,
    actor,
    actorRole,
    target,
    authentication,
    fields,
    extra,
    metadataUnavailable,
  }
}
