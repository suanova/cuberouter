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

/**
 * Cuby apps: the plug-ins of the Cuby desktop client, and which accounts may
 * use them. Cuby asks this instance for an account's list; an administrator
 * keeps that list here.
 */
export type CubyAppStatus = 'enabled' | 'disabled'
export type CubyAppHolderType = 'user' | 'organization'

export interface CubyApp {
  id: number
  /** The id the app's own cuby-app.json carries. Never changes. */
  app_id: string
  name: string
  description: string
  status: CubyAppStatus
  created_at: number
  updated_at: number
  user_grant_count: number
  organization_grant_count: number
}

export interface CubyAppGrant {
  id: number
  app_id: string
  subject_type: CubyAppHolderType
  subject_id: number
  created_at: number
  /** Empty when the organization or user no longer exists. */
  organization_name?: string
  organization_status?: string
  username?: string
  display_name?: string
  email?: string
}

interface CubyAppEnvelope<T = undefined> {
  success: boolean
  message?: string
  data?: T
}

export interface CubyAppCatalog {
  /** Off: this instance tells Cuby it does not manage apps, and Cuby limits nothing. */
  enabled: boolean
  apps: CubyApp[]
}

export async function listCubyApps(): Promise<CubyAppEnvelope<CubyAppCatalog>> {
  const res = await api.get('/api/admin/cuby-apps')
  return res.data
}

export async function setCubyAppManagement(
  enabled: boolean
): Promise<CubyAppEnvelope<{ enabled: boolean }>> {
  const res = await api.put('/api/admin/cuby-app-setting', { enabled })
  return res.data
}

export async function createCubyApp(data: {
  app_id: string
  name: string
  description: string
}): Promise<CubyAppEnvelope<CubyApp>> {
  const res = await api.post('/api/admin/cuby-apps', data)
  return res.data
}

export async function updateCubyApp(
  id: number,
  data: { name?: string; description?: string; status?: CubyAppStatus }
): Promise<CubyAppEnvelope<CubyApp>> {
  const res = await api.patch(`/api/admin/cuby-apps/${id}`, data)
  return res.data
}

export async function deleteCubyApp(id: number): Promise<CubyAppEnvelope> {
  const res = await api.delete(`/api/admin/cuby-apps/${id}`)
  return res.data
}

export async function listCubyAppGrants(
  id: number,
  params: { p: number; page_size: number }
): Promise<CubyAppEnvelope<{ items: CubyAppGrant[]; total: number }>> {
  const res = await api.get(`/api/admin/cuby-apps/${id}/grants`, { params })
  return res.data
}

export async function createCubyAppGrant(
  id: number,
  data: { subject_type: CubyAppHolderType; subject_id: number }
): Promise<CubyAppEnvelope<CubyAppGrant>> {
  const res = await api.post(`/api/admin/cuby-apps/${id}/grants`, data)
  return res.data
}

export async function deleteCubyAppGrant(
  id: number,
  grantId: number
): Promise<CubyAppEnvelope> {
  const res = await api.delete(`/api/admin/cuby-apps/${id}/grants/${grantId}`)
  return res.data
}
