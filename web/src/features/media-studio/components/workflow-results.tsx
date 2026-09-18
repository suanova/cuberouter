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
import { Download, ImagePlus, Pencil, Copy, Check, Clock3 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { usePrivateImage } from '../hooks/use-private-image'
import { isActiveJob, publicCommand } from '../lib/workflow'
import type { StudioAsset, WorkflowJob } from '../workflow-types'
import { PrivateImage } from './private-image'

export function ResultImage(props: {
  asset: StudioAsset
  busy: boolean
  onEdit: () => void
  onTools: () => void
}) {
  const { t } = useTranslation()
  const image = usePrivateImage(props.asset.id)
  return (
    <figure className='bg-background min-w-0 overflow-hidden rounded-xl border'>
      {image.url ? (
        <a href={image.url} target='_blank' rel='noreferrer'>
          <img
            src={image.url}
            alt={t('Generated image')}
            className='max-h-[580px] w-full object-contain'
          />
        </a>
      ) : (
        <div role='status' className='p-12 text-center text-xs'>
          {t(image.failed ? 'Image unavailable or expired' : 'Loading image…')}
        </div>
      )}
      <figcaption className='space-y-2 p-3'>
        <div className='flex items-center justify-between gap-2'>
          <span className='text-muted-foreground text-[11px]'>
            {props.asset.width} × {props.asset.height}
          </span>
          {image.url && (
            <a
              className='text-primary flex items-center gap-1 text-xs'
              href={image.url}
              download={`cuberouter-${props.asset.id}.png`}
            >
              <Download className='size-3' />
              {t('Download')}
            </a>
          )}
        </div>
        <div className='flex flex-wrap gap-2'>
          <Button
            size='sm'
            variant='secondary'
            disabled={props.busy}
            onClick={props.onEdit}
          >
            <ImagePlus className='size-3' />
            {t('Continue editing')}
          </Button>
          <Button
            size='sm'
            variant='outline'
            disabled={props.busy}
            onClick={props.onTools}
          >
            <Pencil className='size-3' />
            {t('Image tools')}
          </Button>
        </div>
      </figcaption>
    </figure>
  )
}

export function WorkflowResults(props: {
  job?: WorkflowJob
  busy: boolean
  onEdit: (asset: StudioAsset, job: WorkflowJob) => void
  onTools: (asset: StudioAsset, job: WorkflowJob) => void
}) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  const [copyFailed, setCopyFailed] = useState(false)
  const job = props.job
  if (!job) {
    return (
      <div className='text-muted-foreground rounded-2xl border border-dashed p-12 text-center text-sm'>
        {t('Choose a template or describe your first image.')}
      </div>
    )
  }
  const active = isActiveJob(job)
  const result = job.result
  let quality = 'Text check is informational; generation is complete.'
  if (result?.text_quality === 'needs_review') {
    quality = 'Image generated. Please review the requested text.'
  }
  if (result?.text_quality === 'passed') {
    quality =
      'Recognized text passed the automated comparison. Please review visually.'
  }
  if (result?.text_quality === 'unavailable') {
    quality = 'Image generated. The automatic text check was unavailable.'
  }
  return (
    <section className='space-y-4' aria-label={t('Generation result')}>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div>
          <h2 className='font-medium'>{t('Your creation')}</h2>
          <p className='text-muted-foreground mt-1 font-mono text-[10px]'>
            {job.id}
          </p>
        </div>
        <span className='bg-muted rounded-full px-3 py-1 text-xs'>
          {t(job.state)}
        </span>
      </div>
      {active && (
        <div
          role='status'
          className='bg-primary/5 space-y-3 rounded-xl border p-5'
        >
          <p className='flex items-center gap-2 text-sm'>
            <Clock3 className='text-primary size-4 animate-pulse' />
            {job.stage}
          </p>
          <progress
            aria-label={t('Generation progress')}
            max={job.total_steps ?? 1}
            value={job.completed_steps ?? 0}
            className='accent-primary h-1.5 w-full'
          />
          <p className='text-muted-foreground text-xs'>
            {t(
              'You can refresh this page. The submitted job continues on the server.'
            )}
          </p>
        </div>
      )}
      {job.error && (
        <p
          role='alert'
          className='bg-destructive/10 text-destructive rounded-xl p-4 text-sm'
        >
          {job.error}
        </p>
      )}
      {result && (
        <>
          <div
            className={`grid gap-3 ${result.images.length > 1 ? 'sm:grid-cols-2' : ''}`}
          >
            {result.images.map((asset) => (
              <ResultImage
                key={asset.id}
                asset={asset}
                busy={props.busy}
                onEdit={() => props.onEdit(asset, job)}
                onTools={() => props.onTools(asset, job)}
              />
            ))}
          </div>
          <div className='text-muted-foreground flex flex-wrap gap-x-5 gap-y-1 text-xs'>
            {typeof result.inference_seconds === 'number' && (
              <span>
                {t(job.mode === 'text' ? 'Text rendering' : 'GPU generation')}:{' '}
                {result.inference_seconds.toFixed(1)} s
              </span>
            )}
            {typeof result.workflow_seconds === 'number' && (
              <span>
                {t('Workflow')}: {result.workflow_seconds.toFixed(1)} s
              </span>
            )}
          </div>
          {job.mode !== 'regional' && job.mode !== 'text' && (
            <div className='bg-muted/60 rounded-xl p-4 text-xs leading-relaxed'>
              <p>{t(quality)}</p>
              <p className='text-muted-foreground mt-1'>
                {t(
                  'OCR is a check, not a guarantee of correct wording or facts. No-text images are not treated as failed generations.'
                )}
              </p>
            </div>
          )}
          {result.comparisons.map((comparison) => (
            <details
              key={comparison.original?.id ?? comparison.revised?.id}
              className='rounded-xl border p-4'
              open
            >
              <summary className='cursor-pointer text-sm font-medium'>
                {t('Before and after comparison')}
              </summary>
              <p className='text-muted-foreground my-3 text-xs'>
                {t(
                  comparison.repair_applied === false
                    ? 'A correction was attempted. The original was kept; the candidate is for review.'
                    : 'Compare the original with the saved version. Check text, boundaries and preserved details.'
                )}
              </p>
              <div className='grid gap-3 sm:grid-cols-2'>
                {(
                  [
                    ['original', 'Before editing'],
                    ['revised', 'After editing'],
                    ['candidate', 'Full model candidate'],
                  ] as const
                ).map(
                  ([key, label]) =>
                    comparison[key] && (
                      <figure key={key}>
                        <PrivateImage id={comparison[key].id} alt={t(label)} />
                        <figcaption className='mt-2 text-xs'>
                          {t(label)}
                        </figcaption>
                      </figure>
                    )
                )}
              </div>
              {comparison.notice && (
                <p className='text-muted-foreground mt-3 text-xs'>
                  {comparison.notice}
                </p>
              )}
            </details>
          ))}
        </>
      )}
      <details className='rounded-xl border p-4'>
        <summary className='cursor-pointer text-xs font-medium'>
          {t('Input command and output result')}
        </summary>
        <div className='mt-3 flex justify-end'>
          <Button
            size='sm'
            variant='ghost'
            onClick={() => {
              navigator.clipboard
                .writeText(publicCommand(job))
                .then(() => {
                  setCopied(true)
                  setCopyFailed(false)
                })
                .catch(() => setCopyFailed(true))
            }}
          >
            {copied ? (
              <Check className='size-3' />
            ) : (
              <Copy className='size-3' />
            )}
            {t('Copy command')}
          </Button>
        </div>
        {copyFailed && (
          <p role='alert' className='text-xs'>
            {t('Copy failed. Select and copy the command below.')}
          </p>
        )}
        <p className='text-muted-foreground mb-2 text-xs'>
          {t(
            'Set CubeRouter to your site URL and SessionToken to your dashboard session token. No channel key is exposed.'
          )}
        </p>
        <pre className='bg-muted max-h-64 overflow-auto rounded-lg p-3 text-[11px]'>
          {publicCommand(job)}
        </pre>
        <pre className='bg-muted mt-3 max-h-80 overflow-auto rounded-lg p-3 text-[11px]'>
          {JSON.stringify(job, null, 2)}
        </pre>
      </details>
      {job.request_id && (
        <p className='text-muted-foreground text-[11px] break-all'>
          {t('Usage log request ID')}: <code>{job.request_id}</code>
        </p>
      )}
    </section>
  )
}
