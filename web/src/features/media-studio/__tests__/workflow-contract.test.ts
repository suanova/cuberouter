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
import { describe, expect, test } from 'vitest'

import catalog from '../catalog.json'
import {
  draftSchema,
  initialDraft,
  publicCommand,
  templateDraft,
} from '../lib/workflow'
import type { StudioTemplate, WorkflowJob } from '../workflow-types'

describe('Studio workflow requests', () => {
  test('text correction examples call the CPU tool rather than starting another paid GPU job', () => {
    const command = publicCommand({
      mode: 'text',
      request: {
        prompt: 'Text correction',
        references: ['source'],
        layers: [],
      },
      parent_id: 'parent',
    } as unknown as WorkflowJob)
    expect(command).toContain('/api/v1/media-studio/text')
    expect(command).toContain('"asset_id": "source"')
    expect(command).not.toContain('/pg/images/generations')
  })
  test('edit templates preserve user references without importing example images', () => {
    const template = catalog.templates.find(
      (item) => item.id === 'pet-comic'
    ) as StudioTemplate
    const draft = templateDraft(
      template,
      { story: 'The cat makes tea', style: 'Watercolor' },
      {
        ...initialDraft,
        references: ['my-photo'],
        parent_id: 'previous-version',
      }
    )
    expect(draft.mode).toBe('edit')
    expect(draft.references).toEqual(['my-photo'])
    expect(draft.parent_id).toBe('previous-version')
    expect(draft.prompt).toContain('The cat makes tea')
    expect(draft.prompt).not.toContain('{{')
    expect(draft.count).toBe(1)
  })
  test('create templates clear edit lineage and use the native aspect-ratio size', () => {
    const template = catalog.templates.find(
      (item) => item.id === 'chibi-animal'
    ) as StudioTemplate
    const draft = templateDraft(
      template,
      {},
      { ...initialDraft, references: ['photo'], parent_id: 'old' }
    )
    expect(draft.references).toEqual([])
    expect(draft.parent_id).toBeUndefined()
    expect(draft.size).toBe('1328x1328')
  })
  test('four-image requests preserve explicit zero seed and CFG while oversized counts fail', () => {
    expect(
      draftSchema.parse({
        ...initialDraft,
        prompt: 'Cat',
        count: 4,
        seed: 0,
        cfg: 0,
      }).count
    ).toBe(4)
    expect(
      draftSchema.safeParse({ ...initialDraft, prompt: 'Cat', count: 5 })
        .success
    ).toBe(false)
    expect(
      draftSchema.safeParse({
        ...initialDraft,
        mode: 'edit',
        prompt: 'Cat',
        references: [],
      }).success
    ).toBe(false)
  })
  test('copyable commands show the public prepare and relay flow without embedding credentials', () => {
    const job = {
      id: 'job',
      request: { ...initialDraft, prompt: 'A cat' },
    } as WorkflowJob
    const command = publicCommand(job)
    expect(command).toContain('/api/v1/media-studio/jobs')
    expect(command).toContain('/pg/images/generations/studio')
    expect(command).toContain('$SessionToken')
    expect(command).not.toContain('studio_token')
    expect(command).not.toContain('165.154.')
  })
})
