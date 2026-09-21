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
export interface StudioAsset {
  id: string
  url: string
  mime: string
}
export interface WorkflowConfig {
  upload_enabled: boolean
  edit_models: string[]
}
export interface WorkflowDraft {
  mode: 'create' | 'edit'
  model: string
  prompt: string
  size: string
  count: number
  advanced: boolean
  steps: number
  seed: number
  cfg: number
  references: StudioAsset[]
  parent_id?: string
}
export interface WorkflowJob {
  id: string
  created_at: number
  request: WorkflowDraft
  images: StudioAsset[]
  elapsed_ms: number
  request_id?: string
}
export interface StudioTemplate {
  id: string
  title: string
  category: string
  collection?: string
  mode: 'create' | 'edit'
  size: string
  description: string
  tip: string
  prompt: string
  preview: string
  before?: string
  seed: number
  steps: number
  cfg: number
  fields: Array<{ key: string; label: string; value: string }>
}
