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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { useAuthStore } from '@/stores/auth-store'

import { BasicMediaStudio } from './basic-studio'
import { workflowAPI } from './workflow-api'
import { WorkflowStudio } from './workflow-studio'
import type { WorkflowConfig } from './workflow-types'

const disconnectedConfig: WorkflowConfig = {
  enabled: false,
  models: {
    create: 'qwen-image-2512',
    edit: 'qwen-image-edit-2511',
    regional: 'qwen-image-edit-2511',
  },
  health: {
    create: 'Not connected',
    edit: 'Not connected',
    tools: 'Not connected',
  },
  retention_days: 30,
}

export function MediaStudio() {
  const { t } = useTranslation()
  const owner = useAuthStore((state) => state.auth.user?.id)
  const [basic, setBasic] = useState(false)
  const config = useQuery({
    queryKey: ['media-studio', owner, 'config'],
    queryFn: workflowAPI.config,
    retry: 1,
    staleTime: 30000,
    refetchInterval: 30000,
  })
  if (config.isPending) {
    return (
      <div role='status' className='text-muted-foreground p-8 text-sm'>
        {t('Loading image studio…')}
      </div>
    )
  }
  if (config.error) {
    return (
      <div className='space-y-4 p-8'>
        <h1 className='text-lg font-semibold'>{t('Media Studio')}</h1>
        <p role='alert' className='text-sm'>
          {t(
            'Image studio is unavailable. Your saved work remains on the server.'
          )}
        </p>
        <Button variant='outline' onClick={() => void config.refetch()}>
          {t('Reconnect')}
        </Button>
      </div>
    )
  }
  if (config.data.enabled === false) {
    if (basic) {
      return (
        <div className='flex min-h-0 flex-1 flex-col'>
          <div className='px-6 pt-4'>
            <Button variant='outline' onClick={() => setBasic(false)}>
              {t('Back to image studio')}
            </Button>
          </div>
          <BasicMediaStudio />
        </div>
      )
    }
    return (
      <WorkflowStudio
        key={`${owner}-disconnected`}
        config={disconnectedConfig}
        onBasic={() => setBasic(true)}
        onReconnect={() => void config.refetch()}
      />
    )
  }
  return <WorkflowStudio key={owner} config={config.data} />
}
