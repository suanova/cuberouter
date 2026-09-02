/*
Copyright (C) 2023-2026 QuantumNous

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
import { nanoid } from 'nanoid'

import type { VideoPriceTable } from '@/features/pricing/types'

/** Editable draft of one resolution row (numbers kept as input strings). */
export type VideoPriceRowDraft = {
  id: string
  resolution: string
  normalPrice: string
  offPeakPrice: string
}

export function createVideoPriceRowDraft(): VideoPriceRowDraft {
  return { id: nanoid(), resolution: '', normalPrice: '', offPeakPrice: '' }
}

export function videoPriceDraftsFromTable(
  table: VideoPriceTable
): VideoPriceRowDraft[] {
  return table.rows.map((row) => ({
    id: nanoid(),
    resolution: row.resolution,
    normalPrice: String(row.normal_price),
    offPeakPrice: String(row.off_peak_price),
  }))
}

function parsePriceDraft(value: string): number {
  const trimmed = value.trim()
  if (trimmed === '') return 0
  const parsed = Number(trimmed)
  return Number.isFinite(parsed) ? parsed : 0
}

/**
 * Emits the table payload for a set of drafts. Fully empty rows are dropped;
 * partially filled rows are kept as-is so backend validation rejects them.
 */
export function videoPriceTableFromDrafts(
  drafts: VideoPriceRowDraft[]
): VideoPriceTable {
  return {
    rows: drafts
      .filter(
        (draft) =>
          draft.resolution.trim() !== '' ||
          draft.normalPrice.trim() !== '' ||
          draft.offPeakPrice.trim() !== ''
      )
      .map((draft) => ({
        resolution: draft.resolution.trim(),
        normal_price: parsePriceDraft(draft.normalPrice),
        off_peak_price: parsePriceDraft(draft.offPeakPrice),
      })),
  }
}

export function addVideoPriceRowDraft(
  drafts: VideoPriceRowDraft[]
): VideoPriceRowDraft[] {
  return [...drafts, createVideoPriceRowDraft()]
}

export function removeVideoPriceRowDraft(
  drafts: VideoPriceRowDraft[],
  index: number
): VideoPriceRowDraft[] {
  return drafts.filter((_, i) => i !== index)
}

export function updateVideoPriceRowDraft(
  drafts: VideoPriceRowDraft[],
  index: number,
  patch: Partial<VideoPriceRowDraft>
): VideoPriceRowDraft[] {
  return drafts.map((draft, i) => (i === index ? { ...draft, ...patch } : draft))
}
