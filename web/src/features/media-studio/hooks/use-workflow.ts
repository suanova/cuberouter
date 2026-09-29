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
import { useEffect, useRef, useState } from 'react'

import {
  collectQwenJob,
  pendingJob,
  releaseQwenResult,
  submitQwenJob,
  type PendingImageJob,
  type QueueStatus,
} from '../lib/qwen-queue'
import {
  deleteStudioJob,
  listStudioJobs,
  saveStudioJob,
} from '../lib/studio-storage'
import { workflowError } from '../lib/workflow'
import { workflowAPI } from '../workflow-api'
import type { WorkflowDraft, WorkflowJob } from '../workflow-types'

export function useWorkflow(owner: number) {
  const client = useQueryClient()
  const key = ['studio-local-history', owner]
  const history = useQuery({
    queryKey: key,
    queryFn: () => listStudioJobs(owner),
    retry: false,
  })
  const [selected, setSelected] = useState<WorkflowJob>()
  const [warning, setWarning] = useState('')
  const [elapsed, setElapsed] = useState(0)
  const started = useRef(0)
  const abort = useRef<AbortController | null>(null)
  const [queueStatus, setQueueStatus] = useState<QueueStatus>()
  const [pending, setPending] = useState<PendingImageJob>()
  const [recovering, setRecovering] = useState(true)
  const generation = useMutation({
    mutationFn: async (draft: WorkflowDraft & { resume?: PendingImageJob }) => {
      started.current = Date.now()
      setElapsed(0)
      setWarning('')
      setSelected(undefined)
      setQueueStatus(undefined)
      abort.current?.abort()
      abort.current = new AbortController()
      let output: { job: WorkflowJob; warning?: string }
      if (draft.queued) {
        const record = draft.resume ?? (await submitQwenJob(owner, draft))
        setPending(record)
        output = {
          job: await collectQwenJob(
            record,
            abort.current.signal,
            setQueueStatus
          ),
        }
      } else output = await workflowAPI.generate(draft)
      let notice = output.warning ?? ''
      try {
        await saveStudioJob(owner, output.job)
        if (draft.queued) {
          await releaseQwenResult(owner, output.job.id)
          setPending(undefined)
        }
      } catch (error) {
        notice = workflowError(error)
      }
      return { ...output, notice }
    },
    retry: false,
    onSuccess: (output) => {
      setSelected(output.job)
      setWarning(output.notice)
      void client.invalidateQueries({ queryKey: key })
    },
    onError: () => {
      void pendingJob(owner)
        .then(setPending)
        .catch(() => undefined)
    },
  })
  const mutate = useRef(generation.mutate)
  mutate.current = generation.mutate
  useEffect(() => {
    let active = true
    void pendingJob(owner)
      .then((record) => {
        if (!active || !record) return
        setPending(record)
        mutate.current({ ...record.draft, resume: record })
      })
      .catch(() => undefined)
      .finally(() => {
        if (active) setRecovering(false)
      })
    return () => {
      active = false
      abort.current?.abort()
    }
  }, [owner])
  useEffect(() => {
    if (!generation.isPending) return
    const timer = setInterval(
      () => setElapsed(Date.now() - started.current),
      1000
    )
    return () => clearInterval(timer)
  }, [generation.isPending])
  const deletion = useMutation({
    mutationFn: (id?: string) => deleteStudioJob(owner, id),
    onSuccess: (_, id) => {
      if (!id || selected?.id === id) setSelected(undefined)
      void client.invalidateQueries({ queryKey: key })
    },
  })
  return {
    generation,
    deletion,
    history,
    selected,
    warning,
    elapsed,
    queueStatus,
    pending,
    recovering,
    reconnect: () => {
      if (pending) generation.mutate({ ...pending.draft, resume: pending })
    },
    clearEndedRequest: async () => {
      // This only removes a local recovery record; the UI enables it after terminal status.
      await pendingJob(owner, null)
      setPending(undefined)
      setQueueStatus(undefined)
      generation.reset()
    },
    select: (id: string) =>
      setSelected(history.data?.find((job) => job.id === id)),
  }
}
