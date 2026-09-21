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

import type { StudioTemplate, WorkflowDraft } from '../workflow-types'

export const initialDraft: WorkflowDraft = {
  mode: 'create',
  model: '',
  prompt: '',
  size: '1024x1024',
  count: 1,
  advanced: false,
  steps: 40,
  seed: 42,
  cfg: 4,
  references: [],
}
export const numericSettings = z.object({
  steps: z.number().int().min(1).max(100),
  seed: z.number().int().min(0).max(9007199254740987),
  cfg: z.number().min(0).max(10),
})
export const draftSchema = z
  .object({
    mode: z.enum(['create', 'edit']),
    model: z.string().min(1),
    prompt: z.string().trim().min(1).max(16000),
    size: z.string().regex(/^[1-9]\d{1,3}x[1-9]\d{1,3}$/),
    count: z.number().int().min(1).max(4),
    advanced: z.boolean(),
    ...numericSettings.shape,
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
  }
  if (draft.advanced) {
    Object.assign(body, {
      num_inference_steps: draft.steps,
      seed: draft.seed,
      true_cfg_scale: draft.cfg,
    })
  }
  if (draft.mode === 'edit') {
    body.images = imageURLs.map((image_url) => ({ image_url }))
  }
  return body
}
export function publicCommand(draft: WorkflowDraft): string {
  const endpoint = draft.mode === 'edit' ? 'edits' : 'generations'
  const body = imageRequest(
    draft,
    draft.references.map(() => '<reference image URL>')
  )
  return `POST /pg/images/${endpoint}\nContent-Type: application/json\n\n${JSON.stringify(body, null, 2)}`
}
