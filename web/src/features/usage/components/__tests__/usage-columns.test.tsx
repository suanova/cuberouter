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

import type { SelfUsageRow } from '../../types'
import { usageFailureText, usageSuccessText } from '../usage-columns'

const textRow: SelfUsageRow = {
  date: '2026-11-03',
  type: 'text',
  billing_account_type: 'personal',
  organization_id: 0,
  organization_name: '',
  token_id: 1,
  token_name: 'k1',
  model_name: 'gpt-4o',
  request_count: 2,
  success_count: 2,
  failure_count: 0,
  prompt_tokens: 100,
  completion_tokens: 40,
  cache_tokens: 30,
  quota: 10,
}

describe('usage table cells', () => {
  test('text rows report requests as success and leave failures blank', () => {
    expect(usageSuccessText(textRow)).toBe('2')
    expect(usageFailureText(textRow)).toBe('-')
  })

  test('task rows use status counts', () => {
    const taskRow = {
      ...textRow,
      type: 'video' as const,
      request_count: 2,
      success_count: 1,
      failure_count: 1,
    }
    expect(usageSuccessText(taskRow)).toBe('1')
    expect(usageFailureText(taskRow)).toBe('1')
  })
})
