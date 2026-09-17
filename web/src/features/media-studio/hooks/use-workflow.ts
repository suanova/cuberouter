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
import { useState } from 'react'

import { useAuthStore } from '@/stores/auth-store'

import { isActiveJob } from '../lib/workflow'
import { workflowAPI } from '../workflow-api'
import type { WorkflowDraft } from '../workflow-types'

export function useWorkflow() {
  const owner = useAuthStore((state) => state.auth.user?.id)
  const queryClient = useQueryClient()
  const key = ['media-studio', owner, 'jobs']
  const [selectedId, setSelectedId] = useState<string>()
  const history = useQuery({
    queryKey: key,
    queryFn: workflowAPI.jobs,
    refetchInterval: (query) =>
      query.state.data?.some(isActiveJob) ? 3000 : 15000,
    retry: 1,
  })
  const refresh = () => queryClient.invalidateQueries({ queryKey: key })
  const generation = useMutation({
    mutationFn: async (draft: WorkflowDraft) => {
      const prepared = await workflowAPI.prepare(draft)
      setSelectedId(prepared.job.id)
      void refresh()
      // Never retry this mutation: a timeout does not mean the GPU did not run.
      await workflowAPI.relay(prepared.relay_body)
    },
    retry: false,
    onSettled: refresh,
  })
  const deletion = useMutation({
    mutationFn: workflowAPI.remove,
    onSuccess: refresh,
  })
  const jobs = history.data ?? []
  const selected = jobs.find((job) => job.id === selectedId) ?? jobs[0]
  return {
    history,
    jobs,
    selected,
    select: setSelectedId,
    generation,
    deletion,
    refresh,
    busy: generation.isPending || jobs.some(isActiveJob),
  }
}
