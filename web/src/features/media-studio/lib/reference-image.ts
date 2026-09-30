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
// 图生图的参考图在下发前缩到长边 REFERENCE_LONG_EDGE 并重新编码。多出来的分辨率模型
// 侧用不到：Qwen-Image-Edit-2511 的处理器会把每张输入缩到约 1024x1024 的面积
// （max_pixels 12845056，边长再向下对齐到 8 的倍数），超过约 1 MP 的部分在生成前就被
// 丢弃。而一张 4032x3024 手机照片原样 base64 是约 13 MB，三张约 40 MB，已经贴到
// ingress 的 proxy-body-size 与 relay 的 MAX_REQUEST_BODY_MB 上；缩到 1024 之后单张
// 约 200~400 KB，模型实际能用的信息一点没少。
//
// 只在「下发时」处理：draft 与本地历史保留用户原始字节，「原图对照」面板、下载与历史
// 都不受影响。「继续编辑」会把上一轮的结果图当参考图，若改在选图时归一化，每编辑一次
// 就多叠一代有损编码，而同一份原始字节无论下发多少次，编码结果都是一样的。
//
// 本模块的契约是「绝不因为压缩失败让生成失败」：任何一步出问题都原样返回入参。

export const REFERENCE_LONG_EDGE = 1024

/** 有损编码质量，WebP 与白底 JPEG 共用。 */
export const REFERENCE_QUALITY = 0.9

type Size = { width: number; height: number }

function isPositive(value: number): boolean {
  return Number.isFinite(value) && value > 0
}

/**
 * 纯函数：目标下发尺寸。只缩不放。
 *
 * 宽高或上限非法（非有限数、<= 0）时返回 null，调用方据此原样下发。返回值每维至少为
 * 1：4000x1 在 cap 1024 下按比例算出的高度是 0.256，四舍五入成 0 会得到零高度画布。
 */
export function referenceScale(
  width: number,
  height: number,
  cap: number = REFERENCE_LONG_EDGE
): Size | null {
  if (!isPositive(width) || !isPositive(height) || !isPositive(cap)) {
    return null
  }
  const longest = Math.max(width, height)
  if (longest <= cap) {
    return { width: Math.round(width), height: Math.round(height) }
  }
  const ratio = cap / longest
  return {
    width: Math.max(1, Math.round(width * ratio)),
    height: Math.max(1, Math.round(height * ratio)),
  }
}

/**
 * 这个环境能不能做逐像素的缩放与编码。
 *
 * 必须先探测再解码，顺序反过来会挂死：本仓测试用的 jsdom 没有装 canvas 包，而 jsdom 的
 * HTMLImageElement 在 Canvas 为 null 时直接从 _updateTheImageData 返回，给 <img> 赋 src
 * 既不会触发 load 也不会触发 error——只监听这两个事件的 promise 会永久 pending。真实
 * 浏览器里 data: URL 一定会 settle（解码失败走 error）。
 */
function canDecodeImages(): boolean {
  if (typeof document === 'undefined') {
    return false
  }
  return document.createElement('canvas').getContext('2d') !== null
}

/** 解码 data URL；失败或本环境解不了时返回 null。 */
function decode(dataURL: string): Promise<HTMLImageElement | null> {
  return new Promise((resolve) => {
    const image = new Image()
    // 监听器必须先于 src 注册，否则事件可能在注册前就派发掉了。
    image.addEventListener('load', () => resolve(image), { once: true })
    image.addEventListener('error', () => resolve(null), { once: true })
    image.src = dataURL
  })
}

/**
 * 按指定类型编码，产物类型不对就当失败。
 *
 * toDataURL 对不支持的类型不会报错，而是静默回退成 PNG，所以只能核对产物本身。
 * 前缀由浏览器生成、恒为小写，这里仍归一一次以防万一。
 */
function encodeAs(
  canvas: HTMLCanvasElement,
  type: string,
  quality?: number
): string | null {
  const encoded = canvas.toDataURL(type, quality)
  return encoded.toLowerCase().startsWith(`data:${type}`) ? encoded : null
}

/** 一块按目标尺寸铺好图、且平滑设置已生效的画布。 */
function drawScaled(
  image: HTMLImageElement,
  target: Size
): HTMLCanvasElement | null {
  const canvas = document.createElement('canvas')
  // 顺序要紧：给 canvas 赋宽高会重置 2D 上下文状态，平滑设置必须在赋值之后再写，
  // 放在前面会被这次重置清掉。
  canvas.width = target.width
  canvas.height = target.height
  const context = canvas.getContext('2d')
  if (!context) {
    return null
  }
  context.imageSmoothingEnabled = true
  context.imageSmoothingQuality = 'high'
  context.drawImage(image, 0, 0, target.width, target.height)
  return canvas
}

/**
 * 白底 JPEG：JPEG 没有 alpha 通道，透明背景的 PNG 直接编码会变成黑底。
 *
 * 用一块新画布而不是在缩放画布上铺底，否则万一 JPEG 这一步也失败，最后回退出的 PNG 就
 * 带着一层白底，透明背景等于永久丢了。
 */
function encodeOnWhite(
  image: HTMLImageElement,
  target: Size
): string | null {
  const canvas = document.createElement('canvas')
  canvas.width = target.width
  canvas.height = target.height
  const context = canvas.getContext('2d')
  if (!context) {
    return null
  }
  // 必须先铺底再画图，反过来的话 fillRect 会把图整个盖掉。
  context.fillStyle = '#ffffff'
  context.fillRect(0, 0, target.width, target.height)
  context.imageSmoothingEnabled = true
  context.imageSmoothingQuality = 'high'
  context.drawImage(image, 0, 0, target.width, target.height)
  return encodeAs(canvas, 'image/jpeg', REFERENCE_QUALITY)
}

/**
 * 编码阶梯，按保真度降序：WebP（保留 alpha）→ 白底 JPEG → 只缩放的 PNG。
 * 返回 null 表示三条路都没走通。
 */
function encode(
  image: HTMLImageElement,
  target: Size
): string | null {
  const canvas = drawScaled(image, target)
  if (!canvas) {
    return null
  }
  const webp = encodeAs(canvas, 'image/webp', REFERENCE_QUALITY)
  if (webp) {
    return webp
  }
  const jpeg = encodeOnWhite(image, target)
  if (jpeg) {
    return jpeg
  }
  return encodeAs(canvas, 'image/png')
}

/**
 * 归一化一张参考图。入参不是 data URL（本模块不代拉远程图，那是 localImage 的职责）、
 * 解码不了、编码不了，或者结果反而更长时，一律原样返回。
 *
 * 最后那个「取更短的」是硬保证：请求体永远不会因为这一步变大。代价是当原始串更短时，
 * 下发尺寸可能仍大于 REFERENCE_LONG_EDGE（例如一张纯色 4000x3000 PNG 只有几十 KB，
 * 它的 1024 WebP 未必更小）。这可以接受：模型本来就会把输入缩到约 1 MP，而这条规则
 * 保证了本模块不会把事情弄糟。
 */
export async function normalizeReference(dataURL: string): Promise<string> {
  if (!/^data:/i.test(dataURL)) {
    return dataURL
  }
  if (!canDecodeImages()) {
    return dataURL
  }
  try {
    const image = await decode(dataURL)
    if (!image) {
      return dataURL
    }
    const target = referenceScale(image.naturalWidth, image.naturalHeight)
    if (!target) {
      return dataURL
    }
    const encoded = encode(image, target)
    if (!encoded) {
      return dataURL
    }
    return encoded.length < dataURL.length ? encoded : dataURL
  } catch {
    return dataURL
  }
}

/**
 * 逐张归一化，串行而非 Promise.all：一张 4032x3024 的参考图解码后约 49 MB（宽高相乘再
 * 乘 4 字节），三张并发就是 150 MB 上下，足以让移动端把标签页杀掉。整个请求本来就要跑
 * 几十秒到几分钟，每张多等几百毫秒无关紧要。
 */
export async function normalizeReferences(
  dataURLs: string[]
): Promise<string[]> {
  const normalized: string[] = []
  for (const dataURL of dataURLs) {
    normalized.push(await normalizeReference(dataURL))
  }
  return normalized
}
