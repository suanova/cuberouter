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
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, test, vi } from 'vitest'

import type { UserModelDiscounts } from '../../types'
import { ModelDiscountEditor } from '../model-discount-editor'

type HarnessProps = {
  initial: UserModelDiscounts
  modelOptions?: string[]
  disabled?: boolean
  onChange?: (next: UserModelDiscounts) => void
}

// The editor is controlled: the harness plays the parent that applies every
// change so the rendered rows always come from the same value the assertions
// look at.
function EditorHarness(props: HarnessProps) {
  const [value, setValue] = useState<UserModelDiscounts>(props.initial)

  return (
    <ModelDiscountEditor
      value={value}
      modelOptions={props.modelOptions ?? ['m', 'm2']}
      disabled={props.disabled}
      onChange={(next) => {
        props.onChange?.(next)
        setValue(next)
      }}
    />
  )
}

// The row added by the button is rendered after every existing row, so its
// model select is always the last one on screen.
function addedRowSelect(): HTMLElement {
  const last = screen.getAllByRole('combobox').at(-1)
  if (!last) {
    throw new Error('Expected at least one model select')
  }
  return last
}

describe('ModelDiscountEditor', () => {
  test('renders an existing discount as its percent', () => {
    render(<EditorHarness initial={{ m: 0.8 }} />)

    expect(screen.getByText('Model Discounts')).toBeInTheDocument()
    expect(screen.getByRole('combobox')).toHaveTextContent('m')
    expect(screen.getByDisplayValue('80')).toBeInTheDocument()
  })

  test('shows float-prone discounts as clean percentages', () => {
    // 0.29 * 100 === 28.999999999999996 and 0.07 * 100 === 7.000000000000001.
    render(<EditorHarness initial={{ m: 0.29, m2: 0.07 }} />)

    expect(screen.getByDisplayValue('29')).toBeInTheDocument()
    expect(screen.getByDisplayValue('7')).toBeInTheDocument()
  })

  test('reports the edited percent to onChange', () => {
    const onChange = vi.fn()
    render(<EditorHarness initial={{ m: 0.8 }} onChange={onChange} />)

    fireEvent.input(screen.getByDisplayValue('80'), {
      target: { value: '90' },
    })

    expect(onChange).toHaveBeenLastCalledWith({ m: 0.9 })
    expect(screen.getByDisplayValue('90')).toBeInTheDocument()
  })

  test('keeps an out-of-range percent visible with an error instead of writing it', () => {
    const onChange = vi.fn()
    render(<EditorHarness initial={{ m: 0.8 }} onChange={onChange} />)

    fireEvent.input(screen.getByDisplayValue('80'), {
      target: { value: '0' },
    })

    expect(
      screen.getByText('Discount must be between 1% and 100%')
    ).toBeInTheDocument()
    expect(screen.getByDisplayValue('0')).toBeInTheDocument()
    expect(onChange).not.toHaveBeenCalled()
  })

  test('removes a row and reports the remaining discounts', () => {
    const onChange = vi.fn()
    render(<EditorHarness initial={{ m: 0.8 }} onChange={onChange} />)

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))

    expect(onChange).toHaveBeenLastCalledWith({})
    expect(screen.queryByDisplayValue('80')).not.toBeInTheDocument()
  })

  test('adds a new row only after a model is picked', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<EditorHarness initial={{ m: 0.8 }} onChange={onChange} />)

    fireEvent.click(screen.getByRole('button', { name: 'Add model discount' }))

    // The added row is a placeholder: nothing is written before a model exists.
    expect(onChange).not.toHaveBeenCalled()
    expect(screen.getByText('Model name is required')).toBeInTheDocument()

    await user.click(addedRowSelect())
    await user.click(await screen.findByRole('option', { name: 'm2' }))

    expect(onChange).toHaveBeenLastCalledWith({ m: 0.8, m2: 1 })
    expect(screen.queryByText('Model name is required')).not.toBeInTheDocument()

    fireEvent.input(screen.getByDisplayValue('100'), {
      target: { value: '50' },
    })

    expect(onChange).toHaveBeenLastCalledWith({ m: 0.8, m2: 0.5 })
  })

  test('writes the percent typed before the model was picked', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<EditorHarness initial={{ m: 0.8 }} onChange={onChange} />)

    fireEvent.click(screen.getByRole('button', { name: 'Add model discount' }))
    fireEvent.input(screen.getByDisplayValue('100'), {
      target: { value: '50' },
    })

    expect(onChange).not.toHaveBeenCalled()

    await user.click(addedRowSelect())
    await user.click(await screen.findByRole('option', { name: 'm2' }))

    expect(onChange).toHaveBeenLastCalledWith({ m: 0.8, m2: 0.5 })
  })

  test('keeps the picked model when the percent was still invalid', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<EditorHarness initial={{ m: 0.8 }} onChange={onChange} />)

    fireEvent.click(screen.getByRole('button', { name: 'Add model discount' }))
    fireEvent.input(screen.getByDisplayValue('100'), {
      target: { value: '0' },
    })
    await user.click(addedRowSelect())
    await user.click(await screen.findByRole('option', { name: 'm2' }))

    expect(onChange).not.toHaveBeenCalled()
    expect(addedRowSelect()).toHaveTextContent('m2')
    expect(
      screen.getByText('Discount must be between 1% and 100%')
    ).toBeInTheDocument()

    fireEvent.input(screen.getByDisplayValue('0'), {
      target: { value: '50' },
    })

    expect(onChange).toHaveBeenLastCalledWith({ m: 0.8, m2: 0.5 })
  })

  test('renames a row to another model keeping its discount', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<EditorHarness initial={{ m: 0.8 }} onChange={onChange} />)

    await user.click(screen.getByRole('combobox'))
    await user.click(await screen.findByRole('option', { name: 'm2' }))

    expect(onChange).toHaveBeenLastCalledWith({ m2: 0.8 })
  })

  test('disables every control while disabled', () => {
    render(<EditorHarness initial={{ m: 0.8 }} disabled />)

    expect(screen.getByRole('combobox')).toBeDisabled()
    expect(screen.getByDisplayValue('80')).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Remove' })).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Add model discount' })
    ).toBeDisabled()
  })
})
