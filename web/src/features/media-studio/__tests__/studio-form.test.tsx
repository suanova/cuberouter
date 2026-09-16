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
import { useState } from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, describe, expect, test, vi } from 'vitest'

import { DEFAULT_PARAMS, LIMITS } from '../constants'
import type { StudioParams } from '../types'
import { StudioForm } from '../components/studio-form'

function StatefulForm(options: {
  params?: StudioParams
  generating?: boolean
  onGenerate?: () => void
  onChangeSpy?: (params: StudioParams) => void
}) {
  const [params, setParams] = useState<StudioParams>(
    options.params ?? { ...DEFAULT_PARAMS, prompt: '' },
  )

  const handleChange = (next: StudioParams) => {
    setParams(next)
    options.onChangeSpy?.(next)
  }

  return (
    <StudioForm
      params={params}
      generating={options.generating ?? false}
      errorText={null}
      onGenerate={options.onGenerate ?? vi.fn()}
      onChange={handleChange}
    />
  )
}

describe('StudioForm', () => {
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      'Prompt': 'Prompt',
      'Describe the image you want to generate…': 'Describe the image you want to generate…',
      'Image aspect ratio': 'Image aspect ratio',
      'Images per batch': 'Images per batch',
      '{{count}} image': '{{count}} image',
      'Advanced settings': 'Advanced settings',
      'Steps': 'Steps',
      'Seed': 'Seed',
      'Randomize seed': 'Randomize seed',
      'CFG scale': 'CFG scale',
      'Generate image': 'Generate image',
      'Generating…': 'Generating…',
      'Reset to defaults': 'Reset to defaults',
    })
  })

  test('disables the generate button while the prompt is empty', () => {
    render(<StatefulForm />)

    expect(
      screen.getByRole('button', { name: 'Generate image' }),
    ).toBeDisabled()
  })

  test('submits on generate when the prompt has content', () => {
    const onGenerate = vi.fn()

    render(<StatefulForm onGenerate={onGenerate} />)
    fireEvent.change(screen.getByLabelText('Prompt'), {
      target: { value: 'a red fox' },
    })
    const form = screen
      .getByRole('button', { name: 'Generate image' })
      .closest('form')
    if (!form) {
      throw new Error('expected the form to be present')
    }
    fireEvent.submit(form)

    expect(onGenerate).toHaveBeenCalledTimes(1)
  })

  test('shows the prompt character count', () => {
    render(<StatefulForm />)
    fireEvent.change(screen.getByLabelText('Prompt'), {
      target: { value: 'hello' },
    })

    expect(screen.getByText('5 / 16000')).toBeInTheDocument()
  })

  test('selecting a ratio reports the change with the full parameter set', () => {
    const onChangeSpy = vi.fn()

    render(<StatefulForm onChangeSpy={onChangeSpy} />)
    fireEvent.click(screen.getByRole('radio', { name: '9:16' }))

    expect(onChangeSpy).toHaveBeenCalledTimes(1)
    expect(onChangeSpy).toHaveBeenCalledWith({
      ...DEFAULT_PARAMS,
      prompt: '',
      ratio: '9:16',
    })
  })

  test('reset restores the default parameters', () => {
    const onChangeSpy = vi.fn()

    const modified: StudioParams = {
      ...DEFAULT_PARAMS,
      prompt: 'something',
      count: 3,
      steps: 77,
      seed: 9,
      cfg: 2,
    }
    render(<StatefulForm params={modified} onChangeSpy={onChangeSpy} />)
    fireEvent.click(screen.getByRole('button', { name: 'Reset to defaults' }))

    expect(onChangeSpy).toHaveBeenCalledTimes(1)
    expect(onChangeSpy).toHaveBeenCalledWith({ ...DEFAULT_PARAMS, prompt: '' })
  })

  test('randomize seed fills a value within the allowed range', () => {
    const onChangeSpy = vi.fn()

    render(<StatefulForm onChangeSpy={onChangeSpy} />)
    fireEvent.click(screen.getByRole('button', { name: 'Advanced settings' }))
    fireEvent.click(screen.getByRole('button', { name: 'Randomize seed' }))

    expect(onChangeSpy).toHaveBeenCalledTimes(1)
    const next = onChangeSpy.mock.calls[0][0]
    expect(next.seed).toBeGreaterThanOrEqual(LIMITS.seedMin)
    expect(next.seed).toBeLessThanOrEqual(LIMITS.seedMax)
  })

  test('disables prompt, ratios and generate while generating', () => {
    render(<StatefulForm generating />)

    expect(screen.getByLabelText('Prompt')).toBeDisabled()
    expect(screen.getByRole('radio', { name: '1:1' })).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Generating…' }),
    ).toBeDisabled()
  })
})
