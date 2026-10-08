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
import { Film, Trash2 } from 'lucide-react'

import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { cn } from '@/lib/utils'

import type { VideoJob } from '../video-types'

/**
 * 本地视频历史：紧凑列表，选中项在结果区展开。
 * 视频字节以 Blob 存在 IndexedDB，列表不预载字节，选中后才生成 object URL。
 */
export function VideoHistory(props: {
  jobs: VideoJob[]
  selected?: string
  busy: boolean
  onSelect: (id: string) => void
  onDelete: (id?: string) => void
}) {
  const { t } = useTranslation()
  const [deleting, setDeleting] = useState<string | null>(null)
  return (
    <section className='space-y-3' aria-label={t('Video history')}>
      <header className='flex items-center justify-between gap-2'>
        <h2 className='text-muted-foreground text-xs font-semibold tracking-wide uppercase'>
          {t('Video history')}
        </h2>
        {!!props.jobs.length && (
          <Button
            variant='ghost'
            size='xs'
            disabled={props.busy}
            onClick={() => setDeleting('')}
          >
            <Trash2 aria-hidden='true' />
            {t('Clear history')}
          </Button>
        )}
      </header>
      {!props.jobs.length && (
        <p className='text-muted-foreground rounded-lg border border-dashed p-4 text-center text-xs'>
          {t('No videos in this browser yet.')}
        </p>
      )}
      <ul className='flex flex-col gap-1.5'>
        {props.jobs.map((job) => {
          const current = job.id === props.selected
          return (
            <li key={job.id}>
              <div className='flex items-start gap-2'>
                <button
                  type='button'
                  aria-pressed={current}
                  aria-label={`${t('Open video')}: ${job.request.prompt}`}
                  onClick={() => props.onSelect(job.id)}
                  className={cn(
                    'flex min-w-0 flex-1 items-center gap-3 rounded-lg border p-2 text-left transition-colors',
                    current
                      ? 'border-primary/60 bg-muted'
                      : 'border-border hover:bg-muted'
                  )}
                >
                  <Film
                    className='text-muted-foreground size-8 shrink-0'
                    aria-hidden='true'
                  />
                  <span className='min-w-0 flex-1'>
                    <span className='block truncate text-xs font-medium'>
                      {job.request.prompt}
                    </span>
                    <span className='text-muted-foreground block text-[11px]'>
                      {job.request.model} ·{' '}
                      {new Date(job.created_at).toLocaleString()}
                      {job.request.mode === 'image'
                        ? ` · ${t('From a first frame image')}`
                        : ''}
                    </span>
                  </span>
                </button>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-xs'
                  disabled={props.busy}
                  aria-label={t('Delete video')}
                  onClick={() => setDeleting(job.id)}
                >
                  <Trash2 aria-hidden='true' />
                </Button>
              </div>
            </li>
          )
        })}
      </ul>
      <Dialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
      >
        <DialogContent>
          <DialogTitle>{t('Delete video history?')}</DialogTitle>
          <DialogDescription>
            {t(
              'This removes copies from this browser only. Provider uploads and usage logs are unaffected.'
            )}
          </DialogDescription>
          <Button
            variant='destructive'
            onClick={() => {
              props.onDelete(deleting || undefined)
              setDeleting(null)
            }}
          >
            {t('Delete')}
          </Button>
        </DialogContent>
      </Dialog>
    </section>
  )
}
