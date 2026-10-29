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

import { CHANNEL_TYPE_IMAGE_EDIT } from '../../constants'
import { channelSchema, type Channel } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  buildSettingJSON,
  transformChannelToFormDefaults,
  type ChannelFormValues,
} from '../channel-form'

// The schema fills every defaulted field, so the fixtures stay minimal and are
// still typed exactly like the API payloads the form receives.
function makeChannel(overrides: Partial<Channel>): Channel {
  return channelSchema.parse({
    id: 1,
    type: CHANNEL_TYPE_IMAGE_EDIT,
    key: 'sk-test',
    status: 1,
    name: 'sglang-image-edit',
    created_time: 0,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
    ...overrides,
  })
}

function makeForm(overrides: Partial<ChannelFormValues>): ChannelFormValues {
  return { ...CHANNEL_FORM_DEFAULT_VALUES, ...overrides }
}

describe('buildSettingJSON image_edit_upstream_format', () => {
  test('writes multipart for an Image Edit (Base64) channel', () => {
    const json = buildSettingJSON(
      makeForm({
        type: CHANNEL_TYPE_IMAGE_EDIT,
        image_edit_upstream_format: 'multipart',
      })
    )

    expect(JSON.parse(json).image_edit_upstream_format).toBe('multipart')
  })

  test('omits the key when the channel keeps the JSON default', () => {
    const json = buildSettingJSON(
      makeForm({
        type: CHANNEL_TYPE_IMAGE_EDIT,
        image_edit_upstream_format: 'json',
      })
    )

    expect(JSON.parse(json)).not.toHaveProperty('image_edit_upstream_format')
  })

  test('omits the key for channels that are not Image Edit (Base64)', () => {
    const json = buildSettingJSON(
      makeForm({ type: 1, image_edit_upstream_format: 'multipart' })
    )

    expect(JSON.parse(json)).not.toHaveProperty('image_edit_upstream_format')
  })
})

describe('transformChannelToFormDefaults image_edit_upstream_format', () => {
  test('restores multipart from a saved channel setting', () => {
    const form = transformChannelToFormDefaults(
      makeChannel({
        setting: JSON.stringify({ image_edit_upstream_format: 'multipart' }),
      })
    )

    expect(form.image_edit_upstream_format).toBe('multipart')
  })

  test('falls back to json for an empty or unrecognised setting', () => {
    const empty = transformChannelToFormDefaults(makeChannel({ setting: null }))
    const unknown = transformChannelToFormDefaults(
      makeChannel({ setting: JSON.stringify({ image_edit_upstream_format: 'x' }) })
    )

    expect(empty.image_edit_upstream_format).toBe('json')
    expect(unknown.image_edit_upstream_format).toBe('json')
  })
})
