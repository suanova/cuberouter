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
import { useMutation, useQuery } from '@tanstack/react-query'

import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { getStudioModels } from './api'
import { VideoComposer } from './components/video-composer'
import { VideoHistory } from './components/video-history'
import { VideoResults } from './components/video-results'
import { useVideo } from './hooks/use-video'
import { initialVideoDraft, videoError } from './lib/video-draft'
import { referenceAsset } from './workflow-api'
import type { VideoDraft } from './video-types'

export function VideoStudio(props: { owner: number }) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState<VideoDraft>({ ...initialVideoDraft })
  const models = useQuery({
    queryKey: ['studio-models', props.owner],
    queryFn: getStudioModels,
    retry: false,
  })
  const catalog = models.data ?? {
    textToImage: [],
    imageToImage: [],
    textToVideo: [],
    imageToVideo: [],
  }
  const eligible =
    draft.mode === 'image' ? catalog.imageToVideo : catalog.textToVideo
  const current = {
    ...draft,
    model: eligible.includes(draft.model) ? draft.model : (eligible[0] ?? ''),
  }
  const video = useVideo(props.owner)
  const reference = useMutation({
    mutationFn: async (files: File[]) => {
      const file = files[0]
      if (!file) {
        throw new Error('Upload a first frame image.')
      }
      return referenceAsset(file)
    },
    onSuccess: (asset) =>
      setDraft((value) => ({ ...value, mode: 'image', image: asset })),
    retry: false,
  })
  const busy = video.busy || reference.isPending
  const errors = [
    models.error,
    reference.error,
    video.submission.error,
    video.deletion.error,
  ]
    .filter(Boolean)
    .map(videoError)
  return (
    <div className='grid items-start gap-6 lg:grid-cols-[350px_minmax(0,1fr)]'>
      <aside className='bg-card rounded-2xl border p-4'>
        <VideoComposer
          draft={current}
          textToVideoModels={catalog.textToVideo}
          imageToVideoModels={catalog.imageToVideo}
          busy={busy}
          loading={models.isPending}
          onChange={setDraft}
          onUpload={(files) => reference.mutate(files)}
          onRemoveImage={() =>
            setDraft((value) => ({ ...value, image: undefined }))
          }
          onGenerate={() => video.submit(current)}
          onReset={() => setDraft({ ...initialVideoDraft })}
        />
      </aside>
      <div className='min-w-0 space-y-4'>
        {errors.map((error) => (
          <p key={error} role='alert' className='text-destructive text-sm'>
            {t(error)}
          </p>
        ))}
        {!!video.warning && (
          <p role='status' className='text-muted-foreground text-sm'>
            {t(video.warning)}
          </p>
        )}
        {video.history.isError && (
          <p role='status' className='text-muted-foreground text-sm'>
            {t(
              'Local history storage is unavailable. Download videos to keep them.'
            )}
          </p>
        )}
        <div className='bg-card rounded-2xl border p-4'>
          <VideoResults
            job={video.selected}
            busy={video.busy}
            elapsed={video.elapsed}
            progress={video.task.data?.progress}
          />
        </div>
        <div className='bg-card rounded-2xl border p-4'>
          <VideoHistory
            jobs={video.history.data ?? []}
            selected={video.selected?.id}
            busy={busy}
            onSelect={video.select}
            onDelete={(id) => video.deletion.mutate(id)}
          />
        </div>
        <p className='text-muted-foreground text-xs'>
          {t(
            'History is stored in this browser for this account, up to 20 videos or 400 MB. It does not sync across devices and may be cleared by your browser. Download important videos.'
          )}
        </p>
      </div>
    </div>
  )
}
