/*
Copyright (C) 2023-2026 QuantumNous

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
import { describe, expect, test } from 'vitest'

import { b64ToDataUrl, extractImages } from '../lib/image-response'

describe('extractImages', () => {
  test('returns url items verbatim', () => {
    const body = { created: 1, data: [{ url: 'https://cdn.example/img.png' }] }

    expect(extractImages(body)).toEqual([
      { url: 'https://cdn.example/img.png' },
    ])
  })

  test('converts b64_json items to data URLs when url is absent', () => {
    const b64 = 'iVBORw0KGgoAAAANSUhEUg'
    const body = { created: 1, data: [{ b64_json: b64 }] }

    expect(extractImages(body)).toEqual([
      { url: `data:image/png;base64,${b64}` },
    ])
  })

  test('prefers url over b64_json when an item carries both', () => {
    const body = {
      data: [{ url: 'https://cdn.example/img.png', b64_json: 'iVBORw0KGgo' }],
    }

    expect(extractImages(body)).toEqual([
      { url: 'https://cdn.example/img.png' },
    ])
  })

  test('skips items that carry neither url nor b64_json', () => {
    const body = { data: [{ revised_prompt: 'a cat' }, null, 42] }

    expect(extractImages(body)).toEqual([])
  })

  test('returns an empty list when data is missing or not an array', () => {
    expect(extractImages({})).toEqual([])
    expect(extractImages({ data: 'not-an-array' })).toEqual([])
  })
})

describe('b64ToDataUrl', () => {
  test('detects the mime type from the base64 magic prefix', () => {
    const samples: Array<[string, string]> = [
      ['iVBORw0KGgoAAAAB', 'image/png'],
      ['/9j/4AAQSkZJRg', 'image/jpeg'],
      ['R0lGODlhAQAB', 'image/gif'],
      ['UklGRiQAAABX', 'image/webp'],
      ['Qk02BgAAA', 'image/bmp'],
    ]

    for (const [b64, mime] of samples) {
      expect(b64ToDataUrl(b64)).toBe(`data:${mime};base64,${b64}`)
    }
  })

  test('defaults to png when the magic prefix is unrecognized', () => {
    expect(b64ToDataUrl('AAAA')).toBe('data:image/png;base64,AAAA')
  })
})
