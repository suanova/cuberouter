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
import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import {
  MAX_LOGO_IMAGE_BYTES,
  isValidLogoFile,
  isValidLogoValue,
} from '../general/logo-value'

describe('logo option value', () => {
  test('allows an empty value and absolute HTTP(S) URLs', () => {
    for (const value of [
      '',
      'https://cdn.example.com/logo.png',
      'http://127.0.0.1:8080/logo.svg',
    ]) {
      assert.equal(isValidLogoValue(value), true, value)
    }
  })

  test('rejects relative, non-HTTP, and malformed URLs', () => {
    for (const value of [
      '/head.png',
      'logo.png',
      'ftp://example.com/logo.png',
      'javascript:alert(1)',
      'data:text/html;base64,PHNjcmlwdD4=',
      'https://',
    ]) {
      assert.equal(isValidLogoValue(value), false, value)
    }
  })

  test('accepts a base64 image data URL', () => {
    assert.equal(
      isValidLogoValue(
        `data:image/png;base64,${btoa(String.fromCharCode(137, 80, 78, 71))}`
      ),
      true
    )
  })

  test('rejects non-base64 and oversized data URLs', () => {
    assert.equal(isValidLogoValue('data:image/png,not-base64'), false)
    assert.equal(
      isValidLogoValue(
        `data:image/png;base64,${'A'.repeat(MAX_LOGO_IMAGE_BYTES * 2)}`
      ),
      false
    )
  })
})

describe('logo file selection', () => {
  const fileOf = (type: string, size: number) =>
    new File([new Uint8Array(size)], 'logo', { type })

  test('accepts a supported image within the size limit', () => {
    for (const type of [
      'image/png',
      'image/jpeg',
      'image/webp',
      'image/gif',
      'image/svg+xml',
    ]) {
      assert.equal(isValidLogoFile(fileOf(type, 1024)), true, type)
    }
  })

  test('rejects unsupported types, empty files, and oversized images', () => {
    assert.equal(isValidLogoFile(fileOf('image/bmp', 1024)), false)
    assert.equal(isValidLogoFile(fileOf('text/plain', 1024)), false)
    assert.equal(isValidLogoFile(fileOf('image/png', 0)), false)
    assert.equal(
      isValidLogoFile(fileOf('image/png', MAX_LOGO_IMAGE_BYTES + 1)),
      false
    )
  })
})
