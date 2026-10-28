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

import {
  CFG_RANGE,
  DEFAULT_PARAMS,
  LIMITS,
  qualitySteps,
  STEPS_RANGE,
} from '../constants'
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
    prompt: z.string().trim().min(1).max(LIMITS.promptMax),
    size: z.string().regex(/^[1-9]\d{1,3}x[1-9]\d{1,3}$/),
    count: z.number().int().min(1).max(4),
    quality: z.enum(['fast', 'standard', 'high']),
    references: z
      .array(z.object({ id: z.string(), url: z.string(), mime: z.string() }))
      .max(LIMITS.maxReferences),
    steps: z
      .number()
      .int()
      .min(STEPS_RANGE.min)
      .max(STEPS_RANGE.max)
      .optional(),
    cfg: z.number().min(CFG_RANGE.min).max(CFG_RANGE.max).optional(),
    seed: z.number().int().min(0).max(Number.MAX_SAFE_INTEGER).optional(),
    mask: z
      .array(
        z.object({
          x: z.number().min(0).max(1),
          y: z.number().min(0).max(1),
          width: z.number().gt(0).max(1),
          height: z.number().gt(0).max(1),
        })
      )
      .max(LIMITS.maxMaskRegions)
      .optional(),
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
    // 模板自带可复现的机器参数，应用时一并写入；用户之后仍可改回留空。
    steps: template.steps,
    cfg: template.cfg,
    seed: template.seed,
    // Templates provide ideas; provider-specific resolutions/settings remain user choices.
    count: 1,
    references: template.mode === 'edit' ? current.references : [],
    // 框选区域与参考图绑定：参考图被清掉时蒙版也一起清，否则会框在不存在的位置上。
    mask: template.mode === 'edit' ? current.mask : undefined,
    parent_id: template.mode === 'edit' ? current.parent_id : undefined,
  }
}
export function imageRequest(
  draft: WorkflowDraft,
  referenceURLs: string[] = [],
  maskURL?: string
): Record<string, unknown> {
  const body: Record<string, unknown> = {
    model: draft.model,
    prompt: draft.prompt.trim(),
    n: draft.count,
    size: draft.size,
    // 画质档位既用于按张计费价目表,也决定机器原生推理步数;显式填写的
    // steps/cfg/seed 优先，留空才回落到档位与机器默认值。
    quality: draft.quality,
    num_inference_steps: draft.steps ?? qualitySteps(draft.quality),
    seed: draft.seed ?? DEFAULT_PARAMS.seed,
    true_cfg_scale: draft.cfg ?? DEFAULT_PARAMS.cfg,
  }
  if (draft.mode === 'edit' && referenceURLs.length) {
    // 编辑渠道只认内联 base64：参考图在 draft 里本来就是 data URL，直接下发，
    // 多张时用数组。下发网址会被上游按 base64 解码，报 Incorrect padding。
    body.image =
      referenceURLs.length === 1 ? referenceURLs[0] : referenceURLs
    // 蒙版是同尺寸 PNG（透明 = 重绘区），只有用户真的框了区域才带。
    if (maskURL) {
      body.mask = maskURL
    }
  }
  return body
}
