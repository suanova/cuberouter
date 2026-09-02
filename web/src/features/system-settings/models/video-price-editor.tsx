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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { FieldGroup } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from '@/components/ui/input-group'
import type { VideoPriceTable } from '@/features/pricing/types'

import { numericDraftRegex } from './model-pricing-core'
import {
  addVideoPriceRowDraft,
  removeVideoPriceRowDraft,
  updateVideoPriceRowDraft,
  videoPriceDraftsFromTable,
  videoPriceTableFromDrafts,
  type VideoPriceRowDraft,
} from './video-price-drafts'

export type VideoPriceEditorProps = {
  modelName?: string
  table: VideoPriceTable
  onChange: (table: VideoPriceTable) => void
}

export const VideoPriceEditor = function VideoPriceEditor(
  props: VideoPriceEditorProps
) {
  const { t } = useTranslation()
  const [drafts, setDrafts] = useState<VideoPriceRowDraft[]>(() =>
    videoPriceDraftsFromTable(props.table)
  )

  const handleDraftsChange = (nextDrafts: VideoPriceRowDraft[]) => {
    setDrafts(nextDrafts)
    props.onChange(videoPriceTableFromDrafts(nextDrafts))
  }

  const handleRowChange = (id: string, patch: Partial<VideoPriceRowDraft>) => {
    const index = drafts.findIndex((draft) => draft.id === id)
    if (index === -1) return
    handleDraftsChange(updateVideoPriceRowDraft(drafts, index, patch))
  }

  const handleAddRow = () => handleDraftsChange(addVideoPriceRowDraft(drafts))

  const handleRemoveRow = (id: string) => {
    const index = drafts.findIndex((draft) => draft.id === id)
    if (index === -1) return
    handleDraftsChange(removeVideoPriceRowDraft(drafts, index))
  }

  const rowGridClass =
    'grid grid-cols-[minmax(0,1fr)_minmax(100px,150px)_minmax(100px,150px)_auto] items-center gap-2'

  return (
    <FieldGroup className='gap-4'>
      <div className='space-y-2'>
        <div className={rowGridClass}>
          <span className='text-muted-foreground text-xs'>
            {t('Resolution')}
          </span>
          <span className='text-muted-foreground text-xs'>
            {t('Video price (¥/s)')}
          </span>
          <span className='text-muted-foreground text-xs'>
            {t('Off-peak price (¥/s)')}
          </span>
          <span />
        </div>
        {drafts.map((draft) => (
          <div key={draft.id} className={rowGridClass}>
            <Input
              value={draft.resolution}
              placeholder='1080p'
              onChange={(event) =>
                handleRowChange(draft.id, { resolution: event.target.value })
              }
            />
            <InputGroup>
              <InputGroupAddon>¥</InputGroupAddon>
              <InputGroupInput
                inputMode='decimal'
                value={draft.normalPrice}
                placeholder='0.75'
                onChange={(event) => {
                  const value = event.target.value
                  if (numericDraftRegex.test(value)) {
                    handleRowChange(draft.id, { normalPrice: value })
                  }
                }}
              />
            </InputGroup>
            <InputGroup>
              <InputGroupAddon>¥</InputGroupAddon>
              <InputGroupInput
                inputMode='decimal'
                value={draft.offPeakPrice}
                placeholder='0.375'
                onChange={(event) => {
                  const value = event.target.value
                  if (numericDraftRegex.test(value)) {
                    handleRowChange(draft.id, { offPeakPrice: value })
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
