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

import type {
  OCRResult,
  PreparedJob,
  StudioAsset,
  TextLayer,
  WorkflowConfig,
  WorkflowDraft,
  WorkflowJob,
} from './workflow-types'

const base = '/api/v1/media-studio'
export const workflowAPI = {
  config: async (): Promise<WorkflowConfig> =>
    (await api.get(`${base}/config`, { skipErrorHandler: true })).data,
  jobs: async (): Promise<WorkflowJob[]> =>
    (await api.get(`${base}/jobs`, { skipErrorHandler: true })).data,
  prepare: async (draft: WorkflowDraft): Promise<PreparedJob> =>
    (await api.post(`${base}/jobs`, draft, { skipErrorHandler: true })).data,
  relay: async (body: Record<string, unknown>): Promise<unknown> =>
    (
      await api.post('/pg/images/generations/studio', body, {
        timeout: 620000,
        skipErrorHandler: true,
      })
    ).data,
  remove: async (id: string): Promise<void> => {
    await api.delete(`${base}/jobs/${id}`)
  },
  upload: async (file: File): Promise<StudioAsset> =>
    (
      await api.post(`${base}/uploads`, file, {
        headers: { 'Content-Type': file.type },
        timeout: 90000,
        skipErrorHandler: true,
      })
    ).data,
  asset: async (id: string, signal?: AbortSignal): Promise<Blob> =>
    (
      await api.get(`${base}/assets/${id}/content`, {
        responseType: 'blob',
        signal,
        disableDuplicate: true,
        skipErrorHandler: true,
      })
    ).data,
  mask: async (assetId: string, png: string): Promise<{ id: string }> =>
    (
      await api.post(
        `${base}/masks`,
        { asset_id: assetId, png_base64: png },
        { skipErrorHandler: true }
      )
    ).data,
  ocr: async (assetId: string, expected: string): Promise<OCRResult> =>
    (
      await api.post(
        `${base}/ocr`,
        { asset_id: assetId, expected_text: expected },
        { timeout: 90000, skipErrorHandler: true }
      )
    ).data,
  text: async (
    assetId: string,
    layers: TextLayer[],
    parentId?: string
  ): Promise<WorkflowJob> =>
    (
      await api.post(
        `${base}/text`,
        { asset_id: assetId, layers, parent_id: parentId },
        { skipErrorHandler: true }
      )
    ).data,
  preview: async (assetId: string, layers: TextLayer[]): Promise<Blob> =>
    (
      await api.post(
        `${base}/preview`,
        { asset_id: assetId, layers },
        { responseType: 'blob', skipErrorHandler: true }
      )
    ).data,
}
