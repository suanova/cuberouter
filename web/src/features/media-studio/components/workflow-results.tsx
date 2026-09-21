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
import { ImageIcon } from 'lucide-react'

import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'

import type { StudioAsset, WorkflowJob } from '../workflow-types'

function formatElapsed(ms: number): string {
  const totalSeconds = Math.floor(ms / 1000)
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes > 0) {
    return `${minutes}:${String(seconds).padStart(2, '0')}`
  }
  return `${seconds}s`
}

export function WorkflowResults(props: {
  job?: WorkflowJob
  busy: boolean
  count: number
  elapsed: number
  onEdit: (asset: StudioAsset, job: WorkflowJob) => void
}) {
  const { t } = useTranslation()
  const job = props.job
  let content
  if (props.busy) {
    content = (
      <div
        role='status'
        className='flex flex-col items-center gap-3 py-10 text-center'
      >
        <Spinner className='text-primary size-8' aria-hidden='true' />
        <p className='text-sm font-medium'>
          {t('Generating {{count}} image…', { count: props.count })}
        </p>
        <p className='text-muted-foreground text-xs'>
          {t('Elapsed {{time}}', { time: formatElapsed(props.elapsed) })}
        </p>
        <p className='text-muted-foreground max-w-sm text-xs'>
          {t(
            'Generation is synchronous and usually takes 40 seconds to 5 minutes. Keep this page open.'
          )}
        </p>
      </div>
    )
  } else if (!job) {
    content = (
      <div className='flex flex-col items-center gap-3 py-10 text-center'>
        <ImageIcon
          className='text-muted-foreground/50 size-10'
          aria-hidden='true'
        />
        <p className='text-muted-foreground text-sm'>
          {t('Your images will appear here.')}
        </p>
      </div>
    )
  } else {
    content = (
      <div className='space-y-4'>
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
      </div>
    )
  }
  return (
    <section className='space-y-3' aria-label={t('Image preview')}>
      <header className='flex items-center justify-between gap-2'>
        <h2 className='text-sm font-semibold'>{t('Image preview')}</h2>
        {!!job && (
          <span className='text-muted-foreground rounded-full border px-2 py-0.5 text-[11px]'>
            {job.request.model} · {formatElapsed(job.elapsed_ms)}
          </span>
        )}
      </header>
      <div className='bg-muted/30 flex flex-1 flex-col justify-center rounded-xl border p-4'>
        {content}
      </div>
    </section>
  )
}
