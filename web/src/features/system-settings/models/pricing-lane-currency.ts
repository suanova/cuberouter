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
import {
  getBillingCurrency,
  localToUsdNumber,
  usdToLocalNumber,
} from '@/lib/currency'

import { formatPricingNumber } from './pricing-format'

/**
 * 分时(TieredPricingEditor)与按量(TaskUsagePricingEditor/矩阵)编辑器价格 lane 的
 * 数字换算边界(model-pricing-core 的字符串换算边界的对偶,供 type=number 输入框):
 * - 结构化状态里的 lane 数值永远是美元(USD/声明单位系数),表达式生成直接使用;
 * - 输入框展示 = ×rate + formatPricingNumber 归整:不归整时浮点尾噪声(如
 *   0.3×7 = 2.1000000000000005)会被写回受控 number input,与用户输入打架;
 *   归整后 display(parse(录入值)) === 录入值,录入手感与汇率无关;
 * - 录入解析 = ÷rate 原样保留双精度:任何位数截断都会让 ×rate 回显偏离超过展示归整
 *   的 1e-12 容差(如 18.3÷7.3 截到 12 位后回显 18.299999999996),因此不做截断;
 * - rate = 1(USD/TOKENS 回落)时两个方向严格恒等、不做任何格式化,保持旧行为
 *   (输入框能原样显示录入的长小数,不因展示格式化截断)。
 */
export function laneUsdToLocalNumber(usdNumber: number): number {
  if (getBillingCurrency().exchangeRate === 1) return usdNumber
  const local = formatPricingNumber(usdToLocalNumber(usdNumber))
  return local === '' ? 0 : Number(local)
}

export function laneLocalToUsdNumber(localNumber: number): number {
  return getBillingCurrency().exchangeRate === 1
    ? localNumber
    : localToUsdNumber(localNumber)
}
