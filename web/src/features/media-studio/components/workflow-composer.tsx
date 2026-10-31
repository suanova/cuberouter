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

import {
  CFG_RANGE,
  COUNT_OPTIONS,
  DEFAULT_PARAMS,
  LIMITS,
  QUALITY_OPTIONS,
  qualitySteps,
  serverImageSteps,
  STEPS_RANGE,
} from '../constants'
import { draftSchema } from '../lib/workflow'
import type { Quality } from '../types'
import type { WorkflowDraft } from '../workflow-types'
import { MaskEditor } from './mask-editor'

export function WorkflowComposer(props: {
  generating?: boolean
  onEditReference?: () => void
  queued?: boolean
  queueAccess?: boolean
  draft: WorkflowDraft
  textToImageModels: string[]
  imageToImageModels: string[]
  /** 声明了 image-mask 的模型：只有它们接受蒙版，才显示框选工具。 */
  maskModels: string[]
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
  const [width, height] = props.draft.size.split('x').map(Number)
  const sizeValid =
    !props.queued ||
    (Number.isInteger(width) &&
      Number.isInteger(height) &&
      width >= 256 &&
      width <= 1664 &&
      height >= 256 &&
      height <= 1664 &&
      width % 32 === 0 &&
      height % 32 === 0 &&
      width * height <= 2097152)
  // 框选只对声明了 image-mask 的编辑模型开放；其他上游会静默丢掉 mask。
  const maskable = editing && props.maskModels.includes(props.draft.model)
  const form = useForm<WorkflowDraft>({
    values: props.draft,
    resolver: zodResolver(draftSchema),
  })
  const update = (patch: Partial<WorkflowDraft>) =>
    props.onChange({ ...props.draft, ...patch })
  // 数字输入清空即撤销覆盖：undefined 不会进入请求体，回落到档位或机器默认值。
  const numeric = (value: string): number | undefined =>
    value === '' ? undefined : Number(value)
  // 超范围时草稿校验失败、生成按钮变灰，这里把原因直接标在输入框上。
  const outOfRange = (
    value: number | undefined,
    min: number,
    max: number,
    integer = true
  ) =>
    value !== undefined &&
    (!Number.isFinite(value) ||
      value < min ||
      value > max ||
      (integer && !Number.isInteger(value)))
  const valid =
    draftSchema.safeParse(props.draft).success &&
    sizeValid &&
    models.includes(props.draft.model) &&
    (!props.queued || !!props.queueAccess)
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
                // 文生图没有参考图，框选区域随之作废。
                mask: mode === 'edit' ? props.draft.mask : undefined,
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
          onChange={(event) => {
            const model = event.target.value
            update({
              model,
              // 框选只对声明了 image-mask 的模型成立：换到别的模型必须把框一起丢掉，
              // 否则草稿编辑器消失了，蒙版却仍会随请求发出去。
              mask: props.maskModels.includes(model)
                ? props.draft.mask
                : undefined,
            })
          }}
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
                      // 框选坐标是相对第一张参考图的：删掉它，蒙版也失效。
                      mask:
                        props.draft.references[0]?.id === asset.id
                          ? undefined
                          : props.draft.mask,
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
                props.draft.references.length >= LIMITS.maxReferences
              }
              onChange={(event) => {
                props.onUpload([...(event.target.files ?? [])])
                event.target.value = ''
              }}
            />
          </label>
          <p className='text-muted-foreground text-xs'>
            {t(
              'PNG, JPEG or WebP · up to 10 MB each · maximum {{max}} references',
              { max: LIMITS.maxReferences }
            )}
          </p>
          {maskable && !!props.draft.references.length && (
            <MaskEditor
              image={props.draft.references[0]}
              regions={props.draft.mask ?? []}
              disabled={props.busy}
              onChange={(regions) =>
                update({ mask: regions.length ? regions : undefined })
              }
            />
          )}
          {!!props.draft.references.length && props.onEditReference && (
            <Button
              type='button'
              variant='outline'
              disabled={props.busy}
              onClick={props.onEditReference}
            >
              {t('Edit reference image')}
            </Button>
          )}
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
          maxLength={LIMITS.promptMax}
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
            aria-invalid={
              !sizeValid || !/^\d{2,4}x\d{2,4}$/.test(props.draft.size)
            }
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
            {(props.queued ? [1] : COUNT_OPTIONS).map((count) => (
              <option key={count}>{count}</option>
            ))}
          </select>
        </label>
      </div>
      <p className='text-muted-foreground text-xs'>
        {t(
          props.queued
            ? 'Use 256–1664 pixels per side, multiples of 32, and at most 2,097,152 pixels in total.'
            : 'Supported sizes and image counts depend on the selected provider.'
        )}
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
      <details className='space-y-2 text-sm'>
        <summary className='cursor-pointer'>{t('Advanced Settings')}</summary>
        <div className='grid grid-cols-3 gap-2 pt-2'>
          <label className='text-xs'>
            {t('Steps')}
            <Input
              type='number'
              min={STEPS_RANGE.min}
              max={STEPS_RANGE.max}
              value={props.draft.steps ?? ''}
              placeholder={String(
                props.queued
                  ? serverImageSteps(props.draft.quality)
                  : qualitySteps(props.draft.quality)
              )}
              aria-invalid={outOfRange(
                props.draft.steps,
                STEPS_RANGE.min,
                STEPS_RANGE.max
              )}
              disabled={props.busy}
              onChange={(event) =>
                update({ steps: numeric(event.target.value) })
              }
            />
          </label>
          <label className='text-xs'>
            {t('CFG')}
            <Input
              type='number'
              min={CFG_RANGE.min}
              max={CFG_RANGE.max}
              // 草稿接受 0..10 的任意小数（如 6.55）；固定 0.1 步长会让原生校验拦下合法值。
              step='any'
              value={props.draft.cfg ?? ''}
              placeholder={String(props.queued ? 1 : DEFAULT_PARAMS.cfg)}
              aria-invalid={outOfRange(
                props.draft.cfg,
                CFG_RANGE.min,
                CFG_RANGE.max,
                false
              )}
              disabled={props.busy}
              onChange={(event) => update({ cfg: numeric(event.target.value) })}
            />
          </label>
          <label className='text-xs'>
            {t('Seed')}
            <Input
              type='number'
              min={0}
              value={props.draft.seed ?? ''}
              placeholder={String(DEFAULT_PARAMS.seed)}
              aria-invalid={outOfRange(
                props.draft.seed,
                0,
                Number.MAX_SAFE_INTEGER
              )}
              disabled={props.busy}
              onChange={(event) =>
                update({ seed: numeric(event.target.value) })
              }
            />
          </label>
        </div>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Leave blank to use the default: Steps follows the selected quality; CFG and Seed use fixed defaults.'
          )}
        </p>
      </details>
      <Button
        type='submit'
        className='w-full'
        disabled={!valid || props.busy || props.loading}
      >
        {t(
          (props.generating ?? props.busy)
            ? 'Generation in progress…'
            : 'Generate image'
        )}
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
