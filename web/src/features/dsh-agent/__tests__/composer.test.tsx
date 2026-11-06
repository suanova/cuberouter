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
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import { DshComposer } from '../components/dsh-composer'

function renderComposer(
  overrides: { isRunning?: boolean; disabled?: boolean } = {}
) {
  const onSubmit = vi.fn()
  render(
    <DshComposer
      disabled={overrides.disabled ?? false}
      isRunning={overrides.isRunning ?? false}
      onSubmit={onSubmit}
    />
  )
  return { onSubmit, textarea: screen.getByRole('textbox') }
}

describe('DshComposer', () => {
  test('sends the trimmed prompt on Enter', () => {
    const { onSubmit, textarea } = renderComposer()

    fireEvent.change(textarea, { target: { value: '  做一份 PPT  ' } })
    fireEvent.keyDown(textarea, { key: 'Enter' })

    expect(onSubmit).toHaveBeenCalledWith('做一份 PPT')
  })

  test('keeps Shift+Enter as a newline', () => {
    const { onSubmit, textarea } = renderComposer()

    fireEvent.change(textarea, { target: { value: '第一行' } })
    fireEvent.keyDown(textarea, { key: 'Enter', shiftKey: true })

    expect(onSubmit).not.toHaveBeenCalled()
  })

  test('does not send Enter used to confirm an IME composition', () => {
    const { onSubmit, textarea } = renderComposer()

    fireEvent.compositionStart(textarea)
    fireEvent.change(textarea, { target: { value: 'hello' } })
    fireEvent.keyDown(textarea, { key: 'Enter' })

    expect(onSubmit).not.toHaveBeenCalled()

    fireEvent.compositionEnd(textarea)
    fireEvent.keyDown(textarea, { key: 'Enter' })

    expect(onSubmit).toHaveBeenCalledWith('hello')
  })

  test('ignores Enter while a turn is running or the composer is disabled', () => {
    const running = renderComposer({ isRunning: true })
    fireEvent.change(running.textarea, { target: { value: '排队中' } })
    fireEvent.keyDown(running.textarea, { key: 'Enter' })
    expect(running.onSubmit).not.toHaveBeenCalled()
  })
})
