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
import * as React from 'react'
import { useTranslation } from 'react-i18next'
import { ImageIcon } from 'lucide-react'

import { Spinner } from '@/components/ui/spinner'

import { ASPECT_RATIOS, qualitySteps } from '../constants'
import type { GenerationErrorInfo } from '../lib/errors'
import type { GenerationResult, GenerationStatus, StudioParams } from '../types'
import { ResultGallery } from './result-gallery'

interface PreviewPanelProps {
  status: GenerationStatus
  params: StudioParams
  elapsedMs: number
  result: GenerationResult | null
  error: GenerationErrorInfo | null
  model: string
}

function formatElapsed(ms: number): string {
  const totalSeconds = Math.floor(ms / 1000)
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes > 0) {
    return `${minutes}:${String(seconds).padStart(2, '0')}`
  }
  return `${seconds}s`
}

export function PreviewPanel({
  status,
  params,
  elapsedMs,
  result,
  error,
  model,
}: PreviewPanelProps) {
  const { t } = useTranslation()

  let content: React.ReactNode
  if (status === 'generating') {
    content = (
      <div
        role='status'
        className='flex flex-col items-center gap-3 py-10 text-center'
      >
        <Spinner className='size-8 text-primary' aria-hidden='true' />
        <p className='text-sm font-medium'>
          {t('Generating {{count}} image…', {
            count: params.count,
          })}
        </p>
        <p className='text-xs text-muted-foreground'>
          {t('Elapsed {{time}}', { time: formatElapsed(elapsedMs) })}
        </p>
        <p className='max-w-sm text-xs text-muted-foreground'>
          {t('Generation is synchronous and usually takes 40 seconds to 5 minutes. Keep this page open.')}
        </p>
      </div>
    )
  } else if (status === 'success' && result) {
    content = (
      <div className='flex flex-col gap-3'>
        <ResultGallery result={result} />
        <p className='text-xs text-muted-foreground'>
          {t('{{count}} image · {{steps}} steps · elapsed {{time}}', {
            count: result.images.length,
            steps: qualitySteps(params.quality),
            time: formatElapsed(elapsedMs),
          })}
        </p>
      </div>
    )
  } else if (status === 'error' && error) {
    content = (
      <div role='alert' className='flex flex-col items-center gap-2 py-10 text-center'>
        <p className='text-sm font-medium text-destructive'>
          {t('Generation failed')}
        </p>
        <p className='max-w-md text-xs text-muted-foreground'>
          {error.kind === 'server' ? error.message : t(error.message)}
        </p>
      </div>
    )
  } else {
    content = (
      <div className='flex flex-col items-center gap-3 py-10 text-center'>
        <ImageIcon aria-hidden='true' className='size-10 text-muted-foreground/50' />
        <p className='text-sm text-muted-foreground'>
          {t('Your images will appear here.')}
        </p>
        {params.ratio ? (
          <p className='text-xs text-muted-foreground/70'>
            {params.ratio} · {ASPECT_RATIOS[params.ratio].join(' × ')}
          </p>
        ) : null}
      </div>
    )
  }

  return (
    <section
      aria-label={t('Image preview')}
      className='flex min-h-[420px] flex-1 flex-col gap-3'
    >
      <header className='flex items-center justify-between gap-2'>
        <h2 className='text-sm font-semibold'>
          {t('Image preview')}
        </h2>
        <span className='rounded-full border border-border px-2 py-0.5 text-[11px] text-muted-foreground'>
          {model}
        </span>
      </header>

      <div className='flex flex-1 flex-col justify-center rounded-xl border border-border bg-muted/30 p-4'>
        {content}
      </div>
    </section>
  )
}
