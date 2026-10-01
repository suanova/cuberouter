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
import { isAxiosError } from 'axios'
import { z } from 'zod'

import {
  VIDEO_DEFAULT_DURATION,
  VIDEO_DEFAULT_RESOLUTION,
} from '../constants'
import type { VideoDraft } from '../video-types'

export const initialVideoDraft: VideoDraft = {
  mode: 'text',
  model: '',
  prompt: '',
  duration: VIDEO_DEFAULT_DURATION,
  resolution: VIDEO_DEFAULT_RESOLUTION,
}

export const videoDraftSchema = z
  .object({
    mode: z.enum(['text', 'image']),
    model: z.string().min(1),
    prompt: z.string().trim().min(1).max(16000),
    duration: z.number().int().min(1).max(3600),
    resolution: z.string().regex(/^[1-9]\d{2,3}p$/),
    image: z
      .object({ id: z.string(), url: z.string(), mime: z.string() })
      .optional(),
  })
  .superRefine((value, ctx) => {
    if (value.mode === 'image' && !value.image) {
      ctx.addIssue({
        code: 'custom',
        path: ['image'],
        message: 'Choose a reference image.',
      })
    }
  })

/**
 * 视频错误文案：任务端点回传 dto.TaskError 形状 { code, message }，
 * 会话前置校验回传 OpenAI 形状 { error: { message } }，两种都要取。
 */
export function videoError(error: unknown): string {
  if (isAxiosError(error)) {
    const direct = error.response?.data?.message
    if (typeof direct === 'string' && direct) return direct
    const message = error.response?.data?.error?.message
    if (typeof message === 'string') return message
  }
  return error instanceof Error
    ? error.message
    : 'Video operation could not be completed.'
}
