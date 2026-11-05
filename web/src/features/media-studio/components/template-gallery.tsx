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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import { withBasePath } from '@/lib/base-path'

import catalog from '../catalog.json'
import type { StudioTemplate } from '../workflow-types'
import { GalleryGrid } from './gallery-grid'

export function TemplateGallery(props: {
  disabled: boolean
  onApply: (template: StudioTemplate, values: Record<string, string>) => void
}) {
  const { t } = useTranslation()
  const [filter, setFilter] = useState('all')
  const [selected, setSelected] = useState<StudioTemplate>()
  const [values, setValues] = useState<Record<string, string>>({})
  const templates = catalog.templates as StudioTemplate[]
  const visible = templates.filter(
    (item) =>
      filter === 'all' || item.mode === filter || item.collection === filter
  )
  return (
    <section aria-label={t('Template gallery')} className='space-y-3'>
      <div className='flex flex-wrap items-center gap-2'>
        {(
          [
            ['all', 'All templates'],
            ['animals', 'Animals'],
            ['create', 'Text to image'],
            ['edit', 'Image to image'],
          ] as const
        ).map(([key, label]) => (
          <Button
            key={key}
            size='sm'
            variant={filter === key ? 'secondary' : 'ghost'}
            aria-pressed={filter === key}
            onClick={() => setFilter(key)}
          >
            {t(label)}
          </Button>
        ))}
      </div>
      <GalleryGrid key={filter} label={t('Templates')}>
        {visible.map((item) => (
          <button
            key={item.id}
            type='button'
            disabled={props.disabled}
            className='group bg-card focus-visible:outline-ring flex min-w-0 flex-col overflow-hidden rounded-xl border text-left focus-visible:outline-2 disabled:opacity-50'
            onClick={() => {
              setSelected(item)
              setValues(
                Object.fromEntries(
                  item.fields.map((field) => [field.key, field.value])
                )
              )
            }}
          >
            <div className='bg-muted aspect-[4/3] w-full shrink-0 overflow-hidden'>
              <img
                src={withBasePath(item.preview)}
                alt={t(item.title)}
                className='h-full w-full object-cover transition-transform duration-300 group-hover:scale-105'
                loading='lazy'
              />
            </div>
            <div className='space-y-1 p-2 wrap-anywhere'>
              <span className='block text-xs leading-snug font-medium'>
                {t(item.title)}
              </span>
              <span className='text-muted-foreground block text-xs'>
                {t(
                  item.mode === 'edit'
                    ? 'Reference image needed'
                    : 'Start from a prompt'
                )}
              </span>
            </div>
          </button>
        ))}
      </GalleryGrid>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Real examples from our image models. Results vary with your prompt and references.'
        )}
      </p>
      <Dialog
        open={Boolean(selected)}
        onOpenChange={(open) => {
          if (!open) setSelected(undefined)
        }}
      >
        <DialogContent className='flex max-h-[90dvh] flex-col overflow-hidden sm:max-w-3xl'>
          <DialogHeader className='shrink-0'>
            <DialogTitle className='pe-8 leading-snug wrap-anywhere'>
              {selected && t(selected.title)}
            </DialogTitle>
            <DialogDescription>
              {selected && t(selected.description)}
            </DialogDescription>
          </DialogHeader>
          {selected && (
            <div className='-mx-1 grid min-h-0 grid-cols-1 gap-5 overflow-y-auto px-1 sm:grid-cols-2'>
              <div className='space-y-2'>
                <img
                  src={withBasePath(selected.preview)}
                  alt={t('Template example')}
                  className='w-full rounded-xl'
                />
                {selected.before && (
                  <details>
                    <summary className='cursor-pointer text-xs'>
                      {t('Example reference image')}
                    </summary>
                    <img
                      src={withBasePath(selected.before)}
                      alt={t('Before editing')}
                      className='mt-2 w-full rounded-xl'
                    />
                  </details>
                )}
              </div>
              <div className='space-y-3'>
                {selected.fields.map((field) => (
                  <label key={field.key} className='block space-y-1 text-sm'>
                    {t(field.label)}
                    <Textarea
                      className='max-h-32 min-h-9 py-1.5'
                      rows={2}
                      value={values[field.key] ?? ''}
                      maxLength={2000}
                      onChange={(event) =>
                        setValues({
                          ...values,
                          [field.key]: event.target.value,
                        })
                      }
                    />
                  </label>
                ))}
                <p className='bg-muted rounded-xl p-3 text-xs leading-relaxed'>
                  {t(selected.tip)}
                </p>
                <details>
                  <summary className='cursor-pointer text-xs'>
                    {t('View template prompt')}
                  </summary>
                  <p className='text-muted-foreground mt-2 text-xs wrap-anywhere whitespace-pre-wrap'>
                    {selected.prompt.replaceAll(
                      /\{\{(\w+)\}\}/g,
                      (_, key: string) => values[key] ?? ''
                    )}
                  </p>
                </details>
              </div>
            </div>
          )}
          {selected && (
            <Button
              className='w-full sm:w-auto sm:self-end'
              disabled={props.disabled}
              onClick={() => {
                props.onApply(selected, values)
                setSelected(undefined)
              }}
            >
              {t('Use this template')}
            </Button>
          )}
        </DialogContent>
      </Dialog>
    </section>
  )
}
