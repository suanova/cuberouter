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
  downloadVideo,
  fetchVideoTask,
  newVideoJobId,
  submitVideoTask,
} from '../video-api'
import {
  clearPendingVideoTask,
  loadPendingVideoTask,
  savePendingVideoTask,
} from '../lib/video-pending'
import {
  deleteVideoJob,
  listVideoJobs,
  saveVideoJob,
} from '../lib/video-storage'
import { videoError } from '../lib/video-draft'
import { workflowError } from '../lib/workflow'
import { VIDEO_POLL_INTERVAL_MS } from '../constants'
import type { VideoDraft, VideoJob } from '../video-types'

export function useVideo(owner: number) {
  const client = useQueryClient()
  const history = useQuery({
    queryKey: ['video-local-history', owner],
    queryFn: () => listVideoJobs(owner),
    retry: false,
  })
  const [selected, setSelected] = useState<VideoJob>()
  const [warning, setWarning] = useState('')
  const [elapsed, setElapsed] = useState(0)
  const [activeDraft, setActiveDraft] = useState<VideoDraft>()
  const [activeTaskId, setActiveTaskId] = useState<string>()
  const started = useRef(0)
  const processedTaskId = useRef<string | undefined>(undefined)

  const submission = useMutation({
    mutationFn: (draft: VideoDraft) => {
      started.current = Date.now()
      processedTaskId.current = undefined
      setElapsed(0)
      setWarning('')
      setSelected(undefined)
      setActiveDraft(draft)
      return submitVideoTask(draft)
    },
    // 持久化 task_id：页面中途离开后重开可恢复轮询（按账号隔离）。
    onSuccess: (taskId, draft) => {
      savePendingVideoTask(owner, {
        task_id: taskId,
        draft,
        started_at: started.current,
      })
      setActiveTaskId(taskId)
    },
    retry: false,
  })

  const task = useQuery({
    queryKey: ['video-task', owner, activeTaskId],
    queryFn: () => fetchVideoTask(activeTaskId ?? ''),
    enabled: !!activeTaskId,
    refetchInterval: (query) => {
      const state = query.state.data
      if (state?.status === 'SUCCESS' || state?.status === 'FAILURE') {
        return false
      }
      return VIDEO_POLL_INTERVAL_MS
    },
    retry: false,
  })

  // 恢复上次未完成的提交：组件挂载时读回 task_id 并重新进入轮询态，
  // 终态处理复用同一套 effect，行为与本次会话内提交完全一致。
  useEffect(() => {
    const pending = loadPendingVideoTask(owner)
    if (!pending) return
    started.current = pending.started_at
    setActiveDraft(pending.draft)
    setActiveTaskId(pending.task_id)
  }, [owner])

  // 终态落库：SUCCESS 下载视频字节并写入本地历史（失败降级为「只可显示」），
  // FAILURE 透出上游失败原因。两种终态都结束轮询。processedTaskId 保证同一
  // 任务的终态只处理一次（setActiveTaskId(undefined) 会再次触发本 effect）。
  useEffect(() => {
    const state = task.data
    const draft = activeDraft
    if (!state || !draft || !activeTaskId) return
    if (processedTaskId.current === activeTaskId) return
    if (state.status === 'SUCCESS') {
      processedTaskId.current = activeTaskId
      const finish = (
        video?: ArrayBuffer,
        videoUrl?: string,
        notice?: string
      ) => {
        const job: VideoJob = {
          id: newVideoJobId(),
          task_id: activeTaskId,
          created_at: Date.now(),
          request: draft,
          elapsed_ms: Date.now() - started.current,
          video,
          video_url: video ? undefined : videoUrl,
          video_mime: 'video/mp4',
        }
        setSelected(job)
        setWarning(notice ?? '')
        setActiveTaskId(undefined)
        clearPendingVideoTask(owner)
        void saveVideoJob(owner, job)
          .then(() =>
            void client.invalidateQueries({
              queryKey: ['video-local-history', owner],
            })
          )
          .catch((error: unknown) => setWarning(workflowError(error)))
      }
      if (!state.url) {
        finish(undefined, undefined, 'The task finished without a video URL.')
        return
      }
      downloadVideo(state.url)
        .then(async (blob) => {
          finish(await blob.arrayBuffer())
        })
        .catch(() =>
          finish(
            undefined,
            state.url,
            'This result could not be saved locally. Download it before leaving this page.'
          )
        )
      return
    }
    if (state.status === 'FAILURE') {
      processedTaskId.current = activeTaskId
      setSelected(undefined)
      setWarning(state.fail_reason)
      setActiveTaskId(undefined)
      clearPendingVideoTask(owner)
    }
  }, [task.data, activeDraft, activeTaskId, owner, client])

  // 轮询请求本身出错（网络中断、5xx）时停止并透出原因，避免按钮永久卡死。
  // processedTaskId 同样保证只处理一次。
  useEffect(() => {
    if (!task.isError || !activeTaskId) return
    if (processedTaskId.current === activeTaskId) return
    processedTaskId.current = activeTaskId
    setSelected(undefined)
    setWarning(videoError(task.error))
    setActiveTaskId(undefined)
    clearPendingVideoTask(owner)
  }, [task.isError, task.error, activeTaskId, owner])

  useEffect(() => {
    if (!activeTaskId) return
    setElapsed(Date.now() - started.current)
    const timer = setInterval(
      () => setElapsed(Date.now() - started.current),
      1000
    )
    return () => clearInterval(timer)
  }, [activeTaskId])

  const deletion = useMutation({
    mutationFn: (id?: string) => deleteVideoJob(owner, id),
    onSuccess: (_, id) => {
      if (!id || selected?.id === id) setSelected(undefined)
      void client.invalidateQueries({ queryKey: ['video-local-history', owner] })
    },
  })

  // activeTaskId 在提交成功后置位，在终态（SUCCESS/FAILURE）或轮询出错后被清空，
  // 因此它是否为空就是「是否还在等一个视频」的准确信号；不能依赖 task.isPending，
  // 那只在首次抓取时为真，第一次轮询返回后就会变假。
  const busy = submission.isPending || activeTaskId !== undefined

  return {
    submission,
    task,
    deletion,
    history,
    selected,
    warning,
    elapsed,
    busy,
    select: (id: string) =>
      setSelected(history.data?.find((job) => job.id === id)),
    submit: (draft: VideoDraft) => submission.mutate(structuredClone(draft)),
  }
}
