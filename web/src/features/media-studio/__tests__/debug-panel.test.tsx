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
import { render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, describe, expect, test } from 'vitest'

import { DebugPanel } from '../components/debug-panel'

describe('DebugPanel', () => {
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      'Request and response': 'Request and response',
      Copy: 'Copy',
    })
  })

  test('truncates oversized strings so b64 blobs cannot bloat the panel', () => {
    const blob = 'iVBOR'.padEnd(10000, 'x')
    render(
      <DebugPanel
        requestBody={{ prompt: 'a cat' }}
        rawResponse={{ created: 1, data: [{ b64_json: blob }] }}
      />
    )

    const panel = screen.getByRole('region', { name: 'Request and response' })
    expect(panel.textContent).toContain('iVBORxxxxxxxx')
    expect(panel.textContent).toContain('(10000 chars total)')
    expect(panel.textContent).not.toContain(blob.slice(0, 250))
  })

  test('renders short payloads in full', () => {
    render(
      <DebugPanel
        requestBody={null}
        rawResponse={{
          created: 1,
          data: [{ url: 'https://cdn.example/img.png' }],
        }}
      />
    )

    const panel = screen.getByRole('region', { name: 'Request and response' })
    expect(panel.textContent).toContain('https://cdn.example/img.png')
  })
})
