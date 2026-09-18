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
export type StudioMode = 'create' | 'edit' | 'regional' | 'text'
export interface StudioAsset {
  id: string
  width: number
  height: number
  expires_at: number
}
export interface WorkflowConfig {
  models: Record<'create' | 'edit' | 'regional', string>
  health: Record<'create' | 'edit' | 'tools', string>
  retention_days: number
  enabled?: boolean
}
export interface WorkflowDraft {
  mode: 'create' | 'edit' | 'regional'
  prompt: string
  size: string
  count: number
  steps: number
  seed: number
  cfg: number
  references: string[]
  negative_prompt: string
  expected_text: string
  parent_id?: string
  mask_id?: string
  edit_mode?: string
}
export interface StudioComparison {
  original?: StudioAsset
  revised?: StudioAsset
  candidate?: StudioAsset
  repair_applied?: boolean
  repair_attempted?: boolean
  method?: string
  notice?: string
}
export interface WorkflowJob {
  id: string
  mode: StudioMode
  state: string
  stage: string
  created_at: number
  expires_at: number
  request: Partial<WorkflowDraft> & { prompt: string; layers?: TextLayer[] }
  parent_id?: string
  completed_steps?: number
  total_steps?: number
  error?: string
  request_id?: string
  billing: 'pending' | 'relay_completed' | 'relay_failed' | 'cpu_tool'
  result?: {
    images: StudioAsset[]
    comparisons: StudioComparison[]
    quality?: unknown[]
    text_quality?: unknown
    notice?: string
    inference_seconds?: number
    workflow_seconds?: number
  }
}
export interface PreparedJob {
  job: WorkflowJob
  relay_body: Record<string, unknown>
}
export type SelectionBox = [number, number, number, number]
export interface TextLayer {
  box: SelectionBox
  text: string
  font_size: number
  color: string
  background: string
  cover: boolean
  align: 'left' | 'center'
}
export interface OCRResult {
  regions: Array<{
    text: string
    box: SelectionBox
    confidence: number
    suggested_background: string
  }>
  recognized_text: string
  comparison: {
    expected_provided: boolean
    matches: boolean
    differences: Array<{ expected: string; recognized: string }>
  }
  seconds: number
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
