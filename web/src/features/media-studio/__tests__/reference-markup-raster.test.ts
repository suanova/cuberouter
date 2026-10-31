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
import { createCanvas, loadImage } from '@napi-rs/canvas'
import { expect, test, vi } from 'vitest'

import * as images from '../lib/reference-image'
import { renderReferenceMarkup, type ImageMark } from '../lib/reference-markup'

test.each(['rectangle', 'brush', 'lasso'] as const)(
  'the real rasterized %s mask clears selected pixels and preserves outside pixels',
  async (tool) => {
    const source = createCanvas(100, 100)
    source.getContext('2d').fillRect(0, 0, 100, 100)
    const decoded = await loadImage(source.toDataURL('image/png'))
    vi.spyOn(images, 'decodeImage').mockResolvedValue(
      decoded as unknown as HTMLImageElement
    )
    const create = document.createElement.bind(document)
    vi.spyOn(document, 'createElement').mockImplementation((name, options) => {
      if (name === 'canvas') {
        return createCanvas(100, 100) as unknown as HTMLCanvasElement
      }
      return create(name, options)
    })
    const points: ImageMark['points'] = [
      { x: 0.2, y: 0.2 },
      { x: 0.6, y: 0.6 },
    ]
    if (tool === 'brush') points[1] = { x: 0.6, y: 0.2 }
    if (tool === 'lasso') points.splice(1, 0, { x: 0.6, y: 0.2 })
    const png = await renderReferenceMarkup('known-source', {
      referenceID: 'photo-a',
      kind: 'region',
      marks: [{ tool, color: 'blue', width: 0.1, points }],
    })
    const mask = await loadImage(png)
    const output = createCanvas(100, 100)
    const context = output.getContext('2d')
    context.drawImage(mask, 0, 0)
    const selectedY = tool === 'brush' ? 20 : 30
    expect(context.getImageData(40, selectedY, 1, 1).data[3]).toBe(0)
    if (tool === 'brush') {
      expect(context.getImageData(40, 16, 1, 1).data[3]).toBe(0)
      expect(context.getImageData(30, 20, 1, 1).data[3]).toBe(0)
    }
    expect(context.getImageData(90, 90, 1, 1).data[3]).toBe(255)
  }
)
