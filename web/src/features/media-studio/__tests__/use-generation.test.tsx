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
import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { generateImages, type GenerationApiResult } from '../api'
import { useGeneration } from '../hooks/use-generation'
import type { StudioParams } from '../types'

vi.mock('../api', () => ({
  generateImages: vi.fn(),
}))

const params: StudioParams = {
  prompt: 'a lighthouse',
  ratio: '16:9',
  count: 2,
  steps: 40,
  seed: 42,
  cfg: 1,
}

const TEST_MODEL = 'qwen-image-2512'

function Harness() {
  const generation = useGeneration()

  return (
    <div>
      <button
        type='button'
        onClick={() => void generation.start(params, TEST_MODEL)}
      >
        start
      </button>
      <span data-testid='status'>{generation.status}</span>
      <span data-testid='elapsed'>{generation.elapsedMs}</span>
      <span data-testid='error'>
        {generation.error ? generation.error.message : ''}
      </span>
      <span data-testid='images'>
        {generation.result ? generation.result.images.length : 0}
      </span>
    </div>
  )
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => {
    resolve = res
  })
  return { promise, resolve }
}

describe('useGeneration', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  test('ticks elapsed time while generating, then reports success with images', async () => {
    const { promise, resolve } = deferred<GenerationApiResult>()
    vi.mocked(generateImages).mockReturnValueOnce(promise)

    render(<Harness />)

    expect(screen.getByTestId('status')).toHaveTextContent('idle')

    fireEvent.click(screen.getByRole('button', { name: 'start' }))

    expect(screen.getByTestId('status')).toHaveTextContent('generating')
    expect(generateImages).toHaveBeenCalledWith(
      expect.objectContaining({ model: TEST_MODEL, n: 2, size: '1664x928' }),
    )

    await vi.advanceTimersByTimeAsync(2500)
    expect(screen.getByTestId('elapsed')).toHaveTextContent('2000')

    resolve({
      images: [
        { url: 'https://img.test/1.png' },
        { url: 'https://img.test/2.png' },
      ],
      created: 1700000000,
      raw: { data: [] },
    })
    await vi.advanceTimersByTimeAsync(0)

    expect(screen.getByTestId('status')).toHaveTextContent('success')
    expect(screen.getByTestId('images')).toHaveTextContent('2')
    expect(screen.getByTestId('elapsed')).toHaveTextContent('2500')
  })

  test('surfaces server error messages from the gateway response', async () => {
    vi.mocked(generateImages).mockRejectedValueOnce({
      isAxiosError: true,
      name: 'AxiosError',
      message: 'Request failed with status code 429',
      response: {
        status: 429,
        data: { error: { message: 'machine is busy' } },
      },
    })

    render(<Harness />)

    fireEvent.click(screen.getByRole('button', { name: 'start' }))
    await vi.advanceTimersByTimeAsync(0)

    expect(screen.getByTestId('status')).toHaveTextContent('error')
    expect(screen.getByTestId('error')).toHaveTextContent('machine is busy')
  })

  test('classifies a timed-out request as a timeout error', async () => {
    vi.mocked(generateImages).mockRejectedValueOnce({
      isAxiosError: true,
      name: 'AxiosError',
      code: 'ECONNABORTED',
      message: 'timeout of 600000ms exceeded',
    })

    render(<Harness />)

    fireEvent.click(screen.getByRole('button', { name: 'start' }))
    await vi.advanceTimersByTimeAsync(0)

    expect(screen.getByTestId('status')).toHaveTextContent('error')
    expect(screen.getByTestId('error')).toHaveTextContent('Generation timed out')
  })

  test('ignores a second start while a generation is in flight', async () => {
    const { promise } = deferred<GenerationApiResult>()
    vi.mocked(generateImages).mockReturnValueOnce(promise)

    render(<Harness />)

    fireEvent.click(screen.getByRole('button', { name: 'start' }))
    fireEvent.click(screen.getByRole('button', { name: 'start' }))

    expect(generateImages).toHaveBeenCalledTimes(1)
  })
})
