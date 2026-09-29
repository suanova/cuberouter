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
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'

import {
  annotatedDraft,
  drawStrokes,
  strokeColors,
  type Stroke,
} from '../lib/qwen-edit'
import { referenceAsset } from '../workflow-api'
import type { WorkflowDraft } from '../workflow-types'

export function QwenEditor(props: {
  draft: WorkflowDraft
  draw: boolean
  busy: boolean
  onClose: () => void
  onSubmit: (draft: WorkflowDraft) => void
}) {
  const { t } = useTranslation()
  const canvas = useRef<HTMLCanvasElement>(null)
  const bitmap = useRef<ImageBitmap | null>(null)
  const active = useRef<Stroke | null>(null)
  const [color, setColor] = useState<string>(strokeColors[0].value)
  const [strokes, setStrokes] = useState<Stroke[]>([])
  const [redo, setRedo] = useState<Stroke[]>([])
  const [instruction, setInstruction] = useState(
    props.draft.user_prompt ?? props.draft.prompt
  )
  const [ready, setReady] = useState(false)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const original = props.draft.references[0]
  useEffect(() => {
    let disposed = false
    void fetch(original.url)
      .then((r) => r.blob())
      .then(createImageBitmap)
      .then((image) => {
        if (disposed) {
          image.close()
          return
        }
        if (image.width * image.height > 8000000) {
          image.close()
          throw new Error('Image is too large to edit.')
        }
        bitmap.current = image
        if (canvas.current) {
          canvas.current.width = image.width
          canvas.current.height = image.height
        }
        setReady(true)
      })
      .catch(() => {
        if (!disposed) setError('Could not load the reference image.')
      })
    return () => {
      disposed = true
      bitmap.current?.close()
      bitmap.current = null
    }
  }, [original.url])
  useEffect(() => {
    const ctx = canvas.current?.getContext('2d')
    if (!ctx || !bitmap.current) return
    ctx.drawImage(bitmap.current, 0, 0)
    drawStrokes(ctx, strokes, bitmap.current.width)
  }, [ready, strokes])
  const busy = props.busy || saving
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !busy) props.onClose()
      }}
    >
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-5xl'>
        <DialogTitle>
          {t(props.draw ? 'Edit with colored strokes' : 'Continue editing')}
        </DialogTitle>
        <DialogDescription>
          {t(
            'Edit the prompt here and submit when ready. The original and output size are preserved.'
          )}
        </DialogDescription>
        {props.draw && (
          <div className='flex flex-wrap gap-2' aria-label={t('Stroke color')}>
            {strokeColors.map((item) => (
              <Button
                key={item.value}
                variant='outline'
                disabled={busy}
                aria-pressed={color === item.value}
                onClick={() => setColor(item.value)}
              >
                {t(item.label)}
              </Button>
            ))}
          </div>
        )}
        <div className='grid items-start gap-4 md:grid-cols-[minmax(0,2fr)_minmax(230px,1fr)]'>
          <div className='min-w-0 space-y-3'>
            <canvas
              ref={canvas}
              aria-label={t('Draw freehand on the reference image')}
              className='block max-h-[60vh] max-w-full touch-none rounded-lg'
              onPointerDown={(event) => {
                if (!props.draw || busy || !ready || strokes.length >= 128) {
                  return
                }
                event.currentTarget.setPointerCapture(event.pointerId)
                const rect = event.currentTarget.getBoundingClientRect()
                active.current = {
                  color,
                  points: [
                    [
                      ((event.clientX - rect.left) *
                        event.currentTarget.width) /
                        rect.width,
                      ((event.clientY - rect.top) *
                        event.currentTarget.height) /
                        rect.height,
                    ],
                  ],
                }
                setStrokes((items) => [
                  ...items,
                  ...(active.current ? [active.current] : []),
                ])
                setRedo([])
              }}
              onPointerMove={(event) => {
                const stroke = active.current
                if (!stroke || stroke.points.length >= 2048) return
                const rect = event.currentTarget.getBoundingClientRect()
                const x = Math.max(
                  0,
                  Math.min(
                    event.currentTarget.width,
                    ((event.clientX - rect.left) * event.currentTarget.width) /
                      rect.width
                  )
                )
                const y = Math.max(
                  0,
                  Math.min(
                    event.currentTarget.height,
                    ((event.clientY - rect.top) * event.currentTarget.height) /
                      rect.height
                  )
                )
                stroke.points.push([x, y])
                setStrokes((items) => [...items])
              }}
              onPointerUp={() => {
                active.current = null
              }}
              onPointerCancel={() => {
                active.current = null
              }}
            />
            {props.draw && (
              <div className='flex flex-wrap gap-2'>
                <Button
                  variant='outline'
                  disabled={busy || !strokes.length}
                  onClick={() => {
                    setRedo([...redo, ...strokes.slice(-1)])
                    setStrokes(strokes.slice(0, -1))
                  }}
                >
                  {t('Undo')}
                </Button>
                <Button
                  variant='outline'
                  disabled={busy || !redo.length}
                  onClick={() => {
                    setStrokes([...strokes, ...redo.slice(-1)])
                    setRedo(redo.slice(0, -1))
                  }}
                >
                  {t('Redo')}
                </Button>
                <Button
                  variant='outline'
                  disabled={busy || !strokes.length}
                  onClick={() => {
                    setStrokes([])
                    setRedo([])
                  }}
                >
                  {t('Clear strokes')}
                </Button>
              </div>
            )}
            {props.draw && (
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Freehand strokes stay open. Describe every marked region in the single prompt. The model may change other areas.'
                )}
              </p>
            )}
          </div>
          <div className='space-y-4'>
            <label className='block space-y-2'>
              {t('Describe your changes')}
              <Textarea
                className='min-h-48'
                value={instruction}
                maxLength={15500}
                disabled={busy}
                onChange={(event) => setInstruction(event.target.value)}
              />
            </label>
            <p>
              {t('Image size')}: {props.draft.size}
            </p>
            {!!error && <p role='alert'>{t(error)}</p>}
            <Button
              disabled={busy || !ready || !instruction.trim()}
              onClick={async () => {
                setSaving(true)
                setError('')
                try {
                  let next: WorkflowDraft = {
                    ...props.draft,
                    prompt: instruction,
                    user_prompt: instruction,
                  }
                  if (props.draw) {
                    const element = canvas.current
                    if (!element) throw new Error('Canvas is unavailable.')
                    // Same original-pixel canvas shown above is encoded as the actual model input.
                    const blob = await new Promise<Blob>((resolve, reject) =>
                      element.toBlob(
                        (value) =>
                          value
                            ? resolve(value)
                            : reject(new Error('Could not encode image.')),
                        'image/png'
                      )
                    )
                    next = annotatedDraft(
                      props.draft,
                      original,
                      await referenceAsset(blob),
                      strokes,
                      instruction
                    )
                  }
                  props.onSubmit(next)
                } catch {
                  setError('Could not prepare the edited image.')
                } finally {
                  setSaving(false)
                }
              }}
            >
              {t('Submit image edit')}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
