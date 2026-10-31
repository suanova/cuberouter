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
import { decodeImage } from './reference-image'

export interface MarkPoint {
  x: number
  y: number
}
export interface ImageMark {
  id?: string
  tool: 'rectangle' | 'lasso' | 'brush'
  color: 'blue' | 'red' | 'green' | 'white'
  width: number
  points: MarkPoint[]
}
export interface ReferenceMarkup {
  referenceID: string
  kind: 'region' | 'annotation'
  marks: ImageMark[]
}

export function validImageMark(mark: ImageMark): boolean {
  return (
    ['rectangle', 'lasso', 'brush'].includes(mark.tool) &&
    ['blue', 'red', 'green', 'white'].includes(mark.color) &&
    Number.isFinite(mark.width) &&
    mark.width >= 0.005 &&
    mark.width <= 0.1 &&
    mark.points.length >= { lasso: 3, rectangle: 2, brush: 1 }[mark.tool] &&
    mark.points.length <= 2048 &&
    mark.points.every(
      (p) =>
        Number.isFinite(p.x) &&
        Number.isFinite(p.y) &&
        p.x >= 0 &&
        p.x <= 1 &&
        p.y >= 0 &&
        p.y <= 1
    )
  )
}

export function markPath(mark: ImageMark, xScale = 1): string {
  if (!validImageMark(mark)) return ''
  const first = mark.points[0],
    last = mark.points.at(-1)
  if (!first || !last) return ''
  if (mark.tool === 'rectangle') {
    return `M ${first.x * xScale} ${first.y} L ${last.x * xScale} ${first.y} L ${last.x * xScale} ${last.y} L ${first.x * xScale} ${last.y} Z`
  }
  if (mark.points.length === 1) {
    return `M ${first.x * xScale} ${first.y} L ${first.x * xScale} ${first.y}`
  }
  return (
    mark.points
      .map(
        (point, i) => `${i === 0 ? 'M' : 'L'} ${point.x * xScale} ${point.y}`
      )
      .join(' ') + (mark.tool === 'lasso' ? ' Z' : '')
  )
}

// Invalid drawing or unavailable canvas fails explicitly; it never silently
// turns a regional edit into an unrestricted redraw.
export async function renderReferenceMarkup(
  url: string,
  markup: ReferenceMarkup
): Promise<string> {
  if (
    !markup.marks.length ||
    markup.marks.length > 100 ||
    markup.marks.some((mark) => !validImageMark(mark))
  ) {
    throw new Error(
      'The reference drawing is invalid. Review it before generating.'
    )
  }
  const source = await decodeImage(url)
  if (!source || source.naturalWidth * source.naturalHeight > 2097152) {
    throw new Error('The reference drawing is too large or could not be read.')
  }
  const canvas = document.createElement('canvas')
  canvas.width = source.naturalWidth
  canvas.height = source.naturalHeight
  const ctx = canvas.getContext('2d')
  if (!ctx) {
    throw new Error(
      'The reference drawing could not be prepared. Nothing was generated.'
    )
  }
  if (markup.kind === 'annotation') ctx.drawImage(source, 0, 0)
  else {
    ctx.fillStyle = 'black'
    ctx.fillRect(0, 0, canvas.width, canvas.height)
  }
  for (const mark of markup.marks) {
    ctx.globalCompositeOperation =
      markup.kind === 'region' ? 'destination-out' : 'source-over'
    ctx.fillStyle = mark.color
    ctx.strokeStyle = mark.color
    ctx.lineWidth = mark.width * Math.min(canvas.width, canvas.height)
    ctx.lineCap = 'round'
    ctx.lineJoin = 'round'
    const first = mark.points[0],
      last = mark.points.at(-1)
    if (!first || !last) {
      throw new Error(
        'The reference drawing is invalid. Review it before generating.'
      )
    }
    if (mark.tool === 'rectangle') {
      const x = Math.min(first.x, last.x) * canvas.width,
        y = Math.min(first.y, last.y) * canvas.height
      const width = Math.abs(first.x - last.x) * canvas.width,
        height = Math.abs(first.y - last.y) * canvas.height
      if (markup.kind === 'region') ctx.fillRect(x, y, width, height)
      else ctx.strokeRect(x, y, width, height)
    } else if (mark.points.length === 1) {
      ctx.beginPath()
      ctx.arc(
        first.x * canvas.width,
        first.y * canvas.height,
        ctx.lineWidth / 2,
        0,
        Math.PI * 2
      )
      ctx.fill()
    } else {
      ctx.beginPath()
      ctx.moveTo(first.x * canvas.width, first.y * canvas.height)
      for (const point of mark.points.slice(1)) {
        ctx.lineTo(point.x * canvas.width, point.y * canvas.height)
      }
      if (mark.tool === 'lasso' && markup.kind === 'region') {
        ctx.closePath()
        ctx.fill()
      } else ctx.stroke()
    }
  }
  const result = canvas.toDataURL('image/png')
  if (!result.startsWith('data:image/png;base64,')) {
    throw new Error(
      'The reference drawing could not be prepared. Nothing was generated.'
    )
  }
  return result
}
