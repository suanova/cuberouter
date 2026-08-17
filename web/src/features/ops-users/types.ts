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
export interface OpsUser {
  id: number
  username: string
  display_name: string
  role: number
  status: number
  group: string
  quota: number
  used_quota: number
  request_count: number
  total_prompt_tokens: number
  total_completion_tokens: number
  created_at: number
  aff_code: string
  aff_count: number
  inviter_id: number
  phone?: string
}

export interface OpsUserColumnMeta {
  key: string
  label: string
  required: boolean
}

export interface OpsUsersListData {
  items: OpsUser[]
  total: number
}
