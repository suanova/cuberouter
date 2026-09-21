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
import { createInstance } from 'i18next'
import { expect, test } from 'vitest'

import english from '@/i18n/locales/en.json'
import traditional from '@/i18n/locales/zh-TW.json'
import simplified from '@/i18n/locales/zh.json'

test('switching to Traditional Chinese translates workflow and template labels from the real resource bundle', async () => {
  const i18n = createInstance()
  await i18n.init({
    lng: 'zh-TW',
    fallbackLng: 'en',
    keySeparator: false,
    resources: { en: english, 'zh-TW': traditional },
  })
  expect(i18n.t('Template gallery')).toBe('模板圖庫')
  expect(i18n.t('Chibi animal sticker')).not.toBe('Chibi animal sticker')
  expect(i18n.t('Before and after comparison')).not.toBe(
    'Before and after comparison'
  )
  expect(i18n.t('Create, refine and keep every version.')).not.toBe(
    'Create, refine and keep every version.'
  )
})

test('Simplified Chinese workflow uses simplified wording', async () => {
  const i18n = createInstance()
  await i18n.init({
    lng: 'zh',
    keySeparator: false,
    resources: { zh: simplified },
  })
  expect(i18n.t('Template gallery')).toBe('模板图库')
  expect(i18n.t('Before and after comparison')).toBe('修改前后对比')
})
