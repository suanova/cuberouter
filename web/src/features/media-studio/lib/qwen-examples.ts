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
import type catalog from '../official-examples.json'
import { referenceAsset } from '../workflow-api'
import type { StudioAsset } from '../workflow-types'

type Example = (typeof catalog.examples)[number]
export async function exampleReferences(
  example: Example
): Promise<StudioAsset[]> {
  const bitmaps = new Map<string, ImageBitmap>()
  try {
    for (const input of example.inputs) {
      if (bitmaps.has(input.asset)) continue
      const response = await fetch(
        `/image-studio/official-qwen21/${input.asset}`,
        { credentials: 'omit' }
      )
      if (!response.ok) throw new Error('Official example could not be loaded.')
      bitmaps.set(input.asset, await createImageBitmap(await response.blob()))
    }
    const assets: StudioAsset[] = []
    for (const input of example.inputs) {
      const image = bitmaps.get(input.asset)
      if (!image) throw new Error('Official example could not be loaded.')
      const crop =
        'crop' in input ? input.crop : [0, 0, image.width, image.height]
      const [x, y, width, height] = crop
      if (
        x < 0 ||
        y < 0 ||
        width < 1 ||
        height < 1 ||
        x + width > image.width ||
        y + height > image.height
      ) {
        throw new Error('Invalid example crop.')
      }
      const canvas = document.createElement('canvas')
      canvas.width = width
      canvas.height = height
      const ctx = canvas.getContext('2d')
      if (!ctx) throw new Error('Canvas is unavailable.')
      ctx.drawImage(image, x, y, width, height, 0, 0, width, height)
      const blob = await new Promise<Blob>((resolve, reject) =>
        canvas.toBlob(
          (value) =>
            value
              ? resolve(value)
              : reject(new Error('Could not encode image.')),
          'image/png'
        )
      )
      assets.push(await referenceAsset(blob))
    }
    return assets
  } finally {
    bitmaps.forEach((image) => image.close())
  }
}
