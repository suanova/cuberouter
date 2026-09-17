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
import { Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'

import { isActiveJob } from '../lib/workflow'
import type { WorkflowJob } from '../workflow-types'
import { PrivateImage } from './private-image'

export function WorkflowHistory(props: {
  jobs: WorkflowJob[]
  selected?: string
  busy: boolean
  onSelect: (id: string) => void
  onDelete: (id: string) => void
}) {
  const { t } = useTranslation()
  const [filter, setFilter] = useState('all')
  const [deleting, setDeleting] = useState<string>()
  const visible = props.jobs.filter(
    (job) => filter === 'all' || job.mode === filter
  )
  return (
    <section className='space-y-4' aria-label={t('Creation history')}>
      <div className='flex flex-wrap gap-1'>
        {(
          [
            ['all', 'All creations'],
            ['create', 'Text to image'],
            ['edit', 'Image to image'],
            ['regional', 'Regional edit'],
            ['text', 'Text layout'],
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
      {!visible.length && (
        <p className='text-muted-foreground py-12 text-center text-sm'>
          {t('Your saved creations will appear here.')}
        </p>
      )}
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-3'>
        {visible.map((job) => (
          <article
            key={job.id}
            className='bg-card overflow-hidden rounded-xl border'
          >
            <button
              type='button'
              aria-pressed={props.selected === job.id}
              aria-label={`${t('Open creation')}: ${job.request.prompt}`}
              className='focus-visible:outline-ring block w-full text-left focus-visible:outline-2'
              onClick={() => props.onSelect(job.id)}
            >
              {job.result?.images[0] ? (
                <PrivateImage
                  id={job.result.images[0].id}
                  alt={job.request.prompt}
                  className='aspect-video w-full object-cover'
                />
              ) : (
                <div className='bg-muted flex aspect-video items-center justify-center text-xs'>
                  {t(job.state)}
                </div>
              )}
              <div className='space-y-1 p-3'>
                <p className='text-primary text-[10px] tracking-wider uppercase'>
                  {t(job.mode)} · {t(job.state)}
                </p>
                <p className='line-clamp-2 text-xs'>{job.request.prompt}</p>
                <p className='text-muted-foreground text-[10px]'>
                  {new Date(job.created_at * 1000).toLocaleString()}
                </p>
                {job.parent_id && (
                  <p className='text-muted-foreground text-[10px]'>
                    {t('Edited from a previous version')}
                  </p>
                )}
              </div>
            </button>
            <div className='flex items-center justify-between px-3 pb-3'>
              <span className='text-muted-foreground text-[10px]'>
                {t('Expires')}:{' '}
                {new Date(job.expires_at * 1000).toLocaleDateString()}
              </span>
              <Button
                size='icon-sm'
                variant='ghost'
                disabled={props.busy || isActiveJob(job)}
                aria-label={t('Delete creation')}
                onClick={() => setDeleting(job.id)}
              >
                <Trash2 className='size-3' />
              </Button>
            </div>
          </article>
        ))}
      </div>
      <Dialog
        open={Boolean(deleting)}
        onOpenChange={(open) => {
          if (!open) setDeleting(undefined)
        }}
      >
        <DialogContent>
          <DialogTitle>{t('Delete this creation?')}</DialogTitle>
          <DialogDescription>
            {t(
              'This removes the saved version and its studio images. Existing completed child versions remain available.'
            )}
          </DialogDescription>
          <Button
            variant='destructive'
            onClick={() => {
              if (deleting) props.onDelete(deleting)
              setDeleting(undefined)
            }}
          >
            {t('Delete creation')}
          </Button>
        </DialogContent>
      </Dialog>
    </section>
  )
}
