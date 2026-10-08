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
import { render, screen } from '@testing-library/react'
import { afterAll, beforeAll, expect, test, vi } from 'vitest'

// The deployment prefix is resolved once, when the module graph loads. Setting
// it before the gallery is pulled in -- which is why the component is imported
// dynamically below rather than at the top of the file -- is what makes it see a
// prefixed deployment.
beforeAll(() => {
  window.__BASE_PATH__ = '/cuberouter'
})

afterAll(() => {
  delete window.__BASE_PATH__
})

// Regression: the catalog stores its previews as root-absolute paths, and the
// browser resolves a leading "/" against the origin root. Unwrapped, every card
// asked for /studio-templates/... and the image 404ed whenever the dashboard was
// published under BASE_PATH.
test('serves template previews from under the deployment prefix', async () => {
  const { TemplateGallery } = await import('../components/template-gallery')

  render(<TemplateGallery disabled={false} onApply={vi.fn()} />)

  const sources = screen
    .getAllByRole('img')
    .map((image) => image.getAttribute('src') ?? '')

  expect(sources).toContain(
    '/cuberouter/studio-templates/animal-infographic-v1.jpg'
  )
  for (const source of sources) {
    expect(source).toMatch(/^\/cuberouter\/studio-templates\//)
  }
})
