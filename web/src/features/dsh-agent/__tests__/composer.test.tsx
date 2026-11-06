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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { uploadDshAttachment } from '../api'
import { DshComposer } from '../components/dsh-composer'
import type { DshAttachment, DshUploadSettings } from '../types'

vi.mock('../api', () => ({
  uploadDshAttachment: vi.fn(),
  getRequestErrorMessage: vi.fn((error: unknown) => String(error)),
}))

const uploads: DshUploadSettings = {
  enabled: true,
  maxBytes: 25 * 1024 * 1024,
  maxFiles: 10,
  accept: ['.pdf', '.png'],
}

function renderComposer(
  overrides: {
    isRunning?: boolean
    disabled?: boolean
    uploads?: DshUploadSettings | null
  } = {}
) {
  const onSubmit = vi.fn()
  render(
    <DshComposer
      disabled={overrides.disabled ?? false}
      isRunning={overrides.isRunning ?? false}
      onSubmit={onSubmit}
      uploads={overrides.uploads === undefined ? uploads : overrides.uploads}
    />
  )
  return { onSubmit, textarea: screen.getByRole('textbox') }
}

function uploadFile(name = 'report.pdf'): DshAttachment {
  return {
    name,
    path: `/data/uploads/1/ab-${name}`,
    mimeType: 'application/pdf',
    size: 8,
  }
}

describe('DshComposer', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  test('sends the trimmed prompt on Enter', () => {
    const { onSubmit, textarea } = renderComposer()

    fireEvent.change(textarea, { target: { value: '  做一份 PPT  ' } })
    fireEvent.keyDown(textarea, { key: 'Enter' })

    expect(onSubmit).toHaveBeenCalledWith('做一份 PPT', [])
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

    expect(onSubmit).toHaveBeenCalledWith('hello', [])
  })

  test('ignores Enter while a turn is running or the composer is disabled', () => {
    const running = renderComposer({ isRunning: true })
    fireEvent.change(running.textarea, { target: { value: '排队中' } })
    fireEvent.keyDown(running.textarea, { key: 'Enter' })
    expect(running.onSubmit).not.toHaveBeenCalled()
  })

  test('uploads a file and submits the gateway path with the prompt', async () => {
    const attachment = uploadFile()
    vi.mocked(uploadDshAttachment).mockResolvedValue(attachment)
    const { onSubmit, textarea } = renderComposer()

    fireEvent.change(screen.getByLabelText('Upload file'), {
      target: {
        files: [
          new File(['%PDF-1.7'], 'report.pdf', { type: 'application/pdf' }),
        ],
      },
    })

    await waitFor(() => {
      expect(screen.getByText('report.pdf')).toBeInTheDocument()
    })
    fireEvent.change(textarea, { target: { value: '看看这份报告' } })
    fireEvent.keyDown(textarea, { key: 'Enter' })

    expect(onSubmit).toHaveBeenCalledWith('看看这份报告', [attachment])
  })

  test('does not send while an attachment is still uploading', () => {
    vi.mocked(uploadDshAttachment).mockReturnValue(new Promise(() => {}))
    const { onSubmit, textarea } = renderComposer()

    fireEvent.change(screen.getByLabelText('Upload file'), {
      target: { files: [new File(['x'], 'slow.pdf')] },
    })
    fireEvent.change(textarea, { target: { value: '等等附件' } })
    fireEvent.keyDown(textarea, { key: 'Enter' })

    expect(onSubmit).not.toHaveBeenCalled()
  })

  test('hides the attach control when uploads are not configured', () => {
    renderComposer({ uploads: { ...uploads, enabled: false } })

    expect(
      screen.queryByRole('button', { name: 'Attach' })
    ).not.toBeInTheDocument()
  })
})
