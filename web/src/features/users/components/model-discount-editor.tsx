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
import { Plus, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import {
  discountToPercent,
  isValidPercent,
  removeDiscount,
  upsertDiscount,
} from '../lib/model-discount'
import type { UserModelDiscounts } from '../types'

type ModelDiscountEditorProps = {
  value: UserModelDiscounts
  onChange: (next: UserModelDiscounts) => void
  modelOptions: string[]
  disabled?: boolean
}

/** The row started by the add button: no model yet, kept out of `value`. */
type NewRow = {
  model: string | null
  percentText: string
}

export function ModelDiscountEditor(props: ModelDiscountEditorProps) {
  const { t } = useTranslation()
  const [percentDrafts, setPercentDrafts] = useState<Record<string, string>>({})
  const [newRow, setNewRow] = useState<NewRow | null>(null)

  const options = useMemo(
    () => [...new Set([...Object.keys(props.value), ...props.modelOptions])],
    [props.value, props.modelOptions]
  )
  const selectItems = useMemo(
    () => options.map((model) => ({ value: model, label: model })),
    [options]
  )

  const dropPercentDraft = (model: string) => {
    setPercentDrafts((prev) => {
      if (!(model in prev)) return prev
      const next = { ...prev }
      delete next[model]
      return next
    })
  }

  const handlePercentInput = (model: string, text: string) => {
    // Keep the typed text as-is so an invalid percent stays visible (with its
    // error) instead of snapping back to the committed value.
    setPercentDrafts((prev) => ({ ...prev, [model]: text }))
    const percent = Number(text)
    if (isValidPercent(percent)) {
      props.onChange(upsertDiscount(props.value, model, percent))
    }
  }

  const handleModelChange = (model: string, nextModel: string | null) => {
    if (!nextModel || nextModel === model) return
    const renamed = removeDiscount(props.value, model)
    renamed[nextModel] = props.value[model]
    dropPercentDraft(model)
    dropPercentDraft(nextModel)
    props.onChange(renamed)
  }

  const handleRemove = (model: string) => {
    dropPercentDraft(model)
    props.onChange(removeDiscount(props.value, model))
  }

  const handleNewRowModel = (nextModel: string | null) => {
    if (!newRow || !nextModel) return
    const percent = Number(newRow.percentText)
    if (!isValidPercent(percent)) {
      // Keep the row so the percent error stays on screen and the pick is kept.
      setNewRow({ ...newRow, model: nextModel })
      return
    }
    setPercentDrafts((prev) => ({ ...prev, [nextModel]: newRow.percentText }))
    setNewRow(null)
    props.onChange(upsertDiscount(props.value, nextModel, percent))
  }

  const handleNewRowPercent = (text: string) => {
    if (!newRow) return
    const { model } = newRow
    const percent = Number(text)
    if (model && isValidPercent(percent)) {
      setPercentDrafts((prev) => ({ ...prev, [model]: text }))
      setNewRow(null)
      props.onChange(upsertDiscount(props.value, model, percent))
      return
    }
    setNewRow({ ...newRow, percentText: text })
  }

  const renderOptions = () => (
    <SelectContent alignItemWithTrigger={false}>
      <SelectGroup>
        {options.map((option) => (
          <SelectItem key={option} value={option}>
            {option}
          </SelectItem>
        ))}
      </SelectGroup>
    </SelectContent>
  )

  return (
    <div className='space-y-3' data-slot='model-discount-editor'>
      <div className='text-sm font-medium'>{t('Model Discounts')}</div>
      <div className='space-y-3'>
        {Object.entries(props.value).map(([model, discount]) => {
          const text =
            percentDrafts[model] ?? String(discountToPercent(discount))
          const invalidPercent = !isValidPercent(Number(text))
          return (
            <div key={model} className='space-y-1'>
              <div className='flex items-center gap-2'>
                <Select
                  items={selectItems}
                  value={model}
                  disabled={props.disabled}
                  onValueChange={(next) => handleModelChange(model, next)}
                >
                  <SelectTrigger
                    className='min-w-0 flex-1'
                    aria-label={t('Model Discounts')}
                  >
                    <SelectValue />
                  </SelectTrigger>
                  {renderOptions()}
                </Select>
                <Input
                  type='number'
                  min={1}
                  max={100}
                  step='any'
                  className='w-24 shrink-0'
                  aria-label={t('Discount (%)')}
                  aria-invalid={invalidPercent || undefined}
                  value={text}
                  disabled={props.disabled}
                  onChange={(event) =>
                    handlePercentInput(model, event.target.value)
                  }
                />
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-sm'
                  className='text-muted-foreground shrink-0'
                  aria-label={t('Remove')}
                  disabled={props.disabled}
                  onClick={() => handleRemove(model)}
                >
                  <Trash2 aria-hidden='true' />
                </Button>
              </div>
              {invalidPercent ? (
                <p className='text-destructive text-xs'>
                  {t('Discount must be between 1% and 100%')}
                </p>
              ) : null}
            </div>
          )
        })}
        {newRow ? (
          <div className='space-y-1'>
            <div className='flex items-center gap-2'>
              <Select
                items={selectItems}
                value={newRow.model}
                disabled={props.disabled}
                onValueChange={handleNewRowModel}
              >
                <SelectTrigger
                  className='min-w-0 flex-1'
                  aria-label={t('Model Discounts')}
                  aria-invalid={newRow.model === null || undefined}
                >
                  <SelectValue />
                </SelectTrigger>
                {renderOptions()}
              </Select>
              <Input
                type='number'
                min={1}
                max={100}
                step='any'
                className='w-24 shrink-0'
                aria-label={t('Discount (%)')}
                aria-invalid={
                  !isValidPercent(Number(newRow.percentText)) || undefined
                }
                value={newRow.percentText}
                disabled={props.disabled}
                onChange={(event) => handleNewRowPercent(event.target.value)}
              />
              <Button
                type='button'
                variant='ghost'
                size='icon-sm'
                className='text-muted-foreground shrink-0'
                aria-label={t('Remove')}
                disabled={props.disabled}
                onClick={() => setNewRow(null)}
              >
                <Trash2 aria-hidden='true' />
              </Button>
            </div>
            {newRow.model === null ? (
              <p className='text-destructive text-xs'>
                {t('Model name is required')}
              </p>
            ) : null}
            {!isValidPercent(Number(newRow.percentText)) ? (
              <p className='text-destructive text-xs'>
                {t('Discount must be between 1% and 100%')}
              </p>
            ) : null}
          </div>
        ) : null}
      </div>
      <Button
        type='button'
        variant='outline'
        size='sm'
        disabled={props.disabled}
        onClick={() => setNewRow({ model: null, percentText: '100' })}
      >
        <Plus className='mr-1 h-3 w-3' aria-hidden='true' />
        {t('Add model discount')}
      </Button>
    </div>
  )
}
