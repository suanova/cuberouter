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
import { Combobox } from '@/components/ui/combobox'
import {
  LogsFilterField,
  LogsFilterToolbar,
} from '@/features/usage-logs/components/logs-filter-toolbar'

import type {
  SelfUsageRow,
  SelfUsageTokenOption,
  UsageOrganizationOption,
} from '../types'

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
  modelOptions: string[]
  tokenOptions: SelfUsageTokenOption[]
  loading: boolean
  onApply: (values: UsageFilterValues) => void
  onReset: () => void
}

const BILLING_ALL = 'all'

// The token filter stores the key name, but the dropdown needs unique values
// because two organizations may use the same key name. The organization id is
// appended for org keys and stripped again on apply.
const TOKEN_OPTION_SEPARATOR = '\u0000'

function tokenOptionValue(option: SelfUsageTokenOption): string {
  return option.organization_id > 0
    ? `${option.name}${TOKEN_OPTION_SEPARATOR}${option.organization_id}`
    : option.name
}

function tokenOptionLabel(option: SelfUsageTokenOption): string {
  return option.organization_id > 0
    ? `${option.name} (${option.organization_name})`
    : option.name
}

function tokenNameFromOptionValue(value: string): string {
  return value.split(TOKEN_OPTION_SEPARATOR)[0]
}

function initialTokenOptionValue(
  options: SelfUsageTokenOption[],
  name?: string
): string {
  if (!name) return ''
  const match = options.find((option) => option.name === name)
  return match ? tokenOptionValue(match) : name
}

export function UsageFilterBar(props: UsageFilterBarProps) {
  const { t } = useTranslation()
  const [model, setModel] = useState(props.initial.model ?? '')
  const [token, setToken] = useState(
    initialTokenOptionValue(props.tokenOptions, props.initial.token)
  )
  const [billing, setBilling] = useState(
    props.initial.billingAccountType || BILLING_ALL
  )
  const [organizationId, setOrganizationId] = useState<number | undefined>(
    props.initial.organizationId
  )

  useEffect(() => {
    setModel(props.initial.model ?? '')
    setToken(initialTokenOptionValue(props.tokenOptions, props.initial.token))
    setBilling(props.initial.billingAccountType || BILLING_ALL)
    setOrganizationId(props.initial.organizationId)
  }, [
    props.initial.model,
    props.initial.token,
    props.initial.billingAccountType,
    props.initial.organizationId,
    props.tokenOptions,
  ])

  const billingItems = [
    { value: BILLING_ALL, label: t('All Billing Types') },
    { value: 'personal', label: t('Personal') },
    { value: 'organization', label: t('Organization') },
  ]
  const modelItems = [
    { value: '', label: t('All Models') },
    ...props.modelOptions.map((name) => ({ value: name, label: name })),
  ]
  const tokenItems = [
    { value: '', label: t('All API tokens') },
    ...props.tokenOptions.map((option) => ({
      value: tokenOptionValue(option),
      label: tokenOptionLabel(option),
    })),
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
      token: tokenNameFromOptionValue(token).trim() || undefined,
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
            <Combobox
              options={modelItems}
              value={model}
              onValueChange={(value) => setModel(value ?? '')}
              placeholder={t('Model')}
              ariaLabel={t('Model')}
              className='h-8 min-w-0 text-sm leading-5'
            />
          </LogsFilterField>
          <LogsFilterField>
            <Combobox
              options={tokenItems}
              value={token}
              onValueChange={(value) => setToken(value ?? '')}
              placeholder={t('API Key')}
              ariaLabel={t('API Key')}
              className='h-8 min-w-0 text-sm leading-5'
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
