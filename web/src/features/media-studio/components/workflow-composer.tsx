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
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Textarea } from '@/components/ui/textarea'

import { QUALITY_OPTIONS } from '../constants'
import { draftSchema } from '../lib/workflow'
import type { Quality } from '../types'
import type { WorkflowConfig, WorkflowDraft } from '../workflow-types'

export function WorkflowComposer(props: {
  draft: WorkflowDraft
  config: WorkflowConfig
  textToImageModels: string[]
  imageToImageModels: string[]
  busy: boolean
  loading: boolean
  onChange: (draft: WorkflowDraft) => void
  onUpload: (files: File[]) => void
  onGenerate: () => void
  onReset: () => void
}) {
  const { t } = useTranslation()
  const editing = props.draft.mode === 'edit'
  const models = editing ? props.imageToImageModels : props.textToImageModels
  const form = useForm<WorkflowDraft>({
    values: props.draft,
    resolver: zodResolver(draftSchema),
  })
  const update = (patch: Partial<WorkflowDraft>) =>
    props.onChange({ ...props.draft, ...patch })
  const valid =
    draftSchema.safeParse(props.draft).success &&
    models.includes(props.draft.model) &&
    (!editing || props.config.upload_enabled)
  return (
    <form
      className='space-y-4'
      onSubmit={form.handleSubmit(() => {
        if (valid && !props.busy) props.onGenerate()
      })}
    >
      <div
        className='bg-muted flex rounded-xl p-1'
        aria-label={t('Creation mode')}
      >
        {(['create', 'edit'] as const).map((mode) => (
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
                mode === 'edit'
                  ? props.imageToImageModels
                  : props.textToImageModels
              update({
                mode,
                model: eligible.includes(props.draft.model)
                  ? props.draft.model
                  : (eligible[0] ?? ''),
                references: mode === 'edit' ? props.draft.references : [],
                parent_id: undefined,
              })
            }}
          >
            {t(mode === 'create' ? 'Text to image' : 'Image to image')}
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
            <option value=''>
              {t(props.loading ? 'Loading...' : 'No image models available')}
            </option>
          )}
          {models.map((name) => (
            <option key={name}>{name}</option>
          ))}
        </select>
      </label>
      {editing && (
        <section className='space-y-2' aria-label={t('Reference images')}>
          {!props.config.upload_enabled && (
            <p role='status' className='text-muted-foreground text-xs'>
              {t('Reference uploads are not configured.')}
            </p>
          )}
          {!models.length && (
            <p role='status' className='text-muted-foreground text-xs'>
              {t('No channel is configured for image editing.')}
            </p>
          )}
          <div className='grid grid-cols-3 gap-2'>
            {props.draft.references.map((asset) => (
              <div key={asset.id} className='space-y-1'>
                <img
                  src={asset.url}
                  alt={t('Reference image')}
                  className='aspect-square w-full rounded-lg object-cover'
                />
                <Button
                  size='sm'
                  type='button'
                  variant='outline'
                  disabled={props.busy}
                  aria-label={t('Remove reference image')}
                  onClick={() =>
                    update({
                      references: props.draft.references.filter(
                        (item) => item.id !== asset.id
                      ),
                    })
                  }
                >
                  {t('Remove')}
                </Button>
              </div>
            ))}
          </div>
          <label className='block text-xs'>
            {t('Upload reference images')}
            <Input
              type='file'
              accept='image/png,image/jpeg,image/webp'
              multiple
              disabled={
                props.busy ||
                !props.config.upload_enabled ||
                props.draft.references.length >= 3
              }
              onChange={(event) => {
                props.onUpload([...(event.target.files ?? [])])
                event.target.value = ''
              }}
            />
          </label>
          <p className='text-muted-foreground text-xs'>
            {t('PNG, JPEG or WebP · up to 10 MB each · maximum 3 references')}
          </p>
        </section>
      )}
      {props.draft.parent_id && (
        <p className='text-primary text-xs'>
          {t('Editing a previous version. The original is preserved.')}
        </p>
      )}
      <label className='block space-y-1 text-sm'>
        {t(editing ? 'Describe your changes' : 'Prompt')}
        <Textarea
          value={props.draft.prompt}
          disabled={props.busy}
          maxLength={16000}
          className='min-h-36'
          onChange={(event) => update({ prompt: event.target.value })}
        />
      </label>
      <div className='grid grid-cols-2 gap-3'>
        <label className='text-xs'>
          {t('Image size')}
          <Input
            value={props.draft.size}
            placeholder='1024x1024'
            disabled={props.busy}
            aria-invalid={!/^\d{2,4}x\d{2,4}$/.test(props.draft.size)}
            onChange={(event) => update({ size: event.target.value })}
          />
        </label>
        <label className='text-xs'>
          {t('Images per request')}
          <select
            value={props.draft.count}
            className='bg-background h-9 w-full rounded-lg border px-2'
            disabled={props.busy}
            onChange={(event) => update({ count: Number(event.target.value) })}
          >
            {[1, 2, 3, 4].map((count) => (
              <option key={count}>{count}</option>
            ))}
          </select>
        </label>
      </div>
      <p className='text-muted-foreground text-xs'>
        {t('Supported sizes and image counts depend on the selected provider.')}
      </p>
      <div className='space-y-1'>
        <span className='block text-sm'>{t('Quality')}</span>
        <RadioGroup
          aria-label={t('Quality')}
          value={props.draft.quality}
          onValueChange={(value) => update({ quality: value as Quality })}
          disabled={props.busy}
          className='grid-cols-3'
        >
          {QUALITY_OPTIONS.map((option) => (
            <div key={option.id} className='flex items-center gap-2'>
              <RadioGroupItem
                value={option.id}
                id={`workflow-quality-${option.id}`}
              />
              <label
                htmlFor={`workflow-quality-${option.id}`}
                className='cursor-pointer text-xs'
              >
                {t(option.labelKey)}
              </label>
            </div>
          ))}
        </RadioGroup>
      </div>
      <Button
        type='submit'
        className='w-full'
        disabled={!valid || props.busy || props.loading}
      >
        {t(props.busy ? 'Generation in progress…' : 'Generate image')}
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
        {t(
          'Generation and editing use your CubeRouter channels, quota and usage logs.'
        )}
      </p>
    </form>
  )
}
