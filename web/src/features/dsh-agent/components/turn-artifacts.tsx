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
import {
  Download01Icon,
  File01Icon,
  Image01Icon,
  MusicNote01Icon,
  Video01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import type { TaskArtifactType } from '@/features/usage-logs/types'

import { fetchTaskArtifacts } from '../api'

type TurnArtifactsProps = {
  taskId: string
}

function artifactIcon(type: TaskArtifactType) {
  switch (type) {
    case 'image':
      return Image01Icon
    case 'video':
      return Video01Icon
    case 'audio':
      return MusicNote01Icon
    case 'file':
      return File01Icon
  }
}

/**
 * 本轮产物。dsh 把智能体写出的文件列在任务上，网关为每个文件签发下载地址；
 * 这里只在任务结算后查询一次，失败或没有产物时整块不占位。
 */
export function TurnArtifacts(props: TurnArtifactsProps) {
  const { t } = useTranslation()

  const artifactsQuery = useQuery({
    queryKey: ['dsh-agent', 'task-artifacts', props.taskId],
    queryFn: () => fetchTaskArtifacts(props.taskId),
    enabled: props.taskId !== '',
    retry: false,
    staleTime: 60_000,
  })

  const artifacts = artifactsQuery.data?.artifacts ?? []
  if (artifacts.length === 0) {
    return null
  }

  return (
    <section
      aria-label={t('Files produced by this turn')}
      className='w-full rounded-md border px-3 py-2'
    >
      <p className='text-muted-foreground text-xs'>
        {t('Files produced by this turn')}
      </p>
      <ul className='mt-1 space-y-1'>
        {artifacts.map((artifact) => (
          <li key={artifact.key}>
            <a
              aria-label={t('Download {{name}}', { name: artifact.key })}
              className='hover:bg-muted focus-visible:ring-ring flex items-center gap-2 rounded-md px-1 py-1 text-sm focus-visible:ring-2 focus-visible:outline-none'
              download={artifact.key}
              href={artifact.content_url}
              rel='noopener noreferrer'
              target='_blank'
            >
              <HugeiconsIcon
                aria-hidden='true'
                className='text-muted-foreground size-4 shrink-0'
                icon={artifactIcon(artifact.type)}
              />
              <span className='truncate font-mono text-xs'>{artifact.key}</span>
              <HugeiconsIcon
                aria-hidden='true'
                className='text-muted-foreground ml-auto size-4 shrink-0'
                icon={Download01Icon}
              />
            </a>
          </li>
        ))}
      </ul>
    </section>
  )
}
