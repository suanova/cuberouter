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
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { MultiSelect } from '@/components/multi-select'
import { listDshAdminChannels } from '@/features/dsh-agent/admin-api'
import { DshOrganizationBindingRow } from '@/features/dsh-agent/components/dsh-organization-binding-row'
import { listPlatformOrganizations } from '@/features/organization/api'
import type { OrganizationManagementView } from '@/features/organization/types'

import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  mergeDshOrganizationSelection,
  normalizeDshOrganizationIds,
} from './dsh-agent-access'

/** The platform list endpoint caps page_size at 100. */
const ORGANIZATIONS_PAGE_SIZE = 100
/** The backend rejects allowlists longer than 1000 entries. */
const ORGANIZATIONS_PAGE_LIMIT = 10

type DshAgentAccessSectionProps = {
  defaultValues: {
    DshAllowedOrganizationIds: number[]
  }
}

async function listActiveOrganizations(): Promise<
  OrganizationManagementView[]
> {
  const organizations: OrganizationManagementView[] = []
  for (let page = 1; page <= ORGANIZATIONS_PAGE_LIMIT; page += 1) {
    const response = await listPlatformOrganizations({
      p: page,
      page_size: ORGANIZATIONS_PAGE_SIZE,
      status: 'active',
    })
    if (!response.success) {
      throw new Error(response.message || 'Failed to load organizations')
    }
    const items = response.data?.items ?? []
    organizations.push(...items)
    const total = response.data?.total ?? organizations.length
    if (organizations.length >= total || items.length === 0) {
      break
    }
  }
  return organizations
}

export function DshAgentAccessSection(props: DshAgentAccessSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const baselineIds = useMemo(
    () =>
      normalizeDshOrganizationIds(props.defaultValues.DshAllowedOrganizationIds),
    [props.defaultValues]
  )
  const baselineRef = useRef(baselineIds)
  const [selected, setSelected] = useState<string[]>(() =>
    baselineIds.map(String)
  )

  useEffect(() => {
    baselineRef.current = baselineIds
    setSelected(baselineIds.map(String))
  }, [baselineIds])

  const organizationsQuery = useQuery({
    queryKey: ['system-settings', 'dsh-agent', 'active-organizations'],
    queryFn: listActiveOrganizations,
    staleTime: 5 * 60 * 1000,
  })

  // 渠道下拉的数据源：绑定行与白名单共用一次请求，绑定时 bound_organization_id
  // 会变，因此保存后统一失效 ['system-settings','dsh-agent'] 前缀。
  const channelsQuery = useQuery({
    queryKey: ['system-settings', 'dsh-agent', 'channels'],
    queryFn: listDshAdminChannels,
    staleTime: 60 * 1000,
  })

  // Organizations that were allowlisted but are no longer active still need a
  // chip so an administrator can see and remove them.
  const options = useMemo(() => {
    const activeOptions = (organizationsQuery.data ?? []).map((organization) => ({
      label: organization.name,
      value: String(organization.id),
    }))
    const knownValues = new Set(activeOptions.map((option) => option.value))
    const missingOptions = selected
      .filter((value) => !knownValues.has(value))
      .map((value) => ({ label: `#${value}`, value }))
    return [...activeOptions, ...missingOptions]
  }, [organizationsQuery.data, selected])

  const organizationNames = useMemo(() => {
    const names = new Map<number, string>()
    for (const organization of organizationsQuery.data ?? []) {
      names.set(organization.id, organization.name)
    }
    return names
  }, [organizationsQuery.data])

  const selectedIds = useMemo(
    () => normalizeDshOrganizationIds(selected),
    [selected]
  )

  // 绑定成功后服务端会自动把组织加入白名单；本地选中态跟着补上，避免下次
  // 保存白名单时把它又删掉。
  const handleBound = (organizationId: number) => {
    setSelected((prev) => mergeDshOrganizationSelection(prev, organizationId))
  }

  const handleSave = async () => {
    const ids = normalizeDshOrganizationIds(selected)
    if (JSON.stringify(ids) === JSON.stringify(baselineRef.current)) {
      toast.info(t('No changes to save'))
      return
    }

    await updateOption.mutateAsync({
      key: 'DshAllowedOrganizationIds',
      value: JSON.stringify(ids),
    })
    baselineRef.current = ids
  }

  return (
    <SettingsSection title={t('DSH Agent Access')}>
      <SettingsForm
        onSubmit={(event) => {
          event.preventDefault()
          void handleSave()
        }}
      >
        <SettingsPageFormActions
          onSave={() => void handleSave()}
          isSaving={updateOption.isPending}
          saveLabel='Save DSH Agent access'
        />
        <div className='flex flex-col gap-2'>
          <label htmlFor='dsh-agent-allowed-organizations' className='text-sm font-medium'>
            {t('Allowed Organizations')}
          </label>
          <MultiSelect
            id='dsh-agent-allowed-organizations'
            options={options}
            selected={selected}
            onChange={setSelected}
            placeholder={t('Select organizations...')}
            emptyText={t('No organizations available')}
            disabled={organizationsQuery.isLoading}
          />
          <p className='text-muted-foreground text-sm'>
            {t(
              'Members of an enabled organization may use the DSH Agent. An organization is enabled only when it is selected here and bound to a DSH channel. Personal tokens cannot call the DSH task API.'
            )}
          </p>
          <p className='text-muted-foreground text-sm'>
            {t(
              'DSH tasks are billed to the organization, not to the personal wallet. Everyone, including platform administrators, must be an active member of an enabled organization.'
            )}
          </p>
          {selectedIds.length > 0 && (
            <div className='flex flex-col gap-2'>
              {selectedIds.map((organizationId) => (
                <DshOrganizationBindingRow
                  channels={channelsQuery.data ?? []}
                  channelsLoading={channelsQuery.isLoading}
                  key={organizationId}
                  onBound={handleBound}
                  organizationId={organizationId}
                  organizationName={
                    organizationNames.get(organizationId) ?? ''
                  }
                />
              ))}
            </div>
          )}
          {channelsQuery.isError && (
            <p role='alert' className='text-destructive text-sm'>
              {t('Failed to load DSH channels')}
            </p>
          )}
          {organizationsQuery.isError && (
            <p role='alert' className='text-destructive text-sm'>
              {t('Failed to load organizations')}
            </p>
          )}
        </div>
      </SettingsForm>
    </SettingsSection>
  )
}
