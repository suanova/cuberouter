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

import { CHANNEL_TYPE_CUBE_STACK } from '../../constants'
import { CHANNEL_TYPE_ADVANCED_CUSTOM } from '../advanced-custom'
import { resolveModelFetchBlock } from '../channel-model-fetch'

// OpenAI 是上游鉴权渠道的代表：constants 里没有为它单独命名常量。
const OPENAI = 1

describe('resolveModelFetchBlock', () => {
  test('allows opening upstream discovery with a blank key for CubeStack', () => {
    // 新增 CubeStack 渠道时 key 可以为空：SGLang 本地部署通常没有鉴权，
    // 而 "Fetch from Upstream" 正是用来发现它实际加载了哪些模型的入口。
    for (const apiKey of [undefined, '', '   ']) {
      expect(
        resolveModelFetchBlock({
          type: CHANNEL_TYPE_CUBE_STACK,
          isEditing: false,
          canEditSensitive: true,
          apiKey,
        })
      ).toBeNull()
    }
  })

  test('still requires a key for channel types whose upstream authenticates', () => {
    // 空 key 例外只给无鉴权发现端点：其余渠道仍必须先填 key，
    // 否则会把无凭据请求打到需要鉴权的上游。
    expect(
      resolveModelFetchBlock({
        type: OPENAI,
        isEditing: false,
        canEditSensitive: true,
        apiKey: '',
      })
    ).toBe('missing_key')
    // 填上 key 后照常放行。
    expect(
      resolveModelFetchBlock({
        type: OPENAI,
        isEditing: false,
        canEditSensitive: true,
        apiKey: 'sk-test',
      })
    ).toBeNull()
  })

  test('keeps the Advanced Custom exemption and skips the key check while editing', () => {
    expect(
      resolveModelFetchBlock({
        type: CHANNEL_TYPE_ADVANCED_CUSTOM,
        isEditing: false,
        canEditSensitive: true,
        apiKey: '',
      })
    ).toBeNull()
    // 编辑态 key 不回填表单，前端无从校验，因此不拦。
    expect(
      resolveModelFetchBlock({
        type: OPENAI,
        isEditing: true,
        canEditSensitive: true,
        apiKey: '',
      })
    ).toBeNull()
  })

  test('rejects types that cannot discover models and callers without permission', () => {
    // 不在 MODEL_FETCHABLE_TYPES 里的类型即使有 key 也不放行。
    expect(
      resolveModelFetchBlock({
        type: 99999,
        isEditing: false,
        canEditSensitive: true,
        apiKey: 'sk-test',
      })
    ).toBe('unsupported_type')
    expect(
      resolveModelFetchBlock({
        type: CHANNEL_TYPE_CUBE_STACK,
        isEditing: false,
        canEditSensitive: false,
        apiKey: '',
      })
    ).toBe('missing_permission')
  })
})
