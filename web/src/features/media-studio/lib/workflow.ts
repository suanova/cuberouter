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
import { isAxiosError } from 'axios'
import { z } from 'zod'

import { DEFAULT_PARAMS, qualitySteps } from '../constants'
import type { StudioTemplate, WorkflowDraft } from '../workflow-types'

export const initialDraft: WorkflowDraft = {
  mode: 'create',
  model: '',
  prompt: '',
  size: '1024x1024',
  count: 1,
  quality: 'standard',
  references: [],
}
export const draftSchema = z
  .object({
    mode: z.enum(['create', 'edit']),
    model: z.string().min(1),
    prompt: z.string().trim().min(1).max(16000),
    size: z.string().regex(/^[1-9]\d{1,3}x[1-9]\d{1,3}$/),
    count: z.number().int().min(1).max(4),
    quality: z.enum(['fast', 'standard', 'high']),
    references: z
      .array(z.object({ id: z.string(), url: z.string(), mime: z.string() }))
      .max(3),
    parent_id: z.string().optional(),
  })
  .superRefine((value, ctx) => {
    if (value.mode === 'edit' && !value.references.length) {
      ctx.addIssue({
        code: 'custom',
        path: ['references'],
        message: 'Choose a reference image.',
      })
    }
  })
export function workflowError(error: unknown): string {
  if (isAxiosError(error)) {
    const message: unknown = error.response?.data?.error?.message
    if (typeof message === 'string') return message
  }
  return error instanceof Error
    ? error.message
    : 'Image operation could not be completed.'
}
export function templateDraft(
  template: StudioTemplate,
  fields: Record<string, string>,
  current: WorkflowDraft
): WorkflowDraft {
  return {
    ...current,
    mode: template.mode,
    prompt: template.prompt.replaceAll(
      /\{\{(\w+)\}\}/g,
      (_, key: string) => fields[key] ?? ''
    ),
    // Templates provide ideas; provider-specific resolutions/settings remain user choices.
    count: 1,
    references: template.mode === 'edit' ? current.references : [],
    parent_id: template.mode === 'edit' ? current.parent_id : undefined,
  }
}
export function imageRequest(
  draft: WorkflowDraft,
  imageURLs: string[] = []
): Record<string, unknown> {
  const body: Record<string, unknown> = {
    model: draft.model,
    prompt: draft.prompt.trim(),
    n: draft.count,
    size: draft.size,
    // 画质档位既用于按张计费价目表,也决定机器原生推理步数;
    // seed 与 true_cfg_scale 是机器原生扩展字段,页面不暴露可调项。
    quality: draft.quality,
    num_inference_steps: qualitySteps(draft.quality),
    seed: DEFAULT_PARAMS.seed,
    true_cfg_scale: DEFAULT_PARAMS.cfg,
  }
  if (draft.mode === 'edit') {
    body.images = imageURLs.map((image_url) => ({ image_url }))
  }
  return body
}
