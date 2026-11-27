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
import type { TFunction } from 'i18next'
import { beforeEach, describe, expect, test } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

import type { ApiKey } from '../../types'
import {
  getApiKeyFormDefaultValues,
  getApiKeyFormSchema,
  transformApiKeyToFormDefaults,
  transformFormDataToPayload,
} from '../api-key-form'

const t = ((key: string) => key) as TFunction

const baseApiKey: ApiKey = {
  id: 1,
  name: 'test',
  key: 'sk-test',
  status: 1,
  remain_quota: 0,
  used_quota: 0,
  unlimited_quota: false,
  expired_time: -1,
  created_time: 1,
  accessed_time: 0,
  group: 'default',
  auto_groups: null,
  cross_group_retry: false,
  model_limits_enabled: false,
  model_limits: '',
  allow_ips: '',
}

function seedDisplayCurrency(type: CurrencyDisplayType, rate: number): void {
  useSystemConfigStore.setState((state) => ({
    config: {
      ...state.config,
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: type,
        usdExchangeRate: rate,
        customCurrencyExchangeRate: rate,
      },
    },
  }))
}

beforeEach(() => seedDisplayCurrency('USD', 1))

describe('personal API key quota form currency conversion', () => {
  test('defaults to 10 in the display currency and stores USD quota', () => {
    const defaults = getApiKeyFormDefaultValues(false)

    expect(defaults.remain_quota_dollars).toBe(10)
    expect(
      transformFormDataToPayload({ ...defaults, unlimited_quota: false })
        .remain_quota
    ).toBe(5_000_000)
  })

  test('converts a CNY amount to native quota once', () => {
    seedDisplayCurrency('CNY', 7.3)
    const defaults = getApiKeyFormDefaultValues(false)

    expect(
      transformFormDataToPayload({ ...defaults, unlimited_quota: false })
        .remain_quota
    ).toBe(684_932)
  })

  test('shows a stored CNY quota as an editable amount and round-trips it', () => {
    seedDisplayCurrency('CNY', 7.3)
    const formValues = transformApiKeyToFormDefaults({
      ...baseApiKey,
      remain_quota: 6_849_315,
    })

    expect(formValues.remain_quota_dollars).toBe(100)
    expect(
      transformFormDataToPayload({ ...formValues, unlimited_quota: false })
        .remain_quota
    ).toBe(6_849_315)
  })

  test('keeps raw points when the site displays tokens', () => {
    seedDisplayCurrency('TOKENS', 7.3)
    const defaults = getApiKeyFormDefaultValues(false)

    expect(
      transformFormDataToPayload({ ...defaults, unlimited_quota: false })
        .remain_quota
    ).toBe(10)
  })

  test('sends zero when the key is unlimited', () => {
    const defaults = getApiKeyFormDefaultValues(false)

    expect(
      transformFormDataToPayload({ ...defaults, unlimited_quota: true })
        .remain_quota
    ).toBe(0)
  })

  test('rejects a negative amount but accepts decimals', () => {
    const schema = getApiKeyFormSchema(t)
    const base = {
      ...getApiKeyFormDefaultValues(false),
      name: 'test',
      unlimited_quota: false,
    }

    expect(
      schema.safeParse({ ...base, remain_quota_dollars: -1 }).success
    ).toBe(false)
    expect(
      schema.safeParse({ ...base, remain_quota_dollars: 99.99 }).success
    ).toBe(true)
  })
})
