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

import { api } from '@/lib/api'

import {
  acknowledgeStudioImage,
  cancelStudioImage,
  collectStudioImage,
  pendingStudioImage,
  submitStudioImage,
  type ImageJobStatus,
  type ImageQueueConfig,
  type PendingStudioImage,
} from '../lib/image-jobs'
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
  const storageBudget = useQuery({
    queryKey: ['studio-browser-budget', owner],
    queryFn: async () => {
      try {
        const estimate = await navigator.storage?.estimate?.()
        return (
          estimate?.quota !== undefined &&
          estimate.quota - (estimate.usage ?? 0) < 50 * 1024 * 1024
        )
      } catch {
        return false
      }
    },
    retry: false,
  })
  const configuration = useQuery({
    queryKey: ['studio-image-configuration', owner],
    queryFn: async () =>
      (
        await api.get<ImageQueueConfig>('/api/media-studio/config', {
          skipErrorHandler: true,
        })
      ).data,
    retry: false,
  })
  const [pending, setPending] = useState<PendingStudioImage>()
  const [queueStatus, setQueueStatus] = useState<ImageJobStatus>()
  const [recovering, setRecovering] = useState(true)
  const abort = useRef<AbortController | null>(null)
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
  const generation = useMutation({
    mutationFn: async (
      draft: WorkflowDraft & { resume?: PendingStudioImage }
    ) => {
      let config: ImageQueueConfig | undefined
      if (!draft.resume) {
        const latest = await configuration.refetch()
        if (!latest.isSuccess || !latest.data) {
          throw new Error(
            'Image Studio configuration is unavailable. Please try again.'
          )
        }
        config = latest.data
      }
      started.current = Date.now()
      setElapsed(0)
      setWarning('')
      setSelected(undefined)
      abort.current?.abort()
      abort.current = new AbortController()
      const queued =
        !!draft.resume ||
        !!(
          config?.image_queue_enabled &&
          config.image_queue_models?.includes(draft.model)
        )
      let output: { job: WorkflowJob; warning?: string }
      let pendingRecord: PendingStudioImage | undefined
      if (queued) {
        const record = draft.resume ?? (await submitStudioImage(owner, draft))
        setPending(record)
        pendingRecord = record
        output = {
          job: await collectStudioImage(
            owner,
            record,
            abort.current.signal,
            setQueueStatus
          ),
        }
      } else output = await workflowAPI.generate(draft)
      let notice = output.warning ?? ''
      let recoveryChanged = false
      try {
        recoveryChanged =
          (await saveStudioJob(owner, output.job, pendingRecord?.key)) === false
        if (!recoveryChanged && pendingRecord) {
          await acknowledgeStudioImage(owner, output.job.id, pendingRecord.key)
          setPending(undefined)
          setQueueStatus(undefined)
        }
      } catch (error) {
        notice = workflowError(error)
      }
      if (recoveryChanged) {
        throw new Error(
          'The image job could not complete. Your draft has been kept.'
        )
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
      void pendingStudioImage(owner)
        .then(setPending)
        .catch(() => undefined)
    },
  })
  const resume = useRef(generation.mutate)
  resume.current = generation.mutate
  useEffect(() => {
    let active = true
    void pendingStudioImage(owner)
      .then((record) => {
        if (active && record) {
          setPending(record)
          resume.current({ ...record.draft, resume: record })
        }
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
  const cancellation = useMutation({
    mutationFn: async () => {
      if (pending && queueStatus) {
        await cancelStudioImage(owner, pending, queueStatus)
      }
    },
    retry: false,
  })
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
    configuration,
    storageLimited: storageBudget.data === true,
    pending,
    queueStatus,
    recovering,
    cancellation,
    reconnect: () => {
      if (pending) generation.mutate({ ...pending.draft, resume: pending })
    },
    clearPending: async () => {
      if (pending) {
        await pendingStudioImage(owner, null, { expectedKey: pending.key })
      }
      setPending(undefined)
      setQueueStatus(undefined)
      generation.reset()
    },
    select: (id: string) =>
      setSelected(history.data?.find((job) => job.id === id)),
  }
}
