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
import { useMutation } from '@tanstack/react-query'
import { Maximize2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'

import {
  loadOfficialSample,
  officialSampleURL,
  type OfficialSample,
} from '../lib/official-samples'
import samples from '../official-samples.json'
import type { WorkflowDraft } from '../workflow-types'
import { GalleryGrid } from './gallery-grid'

export function OfficialSamples(props: {
  disabled: boolean
  onApply: (draft: Partial<WorkflowDraft>, edit: boolean) => void
}) {
  const { t } = useTranslation()
  const load = useMutation({
    mutationFn: async (sample: OfficialSample) => ({
      draft: await loadOfficialSample(sample),
      edit: sample.inputs.length > 0,
    }),
    onSuccess: ({ draft, edit }) => props.onApply(draft, edit),
    retry: false,
  })
  return (
    <div className='space-y-3'>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Choose an example, review its prompt and references, then generate. Official outputs are examples; only input crops are submitted.'
        )}{' '}
        <a
          className='underline'
          href='https://qwen.ai/blog?id=qwen-image-2.1'
          target='_blank'
          rel='noreferrer'
        >
          {t('Qwen Team source')}
        </a>
      </p>
      {load.isError && (
        <p role='alert'>{t('Official example inputs could not be loaded.')}</p>
      )}
      <GalleryGrid label={t('Official Qwen examples')}>
        {samples.map((sample) => {
          const title = t(sample.title)
          const preview = officialSampleURL(sample.preview)
          return (
            <article
              className='bg-card flex min-w-0 flex-col overflow-hidden rounded-xl border'
              key={sample.id}
            >
              <Dialog>
                <DialogTrigger
                  aria-label={t('Enlarge {{title}}', { title })}
                  className='bg-muted focus-visible:outline-ring relative block aspect-[4/3] w-full shrink-0 cursor-zoom-in overflow-hidden focus-visible:outline-2 focus-visible:-outline-offset-2'
                >
                  <img
                    src={preview}
                    alt=''
                    loading='lazy'
                    className='absolute inset-0 size-full object-cover'
                  />
                  <span className='absolute right-2 bottom-2 rounded-md bg-black/70 p-1.5 text-white'>
                    <Maximize2 className='size-4' aria-hidden='true' />
                  </span>
                </DialogTrigger>
                <DialogContent className='max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-5xl'>
                  <DialogTitle className='pe-8 leading-snug wrap-anywhere'>
                    {title}
                  </DialogTitle>
                  <img
                    src={preview}
                    alt={title}
                    className='mx-auto max-h-[calc(100dvh-7rem)] w-auto max-w-full object-contain'
                  />
                </DialogContent>
              </Dialog>
              <div className='flex flex-1 flex-col gap-1 p-2'>
                <h2 className='text-xs leading-snug font-medium wrap-anywhere'>
                  {title}
                </h2>
                <div className='mt-auto flex flex-wrap items-center justify-between gap-1'>
                  <p className='text-muted-foreground text-xs'>
                    {t('{{count}} reference images', {
                      count: sample.inputs.length,
                    })}
                  </p>
                  <Button
                    type='button'
                    size='xs'
                    variant='outline'
                    className='h-auto min-h-6 max-w-full whitespace-normal'
                    disabled={props.disabled || load.isPending}
                    onClick={() => load.mutate(sample)}
                  >
                    {t('Load example')}
                  </Button>
                </div>
              </div>
            </article>
          )
        })}
      </GalleryGrid>
    </div>
  )
}
