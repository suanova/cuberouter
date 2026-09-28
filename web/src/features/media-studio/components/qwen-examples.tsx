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

import { exampleReferences } from '../lib/qwen-examples'
import catalog from '../official-examples.json'
import type { WorkflowDraft } from '../workflow-types'

export function QwenExamples(props: {
  draft: WorkflowDraft
  disabled: boolean
  onApply: (draft: WorkflowDraft) => void
}) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState('')
  const [error, setError] = useState(false)
  return (
    <section
      className='space-y-4'
      aria-label={t('Official Qwen Image 2.1 examples')}
    >
      <h2 className='text-lg font-semibold'>
        {t('Official Qwen Image 2.1 examples')}
      </h2>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Load an example, change its images or prompt, then generate. Only input crops are sent to the model; official results are references, not your output.'
        )}
      </p>
      {error && (
        <p role='alert'>{t('Official example could not be loaded.')}</p>
      )}
      <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3'>
        {catalog.examples.map((example) => (
          <article key={example.id} className='space-y-3 rounded-xl border p-3'>
            <img
              src={`/image-studio/official-qwen21/${example.preview.replace('.png', '-preview.jpg')}`}
              alt={t(example.title)}
              loading='lazy'
              className='h-48 w-full rounded-lg object-contain'
            />
            <h3 className='font-medium'>{t(example.title)}</h3>
            <p className='text-muted-foreground text-xs'>
              {t(example.description)}
            </p>
            <Button
              variant='outline'
              disabled={props.disabled || !!loading}
              onClick={async () => {
                setLoading(example.id)
                setError(false)
                try {
                  const references = await exampleReferences(example)
                  props.onApply({
                    ...props.draft,
                    mode: 'edit',
                    queued: true,
                    count: 1,
                    prompt: t(example.prompt),
                    user_prompt: undefined,
                    size: `${example.width}x${example.height}`,
                    steps: 40,
                    cfg: 1,
                    seed: 42,
                    references,
                    parent_id: undefined,
                    annotation: undefined,
                  })
                } catch {
                  setError(true)
                } finally {
                  setLoading('')
                }
              }}
            >
              {t(loading === example.id ? 'Loading...' : 'Load example')}
            </Button>
          </article>
        ))}
      </div>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Adapted trial prompts; the official article does not provide complete generation settings. Results can differ.'
        )}{' '}
        <a
          className='underline'
          href={catalog.source}
          target='_blank'
          rel='noreferrer'
        >
          {t('Qwen Team · Source and examples')}
        </a>
      </p>
    </section>
  )
}
