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
import { withBasePath } from '@/lib/base-path'

import { newId } from '../workflow-api'
import type { StudioAsset, WorkflowDraft } from '../workflow-types'
import { decodeImage } from './reference-image'

interface SampleInput {
  file: string
  crop?: number[]
}
export interface OfficialSample {
  id: string
  title: string
  preview: string
  size: string
  prompt: string
  inputs: SampleInput[]
}
export const officialSampleURL = (file: string) =>
  withBasePath(`/studio-examples/qwen21/${file}`)

export async function loadOfficialSample(
  sample: OfficialSample
): Promise<Partial<WorkflowDraft>> {
  const references: StudioAsset[] = []
  for (const input of sample.inputs) {
    const source = await decodeImage(officialSampleURL(input.file))
    if (!source) throw new Error('Official example inputs could not be loaded.')
    const [x, y, width, height] = input.crop ?? [
      0,
      0,
      source.naturalWidth,
      source.naturalHeight,
    ]
    if (
      x < 0 ||
      y < 0 ||
      width <= 0 ||
      height <= 0 ||
      x + width > source.naturalWidth ||
      y + height > source.naturalHeight
    ) {
      throw new Error('Official example inputs could not be loaded.')
    }
    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const ctx = canvas.getContext('2d')
    if (!ctx) throw new Error('Official example inputs could not be loaded.')
    ctx.drawImage(source, x, y, width, height, 0, 0, width, height)
    references.push({
      id: newId(),
      mime: 'image/png',
      url: canvas.toDataURL('image/png'),
    })
  }
  return {
    mode: references.length ? 'edit' : 'create',
    prompt: sample.prompt,
    user_prompt: sample.prompt,
    size: sample.size,
    references,
    count: 1,
    steps: 40,
    cfg: 1,
    seed: Math.floor(crypto.getRandomValues(new Uint32Array(1))[0]),
    mask: undefined,
    markup: undefined,
    parent_id: undefined,
  }
}
