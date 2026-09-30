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
import { afterEach, describe, expect, test, vi } from 'vitest'

import {
  REFERENCE_LONG_EDGE,
  normalizeReference,
  normalizeReferences,
  referenceScale,
} from '../lib/reference-image'

/** 比任何编码产物都长的原始串，「取更短的」那条规则才会选中编码结果。 */
const ORIGINAL = `data:image/png;base64,${'A'.repeat(4096)}`
const SHORT_WEBP = 'data:image/webp;base64,UklGRg=='
const SHORT_JPEG = 'data:image/jpeg;base64,/9j/4A=='
const SHORT_PNG = 'data:image/png;base64,iVBORw0KGgo='
const REMOTE = 'https://cdn.example/photo.png'

afterEach(() => {
  vi.unstubAllGlobals()
})

/** 记录画布上真正发生了什么，据此断言编码阶梯的走向与先后顺序。 */
class FakeContext {
  imageSmoothingEnabled = false
  imageSmoothingQuality = 'low'
  fillStyle = '#000000'
  readonly ops: string[] = []
  readonly draws: number[][] = []
  readonly fills: number[][] = []
  readonly fillStyles: string[] = []
  readonly width: number
  readonly height: number

  constructor(canvas: HTMLCanvasElement) {
    // 生产代码先定尺寸再取上下文，所以这里读到的就是目标尺寸。
    this.width = canvas.width
    this.height = canvas.height
  }

  drawImage(_image: unknown, ...args: number[]): void {
    this.ops.push('draw')
    this.draws.push(args)
  }

  fillRect(...args: number[]): void {
    this.ops.push('fill')
    this.fills.push(args)
    this.fillStyles.push(this.fillStyle)
  }
}

type CanvasEntry = { canvas: HTMLCanvasElement; context: FakeContext }
type FakeSource = { width: number; height: number; fail?: boolean }

/**
 * 用可控的假 Image 顶替解码。load/error 在给 src 赋值时同步派发，这样「监听器注册晚于
 * 赋值 src」会立刻暴露，而不是变成偶发失败。
 */
function fakeImageClass(source: FakeSource): unknown {
  return class {
    naturalWidth = 0
    naturalHeight = 0
    private readonly handlers = new Map<string, () => void>()
    private readonly dispatch: (value: string) => void

    constructor() {
      this.dispatch = () => {
        if (source.fail === true) {
          this.handlers.get('error')?.()
          return
        }
        this.naturalWidth = source.width
        this.naturalHeight = source.height
        this.handlers.get('load')?.()
      }
    }

    addEventListener(type: string, listener: () => void): void {
      this.handlers.set(type, listener)
    }

    set src(value: string) {
      this.dispatch(value)
    }
  }
}

/**
 * 装上解码与编码两端的打桩实现。encode 按请求的类型返回产物，返回别的类型就是在模拟
 * 「浏览器不支持这种编码」。
 */
function installCanvas(options: {
  source: FakeSource
  encode: (type: string, quality?: number) => string | null
}): CanvasEntry[] {
  const entries: CanvasEntry[] = []
  vi.stubGlobal('Image', fakeImageClass(options.source))
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockImplementation(
    function (this: HTMLCanvasElement) {
      const context = new FakeContext(this)
      entries.push({ canvas: this, context })
      return context as unknown as CanvasRenderingContext2D
    }
  )
  vi.spyOn(HTMLCanvasElement.prototype, 'toDataURL').mockImplementation(
    function (type?: string, quality?: number) {
      return options.encode(type ?? '', quality) ?? 'data:,'
    }
  )
  return entries
}

/** 只保留真正画了图的画布：探测用的那块会被排除。 */
function drawnEntries(entries: CanvasEntry[]): CanvasEntry[] {
  return entries.filter((entry) => entry.context.draws.length > 0)
}

describe('referenceScale', () => {
  test('caps the long edge and keeps the aspect ratio', () => {
    expect(referenceScale(4032, 3024)).toEqual({ width: 1024, height: 768 })
    expect(referenceScale(3024, 4032)).toEqual({ width: 768, height: 1024 })
    expect(referenceScale(2048, 2048)).toEqual({ width: 1024, height: 1024 })
  })

  test('never upscales an image that is already within the cap', () => {
    expect(referenceScale(800, 600)).toEqual({ width: 800, height: 600 })
    expect(referenceScale(1024, 1024)).toEqual({ width: 1024, height: 1024 })
  })

  test('clamps a sliver to at least one pixel instead of collapsing to zero', () => {
    // 1 * (1024 / 4000) 四舍五入是 0，不夹紧就会得到一块零高度画布。
    expect(referenceScale(4000, 1)).toEqual({ width: 1024, height: 1 })
  })

  test('honours an explicit cap', () => {
    expect(referenceScale(4000, 2000, 2048)).toEqual({
      width: 2048,
      height: 1024,
    })
  })

  test('rejects dimensions or a cap that cannot describe an image', () => {
    expect(referenceScale(0, 100)).toBeNull()
    expect(referenceScale(100, 0)).toBeNull()
    expect(referenceScale(-1, 100)).toBeNull()
    expect(referenceScale(Number.NaN, 100)).toBeNull()
    expect(referenceScale(100, Number.POSITIVE_INFINITY)).toBeNull()
    expect(referenceScale(100, 100, 0)).toBeNull()
  })
})

describe('normalizeReference', () => {
  test('an environment without a 2D canvas sends the original bytes unchanged', async () => {
    // 本仓的 jsdom 没装 canvas 包，这是真实存在的降级路径；也正因为归一化在这里直接
    // 返回，整套编辑模式的测试才不会卡在一次永不到来的解码上。
    await expect(normalizeReference(ORIGINAL)).resolves.toBe(ORIGINAL)
  })

  test('a remote URL is passed through without being fetched', async () => {
    // 代拉远程图是 localImage 的职责；这里只处理已经内联好的 data URL。
    const fetcher = vi.fn()
    vi.stubGlobal('fetch', fetcher)
    const entries = installCanvas({
      source: { width: 10, height: 10 },
      encode: () => null,
    })
    await expect(normalizeReference(REMOTE)).resolves.toBe(REMOTE)
    expect(fetcher).not.toHaveBeenCalled()
    expect(entries).toEqual([])
  })

  test('scales a 12 MP reference to the cap and re-encodes it as WebP', async () => {
    const encode = vi.fn((type: string) =>
      type === 'image/webp' ? SHORT_WEBP : null
    )
    const entries = installCanvas({
      source: { width: 4032, height: 3024 },
      encode,
    })

    await expect(normalizeReference(ORIGINAL)).resolves.toBe(SHORT_WEBP)

    const drawn = drawnEntries(entries)
    expect(drawn).toHaveLength(1)
    expect(drawn[0].canvas.width).toBe(REFERENCE_LONG_EDGE)
    expect(drawn[0].canvas.height).toBe(768)
    expect(drawn[0].context.draws).toEqual([[0, 0, REFERENCE_LONG_EDGE, 768]])
    // 给 canvas 赋宽高会重置上下文状态，所以平滑设置必须写在赋值之后才生效。
    expect(drawn[0].context.imageSmoothingEnabled).toBe(true)
    expect(drawn[0].context.imageSmoothingQuality).toBe('high')
    expect(encode).toHaveBeenCalledWith('image/webp', 0.9)
  })

  test('falls back to JPEG on a white matte when WebP encoding is unavailable', async () => {
    // 老 Safari 对 toDataURL('image/webp') 不报错，只是给一张 PNG。不铺白底的话，
    // 透明背景的参考图会变成黑底。
    const entries = installCanvas({
      source: { width: 4032, height: 3024 },
      encode: (type) => (type === 'image/webp' ? SHORT_PNG : SHORT_JPEG),
    })

    await expect(normalizeReference(ORIGINAL)).resolves.toBe(SHORT_JPEG)

    const drawn = drawnEntries(entries)
    expect(drawn).toHaveLength(2)
    // 缩放画布保持干净，白底只出现在那块专门的画布上，最后回退出的 PNG 才不会带白底。
    expect(drawn[0].context.fills).toEqual([])
    expect(drawn[1].context.fills).toEqual([[0, 0, 1024, 768]])
    expect(drawn[1].context.fillStyles).toEqual(['#ffffff'])
    // 必须先铺底再画图，反过来的话 fillRect 会把图整个盖掉。
    expect(drawn[1].context.ops).toEqual(['fill', 'draw'])
  })

  test('still returns a resized PNG when no lossy encoder answers', async () => {
    const entries = installCanvas({
      source: { width: 4032, height: 3024 },
      encode: () => SHORT_PNG,
    })

    await expect(normalizeReference(ORIGINAL)).resolves.toBe(SHORT_PNG)
    // 只缩放不重编码也拿走了绝大部分收益，所以这条兜底不能变成失败。
    expect(drawnEntries(entries)).toHaveLength(2)
  })

  test('discards an encode that grows the payload', async () => {
    // 「取更短的」是硬保证：请求体不会因为这一步变大。
    const original = 'data:image/png;base64,iVBORw0KGgo='
    const bulky = `data:image/webp;base64,${'B'.repeat(8192)}`
    installCanvas({
      source: { width: 4032, height: 3024 },
      encode: () => bulky,
    })

    await expect(normalizeReference(original)).resolves.toBe(original)
  })

  test('sends an undecodable reference unchanged', async () => {
    const encode = vi.fn(() => SHORT_WEBP)
    const entries = installCanvas({
      source: { width: 0, height: 0, fail: true },
      encode,
    })

    await expect(normalizeReference(ORIGINAL)).resolves.toBe(ORIGINAL)
    expect(encode).not.toHaveBeenCalled()
    expect(drawnEntries(entries)).toHaveLength(0)
  })

  test('re-encodes a reference already within the cap when that shrinks it', async () => {
    // 尺寸不是成本本身：一张 1024x1024 的 PNG 也可能有好几 MB，所以只要编码结果更短
    // 就采用，而不是看到尺寸没超就跳过。
    const entries = installCanvas({
      source: { width: 1024, height: 1024 },
      encode: (type) => (type === 'image/webp' ? SHORT_WEBP : null),
    })

    await expect(normalizeReference(ORIGINAL)).resolves.toBe(SHORT_WEBP)
    expect(drawnEntries(entries)[0].canvas.width).toBe(1024)
  })
})

describe('normalizeReferences', () => {
  test('normalizes every reference and keeps the order', async () => {
    installCanvas({
      source: { width: 4032, height: 3024 },
      encode: (type) => (type === 'image/webp' ? SHORT_WEBP : null),
    })

    await expect(normalizeReferences([ORIGINAL, REMOTE])).resolves.toEqual([
      SHORT_WEBP,
      REMOTE,
    ])
  })

  test('an empty reference list stays empty', async () => {
    await expect(normalizeReferences([])).resolves.toEqual([])
  })
})
