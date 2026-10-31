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

import type { useWorkflow } from '../hooks/use-workflow'
import type { WorkflowDraft } from '../workflow-types'

export function ImageJobPanel(props: {
  workflow: ReturnType<typeof useWorkflow>
  onRestore: (draft: WorkflowDraft) => void
}) {
  const { t } = useTranslation()
  const w = props.workflow
  const status = w.queueStatus
  if (!w.pending) return null
  const active =
    status && ['queued', 'running', 'cancelling'].includes(status.state)
  return (
    <section
      className='mt-4 space-y-2 border-t pt-3 text-sm'
      aria-label={t('Server image job')}
    >
      <p role='status'>
        {status?.state === 'queued'
          ? t('Queued: {{count}} jobs ahead', { count: status.ahead })
          : t(`Image job ${status?.state ?? 'reconnecting'}`)}
      </p>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Refreshing reconnects to this job without generating again. Images are released only after output review.'
        )}
      </p>
      {status?.recovery_pending && (
        <p role='status'>
          {t(
            'Cancellation is recorded. The worker is still cleaning up; your draft is safe.'
          )}
        </p>
      )}
      {active && !status.cancel_requested && (
        <Button
          type='button'
          variant='outline'
          disabled={w.cancellation.isPending}
          onClick={() => w.cancellation.mutate()}
        >
          {t('Cancel image job')}
        </Button>
      )}
      {!w.generation.isPending && (
        <div className='flex flex-wrap gap-2'>
          <Button type='button' variant='outline' onClick={w.reconnect}>
            {t('Reconnect to image job')}
          </Button>
          {status && !active && !status.recovery_pending && (
            <Button
              type='button'
              variant='outline'
              onClick={() => {
                if (w.pending) props.onRestore(w.pending.draft)
                void w.clearPending()
              }}
            >
              {t('Restore draft')}
            </Button>
          )}
        </div>
      )}
    </section>
  )
}
