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

import { formatQuota } from '@/lib/format'

import type { OrganizationInviteRow } from '../types'

function parseInviteModels(raw?: string | null): string[] | null {
  if (raw === null || raw === undefined) {
    return null
  }
  if (raw.trim() === '') {
    return []
  }
  try {
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) {
      return null
    }
    return parsed.filter((entry): entry is string => typeof entry === 'string')
  } catch {
    return null
  }
}

/**
 * The restrictions an invitation carries, in the compact form the list can
 * show: an unconfigured field keeps the member's existing setting on accept,
 * while an explicit empty list / zero clears it, and the two must not read the
 * same.
 */
export function OrganizationInviteRestrictionsCell(props: {
  invite: Pick<OrganizationInviteRow, 'model_access' | 'quota_limit'>
}) {
  const { t } = useTranslation()
  const models = parseInviteModels(props.invite.model_access)
  const quota = props.invite.quota_limit ?? null

  if (models === null && quota === null) {
    return (
      <span className='text-muted-foreground text-sm'>
        {t('Not configured')}
      </span>
    )
  }

  let modelsText: string
  if (models === null) {
    modelsText = t('Models unchanged')
  } else if (models.length === 0) {
    modelsText = t('All models')
  } else {
    modelsText = t('{{count}} model(s)', { count: models.length })
  }

  let quotaText: string
  if (quota === null) {
    quotaText = t('Quota unchanged')
  } else if (quota === 0) {
    quotaText = t('No limit')
  } else {
    quotaText = formatQuota(quota)
  }

  return (
    <span className='text-sm whitespace-nowrap'>
      {modelsText} · {quotaText}
    </span>
  )
}
