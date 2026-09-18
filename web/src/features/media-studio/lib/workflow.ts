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

import type {
  StudioTemplate,
  WorkflowDraft,
  WorkflowJob,
} from '../workflow-types'

export const CREATE_SIZES: Record<string, string> = {
  '1:1': '1328x1328',
  '16:9': '1664x928',
  '9:16': '928x1664',
  '4:3': '1472x1104',
  '3:4': '1104x1472',
  '3:2': '1584x1056',
  '2:3': '1056x1584',
}
export const initialDraft: WorkflowDraft = {
  mode: 'create',
  prompt: '',
  size: '1664x928',
  count: 1,
  steps: 40,
  seed: 42,
  cfg: 1,
  references: [],
  negative_prompt: '',
  expected_text: '',
}
export const draftSchema = z
  .object({
    mode: z.enum(['create', 'edit', 'regional']),
    prompt: z.string().trim().min(1).max(16000),
    size: z.string(),
    count: z.number().int().min(1).max(4),
    steps: z.number().int().min(1).max(100),
    seed: z.number().int().min(0).max(9007199254740987),
    cfg: z.number().min(0).max(10),
    references: z.array(z.string()).max(3),
    negative_prompt: z.string().max(8000),
    expected_text: z.string().max(10000),
    parent_id: z.string().optional(),
    mask_id: z.string().optional(),
    edit_mode: z.string().optional(),
  })
  .superRefine((value, ctx) => {
    if (
      value.mode !== 'create' &&
      (!value.references.length || value.steps > 60)
    ) {
      ctx.addIssue({
        code: 'custom',
        path: ['references'],
        message: 'Editing needs a reference image and at most 60 steps.',
      })
    }
  })

export function isActiveJob(job: WorkflowJob): boolean {
  return ![
    'completed',
    'failed',
    'needs_review',
    'interrupted',
    'prepared',
  ].includes(job.state)
}
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
  const prompt = template.prompt.replaceAll(
    /\{\{(\w+)\}\}/g,
    (_, key: string) => fields[key] ?? ''
  )
  const references = template.mode === 'edit' ? current.references : []
  return {
    ...current,
    mode: template.mode,
    prompt,
    size: CREATE_SIZES[template.size] ?? template.size,
    seed: template.seed,
    steps: template.steps,
    cfg: template.cfg,
    count: 1,
    references,
    parent_id: template.mode === 'edit' ? current.parent_id : undefined,
    mask_id: undefined,
    expected_text: '',
  }
}
export function publicCommand(job: WorkflowJob): string {
  // The private relay capability and channel key must never be copied into examples.
  if (job.mode === 'text') {
    const body = JSON.stringify(
      {
        asset_id: job.request.references?.[0],
        layers: job.request.layers,
        parent_id: job.parent_id,
      },
      null,
      2
    )
    return `Invoke-RestMethod -Method Post -Uri "$CubeRouter/api/v1/media-studio/text" -Headers @{Authorization="Bearer $SessionToken"} -ContentType 'application/json' -Body @'\n${body}\n'@`
  }
  const body = JSON.stringify(job.request, null, 2)
  return `$job = Invoke-RestMethod -Method Post -Uri "$CubeRouter/api/v1/media-studio/jobs" -Headers @{Authorization="Bearer $SessionToken"} -ContentType 'application/json' -Body @'\n${body}\n'@\nInvoke-RestMethod -Method Post -Uri "$CubeRouter/pg/images/generations/studio" -Headers @{Authorization="Bearer $SessionToken"} -ContentType 'application/json' -Body ($job.relay_body | ConvertTo-Json -Depth 10)`
}
