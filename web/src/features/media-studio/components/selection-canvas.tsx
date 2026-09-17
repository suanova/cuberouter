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
import {
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
  type Ref,
} from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

import type { SelectionBox } from '../workflow-types'

export interface SelectionHandle {
  png: () => string
  box: () => SelectionBox | null
  select: (box: SelectionBox) => void
  clear: () => void
}
export function SelectionCanvas(props: {
  url: string
  disabled: boolean
  selectionRef: Ref<SelectionHandle>
}) {
  const { t } = useTranslation()
  const canvas = useRef<HTMLCanvasElement>(null)
  const mask = useRef<HTMLCanvasElement | null>(null)
  const source = useRef<HTMLImageElement | null>(null)
  const snapshot = useRef<ImageData | null>(null)
  const active = useRef(false)
  const points = useRef<Array<[number, number]>>([])
  const [tool, setTool] = useState<'rectangle' | 'lasso' | 'brush'>('rectangle')
  const [brush, setBrush] = useState(32)
  const [box, setBox] = useState<SelectionBox>([0, 0, 100, 100])
  const boxRef = useRef<SelectionBox | null>(null)
  const [error, setError] = useState('')
  function draw() {
    const ctx = canvas.current?.getContext('2d')
    if (!ctx || !source.current || !mask.current) return
    ctx.clearRect(0, 0, source.current.width, source.current.height)
    ctx.drawImage(source.current, 0, 0)
    ctx.globalAlpha = 0.45
    ctx.drawImage(mask.current, 0, 0)
    ctx.globalAlpha = 1
  }
  function select(next: SelectionBox) {
    const target = mask.current
    const ctx = target?.getContext('2d')
    if (!ctx || !target) return
    if (
      next.some((value) => !Number.isInteger(value)) ||
      next[0] < 0 ||
      next[1] < 0 ||
      next[2] < 4 ||
      next[3] < 4 ||
      next[0] + next[2] > target.width ||
      next[1] + next[3] > target.height
    ) {
      setError(
        'Selection must fit inside the image and be at least 4 × 4 pixels.'
      )
      return
    }
    snapshot.current = ctx.getImageData(0, 0, target.width, target.height)
    ctx.clearRect(0, 0, target.width, target.height)
    ctx.fillStyle = '#ff603e'
    ctx.fillRect(...next)
    setBox(next)
    boxRef.current = next
    setError('')
    draw()
  }
  useImperativeHandle(props.selectionRef, () => ({
    select,
    clear: () => {
      const target = mask.current
      target?.getContext('2d')?.clearRect(0, 0, target.width, target.height)
      boxRef.current = null
      draw()
    },
    box: () => boxRef.current,
    png: () => {
      const target = mask.current
      const ctx = target?.getContext('2d')
      if (!target || !ctx) throw new Error('Select an area first.')
      const pixels = ctx.getImageData(0, 0, target.width, target.height)
      let any = false
      for (let i = 0; i < pixels.data.length; i += 4) {
        const selected = pixels.data[i + 3] > 0
        any ||= selected
        pixels.data[i] =
          pixels.data[i + 1] =
          pixels.data[i + 2] =
            selected ? 255 : 0
        pixels.data[i + 3] = 255
      }
      if (!any) throw new Error('Select an area first.')
      const output = document.createElement('canvas')
      output.width = target.width
      output.height = target.height
      output.getContext('2d')?.putImageData(pixels, 0, 0)
      return output.toDataURL('image/png').split(',')[1]
    },
  }))
  useEffect(() => {
    let cancelled = false
    const image = new Image()
    image.onload = () => {
      if (cancelled || !canvas.current) return
      source.current = image
      if (
        !mask.current ||
        mask.current.width !== image.width ||
        mask.current.height !== image.height
      ) {
        mask.current = document.createElement('canvas')
        mask.current.width = image.width
        mask.current.height = image.height
      }
      canvas.current.width = image.width
      canvas.current.height = image.height
      draw()
    }
    image.src = props.url
    return () => {
      cancelled = true
    }
  }, [props.url])
  return (
    <div className='space-y-3'>
      <div className='flex flex-wrap gap-1'>
        {(['rectangle', 'lasso', 'brush'] as const).map((value) => (
          <Button
            key={value}
            type='button'
            size='sm'
            variant={tool === value ? 'secondary' : 'ghost'}
            disabled={props.disabled}
            aria-pressed={tool === value}
            onClick={() => setTool(value)}
          >
            {t(value)}
          </Button>
        ))}
        <Button
          size='sm'
          variant='ghost'
          disabled={props.disabled}
          onClick={() => {
            const ctx = mask.current?.getContext('2d')
            if (ctx && snapshot.current) {
              ctx.putImageData(snapshot.current, 0, 0)
              boxRef.current = null
              draw()
            }
          }}
        >
          {t('Undo selection')}
        </Button>
        <Button
          size='sm'
          variant='ghost'
          disabled={props.disabled}
          onClick={() => {
            const target = mask.current
            if (!target) return
            const ctx = target.getContext('2d')
            if (!ctx) return
            snapshot.current = ctx.getImageData(
              0,
              0,
              target.width,
              target.height
            )
            ctx.clearRect(0, 0, target.width, target.height)
            boxRef.current = null
            draw()
          }}
        >
          {t('Clear selection')}
        </Button>
      </div>
      {tool === 'brush' && (
        <label className='flex items-center gap-2 text-xs'>
          {t('Brush size')}
          <Input
            type='number'
            min={4}
            max={256}
            value={brush}
            disabled={props.disabled}
            onChange={(event) =>
              setBrush(
                Math.max(4, Math.min(256, event.target.valueAsNumber || 4))
              )
            }
            className='w-24'
          />
        </label>
      )}
      <canvas
        ref={canvas}
        aria-label={t('Image selection canvas')}
        aria-disabled={props.disabled}
        className='bg-muted w-full touch-none rounded-xl border'
        onPointerDown={(event) => {
          if (props.disabled || !mask.current) return
          const target = event.currentTarget,
            rect = target.getBoundingClientRect()
          const point: [number, number] = [
            Math.round(
              ((event.clientX - rect.left) * target.width) / rect.width
            ),
            Math.round(
              ((event.clientY - rect.top) * target.height) / rect.height
            ),
          ]
          target.setPointerCapture(event.pointerId)
          active.current = true
          points.current = [point]
          snapshot.current =
            mask.current
              .getContext('2d')
              ?.getImageData(0, 0, target.width, target.height) ?? null
          boxRef.current = null
        }}
        onPointerMove={(event) => {
          if (!active.current || !mask.current) return
          const target = event.currentTarget,
            rect = target.getBoundingClientRect(),
            ctx = mask.current.getContext('2d')
          if (!ctx) return
          const point: [number, number] = [
            Math.max(
              0,
              Math.min(
                target.width,
                Math.round(
                  ((event.clientX - rect.left) * target.width) / rect.width
                )
              )
            ),
            Math.max(
              0,
              Math.min(
                target.height,
                Math.round(
                  ((event.clientY - rect.top) * target.height) / rect.height
                )
              )
            ),
          ]
          const start = points.current[0],
            previous = points.current.at(-1) ?? start
          points.current.push(point)
          ctx.fillStyle = '#ff603e'
          ctx.strokeStyle = '#ff603e'
          ctx.lineWidth = brush
          ctx.lineCap = 'round'
          ctx.lineJoin = 'round'
          if (tool === 'rectangle') {
            ctx.clearRect(0, 0, target.width, target.height)
            const next: SelectionBox = [
              Math.min(start[0], point[0]),
              Math.min(start[1], point[1]),
              Math.abs(point[0] - start[0]),
              Math.abs(point[1] - start[1]),
            ]
            ctx.fillRect(...next)
            setBox(next)
            boxRef.current = next
          } else if (tool === 'brush') {
            ctx.beginPath()
            ctx.moveTo(...previous)
            ctx.lineTo(...point)
            ctx.stroke()
          } else {
            if (snapshot.current) ctx.putImageData(snapshot.current, 0, 0)
            ctx.beginPath()
            ctx.moveTo(...start)
            points.current.forEach((p) => ctx.lineTo(...p))
            ctx.closePath()
            ctx.fill()
          }
          draw()
        }}
        onPointerUp={(event) => {
          active.current = false
          if (event.currentTarget.hasPointerCapture(event.pointerId)) {
            event.currentTarget.releasePointerCapture(event.pointerId)
          }
        }}
        onPointerCancel={() => {
          active.current = false
          const ctx = mask.current?.getContext('2d')
          if (ctx && snapshot.current) ctx.putImageData(snapshot.current, 0, 0)
          boxRef.current = null
          draw()
        }}
      />
      <p className='text-muted-foreground text-xs'>
        {t(
          'The orange area will be modified. Use coordinates below for keyboard selection.'
        )}
      </p>
      <div className='grid grid-cols-4 gap-2'>
        {(['X', 'Y', 'Width', 'Height'] as const).map((label, index) => (
          <label key={label} className='text-xs'>
            {t(label)}
            <Input
              type='number'
              min={0}
              disabled={props.disabled}
              value={box[index]}
              onChange={(event) => {
                const next: SelectionBox = [...box]
                next[index] = event.target.valueAsNumber
                setBox(next)
              }}
            />
          </label>
        ))}
      </div>
      <Button
        size='sm'
        variant='outline'
        disabled={props.disabled}
        onClick={() => select(box)}
      >
        {t('Apply selection coordinates')}
      </Button>
      {error && (
        <p role='alert' className='text-destructive text-xs'>
          {t(error)}
        </p>
      )}
    </div>
  )
}
