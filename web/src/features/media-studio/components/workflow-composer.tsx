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
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

import { draftSchema, numericSettings } from '../lib/workflow'
import type { WorkflowConfig, WorkflowDraft } from '../workflow-types'

export function WorkflowComposer(props: {
  draft: WorkflowDraft
  config: WorkflowConfig
  models: string[]
  busy: boolean
  loading: boolean
  onChange: (draft: WorkflowDraft) => void
  onUpload: (files: File[]) => void
  onGenerate: () => void
  onReset: () => void
}) {
  const { t } = useTranslation()
  const [numbers, setNumbers] = useState({
    steps: String(props.draft.steps),
    seed: String(props.draft.seed),
    cfg: String(props.draft.cfg),
  })
  useEffect(
    () =>
      setNumbers((value) => ({ ...value, steps: String(props.draft.steps) })),
    [props.draft.steps]
  )
  useEffect(
    () => setNumbers((value) => ({ ...value, seed: String(props.draft.seed) })),
    [props.draft.seed]
  )
  useEffect(
    () => setNumbers((value) => ({ ...value, cfg: String(props.draft.cfg) })),
    [props.draft.cfg]
  )
  const editing = props.draft.mode === 'edit'
  const models = props.models.filter(
    (name) => !editing || props.config.edit_models.includes(name)
  )
  const parsed = numericSettings.safeParse(
    Object.fromEntries(
      Object.entries(numbers).map(([key, value]) => [
        key,
        value.trim() === '' ? Number.NaN : Number(value),
      ])
    )
  )
  const numericValid = !props.draft.advanced || parsed.success
  const form = useForm<WorkflowDraft>({
    values: props.draft,
    resolver: zodResolver(draftSchema),
  })
  const update = (patch: Partial<WorkflowDraft>) =>
    props.onChange({ ...props.draft, ...patch })
  const valid =
    draftSchema.safeParse(props.draft).success &&
    numericValid &&
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
              const eligible = props.models.filter(
                (name) =>
                  mode === 'create' || props.config.edit_models.includes(name)
              )
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
      <label className='flex items-center gap-2 text-xs'>
        <input
          type='checkbox'
          checked={props.draft.advanced}
          disabled={props.busy}
          onChange={(event) => update({ advanced: event.target.checked })}
        />
        {t('Send model-specific advanced settings')}
      </label>
      {props.draft.advanced && (
        <div className='grid grid-cols-3 gap-2'>
          {(
            [
              { key: 'steps', label: 'Steps', min: 1, max: 100, step: 1 },
              { key: 'cfg', label: 'CFG', min: 0, max: 10, step: 0.5 },
              {
                key: 'seed',
                label: 'Seed',
                min: 0,
                max: 9007199254740987,
                step: 1,
              },
            ] as const
          ).map((field) => (
            <label className='text-xs' key={field.key}>
              {t(field.label)}
              <Input
                type='number'
                value={numbers[field.key]}
                min={field.min}
                max={field.max}
                step={field.step}
                disabled={props.busy}
                aria-invalid={
                  !numericSettings.shape[field.key].safeParse(
                    numbers[field.key] === ''
                      ? Number.NaN
                      : Number(numbers[field.key])
                  ).success
                }
                onChange={(event) => {
                  const value = event.target.value
                  setNumbers((current) => ({ ...current, [field.key]: value }))
                  if (
                    value !== '' &&
                    numericSettings.shape[field.key].safeParse(Number(value))
                      .success
                  ) {
                    update({ [field.key]: Number(value) })
                  }
                }}
              />
            </label>
          ))}
        </div>
      )}
      {!numericValid && (
        <p role='alert' className='text-destructive text-xs'>
          {t('Check the numeric settings before generating.')}
        </p>
      )}
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
        onClick={() => {
          setNumbers({ steps: '40', seed: '42', cfg: '4' })
          props.onReset()
        }}
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
