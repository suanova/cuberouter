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
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { FieldGroup } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from '@/components/ui/input-group'
import type { ImagePriceTable } from '@/features/pricing/types'
import { useBillingCurrency } from '@/lib/currency'

import {
  addImagePriceRowDraft,
  rebaseImagePriceDrafts,
  removeImagePriceRowDraft,
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

  const handleAddRow = () => handleDraftsChange(addImagePriceRowDraft(drafts))

  const handleRemoveRow = (id: string) => {
    const index = drafts.findIndex((draft) => draft.id === id)
    if (index === -1) return
    handleDraftsChange(removeImagePriceRowDraft(drafts, index))
  }

  const rowGridClass =
    'grid grid-cols-[minmax(0,1fr)_minmax(100px,150px)_auto] items-center gap-2'

  return (
    <FieldGroup className='gap-4'>
      <div className='space-y-2'>
        <div className={rowGridClass}>
          <span className='text-muted-foreground text-xs'>
            {t('Resolution')}
          </span>
          <span className='text-muted-foreground text-xs'>
            {t('Image price ({{symbol}}/image)', { symbol: currencySymbol })}
          </span>
          <span />
        </div>
        {drafts.map((draft) => (
          <div key={draft.id} className={rowGridClass}>
            <Input
              value={draft.resolution}
              placeholder='1024x1024'
              onChange={(event) =>
                handleRowChange(draft.id, { resolution: event.target.value })
              }
            />
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
            <Button
              variant='ghost'
              size='icon'
              onClick={() => handleRemoveRow(draft.id)}
              aria-label={t('Delete')}
            >
              <Trash2 className='text-destructive h-4 w-4' />
            </Button>
          </div>
        ))}
      </div>
      <Button
        type='button'
        variant='outline'
        size='sm'
        onClick={handleAddRow}
        className='w-fit'
      >
        <Plus data-icon='inline-start' />
        {t('Add resolution')}
      </Button>
    </FieldGroup>
  )
}
