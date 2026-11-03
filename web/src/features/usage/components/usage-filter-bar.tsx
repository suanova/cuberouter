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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  LogsFilterField,
  LogsFilterInput,
  LogsFilterToolbar,
} from '@/features/usage-logs/components/logs-filter-toolbar'

import type { SelfUsageRow, UsageOrganizationOption } from '../types'

export interface UsageFilterValues {
  model?: string
  token?: string
  billingAccountType?: string
  organizationId?: number
}

interface UsageFilterBarProps {
  table: Table<SelfUsageRow>
  initial: UsageFilterValues
  organizations: UsageOrganizationOption[]
  loading: boolean
  onApply: (values: UsageFilterValues) => void
  onReset: () => void
}

const BILLING_ALL = 'all'

export function UsageFilterBar(props: UsageFilterBarProps) {
  const { t } = useTranslation()
  const [model, setModel] = useState(props.initial.model ?? '')
  const [token, setToken] = useState(props.initial.token ?? '')
  const [billing, setBilling] = useState(
    props.initial.billingAccountType || BILLING_ALL
  )
  const [organizationId, setOrganizationId] = useState<number | undefined>(
    props.initial.organizationId
  )

  useEffect(() => {
    setModel(props.initial.model ?? '')
    setToken(props.initial.token ?? '')
    setBilling(props.initial.billingAccountType || BILLING_ALL)
    setOrganizationId(props.initial.organizationId)
  }, [
    props.initial.model,
    props.initial.token,
    props.initial.billingAccountType,
    props.initial.organizationId,
  ])

  const billingItems = [
    { value: BILLING_ALL, label: t('All') },
    { value: 'personal', label: t('Personal') },
    { value: 'organization', label: t('Organization') },
  ]
  const organizationItems = props.organizations.map((organization) => ({
    value: String(organization.id),
    label: organization.name,
  }))
  const hasActiveFilters =
    model.trim() !== '' ||
    token.trim() !== '' ||
    billing !== BILLING_ALL ||
    organizationId !== undefined

  const apply = () => {
    props.onApply({
      model: model.trim() || undefined,
      token: token.trim() || undefined,
      billingAccountType: billing === BILLING_ALL ? undefined : billing,
      organizationId:
        billing === 'organization' ? organizationId : undefined,
    })
  }

  const reset = () => {
    setModel('')
    setToken('')
    setBilling(BILLING_ALL)
    setOrganizationId(undefined)
    props.onReset()
  }

  return (
    <LogsFilterToolbar
      table={props.table}
      primaryFilters={
        <>
          <LogsFilterField>
            <LogsFilterInput
              aria-label={t('Model')}
              placeholder={t('Model')}
              value={model}
              onChange={(event) => setModel(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') apply()
              }}
            />
          </LogsFilterField>
          <LogsFilterField>
            <LogsFilterInput
              aria-label={t('API Key')}
              placeholder={t('API Key')}
              value={token}
              onChange={(event) => setToken(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') apply()
              }}
            />
          </LogsFilterField>
          <LogsFilterField>
            <Select
              items={billingItems}
              value={billing}
              onValueChange={(next) => {
                const value = next ?? BILLING_ALL
                setBilling(value)
                if (value !== 'organization') setOrganizationId(undefined)
              }}
            >
              <SelectTrigger size='sm' aria-label={t('Billing Account')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {billingItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </LogsFilterField>
          {billing === 'organization' ? (
            <LogsFilterField>
              <Select
                items={organizationItems}
                value={
                  organizationId === undefined
                    ? null
                    : String(organizationId)
                }
                onValueChange={(next) =>
                  setOrganizationId(next === null ? undefined : Number(next))
                }
              >
                <SelectTrigger size='sm' aria-label={t('Select organization')}>
                  <SelectValue placeholder={t('Select organization')} />
                </SelectTrigger>
                <SelectContent>
                  {organizationItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </LogsFilterField>
          ) : null}
        </>
      }
      hasActiveFilters={hasActiveFilters}
      searchLoading={props.loading}
      onReset={reset}
      onSearch={apply}
    />
  )
}
