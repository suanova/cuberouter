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
import { expect, test, vi } from 'vitest'

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
} from '../channel-form'

test('moderation CA survives creating and reopening a channel without replacing unrelated settings', () => {
  const certificate =
    '-----BEGIN CERTIFICATE-----\npublic-ca\n-----END CERTIFICATE-----'
  const payload = transformFormDataToCreatePayload({
    ...CHANNEL_FORM_DEFAULT_VALUES,
    type: 65,
    name: 'Gemma GPU',
    base_url: 'https://review.example/v1/moderate',
    key: 'service-key',
    models: 'google/shieldgemma-2-4b-it',
    group: ['image-moderation-internal'],
    image_moderation_ca_pem: certificate,
    settings: '{"unrelated":true}',
  })
  // The create wrapper carries channel properties separately from batch mode.
  const channel = payload.channel
  expect(JSON.parse(channel.settings ?? '{}')).toMatchObject({
    image_moderation_ca_pem: certificate,
    unrelated: true,
  })
  const defaults = transformChannelToFormDefaults({
    ...channel,
    id: 1,
    channel_info: {},
  } as Parameters<typeof transformChannelToFormDefaults>[0])
  expect(defaults.image_moderation_ca_pem).toBe(certificate)
})
test('a malformed existing settings field does not crash the channel editor', () => {
  const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
  try {
    expect(
      transformChannelToFormDefaults({
        type: 65,
        settings: '{broken',
        channel_info: {},
      } as Parameters<typeof transformChannelToFormDefaults>[0])
        .image_moderation_ca_pem
    ).toBe('')
  } finally {
    spy.mockRestore()
  }
})
