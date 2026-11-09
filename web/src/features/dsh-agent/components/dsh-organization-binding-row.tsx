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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select'
import { copyToClipboard } from '@/lib/copy-to-clipboard'

import {
  type DshAdminChannel,
  getDshOrganizationConfig,
  updateDshOrganizationConfig,
} from '../admin-api'
import {
  DSH_ADMIN_QUERY_PREFIX,
  dshOrganizationConfigQueryKey,
} from '../admin-query-keys'
import { buildDshChannelChoices } from '@/features/system-settings/operations/dsh-agent-access'
import { DshOrganizationKeyEditor } from './dsh-organization-key-editor'

type DshOrganizationBindingRowProps = {
  organizationId: number
  organizationName: string
  channels: DshAdminChannel[]
  channelsLoading: boolean
  onBound: (organizationId: number) => void
}

/**
 * 一个已开通组织的绑定行：渠道（= 一个 dsh 容器）+ 组织 DeepSeek Key。
 * 渠道变更要显式保存；解绑后该组织的成员会立刻失去 DSH 访问。
 */
export function DshOrganizationBindingRow(
  props: DshOrganizationBindingRowProps
) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [channelDraft, setChannelDraft] = useState(0)

  const configQuery = useQuery({
    queryKey: dshOrganizationConfigQueryKey(props.organizationId),
    queryFn: () => getDshOrganizationConfig(props.organizationId),
  })
  const config = configQuery.data

  useEffect(() => {
    if (config) {
      setChannelDraft(config.channelId)
    }
  }, [config])

  const choices = useMemo(
    () =>
      buildDshChannelChoices(
        props.channels,
        props.organizationId,
        config?.channelId ?? 0
      ),
    [config?.channelId, props.channels, props.organizationId]
  )

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: DSH_ADMIN_QUERY_PREFIX })

  const saveMutation = useMutation({
    mutationFn: (channelId: number) =>
      updateDshOrganizationConfig(props.organizationId, { channelId }),
    onSuccess: async (saved) => {
      await refresh()
      if (saved.channelId > 0) {
        props.onBound(props.organizationId)
      }
      toast.success(t('DSH channel binding saved'))
    },
    onError: (error: Error) => {
      toast.error(t('Failed to save the DSH channel binding'), {
        description: error.message,
      })
    },
  })

  const channelLabel = (choice: (typeof choices)[number]) => {
    if (choice.id === 0) {
      return t('Not bound')
    }
    const name = choice.name || `#${choice.id}`
    switch (choice.reason) {
      case 'missing':
        return t('{{name}} (channel missing)', { name })
      case 'disabled':
        return t('{{name}} (disabled)', { name })
      case 'bound_elsewhere':
        return t('{{name}} (bound to another organization)', { name })
      default:
        return name
    }
  }

  const saving = saveMutation.isPending
  const channelDirty = config !== undefined && channelDraft !== config.channelId

  return (
    <div className='space-y-2 rounded-lg border p-3'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <p className='text-sm font-medium'>
          {props.organizationName}{' '}
          <span className='text-muted-foreground'>#{props.organizationId}</span>
        </p>
        {config?.organizationGroup && (
          <span className='text-muted-foreground text-xs'>
            {t('Organization group')}: {config.organizationGroup}
          </span>
        )}
      </div>
      {configQuery.isError && (
        <p role='alert' className='text-destructive text-sm'>
          {t('Failed to load the DSH organization')}
        </p>
      )}
      <div className='flex flex-wrap items-center gap-2'>
        <NativeSelect
          aria-label={t('DSH channel')}
          className='w-72'
          disabled={props.channelsLoading || configQuery.isLoading || saving}
          onChange={(event) => setChannelDraft(Number(event.target.value) || 0)}
          value={String(channelDraft)}
        >
          {choices.map((choice) => (
            <NativeSelectOption
              disabled={choice.disabled}
              key={choice.id}
              value={String(choice.id)}
            >
              {channelLabel(choice)}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        <Button
          disabled={!channelDirty || saving}
          onClick={() => saveMutation.mutate(channelDraft)}
          size='sm'
          variant='outline'
        >
          {t('Save binding')}
        </Button>
        {config?.suggestedBaseUrl && (
          <Button
            onClick={() => {
              void copyToClipboard(config.suggestedBaseUrl)
            }}
            size='sm'
            variant='ghost'
          >
            {t('Copy base URL')}: {config.suggestedBaseUrl}
          </Button>
        )}
      </div>
      {config && (
        <DshOrganizationKeyEditor
          channelId={config.channelId}
          configured={config.deepseekKeyConfigured}
          onSaved={refresh}
          organizationId={props.organizationId}
          preview={config.deepseekKeyPreview}
        />
      )}
      <p className='text-muted-foreground text-xs'>
        {t(
          'Members share this organization DeepSeek key. The channel Base URL must point at dsh-org-{{id}}; tasks are billed to the organization.',
          { id: props.organizationId }
        )}
      </p>
    </div>
  )
}
