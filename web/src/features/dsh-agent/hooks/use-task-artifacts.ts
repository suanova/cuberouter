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

import { fetchTaskArtifacts } from '../api'

/**
 * 本轮产物清单。产物卡片和回复里的文件链接共用同一个缓存键，两处各调一次也只会
 * 发一次请求；任务运行中或失败时没有产物，由调用方决定是否启用。
 */
export function useTaskArtifacts(taskId: string, enabled = true) {
  return useQuery({
    queryKey: ['dsh-agent', 'task-artifacts', taskId],
    queryFn: () => fetchTaskArtifacts(taskId),
    enabled: enabled && taskId !== '',
    retry: false,
    staleTime: 60_000,
  })
}
