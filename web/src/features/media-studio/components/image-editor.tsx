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
import { useMutation } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

import { usePrivateImage } from '../hooks/use-private-image'
import { initialDraft, workflowError } from '../lib/workflow'
import { workflowAPI } from '../workflow-api'
import type {
  OCRResult,
  StudioAsset,
  TextLayer,
  WorkflowDraft,
  WorkflowJob,
} from '../workflow-types'
import { SelectionCanvas, type SelectionHandle } from './selection-canvas'

export function ImageEditor(props: {
  asset: StudioAsset
  parent?: WorkflowJob
  onClose: () => void
  onSaved: (job: WorkflowJob) => void
  onRegional: (draft: WorkflowDraft) => Promise<void>
}) {
  const { t } = useTranslation()
  const image = usePrivateImage(props.asset.id)
  const selection = useRef<SelectionHandle>(null)
  const [tab, setTab] = useState<'regional' | 'text'>('regional')
  const [prompt, setPrompt] = useState('')
  const [editMode, setEditMode] = useState('auto')
  const [steps, setSteps] = useState(40)
  const [seed, setSeed] = useState(42)
  const [cfg, setCfg] = useState(4)
  const [expected, setExpected] = useState('')
  const [ocr, setOCR] = useState<OCRResult>()
  const [text, setText] = useState('')
  const [fontSize, setFontSize] = useState(48)
  const [color, setColor] = useState('#173f36')
  const [background, setBackground] = useState('#ffffff')
  const [cover, setCover] = useState(true)
  const [align, setAlign] = useState<'left' | 'center'>('left')
  const [layers, setLayers] = useState<TextLayer[]>([])
  const [preview, setPreview] = useState('')
  const previewRef = useRef('')
  const [message, setMessage] = useState('')
  useEffect(
    () => () => {
      if (previewRef.current) URL.revokeObjectURL(previewRef.current)
    },
    []
  )
  function textLayers(): TextLayer[] {
    const box = selection.current?.box()
    if (!box) {
      if (layers.length) return layers
      throw new Error('Select a text rectangle first.')
    }
    if (fontSize < 8 || fontSize > 256 || !Number.isInteger(fontSize)) {
      throw new Error('Font size must be between 8 and 256.')
    }
    const layer: TextLayer = {
      box,
      text,
      font_size: fontSize,
      color,
      background,
      cover,
      align,
    }
    const result = layers.filter(
      (item) => JSON.stringify(item.box) !== JSON.stringify(box)
    )
    if (result.length >= 32) throw new Error('Use at most 32 text layers.')
    result.push(layer)
    return result
  }
  const action = useMutation({
    mutationFn: async (operation: 'ocr' | 'preview' | 'text' | 'regional') => {
      setMessage('')
      if (operation === 'ocr') {
        setOCR(await workflowAPI.ocr(props.asset.id, expected))
        return
      }
      if (operation === 'regional') {
        if (!prompt.trim()) {
          throw new Error('Describe the change to the selected area.')
        }
        const png = selection.current?.png()
        if (!png) throw new Error('Select an area first.')
        const mask = await workflowAPI.mask(props.asset.id, png)
        await props.onRegional({
          ...initialDraft,
          mode: 'regional',
          prompt,
          references: [props.asset.id],
          parent_id: props.parent?.id,
          size: 'auto',
          count: 1,
          steps,
          seed,
          cfg,
          mask_id: mask.id,
          edit_mode: editMode,
        })
        props.onClose()
        return
      }
      const next = textLayers()
      setLayers(next)
      if (operation === 'preview') {
        const blob = await workflowAPI.preview(props.asset.id, next)
        if (previewRef.current) URL.revokeObjectURL(previewRef.current)
        previewRef.current = URL.createObjectURL(blob)
        setPreview(previewRef.current)
        setMessage('Text preview only. Save to create a new version.')
        return
      }
      const job = await workflowAPI.text(props.asset.id, next, props.parent?.id)
      props.onSaved(job)
      props.onClose()
    },
    retry: false,
  })
  const busy = action.isPending
  let comparison =
    'OCR can miss or misread text. Provide the intended text for an exact comparison.'
  if (ocr) {
    comparison =
      'No text was recognized. The image may have no text, or it may be too small to read.'
    if (ocr.regions.length) {
      comparison =
        'Text was recognized. Review it visually or provide the intended wording.'
      if (ocr.comparison.expected_provided) {
        comparison = ocr.comparison.matches
          ? 'Recognized text matches the supplied wording, ignoring whitespace.'
          : 'Recognized text differs from the supplied wording.'
      }
    }
  }
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
    >
      <DialogContent className='max-h-[92vh] overflow-y-auto sm:max-w-6xl'>
        <DialogTitle>{t('Image tools')}</DialogTitle>
        <DialogDescription>
          {t(
            'Modify a selected area or typeset exact text. Every saved change creates a new version.'
          )}
        </DialogDescription>
        <div className='grid min-w-0 gap-5 lg:grid-cols-[minmax(0,1fr)_320px]'>
          <div className='min-w-0'>
            {image.url ? (
              <SelectionCanvas
                key={props.asset.id}
                url={preview || image.url}
                disabled={busy}
                selectionRef={selection}
              />
            ) : (
              <p role='status'>
                {t(
                  image.failed
                    ? 'Image unavailable or expired'
                    : 'Loading image…'
                )}
              </p>
            )}
          </div>
          <div className='space-y-4'>
            <div className='bg-muted flex rounded-xl p-1'>
              {(['regional', 'text'] as const).map((value) => (
                <Button
                  key={value}
                  size='sm'
                  className='flex-1'
                  variant={tab === value ? 'secondary' : 'ghost'}
                  aria-pressed={tab === value}
                  disabled={busy}
                  onClick={() => setTab(value)}
                >
                  {t(
                    value === 'regional' ? 'Regional edit' : 'Text correction'
                  )}
                </Button>
              ))}
            </div>
            {tab === 'regional' ? (
              <>
                <label className='block space-y-1 text-xs'>
                  {t('Edit intent')}
                  <select
                    value={editMode}
                    disabled={busy}
                    className='bg-background h-9 w-full rounded-lg border px-2'
                    onChange={(event) => setEditMode(event.target.value)}
                  >
                    {(
                      [
                        ['auto', 'Follow instruction'],
                        ['add', 'Add an object'],
                        ['replace', 'Replace content'],
                        ['remove', 'Remove content'],
                        ['appearance', 'Change appearance'],
                      ] as const
                    ).map(([value, label]) => (
                      <option value={value} key={value}>
                        {t(label)}
                      </option>
                    ))}
                  </select>
                </label>
                <label className='block space-y-1 text-xs'>
                  {t('Selected area instruction')}
                  <Textarea
                    value={prompt}
                    maxLength={8000}
                    className='min-h-28'
                    disabled={busy}
                    onChange={(event) => setPrompt(event.target.value)}
                    placeholder={t('Describe the change to the selected area.')}
                  />
                </label>
                <div className='grid grid-cols-2 gap-2'>
                  <label className='text-xs'>
                    {t('Steps')}
                    <Input
                      type='number'
                      min={8}
                      max={60}
                      value={steps}
                      disabled={busy}
                      onChange={(event) => setSteps(event.target.valueAsNumber)}
                    />
                  </label>
                  <label className='text-xs'>
                    {t('Seed')}
                    <Input
                      type='number'
                      min={0}
                      max={9007199254740987}
                      value={seed}
                      disabled={busy}
                      onChange={(event) => setSeed(event.target.valueAsNumber)}
                    />
                  </label>
                </div>
                <label className='block text-xs'>
                  {t('Generation preference')}
                  <select
                    className='bg-background mt-1 h-9 w-full rounded-lg border px-2'
                    value={cfg}
                    disabled={busy}
                    onChange={(event) => setCfg(Number(event.target.value))}
                  >
                    <option value={4}>{t('Detail priority · CFG 4')}</option>
                    <option value={1}>{t('Faster preview · CFG 1')}</option>
                  </select>
                </label>
                <p className='bg-muted rounded-xl p-3 text-xs leading-relaxed'>
                  {t(
                    'Only the selected area is composited into the original. Give new objects enough space. Review the saved result against the full model candidate.'
                  )}
                </p>
                <Button
                  className='w-full'
                  disabled={busy || !image.url || !prompt.trim()}
                  onClick={() => action.mutate('regional')}
                >
                  {t('Generate regional edit')}
                </Button>
              </>
            ) : (
              <>
                <label className='block text-xs'>
                  {t('Intended text (optional)')}
                  <Textarea
                    value={expected}
                    maxLength={10000}
                    disabled={busy}
                    onChange={(event) => setExpected(event.target.value)}
                  />
                </label>
                <Button
                  variant='outline'
                  className='w-full'
                  disabled={busy || !image.url}
                  onClick={() => action.mutate('ocr')}
                >
                  {t('Read and compare text')}
                </Button>
                <p className='text-muted-foreground text-xs leading-relaxed'>
                  {t(comparison)}
                </p>
                {ocr && (
                  <div className='max-h-40 space-y-1 overflow-y-auto'>
                    {ocr.regions.map((region) => (
                      <button
                        key={region.box.join(',')}
                        type='button'
                        disabled={busy}
                        className='flex w-full justify-between gap-2 rounded-lg border p-2 text-left text-xs'
                        onClick={() => {
                          selection.current?.select(region.box)
                          setText(region.text)
                          setFontSize(
                            Math.max(
                              8,
                              Math.min(256, Math.round(region.box[3] * 0.75))
                            )
                          )
                          setBackground(region.suggested_background)
                        }}
                      >
                        <span>{region.text}</span>
                        <span className='text-muted-foreground'>
                          {Math.round(region.confidence * 100)}%
                        </span>
                      </button>
                    ))}
                  </div>
                )}
                <label className='block text-xs'>
                  {t('Correct text for this rectangle')}
                  <Textarea
                    value={text}
                    maxLength={2000}
                    disabled={busy}
                    onChange={(event) => setText(event.target.value)}
                  />
                </label>
                <div className='grid grid-cols-2 gap-2'>
                  <label className='text-xs'>
                    {t('Maximum font size')}
                    <Input
                      type='number'
                      min={8}
                      max={256}
                      value={fontSize}
                      disabled={busy}
                      onChange={(event) =>
                        setFontSize(event.target.valueAsNumber)
                      }
                    />
                  </label>
                  <label className='text-xs'>
                    {t('Alignment')}
                    <select
                      value={align}
                      disabled={busy}
                      onChange={(event) =>
                        setAlign(event.target.value as 'left' | 'center')
                      }
                      className='bg-background h-9 w-full rounded-lg border px-2'
                    >
                      <option value='left'>{t('Left')}</option>
                      <option value='center'>{t('Center')}</option>
                    </select>
                  </label>
                  <label className='text-xs'>
                    {t('Text color')}
                    <Input
                      type='color'
                      value={color}
                      disabled={busy}
                      onChange={(event) => setColor(event.target.value)}
                    />
                  </label>
                  <label className='text-xs'>
                    {t('Cover color')}
                    <Input
                      type='color'
                      value={background}
                      disabled={busy}
                      onChange={(event) => setBackground(event.target.value)}
                    />
                  </label>
                </div>
                <label className='flex items-center gap-2 text-xs'>
                  <input
                    type='checkbox'
                    checked={cover}
                    disabled={busy}
                    onChange={(event) => setCover(event.target.checked)}
                  />
                  {t('Cover old text with a solid background')}
                </label>
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'A solid cover replaces the selected background. For textured backgrounds, remove old lettering with Regional edit first.'
                  )}
                </p>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={busy}
                  onClick={() => {
                    try {
                      setLayers(textLayers())
                      setMessage(
                        'Text layer added. Select another rectangle to add more.'
                      )
                    } catch (error) {
                      setMessage(workflowError(error))
                    }
                  }}
                >
                  {t('Add or update text layer')}
                </Button>
                {layers.map((layer, index) => (
                  <div
                    key={layer.box.join(',')}
                    className='flex items-center gap-2 text-xs'
                  >
                    <span className='min-w-0 flex-1 truncate'>
                      {index + 1}. {layer.text || t('Solid cover')}
                    </span>
                    <Button
                      size='sm'
                      variant='ghost'
                      disabled={busy}
                      onClick={() => {
                        setLayers(layers.filter((_, i) => i !== index))
                        selection.current?.clear()
                        setPreview('')
                      }}
                    >
                      {t('Remove')}
                    </Button>
                  </div>
                ))}
                <div className='flex gap-2'>
                  <Button
                    size='sm'
                    variant='outline'
                    disabled={busy || !image.url}
                    onClick={() => action.mutate('preview')}
                  >
                    {t('Preview text')}
                  </Button>
                  <Button
                    size='sm'
                    variant='ghost'
                    disabled={busy}
                    onClick={() => setPreview('')}
                  >
                    {t('Show original')}
                  </Button>
                </div>
                <Button
                  className='w-full'
                  disabled={busy || !image.url}
                  onClick={() => action.mutate('text')}
                >
                  {t('Save text correction')}
                </Button>
              </>
            )}
            {busy && (
              <p role='status' className='text-primary text-xs'>
                {t(
                  'Processing… You can close the editor; submitted jobs continue.'
                )}
              </p>
            )}
            {message && (
              <p role='status' className='text-xs'>
                {t(message)}
              </p>
            )}
            {action.error && (
              <p role='alert' className='text-destructive text-xs'>
                {t(workflowError(action.error))}
              </p>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
