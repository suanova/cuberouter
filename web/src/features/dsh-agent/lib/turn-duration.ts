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
import type { DshConversation } from '../types'

// 没有历史样本时给一个中位数附近的经验值：一次带工具调用的 dsh 轮次通常要跑
// 几分钟（生成脚本 → 执行 → 自检），5 分钟比「预计 30 秒」诚实得多。
const DEFAULT_TURN_DURATION_MS = 5 * 60 * 1000
// 样本里混进异常值（超时、秒回）时不至于把预期带偏。
const MIN_TURN_DURATION_MS = 30 * 1000
const MAX_TURN_DURATION_MS = 30 * 60 * 1000

export type TurnDurationEstimate = {
  milliseconds: number
  fromHistory: boolean
}

/**
 * 用本机已完成轮次的中位数估计一轮要跑多久，运行中的计时上方展示给用户一个预期。
 * 只统计真正跑出结果的轮次：失败、超时、用户手动停止的轮次耗时不能代表正常一轮,
 * 把它们算进去会让卡死过一次之后的预期一路上涨。
 */
export function estimateTurnDuration(
  conversations: DshConversation[]
): TurnDurationEstimate {
  const durations: number[] = []
  for (const conversation of conversations) {
    for (const turn of conversation.turns) {
      if (turn.status !== 'completed' && turn.status !== 'max-tokens') {
        continue
      }
      if (!turn.finishedAt) {
        continue
      }
      const duration = turn.finishedAt - turn.createdAt
      if (duration > 0) {
        durations.push(duration)
      }
    }
  }
  if (durations.length === 0) {
    return { milliseconds: DEFAULT_TURN_DURATION_MS, fromHistory: false }
  }

  durations.sort((a, b) => a - b)
  const median = durations[Math.floor(durations.length / 2)]
  return {
    milliseconds: Math.min(
      MAX_TURN_DURATION_MS,
      Math.max(MIN_TURN_DURATION_MS, median)
    ),
    fromHistory: true,
  }
}

/**
 * 展示用分钟数。预期本质上是大约值，四舍五入到整分钟；最小值 1，避免「约需 0 分钟」。
 */
export function approximateMinutes(milliseconds: number): number {
  return Math.max(1, Math.round(milliseconds / 60_000))
}
