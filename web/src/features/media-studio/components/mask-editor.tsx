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
import { useRef, useState, type PointerEvent as ReactPointerEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { maskRegion, type MaskPoint, type MaskRegion } from '../lib/mask'
import type { StudioAsset } from '../workflow-types'

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
  const [size, setSize] = useState({ width: 0, height: 0 })
  const [drag, setDrag] = useState<{ from: MaskPoint; to: MaskPoint } | null>(
    null
  )
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
        aria-label={t('Selection area')}
        className='bg-muted relative w-full touch-none overflow-hidden rounded-lg border'
        style={{
          aspectRatio:
            size.width > 0 && size.height > 0
              ? `${size.width} / ${size.height}`
              : '1',
        }}
        onPointerDown={(event) => {
          if (props.disabled) return
          const point = at(event)
          if (!point) return
          try {
            // 指针捕获让拖出画布后仍能收到 pointerup；没有活动指针时（合成事件、
            // 老浏览器缺少该 API）会抛异常，此时不捕获也能在画布内完成拖拽。
            event.currentTarget.setPointerCapture?.(event.pointerId)
          } catch {
            // 忽略：捕获只是便利，不是拖拽成立的前提。
          }
          setDrag({ from: point, to: point })
        }}
        onPointerMove={(event) => {
          if (!drag) return
          const point = at(event)
          if (point) setDrag({ from: drag.from, to: point })
        }}
        onPointerUp={(event) => {
          if (!drag) return
          const region = maskRegion(drag.from, at(event) ?? drag.to)
          setDrag(null)
          if (region) props.onChange([...props.regions, region])
        }}
        onPointerCancel={() => setDrag(null)}
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
      </div>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Drag on the image to box the area you want to change. Everything outside the boxes is kept.'
        )}
      </p>
      {!!props.regions.length && (
        <p className='text-muted-foreground text-xs'>
          {t('{{count}} boxed area(s)', { count: props.regions.length })}
        </p>
      )}
    </section>
  )
}
