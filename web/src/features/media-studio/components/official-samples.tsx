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
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import {
  loadOfficialSample,
  officialSampleURL,
  type OfficialSample,
} from '../lib/official-samples'
import samples from '../official-samples.json'
import type { WorkflowDraft } from '../workflow-types'

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
    <section className='space-y-3'>
      <h2 className='text-lg font-medium'>{t('Official Qwen examples')}</h2>
      <p className='text-muted-foreground text-sm'>
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
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-3'>
        {samples.map((sample) => (
          <article
            className='overflow-hidden rounded-xl border'
            key={sample.id}
          >
            <img
              src={officialSampleURL(sample.preview)}
              alt={t(sample.title)}
              loading='lazy'
              className='bg-muted h-40 w-full object-contain'
            />
            <div className='space-y-2 p-3'>
              <h3 className='font-medium'>{t(sample.title)}</h3>
              <p className='text-muted-foreground text-xs'>
                {t('{{count}} reference images', {
                  count: sample.inputs.length,
                })}
              </p>
              <Button
                type='button'
                variant='outline'
                disabled={props.disabled || load.isPending}
                onClick={() => load.mutate(sample)}
              >
                {t('Load example')}
              </Button>
            </div>
          </article>
        ))}
      </div>
    </section>
  )
}
