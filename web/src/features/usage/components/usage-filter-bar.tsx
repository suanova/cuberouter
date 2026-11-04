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
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Combobox } from '@/components/ui/combobox'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  LogsFilterField,
  LogsFilterToolbar,
} from '@/features/usage-logs/components/logs-filter-toolbar'

import type {
  SelfUsageRow,
  SelfUsageTokenOption,
  UsageOrganizationOption,
  UsageTableVariant,
  UsageUserOption,
} from '../types'

export interface UsageFilterValues {
  model?: string
  token?: string
  tokenId?: number
  billingAccountType?: string
  organizationId?: number
  userId?: number
}

interface UsageFilterBarProps {
  table: Table<SelfUsageRow>
  initial: UsageFilterValues
  organizations: UsageOrganizationOption[]
  modelOptions: string[]
  tokenOptions: SelfUsageTokenOption[]
  userOptions?: UsageUserOption[]
  variant?: UsageTableVariant
  loading: boolean
  actionStart?: ReactNode
  onApply: (values: UsageFilterValues) => void
  onReset: () => void
}

const BILLING_ALL = 'all'

// 同名密钥可能分属个人与组织，下拉值用密钥 id，应用时再带上名称。
function tokenOptionValue(option: SelfUsageTokenOption): string {
  return String(option.id)
}

function tokenOptionLabel(option: SelfUsageTokenOption): string {
  return option.organization_id > 0
    ? `${option.name} (${option.organization_name})`
    : option.name
}

function initialTokenOptionValue(
  options: SelfUsageTokenOption[],
  tokenId?: number,
  tokenName?: string
): string {
  if (tokenId) {
    const byId = options.find((option) => option.id === tokenId)
    if (byId) return tokenOptionValue(byId)
  }
  if (tokenName) {
    const byName = options.find((option) => option.name === tokenName)
    if (byName) return tokenOptionValue(byName)
    return tokenName
  }
  if (tokenId) return String(tokenId)
  return ''
}

export function UsageFilterBar(props: UsageFilterBarProps) {
  const { t } = useTranslation()
  const variant = props.variant ?? 'personal'
  const [model, setModel] = useState(props.initial.model ?? '')
  const [token, setToken] = useState(
    initialTokenOptionValue(
      props.tokenOptions,
      props.initial.tokenId,
      props.initial.token
    )
  )
  const [billing, setBilling] = useState(
    props.initial.billingAccountType || BILLING_ALL
  )
  const [organizationId, setOrganizationId] = useState<number | undefined>(
    props.initial.organizationId
  )
  const [userId, setUserId] = useState(
    props.initial.userId ? String(props.initial.userId) : ''
  )
  const appliedTokenKeyRef = useRef('')

  useEffect(() => {
    setModel(props.initial.model ?? '')
    setBilling(props.initial.billingAccountType || BILLING_ALL)
    setOrganizationId(props.initial.organizationId)
    setUserId(props.initial.userId ? String(props.initial.userId) : '')
  }, [
    props.initial.model,
    props.initial.billingAccountType,
    props.initial.organizationId,
    props.initial.userId,
  ])

  // 只在「已应用的密钥」变化时重置草稿；选项随日期刷新时不能清掉用户未应用的编辑。
  useEffect(() => {
    const appliedKey = `${props.initial.tokenId ?? ''}|${props.initial.token ?? ''}`
    if (appliedTokenKeyRef.current === appliedKey) return
    if (appliedKey !== '|' && props.tokenOptions.length === 0) return
    appliedTokenKeyRef.current = appliedKey
    setToken(
      initialTokenOptionValue(
        props.tokenOptions,
        props.initial.tokenId,
        props.initial.token
      )
    )
  }, [props.initial.tokenId, props.initial.token, props.tokenOptions])

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
  const userItems = [
    { value: '', label: t('All Users') },
    ...(props.userOptions ?? []).map((option) => ({
      value: String(option.id),
      label: option.name,
    })),
  ]
  const hasActiveFilters =
    model.trim() !== '' ||
    token.trim() !== '' ||
    billing !== BILLING_ALL ||
    organizationId !== undefined ||
    userId !== ''

  const selectedToken = props.tokenOptions.find(
    (option) => String(option.id) === token
  )
  const apply = () => {
    props.onApply({
      model: model.trim() || undefined,
      token: selectedToken?.name ?? (token.trim() || undefined),
      tokenId: selectedToken?.id,
      billingAccountType:
        variant === 'personal' && billing !== BILLING_ALL ? billing : undefined,
      organizationId:
        variant === 'personal' && billing === 'organization'
          ? organizationId
          : undefined,
      userId: userId === '' ? undefined : Number(userId),
    })
  }

  const reset = () => {
    setModel('')
    setToken('')
    setBilling(BILLING_ALL)
    setOrganizationId(undefined)
    setUserId('')
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
          {variant === 'organization' ? (
            <LogsFilterField>
              <Combobox
                options={userItems}
                value={userId}
                onValueChange={(value) => setUserId(value ?? '')}
                placeholder={t('User')}
                ariaLabel={t('User')}
                className='h-8 min-w-0 text-sm leading-5'
              />
            </LogsFilterField>
          ) : (
            <>
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
                      setOrganizationId(
                        next === null ? undefined : Number(next)
                      )
                    }
                  >
                    <SelectTrigger
                      size='sm'
                      aria-label={t('Select organization')}
                    >
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
          )}
        </>
      }
      hasActiveFilters={hasActiveFilters}
      searchLoading={props.loading}
      actionStart={props.actionStart}
      onReset={reset}
      onSearch={apply}
    />
  )
}
