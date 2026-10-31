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
import { act, fireEvent, render, screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { ReferenceEditor } from '../components/reference-editor'
import { markPath, validImageMark } from '../lib/reference-markup'

const image = {
  id: 'photo-a',
  mime: 'image/png',
  url: 'data:image/png;base64,YQ==',
}

test('coordinate entry, undo and redo preserve the selected reference and normalized rectangle', () => {
  const apply = vi.fn()
  render(
    <ReferenceEditor image={image} regional onClose={vi.fn()} onApply={apply} />
  )
  expect(screen.getByRole('button', { name: 'Use drawing' })).toBeDisabled()
  fireEvent.click(screen.getByRole('button', { name: 'Add rectangle' }))
  fireEvent.click(screen.getByRole('button', { name: 'Undo' }))
  expect(screen.getByRole('button', { name: 'Use drawing' })).toBeDisabled()
  fireEvent.click(screen.getByRole('button', { name: 'Redo' }))
  fireEvent.click(screen.getByRole('button', { name: 'Use drawing' }))
  expect(apply).toHaveBeenCalledWith(
    expect.objectContaining({
      referenceID: 'photo-a',
      kind: 'region',
      marks: [
        expect.objectContaining({
          tool: 'rectangle',
          points: [
            { x: 0.1, y: 0.1 },
            { x: 0.5, y: 0.5 },
          ],
        }),
      ],
    })
  )
})

test('annotation-only models expose colored guides and reject coordinates outside the image', () => {
  render(
    <ReferenceEditor
      image={image}
      regional={false}
      onClose={vi.fn()}
      onApply={vi.fn()}
    />
  )
  expect(
    screen.queryByRole('button', { name: 'Region selection' })
  ).not.toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Stroke color'), {
    target: { value: 'red' },
  })
  fireEvent.change(screen.getByLabelText('Rectangle X'), {
    target: { value: '99' },
  })
  expect(screen.getByRole('button', { name: 'Add rectangle' })).toBeDisabled()
})

test('malformed or unbounded paths cannot be rasterized as a valid region', () => {
  const mark = {
    tool: 'lasso' as const,
    color: 'blue' as const,
    width: 0.02,
    points: [
      { x: 0, y: 0 },
      { x: 0.5, y: 0.5 },
      { x: 1, y: 0 },
    ],
  }
  expect(validImageMark(mark)).toBe(true)
  expect(markPath(mark)).toMatch(/Z$/)
  expect(validImageMark({ ...mark, points: [{ x: Infinity, y: 0 }] })).toBe(
    false
  )
  expect(markPath({ ...mark, width: Number.NaN })).toBe('')
})

test.each(['brush', 'lasso'] as const)(
  'keyboard entry creates a %s mark with the selected tool',
  (tool) => {
    const apply = vi.fn()
    render(
      <ReferenceEditor
        image={image}
        regional
        onClose={vi.fn()}
        onApply={apply}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: tool }))
    const surface = screen.getByLabelText('Reference drawing surface')
    expect(surface).toHaveAttribute('tabindex', '0')
    expect(surface.querySelector('circle')).toBeNull()
    act(() => surface.focus())
    expect(surface).toHaveFocus()
    expect(surface.querySelector('circle')).toBeVisible()
    expect(screen.getByRole('status')).toHaveTextContent(
      'Keyboard cursor: X 50%, Y 50%.'
    )
    fireEvent.keyDown(surface, { key: ' ' })
    fireEvent.keyDown(surface, { key: 'ArrowRight' })
    fireEvent.keyDown(surface, { key: ' ' })
    fireEvent.keyDown(surface, { key: 'ArrowDown' })
    expect(surface.querySelector('circle')).toHaveAttribute('cx', '0.51')
    expect(screen.getByRole('status')).toHaveTextContent(
      'Keyboard cursor: X 51%, Y 51%.'
    )
    fireEvent.keyDown(surface, { key: ' ' })
    fireEvent.keyDown(surface, { key: 'Enter' })
    fireEvent.click(screen.getByRole('button', { name: 'Use drawing' }))
    expect(apply).toHaveBeenCalledWith(
      expect.objectContaining({
        marks: [
          expect.objectContaining({
            tool,
            points: [
              { x: 0.5, y: 0.5 },
              { x: 0.51, y: 0.5 },
              { x: 0.51, y: 0.51 },
            ],
          }),
        ],
      })
    )
    act(() => surface.blur())
    expect(surface.querySelector('circle')).toBeNull()
  }
)

test('a pointer stroke at the point limit survives release', () => {
  const apply = vi.fn()
  render(
    <ReferenceEditor image={image} regional onClose={vi.fn()} onApply={apply} />
  )
  const surface = screen.getByLabelText('Reference drawing surface')
  vi.spyOn(surface, 'getBoundingClientRect').mockReturnValue({
    left: 0,
    top: 0,
    width: 100,
    height: 100,
    right: 100,
    bottom: 100,
    x: 0,
    y: 0,
    toJSON: () => ({}),
  })
  fireEvent.pointerDown(surface, { pointerId: 1, clientX: 10, clientY: 10 })
  act(() => {
    for (let i = 1; i < 2048; i++) {
      fireEvent.pointerMove(surface, { pointerId: 1, clientX: 20, clientY: 20 })
    }
  })
  fireEvent.pointerUp(surface, { pointerId: 1, clientX: 30, clientY: 30 })
  expect(screen.getByRole('button', { name: 'Use drawing' })).toBeEnabled()
  fireEvent.click(screen.getByRole('button', { name: 'Use drawing' }))
  expect(apply.mock.calls[0][0].marks[0].points).toHaveLength(2048)
})
