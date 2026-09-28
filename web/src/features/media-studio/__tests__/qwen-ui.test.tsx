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
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { QwenEditor } from '../components/qwen-editor'
import { QwenExamples } from '../components/qwen-examples'
import { initialDraft } from '../lib/workflow'
import type { WorkflowDraft } from '../workflow-types'

const context = {
  drawImage: vi.fn(),
  beginPath: vi.fn(),
  moveTo: vi.fn(),
  lineTo: vi.fn(),
  stroke: vi.fn(),
}
const draft: WorkflowDraft = {
  ...initialDraft,
  queued: true,
  mode: 'edit',
  model: 'qwen-image-2.1',
  size: '1120x736',
  cfg: 0,
  seed: 0,
  references: [
    {
      id: 'original',
      mime: 'image/png',
      url: 'data:image/png;base64,b3JpZ2luYWw=',
    },
  ],
}
beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => ({
      ok: true,
      blob: async () => new Blob(['source'], { type: 'image/png' }),
    }))
  )
  vi.stubGlobal(
    'createImageBitmap',
    vi.fn(async () => ({ width: 3000, height: 2000, close: vi.fn() }))
  )
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(
    context as unknown as CanvasRenderingContext2D
  )
  vi.spyOn(HTMLCanvasElement.prototype, 'toBlob').mockImplementation(
    (callback) => callback(new Blob(['marked-canvas'], { type: 'image/png' }))
  )
})
afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

test('loading an official sample returns an editable draft without a generation request', async () => {
  const apply = vi.fn()
  render(<QwenExamples draft={draft} disabled={false} onApply={apply} />)
  fireEvent.click(screen.getAllByRole('button', { name: 'Load example' })[0])
  await waitFor(() => expect(apply).toHaveBeenCalledTimes(1))
  const result = apply.mock.calls[0][0] as WorkflowDraft
  expect(result.references).toHaveLength(6)
  expect(result.mode).toBe('edit')
  expect(result.prompt.length).toBeGreaterThan(0)
  for (const [url, options] of vi.mocked(fetch).mock.calls) {
    expect(String(url)).toMatch(/^\/image-studio\/official-qwen21\//)
    expect(options?.method ?? 'GET').toBe('GET')
  }
})

test('an example asset failure preserves the current draft and allows another attempt', async () => {
  vi.mocked(fetch).mockRejectedValueOnce(new Error('offline'))
  const apply = vi.fn()
  render(<QwenExamples draft={draft} disabled={false} onApply={apply} />)
  fireEvent.click(screen.getAllByRole('button', { name: 'Load example' })[0])
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'could not be loaded'
  )
  expect(apply).not.toHaveBeenCalled()
  expect(
    screen.getAllByRole('button', { name: 'Load example' })[0]
  ).toBeEnabled()
})

test('the edit dialog has one instruction and preserves dimensions and zero parameters', async () => {
  const submit = vi.fn()
  render(
    <QwenEditor
      draft={draft}
      draw={false}
      busy={false}
      onClose={vi.fn()}
      onSubmit={submit}
    />
  )
  expect(screen.getAllByRole('textbox')).toHaveLength(1)
  expect(screen.getByText(/1120x736/)).toBeInTheDocument()
  fireEvent.change(
    screen.getByRole('textbox', { name: 'Describe your changes' }),
    { target: { value: 'Remove the moon' } }
  )
  const button = screen.getByRole('button', { name: 'Submit image edit' })
  await waitFor(() => expect(button).toBeEnabled())
  expect(submit).not.toHaveBeenCalled()
  fireEvent.click(button)
  await waitFor(() =>
    expect(submit).toHaveBeenCalledWith(
      expect.objectContaining({
        prompt: 'Remove the moon',
        size: '1120x736',
        cfg: 0,
        seed: 0,
        references: draft.references,
      })
    )
  )
})

test('colored editing submits the canvas bytes as the sole reference and retains the original', async () => {
  const submit = vi.fn()
  render(
    <QwenEditor
      draft={draft}
      draw
      busy={false}
      onClose={vi.fn()}
      onSubmit={submit}
    />
  )
  fireEvent.click(screen.getByRole('button', { name: 'White' }))
  expect(
    screen.getByRole('button', { name: 'White' })
  ).toHaveAttribute('aria-pressed', 'true')
  fireEvent.change(
    screen.getByRole('textbox', { name: 'Describe your changes' }),
    { target: { value: 'Add a diver at every white mark' } }
  )
  const button = screen.getByRole('button', { name: 'Submit image edit' })
  await waitFor(() => expect(button).toBeEnabled())
  fireEvent.click(button)
  await waitFor(() => expect(submit).toHaveBeenCalledTimes(1))
  const result = submit.mock.calls[0][0] as WorkflowDraft
  expect(result.references).toHaveLength(1)
  expect(result.references[0].url).toBe(
    'data:image/png;base64,bWFya2VkLWNhbnZhcw=='
  )
  expect(result.annotation?.original).toEqual(draft.references[0])
  expect(result.user_prompt).toBe('Add a diver at every white mark')
  expect(result.prompt).toContain(result.user_prompt)
})
