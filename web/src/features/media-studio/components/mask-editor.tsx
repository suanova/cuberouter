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
  useId,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
  type PointerEvent as ReactPointerEvent,
} from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { LIMITS } from '../constants'
import { maskRegion, type MaskPoint, type MaskRegion } from '../lib/mask'
import type { StudioAsset } from '../workflow-types'

/** 键盘走位步长；按住 Shift 用细步长，便于把框对齐到细节边缘。 */
const KEYBOARD_STEP = 0.05
const KEYBOARD_FINE_STEP = 0.01
/** 键盘落的框是固定大小的方形，以光标为中心。 */
const KEYBOARD_BOX_SIZE = 0.2

/** 一次拖拽：记下起始指针，其他手指的事件一律忽略。 */
type Drag = { pointerId: number | undefined; from: MaskPoint; to: MaskPoint }

function clamp01(value: number): number {
  return Math.min(1, Math.max(0, value))
}

/**
 * 在参考图上拉框选择要重绘的区域。
 *
 * 容器宽高比跟随图片本身（onLoad 后测量），坐标按容器实际尺寸归一化，所以框选位置
 * 与下发时按图片像素栅格化出来的蒙版逐点对应；用 object-contain 加方形容器会在
 * 非方图上留白边，框选位置就会整体偏移。
 */
export function MaskEditor(props: {
  image: StudioAsset
  regions: MaskRegion[]
  disabled: boolean
  onChange: (regions: MaskRegion[]) => void
}) {
  const { t } = useTranslation()
  const surface = useRef<HTMLDivElement>(null)
  // 键盘说明的 id：把说明挂到可聚焦的画布上，焦点落到画布时就能听到怎么操作。
  const helpId = useId()
  const [size, setSize] = useState({ width: 0, height: 0 })
  const [drag, setDrag] = useState<Drag | null>(null)
  // 键盘光标：方向键移动，Enter/空格在光标处落框。初始在画面中心。
  const [cursor, setCursor] = useState<MaskPoint>({ x: 0.5, y: 0.5 })
  const [focused, setFocused] = useState(false)
  const at = (event: ReactPointerEvent): MaskPoint | null => {
    const box = surface.current?.getBoundingClientRect()
    if (!box || box.width <= 0 || box.height <= 0) {
      return null
    }
    return {
      x: (event.clientX - box.left) / box.width,
      y: (event.clientY - box.top) / box.height,
    }
  }
  const full = props.regions.length >= LIMITS.maxMaskRegions
  const append = (region: MaskRegion | null) => {
    // 超过上限的框进不了草稿校验（draftSchema 会拒绝），会连生成按钮一起按灰，
    // 所以在编辑阶段就挡住。上限提示见下方文案。
    if (!region || full) return
    props.onChange([...props.regions, region])
  }
  const moves: Record<string, MaskPoint> = {
    ArrowLeft: { x: -1, y: 0 },
    ArrowRight: { x: 1, y: 0 },
    ArrowUp: { x: 0, y: -1 },
    ArrowDown: { x: 0, y: 1 },
  }
  const onKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (props.disabled) return
    const move = moves[event.key]
    if (move) {
      event.preventDefault()
      const step = event.shiftKey ? KEYBOARD_FINE_STEP : KEYBOARD_STEP
      setCursor((value) => ({
        x: clamp01(value.x + move.x * step),
        y: clamp01(value.y + move.y * step),
      }))
      return
    }
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      const half = KEYBOARD_BOX_SIZE / 2
      append(
        maskRegion(
          { x: cursor.x - half, y: cursor.y - half },
          { x: cursor.x + half, y: cursor.y + half }
        )
      )
    }
  }
  const preview = drag ? maskRegion(drag.from, drag.to) : null
  // 覆盖层按几何值取 key：同一个框在重渲染中保持同一个 span，正在拖的预览单独一个 key。
  const boxes = [
    ...props.regions.map((region) => ({
      region,
      key: `box-${region.x}-${region.y}-${region.width}-${region.height}`,
    })),
    ...(preview ? [{ region: preview, key: 'preview' }] : []),
  ]
  return (
    <section className='space-y-2' aria-label={t('Region editing')}>
      <div className='flex items-center justify-between gap-2'>
        <span className='text-xs font-medium'>{t('Region editing')}</span>
        <div className='flex gap-1'>
          <Button
            size='sm'
            type='button'
            variant='ghost'
            disabled={props.disabled || !props.regions.length}
            onClick={() => props.onChange(props.regions.slice(0, -1))}
          >
            {t('Undo box')}
          </Button>
          <Button
            size='sm'
            type='button'
            variant='ghost'
            disabled={props.disabled || !props.regions.length}
            onClick={() => props.onChange([])}
          >
            {t('Clear boxes')}
          </Button>
        </div>
      </div>
      <div
        ref={surface}
        role='group'
        tabIndex={props.disabled ? -1 : 0}
        aria-label={t('Selection area')}
        aria-describedby={helpId}
        aria-disabled={props.disabled}
        className='bg-muted focus-visible:ring-primary relative w-full touch-none overflow-hidden rounded-lg border focus-visible:ring-2 focus-visible:outline-none'
        style={{
          aspectRatio:
            size.width > 0 && size.height > 0
              ? `${size.width} / ${size.height}`
              : '1',
        }}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        onKeyDown={onKeyDown}
        onPointerDown={(event) => {
          if (props.disabled || drag) return
          const point = at(event)
          if (!point) return
          try {
            // 指针捕获让拖出画布后仍能收到 pointerup；没有活动指针时（合成事件、
            // 老浏览器缺少该 API）会抛异常，此时不捕获也能在画布内完成拖拽。
            event.currentTarget.setPointerCapture?.(event.pointerId)
          } catch {
            // 忽略：捕获只是便利，不是拖拽成立的前提。
          }
          setDrag({ pointerId: event.pointerId, from: point, to: point })
        }}
        onPointerMove={(event) => {
          // 只跟起始的那根手指：第二根手指的移动不能改写这次框选。
          if (!drag || event.pointerId !== drag.pointerId) return
          const point = at(event)
          if (point) setDrag({ ...drag, to: point })
        }}
        onPointerUp={(event) => {
          if (!drag || event.pointerId !== drag.pointerId) return
          const region = maskRegion(drag.from, at(event) ?? drag.to)
          setDrag(null)
          append(region)
        }}
        onPointerCancel={(event) => {
          if (drag && event.pointerId !== drag.pointerId) return
          setDrag(null)
        }}
      >
        <img
          src={props.image.url}
          alt={t('Reference image')}
          draggable={false}
          className='pointer-events-none h-full w-full select-none'
          onLoad={(event) =>
            setSize({
              width: event.currentTarget.naturalWidth,
              height: event.currentTarget.naturalHeight,
            })
          }
        />
        {boxes.map((box) => (
          <span
            key={box.key}
            aria-hidden='true'
            className='border-primary bg-primary/25 pointer-events-none absolute border-2'
            style={{
              left: `${box.region.x * 100}%`,
              top: `${box.region.y * 100}%`,
              width: `${box.region.width * 100}%`,
              height: `${box.region.height * 100}%`,
            }}
          />
        ))}
        {focused && !props.disabled && (
          <span
            aria-hidden='true'
            className='border-primary pointer-events-none absolute size-4 -translate-x-1/2 -translate-y-1/2 rounded-full border-2'
            style={{
              left: `${cursor.x * 100}%`,
              top: `${cursor.y * 100}%`,
            }}
          />
        )}
      </div>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Drag on the image to box the area you want to change. Everything outside the boxes is kept.'
        )}
      </p>
      <p id={helpId} className='text-muted-foreground text-xs'>
        {t(
          'Or focus the image, move the pointer with the arrow keys (hold Shift for finer steps) and press Enter to place a box.'
        )}
      </p>
      {!!props.regions.length && (
        <p className='text-muted-foreground text-xs'>
          {t('{{count}} boxed area(s)', { count: props.regions.length })}
        </p>
      )}
      {full && (
        <p role='status' className='text-muted-foreground text-xs'>
          {t('At most {{max}} boxes per image.', {
            max: LIMITS.maxMaskRegions,
          })}
        </p>
      )}
    </section>
  )
}
