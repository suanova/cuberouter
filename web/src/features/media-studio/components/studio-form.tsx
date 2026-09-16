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
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Dices, RotateCcw } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'

import { COUNT_OPTIONS, DEFAULT_PARAMS, LIMITS } from '../constants'
import type { AspectRatio, StudioParams } from '../types'
import { RatioGrid } from './ratio-grid'

interface StudioFormProps {
  params: StudioParams
  generating: boolean
  errorText: string | null
  models: string[]
  modelsLoading: boolean
  model: string
  onModelChange: (model: string) => void
  onChange: (params: StudioParams) => void
  onGenerate: () => void
}

function clamp(value: number, min: number, max: number): number {
  if (Number.isNaN(value)) {
    return min
  }
  return Math.min(max, Math.max(min, value))
}

export function StudioForm({
  params,
  generating,
  errorText,
  models,
  modelsLoading,
  model,
  onModelChange,
  onChange,
  onGenerate,
}: StudioFormProps) {
  const { t } = useTranslation()
  const [advancedOpen, setAdvancedOpen] = useState(false)

  const update = (patch: Partial<StudioParams>) => {
    onChange({ ...params, ...patch })
  }

  const hasModel = model !== ''
  const canGenerate =
    !generating && hasModel && params.prompt.trim() !== ''

  let modelOptions: ReactNode
  if (modelsLoading) {
    modelOptions = <option value=''>{t('Loading...')}</option>
  } else if (models.length === 0) {
    modelOptions = <option value=''>{t('No image models available')}</option>
  } else {
    modelOptions = models.map((name) => (
      <option key={name} value={name}>
        {name}
      </option>
    ))
  }

  const handleSeedRandom = () => {
    update({
      seed: Math.floor(Math.random() * (LIMITS.seedMax + 1)),
    })
  }

  return (
    <form
      className='flex flex-col gap-4'
      onSubmit={(e) => {
        e.preventDefault()
        if (canGenerate) {
          onGenerate()
        }
      }}
    >
      <div>
        <label
          htmlFor='studio-model'
          className='mb-1.5 block text-sm font-medium'
        >
          {t('Model')}
        </label>
        <select
          id='studio-model'
          value={model}
          disabled={generating || modelsLoading}
          onChange={(e) => onModelChange(e.target.value)}
          className='h-9 w-full rounded-md border border-input bg-background px-2 text-sm focus-visible:outline-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-50'
        >
          {modelOptions}
        </select>
      </div>

      <div>
        <label
          htmlFor='studio-prompt'
          className='mb-1.5 block text-sm font-medium'
        >
          {t('Prompt')}
        </label>
        <Textarea
          id='studio-prompt'
          value={params.prompt}
          onChange={(e) => update({ prompt: e.target.value })}
          maxLength={LIMITS.promptMax}
          rows={6}
          disabled={generating}
          placeholder={t('Describe the image you want to generate…')}
          className='min-h-28 resize-y text-sm'
        />
        <p className='mt-1 text-right text-[11px] text-muted-foreground'>
          {params.prompt.length} / {LIMITS.promptMax}
        </p>
      </div>

      <div>
        <span className='mb-1.5 block text-sm font-medium'>
          {t('Image aspect ratio')}
        </span>
        <RatioGrid
          value={params.ratio}
          onChange={(ratio: AspectRatio) => update({ ratio })}
          disabled={generating}
        />
      </div>

      <div>
        <label
          htmlFor='studio-count'
          className='mb-1.5 block text-sm font-medium'
        >
          {t('Images per batch')}
        </label>
        <select
          id='studio-count'
          value={params.count}
          disabled={generating}
          onChange={(e) => update({ count: Number(e.target.value) })}
          className='h-9 w-full rounded-md border border-input bg-background px-2 text-sm focus-visible:outline-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-50'
        >
          {COUNT_OPTIONS.map((count) => (
            <option key={count} value={count}>
              {t('{{count}} image', { count })}
            </option>
          ))}
        </select>
      </div>

      <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
        <CollapsibleTrigger className='flex w-full items-center justify-between text-sm font-medium text-muted-foreground hover:text-foreground'>
          {t('Advanced settings')}
          <svg
            aria-hidden='true'
            viewBox='0 0 16 16'
            className={`size-4 transition-transform ${advancedOpen ? 'rotate-180' : ''}`}
          >
            <path
              d='M4 6l4 4 4-4'
              fill='none'
              stroke='currentColor'
              strokeWidth='1.5'
              strokeLinecap='round'
              strokeLinejoin='round'
            />
          </svg>
        </CollapsibleTrigger>
        <CollapsibleContent className='mt-3 grid grid-cols-3 gap-3'>
          <div>
            <label
              htmlFor='studio-steps'
              className='mb-1.5 block text-xs font-medium'
            >
              {t('Steps')}
            </label>
            <input
              id='studio-steps'
              type='number'
              inputMode='numeric'
              min={LIMITS.stepsMin}
              max={LIMITS.stepsMax}
              value={params.steps}
              disabled={generating}
              onChange={(e) =>
                update({
                  steps: Math.round(clamp(Number(e.target.value), LIMITS.stepsMin, LIMITS.stepsMax)),
                })
              }
              className='h-9 w-full rounded-md border border-input bg-background px-2 text-sm focus-visible:outline-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-50'
            />
          </div>
          <div>
            <label
              htmlFor='studio-seed'
              className='mb-1.5 block text-xs font-medium'
            >
              {t('Seed')}
            </label>
            <div className='flex gap-1'>
              <input
                id='studio-seed'
                type='number'
                inputMode='numeric'
                min={LIMITS.seedMin}
                max={LIMITS.seedMax}
                value={params.seed}
                disabled={generating}
                onChange={(e) =>
                  update({
                    seed: Math.round(clamp(Number(e.target.value), LIMITS.seedMin, LIMITS.seedMax)),
                  })
                }
                className='h-9 w-full min-w-0 rounded-md border border-input bg-background px-2 text-sm focus-visible:outline-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-50'
              />
              <Button
                type='button'
                variant='outline'
                size='icon-sm'
                onClick={handleSeedRandom}
                disabled={generating}
                aria-label={t('Randomize seed')}
                title={t('Randomize seed')}
              >
                <Dices aria-hidden='true' />
              </Button>
            </div>
          </div>
          <div>
            <label
              htmlFor='studio-cfg'
              className='mb-1.5 block text-xs font-medium'
            >
              {t('CFG scale')}
            </label>
            <input
              id='studio-cfg'
              type='number'
              inputMode='decimal'
              min={LIMITS.cfgMin}
              max={LIMITS.cfgMax}
              step='0.1'
              value={params.cfg}
              disabled={generating}
              onChange={(e) =>
                update({
                  cfg: clamp(Number(e.target.value), LIMITS.cfgMin, LIMITS.cfgMax),
                })
              }
              className='h-9 w-full rounded-md border border-input bg-background px-2 text-sm focus-visible:outline-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-50'
            />
          </div>
        </CollapsibleContent>
      </Collapsible>

      <div className='flex items-center gap-2'>
        <Button
          type='submit'
          className='flex-1'
          disabled={!canGenerate}
        >
          {generating ? (
            <>
              <Spinner aria-hidden='true' />
              {t('Generating…')}
            </>
          ) : (
            t('Generate image')
          )}
        </Button>
        <Button
          type='button'
          variant='ghost'
          disabled={generating}
          onClick={() => onChange({ ...DEFAULT_PARAMS, prompt: '' })}
          aria-label={t('Reset to defaults')}
          title={t('Reset to defaults')}
        >
          <RotateCcw aria-hidden='true' />
        </Button>
      </div>

      {errorText ? (
        <p role='alert' className='text-sm text-destructive'>
          {errorText}
        </p>
      ) : null}
    </form>
  )
}
