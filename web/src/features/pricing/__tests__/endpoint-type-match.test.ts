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
import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import { ENDPOINT_TYPES } from '../constants'
import { matchesEndpointType } from '../lib/filters'
import type { PricingModel } from '../types'

function modelWithEndpoints(endpoints?: string[]): PricingModel {
  return {
    id: 1,
    model_name: 'test-model',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    supported_endpoint_types: endpoints,
  }
}

describe('endpoint type filter matching', () => {
  test('the merged Video value matches both video styles', () => {
    assert.equal(
      matchesEndpointType(
        modelWithEndpoints([ENDPOINT_TYPES.OPENAI_VIDEO]),
        ENDPOINT_TYPES.VIDEO
      ),
      true
    )
    assert.equal(
      matchesEndpointType(
        modelWithEndpoints([ENDPOINT_TYPES.ARK_VIDEO]),
        ENDPOINT_TYPES.VIDEO
      ),
      true
    )
  })

  test('concrete values keep matching only themselves', () => {
    assert.equal(
      matchesEndpointType(
        modelWithEndpoints([ENDPOINT_TYPES.OPENAI_VIDEO]),
        ENDPOINT_TYPES.ARK_VIDEO
      ),
      false
    )
    assert.equal(
      matchesEndpointType(
        modelWithEndpoints([ENDPOINT_TYPES.OPENAI]),
        ENDPOINT_TYPES.VIDEO
      ),
      false
    )
    assert.equal(
      matchesEndpointType(
        modelWithEndpoints([ENDPOINT_TYPES.ARK_VIDEO]),
        ENDPOINT_TYPES.ARK_VIDEO
      ),
      true
    )
  })

  test('models without endpoint types never match', () => {
    assert.equal(
      matchesEndpointType(modelWithEndpoints(), ENDPOINT_TYPES.VIDEO),
      false
    )
  })

  test('per-second video models match Video without a raw video endpoint type', () => {
    // Task-platform video models (e.g. MiniMax, Vidu) carry no openai-video /
    // ark-video endpoint type — their channels fall back to plain openai — but
    // the model card already labels them 视频 from video_prices, so the filter
    // must match them too.
    const taskVideoModel: PricingModel = {
      ...modelWithEndpoints([ENDPOINT_TYPES.OPENAI]),
      video_prices: {
        rows: [
          { resolution: '1080p', normal_price: 0.1, off_peak_price: 0.05 },
        ],
      },
    }
    assert.equal(
      matchesEndpointType(taskVideoModel, ENDPOINT_TYPES.VIDEO),
      true
    )
    // ...but a video-priced model must not leak into other endpoint filters.
    assert.equal(
      matchesEndpointType(taskVideoModel, ENDPOINT_TYPES.OPENAI),
      true
    )
    assert.equal(
      matchesEndpointType(taskVideoModel, ENDPOINT_TYPES.IMAGE_GENERATION),
      false
    )
  })
})
