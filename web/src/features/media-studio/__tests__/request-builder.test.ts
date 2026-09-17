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
import { describe, expect, test } from 'vitest'

import { ASPECT_RATIOS, ASPECT_RATIO_ORDER, DEFAULT_PARAMS } from '../constants'
import { buildGenerationRequest } from '../lib/request-builder'
import type { Quality, StudioParams } from '../types'

const TEST_MODEL = 'qwen-image-2512'

describe('buildGenerationRequest', () => {
  test('maps every studio parameter onto the agreed request contract', () => {
    const params: StudioParams = {
      prompt: '  night cafe  ',
      ratio: '3:2',
      count: 4,
      quality: 'standard',
      seed: 1234,
      cfg: 2.5,
    }

    expect(buildGenerationRequest(params, TEST_MODEL)).toEqual({
      model: TEST_MODEL,
      prompt: 'night cafe',
      n: 4,
      size: '1584x1056',
      seed: 1234,
      num_inference_steps: 30,
      true_cfg_scale: 2.5,
      quality: 'standard',
    })
  })

  test('trims surrounding whitespace from the prompt', () => {
    const body = buildGenerationRequest(
      {
        ...DEFAULT_PARAMS,
        prompt: '   a red fox   ',
      },
      TEST_MODEL,
    )

    expect(body.prompt).toBe('a red fox')
  })

  test('sends the native pixel size for every supported ratio', () => {
    for (const ratio of ASPECT_RATIO_ORDER) {
      const [width, height] = ASPECT_RATIOS[ratio]
      const body = buildGenerationRequest(
        {
          ...DEFAULT_PARAMS,
          prompt: 'p',
          ratio,
        },
        TEST_MODEL,
      )

      expect(body.size).toBe(`${width}x${height}`)
    }
  })

  test('uses the model selected on the page', () => {
    const body = buildGenerationRequest(
      { ...DEFAULT_PARAMS, prompt: 'p' },
      'other-image-model',
    )

    expect(body.model).toBe('other-image-model')
  })

  test('maps each quality tier to its step count', () => {
    const cases: Array<{ quality: Quality; steps: number }> = [
      { quality: 'fast', steps: 20 },
      { quality: 'standard', steps: 30 },
      { quality: 'high', steps: 50 },
    ]
    for (const { quality, steps } of cases) {
      const body = buildGenerationRequest(
        { ...DEFAULT_PARAMS, prompt: 'p', quality },
        TEST_MODEL,
      )

      expect(body.num_inference_steps).toBe(steps)
      expect(body.quality).toBe(quality)
    }
  })

  test('sends true_cfg_scale 4 by default', () => {
    const body = buildGenerationRequest(
      { ...DEFAULT_PARAMS, prompt: 'p' },
      TEST_MODEL,
    )

    expect(body.true_cfg_scale).toBe(4)
  })
})
