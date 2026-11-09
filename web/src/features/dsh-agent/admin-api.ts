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
import { api } from '@/lib/api'

// 管理端 DSH 接口：组织绑定哪个 dsh 实例（渠道）与组织自己的 DeepSeek Key。
// 全部走 RootAuth，错误由调用方内联展示，因此统一跳过全局拦截器。
const DSH_ADMIN_BASE = '/api/admin/dsh'

export type DshAdminChannel = {
  id: number
  name: string
  status: number
  enabled: boolean
  baseUrl: string
  groups: string[]
  // 已绑定到别的组织时非 0，用于下拉项去重提示。
  boundOrganizationId: number
}

export type DshOrganizationConfig = {
  organizationId: number
  organizationName: string
  organizationGroup: string
  organizationStatus: string
  allowed: boolean
  channelId: number
  channelAvailable: boolean
  channelBaseUrl: string
  channelGroups: string[]
  // 该组织的容器名固定为 dsh-org-<id>，这一项只是给管理员抄进渠道表单。
  suggestedBaseUrl: string
  deepseekKeyConfigured: boolean
  deepseekKeyPreview: string
}

export type DshOrganizationUpdate = {
  channelId: number
  deepseekKey?: string
  clearDeepseekKey?: boolean
}

type ApiEnvelope<T> = {
  success?: boolean
  message?: string
  data?: T
}

type DshAdminChannelPayload = {
  id?: unknown
  name?: unknown
  status?: unknown
  base_url?: unknown
  groups?: unknown
  bound_organization_id?: unknown
}

type DshOrganizationConfigPayload = {
  organization_id?: unknown
  organization_name?: unknown
  organization_group?: unknown
  organization_status?: unknown
  allowed?: unknown
  channel_id?: unknown
  channel_available?: unknown
  channel_base_url?: unknown
  channel_groups?: unknown
  suggested_base_url?: unknown
  deepseek_key_configured?: unknown
  deepseek_key_preview?: unknown
}

function readEnvelope<T>(data: unknown): T {
  const envelope = (data ?? {}) as ApiEnvelope<T>
  if (envelope.success === false) {
    throw new Error(envelope.message || 'Request failed')
  }
  return envelope.data as T
}

function toPositiveInt(value: unknown): number {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : 0
}

function toStringArray(value: unknown): string[] {
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === 'string')
    : []
}

function toChannel(payload: DshAdminChannelPayload): DshAdminChannel {
  const status = Number(payload.status) || 0
  return {
    id: Number(payload.id) || 0,
    name: typeof payload.name === 'string' ? payload.name : '',
    status,
    enabled: status === 1,
    baseUrl: typeof payload.base_url === 'string' ? payload.base_url : '',
    groups: toStringArray(payload.groups),
    boundOrganizationId: toPositiveInt(payload.bound_organization_id),
  }
}

function toConfig(payload: DshOrganizationConfigPayload): DshOrganizationConfig {
  return {
    organizationId: Number(payload.organization_id) || 0,
    organizationName:
      typeof payload.organization_name === 'string'
        ? payload.organization_name
        : '',
    organizationGroup:
      typeof payload.organization_group === 'string'
        ? payload.organization_group
        : '',
    organizationStatus:
      typeof payload.organization_status === 'string'
        ? payload.organization_status
        : '',
    allowed: payload.allowed === true,
    channelId: toPositiveInt(payload.channel_id),
    channelAvailable: payload.channel_available === true,
    channelBaseUrl:
      typeof payload.channel_base_url === 'string'
        ? payload.channel_base_url
        : '',
    channelGroups: toStringArray(payload.channel_groups),
    suggestedBaseUrl:
      typeof payload.suggested_base_url === 'string'
        ? payload.suggested_base_url
        : '',
    deepseekKeyConfigured: payload.deepseek_key_configured === true,
    deepseekKeyPreview:
      typeof payload.deepseek_key_preview === 'string'
        ? payload.deepseek_key_preview
        : '',
  }
}

export async function listDshAdminChannels(): Promise<DshAdminChannel[]> {
  const res = await api.get(`${DSH_ADMIN_BASE}/channels`, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  const payload = readEnvelope<DshAdminChannelPayload[]>(res.data)
  return Array.isArray(payload) ? payload.map(toChannel) : []
}

export async function getDshOrganizationConfig(
  organizationId: number
): Promise<DshOrganizationConfig> {
  const res = await api.get(
    `${DSH_ADMIN_BASE}/organizations/${organizationId}`,
    { skipErrorHandler: true } as Record<string, unknown>
  )
  return toConfig(readEnvelope<DshOrganizationConfigPayload>(res.data))
}

export async function updateDshOrganizationConfig(
  organizationId: number,
  update: DshOrganizationUpdate
): Promise<DshOrganizationConfig> {
  const res = await api.put(
    `${DSH_ADMIN_BASE}/organizations/${organizationId}`,
    {
      channel_id: update.channelId,
      ...(update.deepseekKey !== undefined
        ? { deepseek_key: update.deepseekKey }
        : {}),
      ...(update.clearDeepseekKey ? { clear_deepseek_key: true } : {}),
    },
    { skipErrorHandler: true } as Record<string, unknown>
  )
  return toConfig(readEnvelope<DshOrganizationConfigPayload>(res.data))
}

export async function revealDshOrganizationSecret(
  organizationId: number
): Promise<string> {
  const res = await api.post(
    `${DSH_ADMIN_BASE}/organizations/${organizationId}/secret`,
    {},
    { skipErrorHandler: true } as Record<string, unknown>
  )
  const payload = readEnvelope<{ deepseek_key?: unknown }>(res.data)
  const secret = payload?.deepseek_key
  if (typeof secret !== 'string' || secret === '') {
    throw new Error('Organization secret is not configured')
  }
  return secret
}
