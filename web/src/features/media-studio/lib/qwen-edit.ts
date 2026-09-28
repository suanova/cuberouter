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
import type { StudioAsset, WorkflowDraft } from '../workflow-types'

export const strokeColors = [
  { label: 'Blue', value: '#008cff' },
  { label: 'Red', value: '#ff3030' },
  { label: 'Green', value: '#16bd47' },
  { label: 'White', value: '#ffffff' },
] as const
export interface Stroke {
  color: string
  points: [number, number][]
}

export function drawStrokes(
  ctx: CanvasRenderingContext2D,
  strokes: Stroke[],
  width: number
): void {
  ctx.lineCap = ctx.lineJoin = 'round'
  ctx.lineWidth = Math.max(3, width / 180)
  for (const stroke of strokes) {
    if (!stroke.points.length) continue
    ctx.strokeStyle = stroke.color
    ctx.beginPath()
    ctx.moveTo(...stroke.points[0])
    for (const point of stroke.points.slice(1)) ctx.lineTo(...point)
    ctx.stroke() // Never close the path or fill a region.
  }
}

export function editDraft(
  draft: WorkflowDraft,
  asset: StudioAsset,
  parent: string
): WorkflowDraft {
  return {
    ...draft,
    mode: 'edit',
    count: 1,
    prompt: '',
    user_prompt: undefined,
    references: [asset],
    parent_id: parent,
    annotation: undefined,
  }
}

export function annotatedDraft(
  draft: WorkflowDraft,
  original: StudioAsset,
  marked: StudioAsset,
  strokes: Stroke[],
  instruction: string
): WorkflowDraft {
  return {
    ...draft,
    mode: 'edit',
    count: 1,
    references: [marked],
    user_prompt: instruction,
    prompt: `Edit this image. Colored strokes mark locations, not image content.\n${instruction.trim()}\nRemove all guide strokes after editing. Preserve the remaining content and composition. Output only the edited image.`,
    annotation: { original, strokes, instruction },
  }
}
