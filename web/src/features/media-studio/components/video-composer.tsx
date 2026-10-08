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
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

import {
  VIDEO_DURATION_OPTIONS,
  VIDEO_RESOLUTION_OPTIONS,
} from '../constants'
import { videoDraftSchema } from '../lib/video-draft'
import type { VideoDraft, VideoMode } from '../video-types'

export function VideoComposer(props: {
  draft: VideoDraft
  textToVideoModels: string[]
  imageToVideoModels: string[]
  busy: boolean
  loading: boolean
  onChange: (draft: VideoDraft) => void
  onUpload: (files: File[]) => void
  onRemoveImage: () => void
  onGenerate: () => void
  onReset: () => void
}) {
  const { t } = useTranslation()
  const imageMode = props.draft.mode === 'image'
  const models = imageMode
    ? props.imageToVideoModels
    : props.textToVideoModels
  const update = (patch: Partial<VideoDraft>) =>
    props.onChange({ ...props.draft, ...patch })
  const valid =
    videoDraftSchema.safeParse(props.draft).success &&
    models.includes(props.draft.model)
  let modelPlaceholder = 'No text-to-video models available'
  if (props.loading) {
    modelPlaceholder = 'Loading...'
  } else if (imageMode) {
    modelPlaceholder = 'No image-to-video models available'
  }
  return (
    <form
      className='space-y-4'
      onSubmit={(event) => {
        event.preventDefault()
        if (valid && !props.busy) props.onGenerate()
      }}
    >
      <div
        className='bg-muted flex rounded-xl p-1'
        aria-label={t('Video mode')}
      >
        {(['text', 'image'] as const).map((mode: VideoMode) => (
          <Button
            key={mode}
            type='button'
            size='sm'
            className='flex-1'
            aria-pressed={props.draft.mode === mode}
            variant={props.draft.mode === mode ? 'secondary' : 'ghost'}
            disabled={props.busy}
            onClick={() => {
              const eligible =
                mode === 'image'
                  ? props.imageToVideoModels
                  : props.textToVideoModels
              update({
                mode,
                model: eligible.includes(props.draft.model)
                  ? props.draft.model
                  : (eligible[0] ?? ''),
                image: mode === 'image' ? props.draft.image : undefined,
              })
            }}
          >
            {t(mode === 'text' ? 'Text to video' : 'Image to video')}
          </Button>
        ))}
      </div>
      <label className='block space-y-1 text-sm'>
        {t('Model')}
        <select
          className='bg-background h-9 w-full rounded-lg border px-2'
          value={props.draft.model}
          disabled={props.busy || props.loading}
          onChange={(event) => update({ model: event.target.value })}
        >
          {!models.length && (
            <option value=''>{t(modelPlaceholder)}</option>
          )}
          {models.map((name) => (
            <option key={name}>{name}</option>
          ))}
        </select>
      </label>
      {imageMode && (
        <section className='space-y-2' aria-label={t('First frame image')}>
          {!models.length && (
            <p role='status' className='text-muted-foreground text-xs'>
              {t('No channel is configured for image-to-video.')}
            </p>
          )}
          {props.draft.image && (
            <div className='space-y-1'>
              <img
                src={props.draft.image.url}
                alt={t('First frame image')}
                className='w-full rounded-lg'
              />
              <Button
                size='sm'
                type='button'
                variant='outline'
                disabled={props.busy}
                onClick={() => props.onRemoveImage()}
              >
                {t('Remove')}
              </Button>
            </div>
          )}
          <label className='block text-xs'>
            {t('Upload a first frame image')}
            <Input
              type='file'
              accept='image/png,image/jpeg,image/webp'
              disabled={props.busy || !!props.draft.image}
              onChange={(event) => {
                props.onUpload([...(event.target.files ?? [])])
                event.target.value = ''
              }}
            />
          </label>
          <p className='text-muted-foreground text-xs'>
            {t('PNG, JPEG or WebP · up to 10 MB · one image')}
          </p>
        </section>
      )}
      <label className='block space-y-1 text-sm'>
        {t('Prompt')}
        <Textarea
          value={props.draft.prompt}
          disabled={props.busy}
          maxLength={16000}
          className='min-h-32'
          onChange={(event) => update({ prompt: event.target.value })}
        />
      </label>
      <div className='grid grid-cols-2 gap-3'>
        <label className='text-xs'>
          {t('Duration (seconds)')}
          <select
            value={props.draft.duration}
            className='bg-background h-9 w-full rounded-lg border px-2'
            disabled={props.busy}
            onChange={(event) =>
              update({ duration: Number(event.target.value) })
            }
          >
            {VIDEO_DURATION_OPTIONS.map((duration) => (
              <option key={duration} value={duration}>
                {duration}
              </option>
            ))}
          </select>
        </label>
        <label className='text-xs'>
          {t('Resolution')}
          <select
            value={props.draft.resolution}
            className='bg-background h-9 w-full rounded-lg border px-2'
            disabled={props.busy}
            onChange={(event) => update({ resolution: event.target.value })}
          >
            {VIDEO_RESOLUTION_OPTIONS.map((resolution) => (
              <option key={resolution}>{resolution}</option>
            ))}
          </select>
        </label>
      </div>
      <p className='text-muted-foreground text-xs'>
        {t('Supported durations and resolutions depend on the selected provider.')}
      </p>
      <Button
        type='submit'
        className='w-full'
        disabled={!valid || props.busy || props.loading}
      >
        {t(props.busy ? 'Video generation in progress…' : 'Generate video')}
      </Button>
      <Button
        type='button'
        variant='ghost'
        className='w-full'
        disabled={props.busy}
        onClick={() => props.onReset()}
      >
        {t('Reset settings')}
      </Button>
      <p className='text-muted-foreground border-t pt-3 text-xs'>
        {t('Video generation is asynchronous and can take several minutes.')}
      </p>
    </form>
  )
}
