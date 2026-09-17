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
import { ImagePlus, Sparkles, Upload, X } from 'lucide-react'
import { useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

import { CREATE_SIZES, draftSchema } from '../lib/workflow'
import type {
  StudioAsset,
  WorkflowConfig,
  WorkflowDraft,
} from '../workflow-types'
import { PrivateImage } from './private-image'

export function WorkflowComposer(props: {
  draft: WorkflowDraft
  config: WorkflowConfig
  references: StudioAsset[]
  busy: boolean
  uploading: boolean
  onChange: (draft: WorkflowDraft) => void
  onUpload: (files: File[]) => void
  onGenerate: () => void
  onTemplates: () => void
  onTools: (asset: StudioAsset) => void
  onReset: () => void
}) {
  const { t } = useTranslation()
  const upload = useRef<HTMLInputElement>(null)
  const form = useForm<WorkflowDraft>({
    values: props.draft,
    resolver: zodResolver(draftSchema),
  })
  const editing = props.draft.mode !== 'create'
  const locked = props.busy || props.uploading
  const update = (patch: Partial<WorkflowDraft>) =>
    props.onChange({ ...props.draft, ...patch })
  return (
    <form onSubmit={form.handleSubmit(props.onGenerate)} className='space-y-5'>
      <div
        className='bg-muted flex rounded-xl p-1'
        aria-label={t('Creation mode')}
      >
        {(['create', 'edit'] as const).map((mode) => (
          <Button
            key={mode}
            type='button'
            className='flex-1'
            size='sm'
            variant={props.draft.mode === mode ? 'secondary' : 'ghost'}
            disabled={locked}
            aria-pressed={props.draft.mode === mode}
            onClick={() =>
              update({
                mode,
                size: mode === 'create' ? '1664x928' : 'auto',
                steps: Math.min(props.draft.steps, 60),
                references: mode === 'edit' ? props.draft.references : [],
                parent_id: undefined,
              })
            }
          >
            {t(mode === 'create' ? 'Text to image' : 'Image to image')}
          </Button>
        ))}
      </div>
      <div className='bg-background rounded-xl border px-3 py-2'>
        <span className='text-muted-foreground text-[10px] tracking-widest uppercase'>
          {t('Model')}
        </span>
        <p className='mt-1 text-sm font-medium'>
          {props.config.models[editing ? 'edit' : 'create']}
        </p>
      </div>
      <Button
        type='button'
        variant='outline'
        className='w-full justify-start'
        onClick={props.onTemplates}
      >
        <Sparkles className='size-4' />
        {t('Browse templates')}
      </Button>
      {editing && (
        <section className='space-y-2' aria-label={t('Reference images')}>
          <p className='text-sm font-medium'>
            {t('Reference images')}{' '}
            <span className='text-muted-foreground font-normal'>
              {props.draft.references.length}/3
            </span>
          </p>
          <div className='grid grid-cols-3 gap-2'>
            {props.references
              .filter((image) => props.draft.references.includes(image.id))
              .map((image) => (
                <div key={image.id} className='relative rounded-xl border p-1'>
                  <PrivateImage
                    id={image.id}
                    alt={t('Reference image')}
                    className='aspect-square w-full rounded-lg object-cover'
                  />
                  <Button
                    type='button'
                    aria-label={t('Remove reference image')}
                    size='icon-sm'
                    variant='secondary'
                    className='absolute top-1 right-1'
                    disabled={locked}
                    onClick={() =>
                      update({
                        references: props.draft.references.filter(
                          (id) => id !== image.id
                        ),
                        parent_id: undefined,
                      })
                    }
                  >
                    <X className='size-3' />
                  </Button>
                  <button
                    type='button'
                    className='text-primary w-full py-1 text-xs'
                    disabled={locked}
                    onClick={() => props.onTools(image)}
                  >
                    {t('Image tools')}
                  </button>
                </div>
              ))}
            {props.draft.references.length < 3 && (
              <Button
                type='button'
                variant='outline'
                className='aspect-square h-auto flex-col border-dashed'
                disabled={locked}
                onClick={() => upload.current?.click()}
              >
                <Upload className='size-5' />
                <span className='text-xs'>
                  {t(props.uploading ? 'Uploading…' : 'Upload')}
                </span>
              </Button>
            )}
          </div>
          <input
            ref={upload}
            type='file'
            accept='image/png,image/jpeg,image/webp'
            multiple
            className='hidden'
            aria-label={t('Upload reference images')}
            onChange={(event) => {
              props.onUpload([...(event.target.files ?? [])])
              event.target.value = ''
            }}
          />
          <p className='text-muted-foreground text-xs'>
            {t('PNG, JPEG or WebP · up to 10 MB each · maximum 3 references')}
          </p>
        </section>
      )}
      {props.draft.parent_id && (
        <p className='bg-primary/10 text-primary rounded-lg p-2 text-xs'>
          {t('Editing a previous version. The original is preserved.')}
        </p>
      )}
      <label className='block space-y-2 text-sm font-medium'>
        {t(editing ? 'Describe your changes' : 'Prompt')}
        <Textarea
          {...form.register('prompt')}
          value={props.draft.prompt}
          disabled={locked}
          maxLength={16000}
          className='min-h-36 resize-y font-normal'
          placeholder={t(
            editing
              ? 'What should change, and what should stay the same?'
              : 'Describe the subject, scene, lighting and style…'
          )}
          onChange={(event) => update({ prompt: event.target.value })}
        />
      </label>
      <div className='grid grid-cols-2 gap-3'>
        <label className='space-y-1 text-xs'>
          {t('Image size')}
          <select
            className='bg-background h-9 w-full rounded-lg border px-2 text-sm'
            value={props.draft.size}
            disabled={locked}
            onChange={(event) => update({ size: event.target.value })}
          >
            {editing ? (
              <>
                <option value='auto'>{t('Follow reference')}</option>
                <option value='1024x1024'>1:1 · 1024</option>
                <option value='1344x768'>16:9 · 1344×768</option>
                <option value='768x1344'>9:16 · 768×1344</option>
              </>
            ) : (
              Object.entries(CREATE_SIZES).map(([ratio, size]) => (
                <option key={size} value={size}>
                  {ratio} · {size}
                </option>
              ))
            )}
          </select>
        </label>
        <label className='space-y-1 text-xs'>
          {t('Images per request')}
          <select
            className='bg-background h-9 w-full rounded-lg border px-2 text-sm'
            value={props.draft.count}
            disabled={locked}
            onChange={(event) => update({ count: Number(event.target.value) })}
          >
            {[1, 2, 3, 4].map((count) => (
              <option key={count}>{count}</option>
            ))}
          </select>
        </label>
      </div>
      <details className='rounded-xl border p-3'>
        <summary className='cursor-pointer text-xs font-medium'>
          {t('Advanced settings and text checking')}
        </summary>
        <div className='mt-4 grid grid-cols-2 gap-3'>
          {(
            [
              {
                key: 'steps',
                label: 'Steps',
                min: 1,
                max: editing ? 60 : 100,
                step: 1,
              },
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
            <label key={field.key} className='space-y-1 text-xs'>
              {t(field.label)}
              <Input
                type='number'
                value={props.draft[field.key]}
                min={field.min}
                max={field.max}
                step={field.step}
                disabled={locked}
                onChange={(event) =>
                  update({ [field.key]: event.target.valueAsNumber })
                }
              />
            </label>
          ))}
        </div>
        {editing && (
          <label className='mt-3 block space-y-1 text-xs'>
            {t('Negative prompt')}
            <Textarea
              value={props.draft.negative_prompt}
              maxLength={8000}
              disabled={locked}
              onChange={(event) =>
                update({ negative_prompt: event.target.value })
              }
            />
          </label>
        )}
        <label className='mt-3 block space-y-1 text-xs'>
          {t('Exact text expected in the image (optional)')}
          <Textarea
            value={props.draft.expected_text}
            maxLength={10000}
            disabled={locked}
            onChange={(event) => update({ expected_text: event.target.value })}
          />
        </label>
        <p className='text-muted-foreground mt-2 text-xs leading-relaxed'>
          {t(
            'OCR runs automatically. It can miss or misread text. When a repair is attempted, both versions are retained for comparison.'
          )}
        </p>
      </details>
      {Object.keys(form.formState.errors).length > 0 && (
        <p role='alert' className='text-destructive text-xs'>
          {t('Check the prompt, reference images and numeric settings.')}
        </p>
      )}
      <Button
        type='submit'
        className='h-11 w-full gap-2'
        disabled={
          locked ||
          !props.draft.prompt.trim() ||
          (editing && !props.draft.references.length)
        }
      >
        <ImagePlus className='size-4' />
        {t(props.busy ? 'Generation in progress…' : 'Generate image')}
      </Button>
      <div className='flex justify-between text-xs'>
        <button
          type='button'
          disabled={locked}
          onClick={props.onReset}
          className='text-muted-foreground underline underline-offset-4'
        >
          {t('Reset settings')}
        </button>
        <span className='text-muted-foreground'>
          {t('Originals are preserved')}
        </span>
      </div>
      <p className='text-muted-foreground border-t pt-3 text-[11px] leading-relaxed'>
        {t(
          'Generation and AI edits use CubeRouter model pricing and appear in usage logs. Text layout and OCR are CPU tools.'
        )}
      </p>
    </form>
  )
}
