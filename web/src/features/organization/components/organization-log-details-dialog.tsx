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

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import {
  BillingBreakdown,
  DetailRow,
  DetailSection,
} from '@/features/usage-logs/components/dialogs/details-dialog'
import {
  hasAnyCacheTokens,
  isViolationFeeLog,
  parseLogOther,
} from '@/features/usage-logs/lib/format'
import type { LogOtherData } from '@/features/usage-logs/types'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { formatLogQuota, formatTimestamp, formatUseTime } from '@/lib/format'

import {
  organizationLogModelInfo,
  organizationLogResponsibleName,
  organizationLogTokenName,
  organizationLogTypeMeta,
} from '../lib'
import type { OrganizationLogRow } from '../types'

type OrganizationLogDetailsDialogProps = {
  /** The entry to describe. `null` closes the dialog. */
  row: OrganizationLogRow | null
  onOpenChange: (open: boolean) => void
  /** The caller reads the whole organization's records, so the holder is named. */
  showResponsible: boolean
}

/**
 * Everything one organization log entry recorded.
 *
 * The list row is a summary; this is the record. The billing breakdown comes
 * from the shared usage-logs component so an organization's charges are
 * explained exactly the way a personal one is — but with `isAdmin` false: the
 * platform-side detail that component can render (node name, server IP, the
 * route the request took through the deployment) belongs to the operator of the
 * installation, not to an organization, however senior the reader is within it.
 */
export function OrganizationLogDetailsDialog(
  props: OrganizationLogDetailsDialogProps
) {
  const { t } = useTranslation()
  const row = props.row
  if (!row) {
    return null
  }

  const other = parseLogOther(row.other ?? '')
  const typeMeta = organizationLogTypeMeta(row.type)
  const model = organizationLogModelInfo(row)
  const isViolation = isViolationFeeLog(other)

  return (
    <Dialog
      open
      onOpenChange={props.onOpenChange}
      title={model.name || t('Log details')}
      description={
        <span className='flex flex-wrap items-center gap-1.5'>
          <StatusBadge
            label={t(typeMeta.labelKey)}
            variant={typeMeta.variant}
            copyable={false}
          />
          <span className='text-muted-foreground text-xs tabular-nums'>
            {formatTimestamp(row.created_at)}
          </span>
        </span>
      }
      contentClassName='sm:max-w-3xl'
    >
      <div className='max-h-[70vh] space-y-3 overflow-y-auto pr-1'>
        <DetailSection label={t('Request')}>
          <DetailRow
            label={t('Request ID')}
            value={
              <CopyableValue value={row.request_id} label={t('Request ID')} />
            }
            mono
          />
          <DetailRow
            label={t('Path')}
            value={other?.request_path || '-'}
            mono
          />
          <DetailRow label={t('Model')} value={model.name || '-'} mono />
          {model.actualModel ? (
            <DetailRow
              label={t('Upstream model')}
              value={model.actualModel}
              mono
            />
          ) : null}
          {props.showResponsible ? (
            <DetailRow
              label={t('Responsible user')}
              value={organizationLogResponsibleName(row)}
            />
          ) : null}
          <DetailRow
            label={t('Key')}
            value={organizationLogTokenName(row)}
            mono
          />
          <DetailRow label={t('Group')} value={row.group || '-'} mono />
          <DetailRow
            label={t('IP')}
            value={<CopyableValue value={row.ip} label={t('IP')} />}
            mono
          />
          <DetailRow
            label={t('Time')}
            value={formatTimestamp(row.created_at)}
          />
        </DetailSection>

        <DetailSection label={t('Usage')}>
          <DetailRow
            label={t('Input')}
            value={row.prompt_tokens.toLocaleString()}
            mono
          />
          <DetailRow
            label={t('Output')}
            value={row.completion_tokens.toLocaleString()}
            mono
          />
          {hasAnyCacheTokens(other) ? (
            <>
              <DetailRow
                label={t('Cache read')}
                value={(other?.cache_tokens || 0).toLocaleString()}
                mono
              />
              <DetailRow
                label={t('Cache write')}
                value={formatCacheWriteTokens(other)}
                mono
              />
            </>
          ) : null}
          {other?.reasoning_effort ? (
            <DetailRow
              label={t('Reasoning effort')}
              value={other.reasoning_effort}
            />
          ) : null}
          <DetailRow
            label={t('Duration')}
            value={formatUseTime(row.use_time)}
            mono
          />
          <DetailRow
            label={t('Stream')}
            value={row.is_stream ? t('Yes') : t('No')}
          />
        </DetailSection>

        {isViolation ? (
          <DetailSection label={t('Violation fee')}>
            <DetailRow
              label={t('Code')}
              value={other?.violation_fee_code || other?.violation_fee_marker || '-'}
              mono
            />
            <DetailRow
              label={t('Cost')}
              value={formatLogQuota(other?.fee_quota ?? row.quota)}
              mono
            />
          </DetailSection>
        ) : null}

        {!isViolation && row.type === 2 && other ? (
          <BillingBreakdown
            quota={row.quota}
            other={other}
            isAdmin={false}
          />
        ) : null}

        {other?.billing_source === 'subscription' ? (
          <DetailSection label={t('Subscription deduction')}>
            <DetailRow
              label={t('Plan')}
              value={
                other.subscription_plan_id
                  ? `#${other.subscription_plan_id} ${other.subscription_plan_title ?? ''}`.trim()
                  : '-'
              }
            />
            <DetailRow
              label={t('Instance')}
              value={other.subscription_id ? `#${other.subscription_id}` : '-'}
              mono
            />
            <DetailRow
              label={t('Pre-consumed')}
              value={(other.subscription_pre_consumed ?? 0).toString()}
              mono
            />
            <DetailRow
              label={t('Settlement delta')}
              value={formatSigned(other.subscription_post_delta ?? 0)}
              mono
            />
            <DetailRow
              label={t('Final consumed')}
              value={(
                other.subscription_consumed ??
                (other.subscription_pre_consumed ?? 0) +
                  (other.subscription_post_delta ?? 0)
              ).toString()}
              mono
            />
            {other.subscription_remain !== undefined &&
            other.subscription_total !== undefined ? (
              <DetailRow
                label={t('Remaining')}
                value={`${other.subscription_remain}/${other.subscription_total}`}
                mono
              />
            ) : null}
          </DetailSection>
        ) : null}
      </div>
    </Dialog>
  )
}

/**
 * Cache-write tokens, as one number.
 *
 * A record may carry the 5-minute and 1-hour buckets separately, in which case
 * the plain total is absent and the two are what there is. Otherwise the single
 * field is the answer.
 */
function formatCacheWriteTokens(other: LogOtherData | null): string {
  const fiveMinutes = other?.cache_creation_tokens_5m || 0
  const oneHour = other?.cache_creation_tokens_1h || 0
  if (fiveMinutes > 0 || oneHour > 0) {
    return (fiveMinutes + oneHour).toLocaleString()
  }
  return (other?.cache_creation_tokens || 0).toLocaleString()
}

function formatSigned(value: number): string {
  return value > 0 ? `+${value}` : value.toString()
}

/**
 * A value worth copying, and copyable.
 *
 * Request ids and addresses are read to be pasted somewhere else — into a
 * support thread, a firewall rule — so the click copies rather than selects.
 */
function CopyableValue(props: { value?: string; label: string }) {
  const { t } = useTranslation()
  if (!props.value) {
    return '-'
  }
  return (
    <button
      type='button'
      className='hover:text-foreground max-w-full truncate text-left underline decoration-dotted underline-offset-2'
      title={t('Copy')}
      onClick={() => void copyToClipboard(props.value ?? '')}
    >
      {props.value}
    </button>
  )
}
