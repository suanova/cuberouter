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
import * as z from 'zod'

import { combineBillingExpr } from '@/features/pricing/lib/billing-expr'
import type { VideoPriceTable } from '@/features/pricing/types'
import {
  getBillingCurrency,
  localToUsdNumber,
  usdToLocalNumber,
} from '@/lib/currency'

import { formatPricingNumber } from './pricing-format'

/**
 * 编辑器的货币换算边界:
 * - 加载/联动显示:内部美元价 → 显示货币字符串;
 * - 解析/提交:显示货币数值 → 内部美元数。
 * 换算经 formatPricingNumber/snapFloatDrift 归整:≥1e-12 量级的现实定价往返保真;
 * 更小量级会归零(现实定价不可达),故显示货币与 USD 相同(或汇率非法回落 1)时
 * 仅跳过 ×rate,不保证字符串级恒等(尾零会被修剪、极小量级归 "0")。
 */
export function usdPriceToDisplay(usdNumber: number): string {
  return formatPricingNumber(usdToLocalNumber(usdNumber))
}

export function displayPriceToUsd(localNumber: number): number {
  return localToUsdNumber(localNumber)
}

export const createModelPricingSchema = (t: (key: string) => string) =>
  z.object({
    name: z.string().min(1, t('Model name is required')),
    price: z.string().optional(),
    ratio: z.string().optional(),
    cacheRatio: z.string().optional(),
    createCacheRatio: z.string().optional(),
    completionRatio: z.string().optional(),
    imageRatio: z.string().optional(),
    audioRatio: z.string().optional(),
    audioCompletionRatio: z.string().optional(),
  })

export type ModelPricingFormValues = z.infer<
  ReturnType<typeof createModelPricingSchema>
>

export type PricingMode =
  | 'per-token'
  | 'per-request'
  | 'tiered_expr'
  | 'video-per-second'

export type LaneKey =
  | 'completion'
  | 'cache'
  | 'createCache'
  | 'image'
  | 'audioInput'
  | 'audioOutput'

export type ModelRatioData = {
  name: string
  price?: string
  ratio?: string
  cacheRatio?: string
  createCacheRatio?: string
  completionRatio?: string
  imageRatio?: string
  audioRatio?: string
  audioCompletionRatio?: string
  billingMode?: PricingMode
  billingExpr?: string
  requestRuleExpr?: string
  /** Per-second video price table, present when the model is billed per second */
  videoPrices?: VideoPriceTable
}

/**
 * Initial billing mode for the editor: a model with a video price table (or an
 * explicit video billing mode) opens in the video-per-second tab.
 */
export function getInitialPricingMode(
  editData?: ModelRatioData | null
): PricingMode {
  if (!editData) return 'per-token'
  if (editData.billingMode === 'tiered_expr') return 'tiered_expr'
  if (
    editData.billingMode === 'video-per-second' ||
    (editData.videoPrices && editData.videoPrices.rows.length > 0)
  ) {
    return 'video-per-second'
  }
  return editData.price ? 'per-request' : 'per-token'
}

export type PreviewRow = {
  key: string
  label: string
  value: string
  multiline?: boolean
}

export const numericDraftRegex = /^(\d+(\.\d*)?|\.\d*)?$/

export const EMPTY_LANE_PRICES: Record<LaneKey, string> = {
  completion: '',
  cache: '',
  createCache: '',
  image: '',
  audioInput: '',
  audioOutput: '',
}

export const EMPTY_LANE_ENABLED: Record<LaneKey, boolean> = {
  completion: false,
  cache: false,
  createCache: false,
  image: false,
  audioInput: false,
  audioOutput: false,
}

export const ratioFieldByLane: Record<LaneKey, keyof ModelPricingFormValues> = {
  completion: 'completionRatio',
  cache: 'cacheRatio',
  createCache: 'createCacheRatio',
  image: 'imageRatio',
  audioInput: 'audioRatio',
  audioOutput: 'audioCompletionRatio',
}

export const laneConfigs: Array<{
  key: LaneKey
  titleKey: string
  descriptionKey: string
  placeholder: string
}> = [
  {
    key: 'completion',
    titleKey: 'Completion price',
    descriptionKey: 'Output token price for generated tokens.',
    placeholder: '15',
  },
  {
    key: 'cache',
    titleKey: 'Cache read price',
    descriptionKey: 'Token price for cache reads.',
    placeholder: '0.3',
  },
  {
    key: 'createCache',
    titleKey: 'Cache write price',
    descriptionKey: 'Token price for creating cache entries.',
    placeholder: '3.75',
  },
  {
    key: 'image',
    titleKey: 'Image input price',
    descriptionKey: 'Token price for image input.',
    placeholder: '2.5',
  },
  {
    key: 'audioInput',
    titleKey: 'Audio input price',
    descriptionKey: 'Token price for audio input.',
    placeholder: '3.81',
  },
  {
    key: 'audioOutput',
    titleKey: 'Audio output price',
    descriptionKey: 'Token price for audio output.',
    placeholder: '15.11',
  },
]

export function hasValue(value: unknown): boolean {
  return (
    value !== '' && value !== null && value !== undefined && value !== false
  )
}

export function toNumberOrNull(value: unknown): number | null {
  if (!hasValue(value) && value !== 0) return null
  const num = Number(value)
  return Number.isFinite(num) ? num : null
}

function ratioToBasePrice(ratio: unknown): string {
  const num = toNumberOrNull(ratio)
  if (num === null) return ''
  // ratio 是倍率,主输入价(美元/1M)= ratio × 2(2 为美元计价基准),出口换算为显示货币
  return usdPriceToDisplay(num * 2)
}

/**
 * ratioToBasePrice 的编辑期逆推导(同一 $2/1M 美元基准):
 * 显示货币主价 → USD → ÷2 → 落库基准倍率 ratio。汇率在 ÷rate 处消除,任何显示
 * 货币下推导结果都与 USD 模式直接输入美元主价一致(倍率,汇率无关)。不可解析
 * 输入返回空串,与表单 ratio 空值语义一致。
 */
export function basePriceToRatio(displayPrice: string): string {
  const num = toNumberOrNull(displayPrice)
  if (num === null) return ''
  return formatPricingNumber(displayPriceToUsd(num) / 2)
}

/**
 * 派生 lane 价 = lane 倍率 × 主价。倍率无量纲,denominator 为显示货币价格串时
 * 结果即显示货币价(汇率在两个同货币量中已约去),无需在此换算。
 */
function deriveLanePrice(
  ratio: unknown,
  denominator: unknown,
  fallback = ''
): string {
  const ratioNumber = toNumberOrNull(ratio)
  const denominatorNumber = toNumberOrNull(denominator)
  if (ratioNumber === null || denominatorNumber === null) return fallback
  return formatPricingNumber(ratioNumber * denominatorNumber)
}

/**
 * 由库中倍率(ratio 家族,美元无关)还原 per-token 主价与各 lane 价。
 * 返回的价格串为显示货币(经 usdPriceToDisplay/汇率约去推导)。
 */
export function createInitialLaneState(data?: ModelRatioData | null) {
  if (!data) {
    return {
      promptPrice: '',
      prices: { ...EMPTY_LANE_PRICES },
      enabled: { ...EMPTY_LANE_ENABLED },
    }
  }

  const promptPrice = ratioToBasePrice(data.ratio)
  const audioInputPrice = deriveLanePrice(data.audioRatio, promptPrice)
  const prices: Record<LaneKey, string> = {
    completion: deriveLanePrice(data.completionRatio, promptPrice),
    cache: deriveLanePrice(data.cacheRatio, promptPrice),
    createCache: deriveLanePrice(data.createCacheRatio, promptPrice),
    image: deriveLanePrice(data.imageRatio, promptPrice),
    audioInput: audioInputPrice,
    audioOutput: deriveLanePrice(data.audioCompletionRatio, audioInputPrice),
  }

  return {
    promptPrice,
    prices,
    enabled: {
      completion: hasValue(data.completionRatio),
      cache: hasValue(data.cacheRatio),
      createCache: hasValue(data.createCacheRatio),
      image: hasValue(data.imageRatio),
      audioInput: hasValue(data.audioRatio),
      audioOutput: hasValue(data.audioCompletionRatio),
    },
  }
}

export function buildPreviewRows(
  values: ModelPricingFormValues,
  mode: PricingMode,
  billingExpr: string,
  requestRuleExpr: string,
  promptPrice: string,
  lanePrices: Record<LaneKey, string>,
  laneEnabled: Record<LaneKey, boolean>,
  t: (key: string) => string,
  videoPriceTable?: VideoPriceTable
): PreviewRow[] {
  if (mode === 'tiered_expr') {
    const effectiveExpr = combineBillingExpr(billingExpr, requestRuleExpr)
    return [
      { key: 'mode', label: 'BillingMode', value: 'tiered_expr' },
      {
        key: 'expr',
        label: t('Expression'),
        value: effectiveExpr || t('Empty'),
        multiline: true,
      },
    ]
  }

  if (mode === 'per-request') {
    return [
      {
        key: 'price',
        label: 'ModelPrice',
        value: values.price || t('Empty'),
      },
    ]
  }

  if (mode === 'video-per-second') {
    const rows = videoPriceTable?.rows ?? []
    return [
      { key: 'mode', label: t('Mode'), value: t('Video per second') },
      {
        key: 'videoRows',
        label: t('Resolution'),
        value:
          rows.length > 0
            ? rows.map((row) => row.resolution).join(', ')
            : t('Empty'),
      },
    ]
  }

  // 预览金额已是显示货币串,符号随计费货币补齐(与输入框 addon 同源)。
  // per-request/tiered_expr 预览行只回显录入原文,不带前缀。
  const symbol = getBillingCurrency().symbol

  return [
    {
      key: 'inputPrice',
      label: t('Input price'),
      value: promptPrice ? `${symbol}${promptPrice}` : t('Empty'),
    },
    {
      key: 'completion',
      label: t('Completion price'),
      value:
        laneEnabled.completion && lanePrices.completion
          ? `${symbol}${lanePrices.completion}`
          : t('Empty'),
    },
    {
      key: 'cache',
      label: t('Cache read price'),
      value:
        laneEnabled.cache && lanePrices.cache
          ? `${symbol}${lanePrices.cache}`
          : t('Empty'),
    },
    {
      key: 'createCache',
      label: t('Cache write price'),
      value:
        laneEnabled.createCache && lanePrices.createCache
          ? `${symbol}${lanePrices.createCache}`
          : t('Empty'),
    },
    {
      key: 'image',
      label: t('Image input price'),
      value:
        laneEnabled.image && lanePrices.image
          ? `${symbol}${lanePrices.image}`
          : t('Empty'),
    },
    {
      key: 'audio',
      label: t('Audio input price'),
      value:
        laneEnabled.audioInput && lanePrices.audioInput
          ? `${symbol}${lanePrices.audioInput}`
          : t('Empty'),
    },
    {
      key: 'audioCompletion',
      label: t('Audio output price'),
      value:
        laneEnabled.audioOutput && lanePrices.audioOutput
          ? `${symbol}${lanePrices.audioOutput}`
          : t('Empty'),
    },
  ]
}

/**
 * Assembles the submitted model pricing data for the current mode, including
 * mode-specific fields (expression strings, video price table).
 */
export function buildPricingSubmitData(
  values: ModelPricingFormValues,
  mode: PricingMode,
  extra: {
    billingExpr: string
    requestRuleExpr: string
    videoPrices?: VideoPriceTable
  }
): ModelRatioData {
  // price 为 per-request 美元单价,编辑态是显示货币录入值,落库前折算为 USD;
  // ratio 家族是倍率(汇率约去),以录入值原样落库。
  const priceNumber = toNumberOrNull(values.price)
  const data: ModelRatioData = {
    name: values.name.trim(),
    billingMode: mode,
    price:
      priceNumber === null
        ? values.price || ''
        : formatPricingNumber(displayPriceToUsd(priceNumber)),
    ratio: values.ratio || '',
    cacheRatio: values.cacheRatio || '',
    createCacheRatio: values.createCacheRatio || '',
    completionRatio: values.completionRatio || '',
    imageRatio: values.imageRatio || '',
    audioRatio: values.audioRatio || '',
    audioCompletionRatio: values.audioCompletionRatio || '',
  }

  if (mode === 'tiered_expr') {
    data.billingExpr = extra.billingExpr
    data.requestRuleExpr = extra.requestRuleExpr
  }

  if (mode === 'video-per-second') {
    data.videoPrices = extra.videoPrices
  }

  return data
}
