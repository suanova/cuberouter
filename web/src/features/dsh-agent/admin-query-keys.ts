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

/**
 * DSH 管理端设置的查询键前缀。组织绑定（渠道 + DeepSeek Key）与渠道列表
 * 都在这个前缀下，保存任何一项后统一失效即可一次刷新整块设置。
 */
export const DSH_ADMIN_QUERY_PREFIX = ['system-settings', 'dsh-agent'] as const

export function dshOrganizationConfigQueryKey(organizationId: number) {
  return [...DSH_ADMIN_QUERY_PREFIX, 'organization', organizationId] as const
}
