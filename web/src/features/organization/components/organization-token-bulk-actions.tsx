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
import type { Table } from '@tanstack/react-table'
import { Copy, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { copyToClipboard } from '@/lib/copy-to-clipboard'

import { getOrganizationTokenBatchDeletePlan } from '../lib'
import type { OrganizationTokenRow } from '../types'
import { OrganizationTokensBatchDeleteDialog } from './organization-token-delete-dialogs'

type OrganizationTokensBulkActionsProps = {
  organizationId: number
  table: Table<OrganizationTokenRow>
  /** Whether the caller may remove this particular key. */
  canDeleteToken: (token: OrganizationTokenRow) => boolean
  /** Whether the caller may take this particular key away. */
  canCopyToken: (token: OrganizationTokenRow) => boolean
  onDeleted: () => Promise<unknown> | unknown
}

/**
 * What can be done to several keys at once: copy them out, or remove them.
 *
 * Copying is offered as a choice rather than a single action because the two
 * shapes answer different questions — a list of bare keys feeds a script, a list
 * of `name<TAB>key` pairs feeds a person — and guessing which one was wanted
 * produces a clipboard the user has to rebuild by hand.
 */
export function OrganizationTokensBulkActions(
  props: OrganizationTokensBulkActionsProps
) {
  const { t } = useTranslation()
  const [isCopyFormatOpen, setIsCopyFormatOpen] = useState(false)
  // Set when the clipboard refuses the write, so the text can still be taken
  // away by hand rather than being lost.
  const [manualCopyText, setManualCopyText] = useState('')
  const [isDeleteOpen, setIsDeleteOpen] = useState(false)

  const selected = props.table
    .getFilteredSelectedRowModel()
    .rows.map((row) => row.original)

  const copyable = selected.filter((token) => props.canCopyToken(token))

  const copy = async (text: string, count: number) => {
    const ok = await copyToClipboard(text)
    setIsCopyFormatOpen(false)
    if (ok) {
      toast.success(t('Copied {{count}} key(s)', { count }))
      return
    }
    setManualCopyText(text)
  }

  const copyWithNames = () =>
    void copy(
      copyable
        .map((token) => `${token.name}\t${fullKeyOf(token)}`)
        .filter((line) => line.trim().length > 1)
        .join('\n'),
      copyable.length
    )

  const copyKeysOnly = () =>
    void copy(
      copyable
        .map((token) => fullKeyOf(token))
        .filter(Boolean)
        .join('\n'),
      copyable.length
    )

  /**
   * Whether any of the selected keys is outside the caller's authority.
   *
   * The backend removes a batch all-or-nothing, so a selection containing one
   * key the caller may not touch would fail as a whole. Saying so up front beats
   * a refusal that names no row.
   */
  const deleteSelection = () => {
    const plan = getOrganizationTokenBatchDeletePlan(selected, props.canDeleteToken)
    if (plan.selectedCount === 0) {
      toast.error(t('Select at least one key first.'))
      return
    }
    if (!plan.request) {
      toast.error(
        t(
          'Cannot delete: {{unauthorized}} of the {{total}} selected keys are not yours to remove.',
          {
            unauthorized: plan.unauthorizedCount,
            total: plan.selectedCount,
          }
        )
      )
      return
    }
    setIsDeleteOpen(true)
  }

  return (
    <>
      <BulkActionsToolbar table={props.table} entityName={t('organization key')}>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                className='size-8'
                aria-label={t('Copy selected keys')}
                onClick={() => {
                  if (copyable.length === 0) {
                    toast.error(t('Select at least one key first.'))
                    return
                  }
                  setIsCopyFormatOpen(true)
                }}
              />
            }
          >
            <Copy className='size-4' />
            <span className='sr-only'>{t('Copy selected keys')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Copy selected keys')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='destructive'
                size='icon'
                className='size-8'
                aria-label={t('Delete selected keys')}
                onClick={deleteSelection}
              />
            }
          >
            <Trash2 className='size-4' />
            <span className='sr-only'>{t('Delete selected keys')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Delete selected keys')}</p>
          </TooltipContent>
        </Tooltip>
      </BulkActionsToolbar>

      <Dialog
        open={isCopyFormatOpen}
        onOpenChange={setIsCopyFormatOpen}
        title={t('Copy selected keys')}
        description={t(
          'Choose what to put on the clipboard. {{count}} key(s) will be copied.',
          { count: copyable.length }
        )}
        contentHeight='auto'
        footer={
          <Button
            type='button'
            variant='outline'
            onClick={() => setIsCopyFormatOpen(false)}
          >
            {t('Cancel')}
          </Button>
        }
      >
        <div className='flex flex-col gap-2 sm:flex-row'>
          <Button
            type='button'
            variant='outline'
            className='flex-1'
            onClick={copyWithNames}
          >
            {t('Name and key')}
          </Button>
          <Button type='button' className='flex-1' onClick={copyKeysOnly}>
            {t('Key only')}
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={manualCopyText !== ''}
        onOpenChange={(open) => !open && setManualCopyText('')}
        title={t('Copy these keys')}
        description={t(
          'The clipboard is unavailable in this browser, so the keys are shown here to copy by hand.'
        )}
        contentHeight='min(50vh, 360px)'
        footer={
          <Button
            type='button'
            variant='outline'
            onClick={() => setManualCopyText('')}
          >
            {t('Close')}
          </Button>
        }
      >
        <Textarea
          readOnly
          value={manualCopyText}
          className='min-h-40 font-mono text-xs'
          onFocus={(event) => event.currentTarget.select()}
        />
      </Dialog>

      <OrganizationTokensBatchDeleteDialog
        organizationId={props.organizationId}
        tokens={isDeleteOpen ? selected : []}
        onOpenChange={setIsDeleteOpen}
        onDeleted={async () => {
          // The removed rows no longer exist, so keeping them selected would
          // leave the toolbar acting on keys that are gone.
          props.table.resetRowSelection()
          await props.onDeleted()
        }}
      />
    </>
  )
}

/** The plaintext key, with the `sk-` prefix the relay expects. */
function fullKeyOf(token: OrganizationTokenRow): string {
  const key = (token.key ?? '').trim()
  if (!key) return ''
  return key.startsWith('sk-') ? key : `sk-${key}`
}
