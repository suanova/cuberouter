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
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { FieldGroup } from '@/components/ui/field'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from '@/components/ui/input-group'
import { imagePriceTierLabelKey } from '@/features/pricing/lib/image-price'
import type { ImagePriceTable } from '@/features/pricing/types'
import { useBillingCurrency } from '@/lib/currency'

import {
  rebaseImagePriceDrafts,
  updateImagePriceRowDraft,
  imagePriceDraftsFromTable,
  imagePriceTableFromDrafts,
  type ImagePriceRowDraft,
} from './image-price-drafts'
import { numericDraftRegex, usdPriceToDisplay } from './model-pricing-core'

export type ImagePriceEditorProps = {
  modelName?: string
  table: ImagePriceTable
  onChange: (table: ImagePriceTable) => void
}

/**
 * 图片按张计费编辑器:固定 Fast/Standard/High 三个画质档位行(不可增删),
 * 每行填单张价格;留空的档位视为未定价,提交时不带该档(后端按锚点计费)。
 */
export const ImagePriceEditor = function ImagePriceEditor(
  props: ImagePriceEditorProps
) {
  const { t } = useTranslation()
  const { symbol: currencySymbol, exchangeRate } = useBillingCurrency()
  const [drafts, setDrafts] = useState<ImagePriceRowDraft[]>(() =>
    imagePriceDraftsFromTable(props.table)
  )
  // 草稿按加载时的汇率换算成显示货币;汇率变化时 rebase(保留底层 USD 意图),
  // 否则后续任一行的编辑会按新汇率重换算整张旧汇率草稿,写错未编辑行的价。
  const draftRateRef = useRef(exchangeRate)

  useEffect(() => {
    const nextRate = exchangeRate
    const prevRate = draftRateRef.current
    if (nextRate === prevRate) return
    draftRateRef.current = nextRate
    setDrafts((prev) => rebaseImagePriceDrafts(prev, prevRate, nextRate))
  }, [exchangeRate])

  const handleDraftsChange = (nextDrafts: ImagePriceRowDraft[]) => {
    setDrafts(nextDrafts)
    props.onChange(imagePriceTableFromDrafts(nextDrafts))
  }

  const handleRowChange = (id: string, patch: Partial<ImagePriceRowDraft>) => {
    const index = drafts.findIndex((draft) => draft.id === id)
    if (index === -1) return
    handleDraftsChange(updateImagePriceRowDraft(drafts, index, patch))
  }

  const rowGridClass =
    'grid grid-cols-[minmax(0,1fr)_minmax(100px,150px)] items-center gap-2'

  return (
    <FieldGroup className='gap-4'>
      <div className='space-y-2'>
        <div className={rowGridClass}>
          <span className='text-muted-foreground text-xs'>{t('Quality')}</span>
          <span className='text-muted-foreground text-xs'>
            {t('Image price ({{symbol}}/image)', { symbol: currencySymbol })}
          </span>
        </div>
        {drafts.map((draft) => {
          const labelKey = imagePriceTierLabelKey(draft.tier)
          return (
            <div key={draft.id} className={rowGridClass}>
              <span className='text-sm'>
                {labelKey ? t(labelKey) : draft.tier}
              </span>
              <InputGroup>
                <InputGroupAddon>{currencySymbol}</InputGroupAddon>
                <InputGroupInput
                  inputMode='decimal'
                  value={draft.price}
                  placeholder={usdPriceToDisplay(0.02)}
                  onChange={(event) => {
                    const value = event.target.value
                    if (numericDraftRegex.test(value)) {
                      handleRowChange(draft.id, { price: value })
                    }
                  }}
                />
              </InputGroup>
            </div>
          )
        })}
      </div>
    </FieldGroup>
  )
}
