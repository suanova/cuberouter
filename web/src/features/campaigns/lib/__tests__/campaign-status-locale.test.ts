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
import { expect, describe, it } from 'vitest'

import { createInstance } from 'i18next'

import zhTW from '../../../../i18n/locales/zh-TW.json'
import zh from '../../../../i18n/locales/zh.json'

// The campaign row actions toast renders t('Campaign {{action}}', { action:
// t(labelKey) }) with labelKey one of Activated / Paused / Ended. The action
// translations already carry 已 (已启用/已暂停/已结束), so the template must not
// add a second one. Pins the exact toast outputs for the quick-status actions.
async function campaignStatusToast(
  translations: Record<string, string>,
  lang: string,
  labelKey: string
): Promise<string> {
  const instance = createInstance()
  await instance.init({
    lng: lang,
    fallbackLng: false,
    resources: { [lang]: { translation: translations } },
  })
  return instance.t('Campaign {{action}}', { action: instance.t(labelKey) })
}

describe('campaign status toast', () => {
  const actions = [
    ['Activated', 'Activate'] as const,
    ['Paused', 'Pause'] as const,
    ['Ended', 'End'] as const,
  ]

  it('renders exactly one 已 in zh for every quick-status action', async () => {
    const expected = new Map<string, string>([
      ['Activated', '活动已启用'],
      ['Paused', '活动已暂停'],
      ['Ended', '活动已结束'],
    ])
    for (const [labelKey] of actions) {
      const toast = await campaignStatusToast(zh.translation, 'zh', labelKey)
      expect(toast).toBe(expected.get(labelKey))
      expect(toast.split('已').length - 1).toBe(1)
    }
  })

  it('renders exactly one 已 in zh-TW for every quick-status action', async () => {
    const expected = new Map<string, string>([
      ['Activated', '活動已啟用'],
      ['Paused', '活動已暫停'],
      ['Ended', '活動已結束'],
    ])
    for (const [labelKey] of actions) {
      const toast = await campaignStatusToast(
        zhTW.translation,
        'zh-TW',
        labelKey
      )
      expect(toast).toBe(expected.get(labelKey))
      expect(toast.split('已').length - 1).toBe(1)
    }
  })
})
