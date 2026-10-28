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
import { expect, test } from 'vitest'

import { maskImage, maskRegion, regionPixels } from '../lib/mask'

test('a drag becomes a normalized box no matter which corner it starts from', () => {
  // 右下往左上拖与左上往右下拖必须是同一个框：用户不关心方向。
  const expected = { x: 0.25, y: 0.5, width: 0.5, height: 0.25 }
  expect(maskRegion({ x: 0.25, y: 0.5 }, { x: 0.75, y: 0.75 })).toEqual(
    expected
  )
  expect(maskRegion({ x: 0.75, y: 0.75 }, { x: 0.25, y: 0.5 })).toEqual(
    expected
  )
})

test('a drag past the edge is clipped to the image', () => {
  const clipped = maskRegion({ x: 0.8, y: 0.8 }, { x: 1.4, y: 1.2 })
  expect(clipped).not.toBeNull()
  expect(clipped?.x).toBeCloseTo(0.8)
  expect(clipped?.y).toBeCloseTo(0.8)
  expect(clipped?.width).toBeCloseTo(0.2)
  expect(clipped?.height).toBeCloseTo(0.2)
})

test('a click or a jitter smaller than the minimum side is not a box', () => {
  // 误触不能变成 1 像素的蒙版区域：那种框既看不见，也会让上游以为用户只想改一个点。
  expect(maskRegion({ x: 0.4, y: 0.4 }, { x: 0.4, y: 0.4 })).toBeNull()
  expect(maskRegion({ x: 0.4, y: 0.4 }, { x: 0.405, y: 0.9 })).toBeNull()
  expect(maskRegion({ x: Number.NaN, y: 0 }, { x: 0.5, y: 0.5 })).toBeNull()
})

test('normalized boxes map onto the pixels of the image sent upstream', () => {
  // 蒙版按归一化坐标换算，所以参考图缩到长边 1024 之后框选位置仍然对得上。
  expect(
    regionPixels({ x: 0.25, y: 0.5, width: 0.5, height: 0.25 }, 1024, 512)
  ).toEqual({ x: 256, y: 256, width: 512, height: 128 })
})

test('a degenerate box or a zero-sized image produces no pixels', () => {
  expect(
    regionPixels({ x: 1, y: 0, width: 0.01, height: 0.5 }, 100, 100)
  ).toBeNull()
  expect(
    regionPixels({ x: 0, y: 0, width: 0.5, height: 0.5 }, 0, 100)
  ).toBeNull()
})

test('no mask is produced when this environment cannot rasterize one', async () => {
  // jsdom 没有 canvas：必须返回 undefined 让调用方退回整图编辑，而不是抛错或发空蒙版。
  await expect(
    maskImage('data:image/png;base64,YQ==', [
      { x: 0.1, y: 0.1, width: 0.2, height: 0.2 },
    ])
  ).resolves.toBeUndefined()
  await expect(
    maskImage('data:image/png;base64,YQ==', [])
  ).resolves.toBeUndefined()
})
