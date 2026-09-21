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
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'

import type { WorkflowJob } from '../workflow-types'

export function WorkflowHistory(props: {
  jobs: WorkflowJob[]
  busy: boolean
  onSelect: (id: string) => void
  onDelete: (id?: string) => void
}) {
  const { t } = useTranslation()
  const [deleting, setDeleting] = useState<string | null>(null)
  return (
    <section className='space-y-4' aria-label={t('Creation history')}>
      {!!props.jobs.length && (
        <Button
          variant='outline'
          size='sm'
          disabled={props.busy}
          onClick={() => setDeleting('')}
        >
          {t('Clear history')}
        </Button>
      )}
      {!props.jobs.length && (
        <p className='text-muted-foreground py-12 text-center text-sm'>
          {t('No generations in this browser yet.')}
        </p>
      )}
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-3'>
        {props.jobs.map((job) => (
          <article
            key={job.id}
            className='bg-card overflow-hidden rounded-xl border'
          >
            <button
              type='button'
              className='w-full text-left'
              aria-label={`${t('Open creation')}: ${job.request.prompt}`}
              onClick={() => props.onSelect(job.id)}
            >
              <img
                src={job.images[0]?.url}
                alt={job.request.prompt}
                className='aspect-video w-full object-cover'
              />
              <div className='space-y-1 p-3'>
                <p className='line-clamp-2 text-xs'>{job.request.prompt}</p>
                <p className='text-muted-foreground text-xs'>
                  {job.request.model} ·{' '}
                  {new Date(job.created_at).toLocaleString()}
                </p>
                {job.request.parent_id && (
                  <p className='text-muted-foreground text-xs'>
                    {t('Edited from a previous version')}
                  </p>
                )}
              </div>
            </button>
            <Button
              variant='ghost'
              size='sm'
              disabled={props.busy}
              onClick={() => setDeleting(job.id)}
            >
              {t('Delete creation')}
            </Button>
          </article>
        ))}
      </div>
      <Dialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
      >
        <DialogContent>
          <DialogTitle>{t('Delete local history?')}</DialogTitle>
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
