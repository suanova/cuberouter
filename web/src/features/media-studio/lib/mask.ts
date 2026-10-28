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
// 框选区域编辑：用户在参考图上拉出的矩形以归一化坐标（0..1）存进草稿，生成时再按
// 实际下发的参考图尺寸栅格化成 PNG 蒙版。存矩形而不是存位图，是因为下发前参考图会
// 被缩到长边 1024（见 reference-image.ts）：位图蒙版一旦缩放就和图片错位，而矩形
// 在两个尺寸里指向同一块区域。
import { canDecodeImages, decodeImage } from './reference-image'

/** 归一化矩形：x/y 是左上角，相对参考图宽高的比例。 */
export interface MaskRegion {
  x: number
  y: number
  width: number
  height: number
}

/** 归一化坐标。 */
export interface MaskPoint {
  x: number
  y: number
}

/** 短边小于该比例的框是误触（单击、手抖），不记录。 */
export const MIN_MASK_SIDE = 0.01

function clamp(value: number): number {
  return Math.min(1, Math.max(0, value))
}

/**
 * 由一次拖拽的起点与终点得到归一化矩形。
 * 方向无关（右下往左上拖同样成立），坐标裁剪到画面内；太小或含非有限数时返回 null。
 */
export function maskRegion(from: MaskPoint, to: MaskPoint): MaskRegion | null {
  if (![from.x, from.y, to.x, to.y].every(Number.isFinite)) {
    return null
  }
  const x = clamp(Math.min(from.x, to.x))
  const y = clamp(Math.min(from.y, to.y))
  const width = clamp(Math.max(from.x, to.x)) - x
  const height = clamp(Math.max(from.y, to.y)) - y
  if (width < MIN_MASK_SIDE || height < MIN_MASK_SIDE) {
    return null
  }
  return { x, y, width, height }
}

/** 归一化矩形 → 像素矩形。裁剪到画布内；落在画布外或不足 1 像素时返回 null。 */
export function regionPixels(
  region: MaskRegion,
  width: number,
  height: number
): { x: number; y: number; width: number; height: number } | null {
  if (!(width > 0) || !(height > 0)) {
    return null
  }
  const x = Math.floor(clamp(region.x) * width)
  const y = Math.floor(clamp(region.y) * height)
  const right = Math.ceil(clamp(region.x + region.width) * width)
  const bottom = Math.ceil(clamp(region.y + region.height) * height)
  if (right <= x || bottom <= y) {
    return null
  }
  return {
    x,
    y,
    width: Math.min(right - x, width - x),
    height: Math.min(bottom - y, height - y),
  }
}

/**
 * 栅格化蒙版：与参考图同尺寸的 PNG，框选区域透明、其余不透明。
 *
 * alpha 才是 /v1/images/edits 的 mask 语义（OpenAI：alpha 为 0 的区域会被重绘），
 * 所以这里先铺满不透明黑，再用 destination-out 把框选区域擦成透明——不能画白块，
 * 白块在不透明画布上等于「整张图都不重绘」。
 *
 * 解不出图或本环境没有 canvas（jsdom 测试）时返回 undefined：调用方不下发 mask，
 * 退化成整图编辑，绝不发一张尺寸对不上或全不透明的蒙版让上游静默改错区域。
 */
export async function maskImage(
  baseURL: string,
  regions: MaskRegion[]
): Promise<string | undefined> {
  if (!baseURL || !regions.length || !canDecodeImages()) {
    return undefined
  }
  try {
    const image = await decodeImage(baseURL)
    if (!image) {
      return undefined
    }
    const canvas = document.createElement('canvas')
    canvas.width = image.naturalWidth
    canvas.height = image.naturalHeight
    const context = canvas.getContext('2d')
    if (!context || canvas.width === 0 || canvas.height === 0) {
      return undefined
    }
    context.fillStyle = '#000000'
    context.fillRect(0, 0, canvas.width, canvas.height)
    context.globalCompositeOperation = 'destination-out'
    for (const region of regions) {
      const box = regionPixels(region, canvas.width, canvas.height)
      if (box) {
        context.fillRect(box.x, box.y, box.width, box.height)
      }
    }
    return canvas.toDataURL('image/png')
  } catch {
    return undefined
  }
}
