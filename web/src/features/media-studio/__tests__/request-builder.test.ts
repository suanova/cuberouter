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

import { ASPECT_RATIO_ORDER, DEFAULT_PARAMS } from '../constants'
import { buildGenerationRequest } from '../lib/request-builder'
import type { StudioParams } from '../types'

describe('buildGenerationRequest', () => {
  test('maps every studio parameter onto the agreed request contract', () => {
    const params: StudioParams = {
      prompt: '  night cafe  ',
      ratio: '3:2',
      count: 4,
      steps: 30,
      seed: 1234,
      cfg: 2.5,
    }

    expect(buildGenerationRequest(params)).toEqual({
      model: 'qwen-image-2512',
      prompt: 'night cafe',
      n: 4,
      size: '3:2',
      seed: 1234,
      num_inference_steps: 30,
      true_cfg_scale: 2.5,
    })
  })

  test('trims surrounding whitespace from the prompt', () => {
    const body = buildGenerationRequest({
      ...DEFAULT_PARAMS,
      prompt: '   a red fox   ',
    })

    expect(body.prompt).toBe('a red fox')
  })

  test('sends the ratio string as size for every supported ratio', () => {
    for (const ratio of ASPECT_RATIO_ORDER) {
      const body = buildGenerationRequest({
        ...DEFAULT_PARAMS,
        prompt: 'p',
        ratio,
      })

      expect(body.size).toBe(ratio)
    }
  })

  test('keeps the fixed model name from the contract', () => {
    const body = buildGenerationRequest({ ...DEFAULT_PARAMS, prompt: 'p' })

    expect(body.model).toBe('qwen-image-2512')
  })
})
