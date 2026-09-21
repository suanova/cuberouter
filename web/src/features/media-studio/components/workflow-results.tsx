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
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { publicCommand } from '../lib/workflow'
import type { StudioAsset, WorkflowDraft, WorkflowJob } from '../workflow-types'

export function WorkflowResults(props: {
  job?: WorkflowJob
  busy: boolean
  submitted?: WorkflowDraft
  elapsed: number
  onEdit: (asset: StudioAsset, job: WorkflowJob) => void
}) {
  const { t } = useTranslation()
  const job = props.job
  if (props.busy) {
    return (
      <div role='status' className='bg-card space-y-3 rounded-xl border p-6'>
        <p>{t('Generation in progress…')}</p>
        <p>
          {t('Elapsed {{time}}', {
            time: `${Math.floor(props.elapsed / 1000)}s`,
          })}
        </p>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Keep this page open. A timeout may still have consumed quota; check Usage Logs before retrying.'
          )}
        </p>
        {props.submitted && (
          <pre className='overflow-auto text-xs'>
            {publicCommand(props.submitted)}
          </pre>
        )}
      </div>
    )
  }
  if (!job) {
    return (
      <p className='text-muted-foreground py-16 text-center'>
        {t('Your images will appear here.')}
      </p>
    )
  }
  return (
    <section className='space-y-4' aria-label={t('Image preview')}>
      <p className='text-muted-foreground text-xs'>
        {job.request.model} · {Math.round(job.elapsed_ms / 1000)}s
      </p>
      <div className='grid gap-4 sm:grid-cols-2'>
        {job.images.map((asset, index) => (
          <article
            className='bg-card space-y-2 rounded-xl border p-3'
            key={asset.id}
          >
            <img
              src={asset.url}
              alt={t('Generated image')}
              className='w-full rounded-lg'
              referrerPolicy='no-referrer'
            />
            <div className='flex flex-wrap gap-2'>
              <a
                className='rounded-lg border px-3 py-2 text-xs'
                href={asset.url}
                download={`image-${index + 1}.png`}
                target='_blank'
                rel='noreferrer'
              >
                {t('Download')}
              </a>
              <Button
                size='sm'
                disabled={props.busy}
                onClick={() => props.onEdit(asset, job)}
              >
                {t('Continue editing')}
              </Button>
            </div>
          </article>
        ))}
      </div>
      {job.request.mode === 'edit' && (
        <details>
          <summary className='cursor-pointer text-sm'>
            {t('Before and after comparison')}
          </summary>
          <div className='mt-2 grid grid-cols-3 gap-3'>
            {job.request.references.map((asset) => (
              <img
                key={asset.id}
                src={asset.url}
                alt={t('Original image')}
                className='rounded-lg'
              />
            ))}
          </div>
        </details>
      )}
      <details>
        <summary className='cursor-pointer text-sm'>
          {t('Input command and output')}
        </summary>
        <pre className='bg-muted mt-2 overflow-auto rounded-lg p-3 text-xs'>
          {publicCommand(job.request)}
        </pre>
        <pre className='bg-muted mt-2 overflow-auto rounded-lg p-3 text-xs'>
          {JSON.stringify(
            {
              images: job.images.map((asset) => ({
                id: asset.id,
                mime: asset.mime,
              })),
              request_id: job.request_id,
              elapsed_ms: job.elapsed_ms,
            },
            null,
            2
          )}
        </pre>
      </details>
    </section>
  )
}
