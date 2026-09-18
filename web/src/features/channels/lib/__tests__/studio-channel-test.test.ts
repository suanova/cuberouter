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
import { toast } from 'sonner'
import { beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { handleTestChannel } from '../channel-actions'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn() } }))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('i18next', () => ({ default: { t: (key: string) => key } }))

beforeEach(() => vi.clearAllMocks())

test('studio readiness success explicitly says no image was generated', async () => {
  vi.mocked(api.get).mockResolvedValue({
    data: { success: true, time: 0.25, test_mode: 'studio-readiness' },
  })
  const complete = vi.fn()
  await handleTestChannel(1, { channelName: 'Qwen' }, complete)
  expect(toast.success).toHaveBeenCalledWith(
    '{{target}} connection check passed',
    expect.objectContaining({
      description:
        'Model service is ready. No image was generated; test generation and editing in Image Studio.',
    })
  )
  expect(complete).toHaveBeenCalledWith(
    true,
    250,
    undefined,
    undefined,
    'studio-readiness'
  )
})

test('ordinary inference tests keep their existing success message', async () => {
  vi.mocked(api.get).mockResolvedValue({ data: { success: true, time: 1 } })
  await handleTestChannel(3)
  expect(toast.success).toHaveBeenCalledWith(
    '{{target}} test succeeded',
    expect.any(Object)
  )
})

test('an unavailable studio remains a failed test', async () => {
  vi.mocked(api.get).mockResolvedValue({
    data: {
      success: false,
      test_mode: 'studio-readiness',
      message: 'Model service is unavailable',
    },
  })
  const complete = vi.fn()
  await handleTestChannel(2, undefined, complete)
  expect(toast.success).not.toHaveBeenCalled()
  expect(toast.error).toHaveBeenCalled()
  expect(complete).toHaveBeenCalledWith(
    false,
    undefined,
    'Model service is unavailable',
    undefined,
    'studio-readiness'
  )
})
