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
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'

import catalog from '../catalog.json'
import type { StudioTemplate } from '../workflow-types'

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
    <section aria-label={t('Template gallery')} className='space-y-4'>
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
      <div className='grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-5'>
        {visible.map((item) => (
          <button
            key={item.id}
            type='button'
            disabled={props.disabled}
            className='group bg-muted focus-visible:outline-ring relative aspect-[4/5] overflow-hidden rounded-2xl text-left focus-visible:outline-2 disabled:opacity-50'
            onClick={() => {
              setSelected(item)
              setValues(
                Object.fromEntries(
                  item.fields.map((field) => [field.key, field.value])
                )
              )
            }}
          >
            <img
              src={item.preview}
              alt={t(item.title)}
              className='h-full w-full object-cover transition-transform duration-300 group-hover:scale-105'
              loading='lazy'
            />
            <div className='absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/90 to-transparent px-2 pt-8 pb-2 text-white'>
              <span className='mb-0.5 block text-[10px] tracking-wider uppercase'>
                {t(
                  item.mode === 'edit'
                    ? 'Reference image needed'
                    : 'Start from a prompt'
                )}
              </span>
              <span className='text-xs font-medium'>{t(item.title)}</span>
            </div>
          </button>
        ))}
      </div>
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
        <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-3xl'>
          <DialogTitle>{selected && t(selected.title)}</DialogTitle>
          <DialogDescription>
            {selected && t(selected.description)}
          </DialogDescription>
          {selected && (
            <div className='grid gap-5 sm:grid-cols-2'>
              <div className='space-y-2'>
                <img
                  src={selected.preview}
                  alt={t('Template example')}
                  className='w-full rounded-xl'
                />
                {selected.before && (
                  <details>
                    <summary className='cursor-pointer text-xs'>
                      {t('Example reference image')}
                    </summary>
                    <img
                      src={selected.before}
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
                    <Input
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
                  <p className='text-muted-foreground mt-2 text-xs whitespace-pre-wrap'>
                    {selected.prompt.replaceAll(
                      /\{\{(\w+)\}\}/g,
                      (_, key: string) => values[key] ?? ''
                    )}
                  </p>
                </details>
                <Button
                  className='w-full'
                  disabled={props.disabled}
                  onClick={() => {
                    props.onApply(selected, values)
                    setSelected(undefined)
                  }}
                >
                  {t('Use this template')}
                </Button>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </section>
  )
}
