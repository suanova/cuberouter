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
import { Film } from 'lucide-react'

import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'

import type { VideoJob } from '../video-types'

function formatElapsed(ms: number): string {
  const totalSeconds = Math.floor(ms / 1000)
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes > 0) {
    return `${minutes}:${String(seconds).padStart(2, '0')}`
  }
  return `${seconds}s`
}

/** 本地历史的视频以 ArrayBuffer 入库，播放/下载前包成 Blob 换成 object URL，卸载或切换时回收。 */
function useVideoSrc(job?: VideoJob): string | undefined {
  const [src, setSrc] = useState<string>()
  useEffect(() => {
    if (!job?.video) {
      setSrc(job?.video_url)
      return
    }
    const objectUrl = URL.createObjectURL(
      new Blob([job.video], { type: job.video_mime })
    )
    setSrc(objectUrl)
    return () => URL.revokeObjectURL(objectUrl)
  }, [job?.video, job?.video_url, job?.id, job?.video_mime])
  return src
}

export function VideoResults(props: {
  job?: VideoJob
  busy: boolean
  elapsed: number
  progress?: string
}) {
  const { t } = useTranslation()
  const job = props.job
  const src = useVideoSrc(job)
  let content
  if (props.busy) {
    content = (
      <div
        role='status'
        className='flex flex-col items-center gap-3 py-10 text-center'
      >
        <Spinner className='text-primary size-8' aria-hidden='true' />
        <p className='text-sm font-medium'>{t('Generating video…')}</p>
        <p className='text-muted-foreground text-xs'>
          {t('Elapsed {{time}}', { time: formatElapsed(props.elapsed) })}
          {props.progress ? ` · ${props.progress}` : ''}
        </p>
        <p className='text-muted-foreground max-w-sm text-xs'>
          {t(
            'Video generation is asynchronous and usually takes a few minutes. Keep this page open.'
          )}
        </p>
      </div>
    )
  } else if (!job) {
    content = (
      <div className='flex flex-col items-center gap-3 py-10 text-center'>
        <Film
          className='text-muted-foreground/50 size-10'
          aria-hidden='true'
        />
        <p className='text-muted-foreground text-sm'>
          {t('Your videos will appear here.')}
        </p>
      </div>
    )
  } else if (!src) {
    content = (
      <div className='flex flex-col items-center gap-3 py-10 text-center'>
        <Film
          className='text-muted-foreground/50 size-10'
          aria-hidden='true'
        />
        <p className='text-muted-foreground text-sm'>
          {t('This video is no longer available.')}
        </p>
      </div>
    )
  } else {
    content = (
      <div className='space-y-4'>
        <div className='space-y-2'>
          <video
            src={src}
            controls
            preload='metadata'
            className='w-full rounded-lg bg-black'
            aria-label={t('Generated video')}
          />
          <div className='flex flex-wrap gap-2'>
            <Button
              variant='outline'
              size='sm'
              render={
                <a
                  href={src}
                  download={`video-${job.created_at}.mp4`}
                  target='_blank'
                  rel='noreferrer'
                />
              }
            >
              {t('Download')}
            </Button>
          </div>
        </div>
        {job.request.mode === 'image' && job.request.image && (
          <details>
            <summary className='cursor-pointer text-sm'>
              {t('First frame comparison')}
            </summary>
            <div className='mt-2 grid grid-cols-2 gap-3'>
              <img
                src={job.request.image.url}
                alt={t('First frame image')}
                className='rounded-lg'
              />
            </div>
          </details>
        )}
      </div>
    )
  }
  return (
    <section className='space-y-3' aria-label={t('Video preview')}>
      <header className='flex items-center justify-between gap-2'>
        <h2 className='text-sm font-semibold'>{t('Video preview')}</h2>
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
