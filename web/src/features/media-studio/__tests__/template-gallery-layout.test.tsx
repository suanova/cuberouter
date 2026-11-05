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
import { fireEvent, render, screen, within } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { TemplateGallery } from '../components/template-gallery'

test('template columns grow with gallery width when the sidebar leaves less room', () => {
  render(<TemplateGallery disabled={false} onApply={vi.fn()} />)
  const gallery = screen.getByRole('region', { name: 'Template gallery' })
  expect(gallery.querySelector('div.grid')).toHaveClass(
    'grid-cols-2',
    '@md/gallery:grid-cols-3',
    '@xl/gallery:grid-cols-4',
    '@3xl/gallery:grid-cols-5'
  )
})

test('all templates share one scroll area while the filters stay above it', () => {
  render(<TemplateGallery disabled={false} onApply={vi.fn()} />)
  const viewport = screen.getByRole('region', { name: 'Templates' })
  const animals = screen.getByRole('button', { name: 'Animals' })
  expect(viewport).toHaveClass('overflow-y-auto')
  expect(viewport).toHaveAttribute('tabindex', '0')
  expect(viewport).not.toContainElement(animals)
  expect(
    within(viewport).getByAltText('Animal infographic')
  ).toBeInTheDocument()
  expect(
    within(viewport).getByAltText('Photo to watercolor')
  ).toBeInTheDocument()
})

test('choosing a template filter resets the gallery scroll position', () => {
  render(<TemplateGallery disabled={false} onApply={vi.fn()} />)
  const viewport = screen.getByRole('region', { name: 'Templates' })
  viewport.scrollTop = 100

  fireEvent.click(screen.getByRole('button', { name: 'Animals' }))
  expect(screen.getByRole('region', { name: 'Templates' }).scrollTop).toBe(0)
  expect(screen.getByAltText('Animal infographic')).toBeVisible()
  expect(screen.queryByAltText('City travel poster')).not.toBeInTheDocument()
})

test('the pet comic template lets the long story wrap as it is edited', () => {
  render(<TemplateGallery disabled={false} onApply={vi.fn()} />)
  fireEvent.click(screen.getByRole('button', { name: /Pet comic strip/ }))

  expect(
    screen.getByRole('textbox', { name: 'Four-panel story' })
  ).toHaveProperty('type', 'textarea')
})

test('the template action stays outside the scrolling content on short screens', () => {
  render(<TemplateGallery disabled={false} onApply={vi.fn()} />)
  fireEvent.click(screen.getByRole('button', { name: /Pet comic strip/ }))

  const story = screen.getByRole('textbox', { name: 'Four-panel story' })
  const apply = screen.getByRole('button', { name: 'Use this template' })
  expect(story.parentElement?.closest('.overflow-y-auto')).toContainElement(
    story
  )
  expect(apply.closest('.overflow-y-auto')).toBeNull()
})
