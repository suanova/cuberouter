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
import { ChevronRight, Copy } from 'lucide-react'
import { memo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import {
  DEFAULT_TOKEN_UNIT,
  getModelEndpointLabels,
  MAX_CARD_TAGS_DISPLAY,
} from '../constants'
import {
  getCardExamplePrice,
  getDynamicDisplayGroupRatio,
  getDynamicPriceUnitLabelKey,
  getDynamicPricingSummary,
  isUnconfiguredTaskUsageModel,
} from '../lib/dynamic-price'
import { parseTags } from '../lib/filters'
import { isTokenBasedModel } from '../lib/model-helpers'
import { formatPrice, formatRequestPrice } from '../lib/price'
import { getTaskNumberFields } from '../lib/task-expr'
import { formatVideoPriceMoney } from '../lib/video-price'
import type { PricingModel, TokenUnit } from '../types'
import { ModelPerfBadge, type ModelPerfBadgeData } from './model-perf-badge'

export interface ModelCardProps {
  model: PricingModel
  onClick: () => void
  priceRate?: number
  usdExchangeRate?: number
  tokenUnit?: TokenUnit
  showRechargePrice?: boolean
  selectedGroup?: string
  perf?: ModelPerfBadgeData
}

export const ModelCard = memo(function ModelCard(props: ModelCardProps) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const tokenUnit = props.tokenUnit ?? DEFAULT_TOKEN_UNIT
  const priceRate = props.priceRate ?? 1
  const usdExchangeRate = props.usdExchangeRate ?? 1
  const showRechargePrice = props.showRechargePrice ?? false
  const isTokenBased = isTokenBasedModel(props.model)
  const tags = parseTags(props.model.tags)
  const modelIconKey = props.model.icon || props.model.vendor_icon
  const modelIcon = modelIconKey ? getLobeIcon(modelIconKey, 28) : null
  const initial = props.model.model_name?.charAt(0).toUpperCase() || '?'
  const isDynamicPricing =
    props.model.billing_mode === 'tiered_expr' &&
    Boolean(props.model.billing_expr)
  const isUnconfiguredTaskUsage = isUnconfiguredTaskUsageModel(props.model)
  const dynamicPriceOptions = {
    tokenUnit,
    showRechargePrice,
    priceRate,
    usdExchangeRate,
    groupRatioMultiplier: getDynamicDisplayGroupRatio(
      props.model,
      props.selectedGroup
    ),
  }
  const dynamicSummary = isDynamicPricing
    ? getDynamicPricingSummary(props.model, dynamicPriceOptions)
    : null
  const cardExamplePrice = getCardExamplePrice(props.model, dynamicPriceOptions)
  const showTaskFieldLabels =
    getTaskNumberFields(props.model.billing_usage_schema).length > 1

  // 标签与端点各占一行:标签截断到上限,端点全部展开,不再用 +N 折叠
  const cardTags = tags.slice(0, MAX_CARD_TAGS_DISPLAY)
  const endpointLabels = getModelEndpointLabels(props.model, t)

  const handleCopy = (e: React.MouseEvent) => {
    e.stopPropagation()
    copyToClipboard(props.model.model_name || '')
  }

  let priceSummary: ReactNode
  if (props.model.video_prices) {
    // 卡片只展示起步价:输入固定为 -,输出取各分辨率正常价的最低值,
    // 完整分辨率/错峰价格表在详情弹窗展示。
    const minNormalPrice = Math.min(
      ...props.model.video_prices.rows.map((row) => row.normal_price)
    )
    priceSummary = (
      <>
        <span className='text-muted-foreground whitespace-nowrap'>
          {t('Input')}{' '}
          <span className='text-foreground font-mono font-semibold'>-</span>
        </span>
        <span className='text-muted-foreground whitespace-nowrap'>
          {t('Output')}{' '}
          <span className='text-foreground font-mono font-semibold'>
            {t('From {{price}}', {
              price: formatVideoPriceMoney(minNormalPrice),
            })}
            /{t('s')}
          </span>
        </span>
      </>
    )
  } else if (props.model.image_prices) {
    // 图片按张计费:与视频同样窄列堆叠,分辨率 + 单价/张。
    const { rows } = props.model.image_prices
    priceSummary = (
      <div className='mt-2 w-full min-w-0'>
        <div className='space-y-1'>
          {rows.map((row) => (
            <div
              key={row.resolution || row.price}
              className='flex items-baseline justify-between gap-x-2 text-xs whitespace-nowrap'
            >
              <span className='text-muted-foreground'>{row.resolution}</span>
              <span className='text-foreground font-mono tabular-nums'>
                {formatVideoPriceMoney(row.price)}/{t('image')}
              </span>
            </div>
          ))}
        </div>
      </div>
    )
  } else if (dynamicSummary) {
    if (dynamicSummary.isSpecialExpression) {
      priceSummary = (
        <span className='min-w-0'>
          <span className='text-amber-700 dark:text-amber-300'>
            {t('Special billing expression')}
          </span>
          <code className='text-muted-foreground/70 mt-0.5 line-clamp-1 block font-mono text-[11px] break-all'>
            {dynamicSummary.rawExpression}
          </code>
        </span>
      )
    } else if (dynamicSummary.primaryEntries.length > 0) {
      priceSummary = (
        <>
          {dynamicSummary.primaryEntries.map((entry) => {
            const unitLabelKey = getDynamicPriceUnitLabelKey(entry)
            let fieldPrefix: ReactNode = null
            if (entry.labelKind !== 'schema') {
              fieldPrefix = <>{t(entry.shortLabel)} </>
            } else if (showTaskFieldLabels) {
              fieldPrefix = (
                <>
                  <code className='font-mono text-[11px]'>
                    {entry.shortLabel}
                  </code>{' '}
                </>
              )
            }
            return (
              <span
                key={entry.key}
                className='text-muted-foreground whitespace-nowrap'
              >
                {fieldPrefix}
                <span className='text-foreground font-mono font-semibold'>
                  {entry.formattedRange ?? entry.formatted}
                  {unitLabelKey && <>/{t(unitLabelKey)}</>}
                </span>
              </span>
            )
          })}
          {cardExamplePrice && (
            <span className='text-muted-foreground/70 max-w-full min-w-0 truncate text-xs'>
              {cardExamplePrice.label} ≈ {cardExamplePrice.formatted}
            </span>
          )}
          {dynamicSummary.isTaskUsage &&
            dynamicSummary.tier?.label &&
            !dynamicSummary.primaryEntries.some(
              (entry) => entry.formattedRange
            ) && (
              <span className='text-muted-foreground text-xs'>
                ({dynamicSummary.tier.label})
              </span>
            )}
        </>
      )
    } else {
      priceSummary = (
        <span className='text-muted-foreground text-sm'>
          {t('Dynamic Pricing')}
        </span>
      )
    }
  } else if (isUnconfiguredTaskUsage) {
    priceSummary = (
      <span className='text-muted-foreground text-sm'>
        {t('Usage-based billing · price not configured')}
      </span>
    )
  } else if (isTokenBased) {
    priceSummary = (
      <>
        <span className='text-muted-foreground whitespace-nowrap'>
          {t('Input')}{' '}
          <span className='text-foreground font-mono font-semibold'>
            {formatPrice(
              props.model,
              'input',
              tokenUnit,
              showRechargePrice,
              priceRate,
              usdExchangeRate,
              props.selectedGroup
            )}
          </span>
        </span>
        <span className='text-muted-foreground whitespace-nowrap'>
          {t('Output')}{' '}
          <span className='text-foreground font-mono font-semibold'>
            {formatPrice(
              props.model,
              'output',
              tokenUnit,
              showRechargePrice,
              priceRate,
              usdExchangeRate,
              props.selectedGroup
            )}
          </span>
        </span>
      </>
    )
  } else {
    priceSummary = (
      <span className='text-muted-foreground whitespace-nowrap'>
        <span className='text-foreground font-mono font-semibold'>
          {formatRequestPrice(
            props.model,
            showRechargePrice,
            priceRate,
            usdExchangeRate,
            props.selectedGroup
          )}
        </span>{' '}
        / {t('request')}
      </span>
    )
  }

  return (
    <div
      className={cn(
        'group relative flex flex-col rounded-xl border p-3 transition-colors sm:p-5',
        'hover:bg-muted/20'
      )}
    >
      {/* Header: icon + name + price + actions */}
      <div className='flex items-start justify-between gap-2.5 sm:gap-3'>
        <div className='flex min-w-0 items-start gap-2.5 sm:gap-3'>
          <div className='bg-muted/40 flex size-9 shrink-0 items-center justify-center rounded-lg sm:size-10 sm:rounded-xl'>
            {modelIcon || (
              <span className='text-muted-foreground text-sm font-bold'>
                {initial}
              </span>
            )}
          </div>
          <div className='min-w-0'>
            <h3 className='text-foreground truncate font-mono text-[15px] leading-tight font-bold'>
              {props.model.model_name}
            </h3>
            <div className='mt-0.5 flex flex-wrap items-baseline gap-x-2 gap-y-0.5 text-sm sm:mt-1 sm:gap-x-3'>
              {priceSummary}
            </div>
          </div>
        </div>

        <div className='flex shrink-0 items-center gap-1.5'>
          <button
            type='button'
            onClick={props.onClick}
            className='text-muted-foreground hover:text-foreground hover:bg-muted inline-flex items-center gap-1 rounded-md border px-2 py-1 text-xs transition-colors sm:px-2.5 sm:py-1.5'
          >
            {t('Details')}
            <ChevronRight className='size-3.5' />
          </button>
          <button
            type='button'
            onClick={handleCopy}
            className='text-muted-foreground hover:text-foreground hover:bg-muted rounded-md border p-1.5 transition-colors'
            title={t('Copy')}
          >
            <Copy className='size-3.5' />
          </button>
        </div>
      </div>

      {/* Description */}
      <p className='text-muted-foreground mt-2 line-clamp-1 flex-1 text-[13px] leading-relaxed sm:mt-4 sm:line-clamp-2 sm:min-h-[2.5rem]'>
        {props.model.description || t('No description available.')}
      </p>

      {/* Footer: 左列上标签、下端点,右列性能摘要跨这两行 */}
      <div className='mt-2 grid grid-cols-[minmax(0,1fr)_auto] items-start gap-x-2 gap-y-1 sm:mt-4'>
        <div className='flex min-w-0 flex-wrap items-center gap-x-2.5 gap-y-0.5 sm:gap-x-3 sm:gap-y-1'>
          {cardTags.map((tag) => (
            <span key={tag} className='text-muted-foreground/70 text-xs'>
              {tag}
            </span>
          ))}
        </div>
        <ModelPerfBadge perf={props.perf} className='row-span-2 self-start' />

        <div className='flex min-w-0 flex-wrap items-center gap-x-2.5 gap-y-0.5 sm:gap-x-3 sm:gap-y-1'>
          {endpointLabels.map((label) => (
            <span key={label} className='text-muted-foreground/70 text-xs'>
              {label}
            </span>
          ))}
        </div>
      </div>
    </div>
  )
})
