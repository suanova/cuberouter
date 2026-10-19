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
import type { TFunction } from 'i18next'
import { describe, expect, test } from 'vitest'

import { ENDPOINT_TYPES, getModelEndpointLabels } from '../constants'
import type { PricingModel } from '../types'

// 翻译键本身是契约,断言键名即可,避免绑定某一种语言的完整文案
const t = ((key: string) => `t:${key}`) as unknown as TFunction

function model(overrides: Partial<PricingModel>): PricingModel {
  return {
    id: 1,
    model_name: 'test-model',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    ...overrides,
  }
}

describe('getModelEndpointLabels', () => {
  test('maps each endpoint type to its display label in the declared order', () => {
    const labels = getModelEndpointLabels(
      model({
        supported_endpoint_types: [
          ENDPOINT_TYPES.ANTHROPIC,
          ENDPOINT_TYPES.OPENAI,
          ENDPOINT_TYPES.OPENAI_RESPONSE,
        ],
      }),
      t
    )

    expect(labels).toEqual(['Anthropic', 'Chat', 'Response'])
  })

  test('translates the endpoint types whose labels come from i18n', () => {
    const labels = getModelEndpointLabels(
      model({
        supported_endpoint_types: [
          ENDPOINT_TYPES.IMAGE_GENERATION,
          ENDPOINT_TYPES.EMBEDDINGS,
        ],
      }),
      t
    )

    expect(labels).toEqual(['t:Image', 't:Embeddings'])
  })

  test('collapses both raw video styles onto one video label', () => {
    const labels = getModelEndpointLabels(
      model({
        supported_endpoint_types: [
          ENDPOINT_TYPES.OPENAI_VIDEO,
          ENDPOINT_TYPES.ARK_VIDEO,
        ],
      }),
      t
    )

    expect(labels).toEqual(['t:Video'])
  })

  test('reads the task-plugin video endpoint as the video label', () => {
    const labels = getModelEndpointLabels(
      model({
        supported_endpoint_types: [ENDPOINT_TYPES.VIDEO_GENERATION],
      }),
      t
    )

    expect(labels).toEqual(['t:Video'])
  })

  test('reads a per-second video model as the video endpoint only', () => {
    const labels = getModelEndpointLabels(
      model({
        supported_endpoint_types: [ENDPOINT_TYPES.OPENAI],
        video_prices: {
          rows: [
            { resolution: '720p', normal_price: 0.1, off_peak_price: 0.07 },
          ],
        },
      }),
      t
    )

    expect(labels).toEqual(['t:Video'])
  })

  test('falls back to the raw value for an endpoint type it does not know', () => {
    const labels = getModelEndpointLabels(
      model({ supported_endpoint_types: ['brand-new-endpoint'] }),
      t
    )

    expect(labels).toEqual(['brand-new-endpoint'])
  })

  test('returns no labels when the model declares no endpoints', () => {
    expect(getModelEndpointLabels(model({}), t)).toEqual([])
  })
})
