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
import { describe, expect, test } from 'vitest'

// `useMediaQuery` reads `matchMedia`, which jsdom does not implement.
Object.defineProperty(window, 'matchMedia', {
  configurable: true,
  value: () => ({
    matches: false,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
  }),
})

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { ApiKeyGroupCombobox } = await import('../api-key-group-combobox')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Search...': 'Search...',
        'No group found.': 'No group found.',
        'Select a group': 'Select a group',
      },
    },
  },
})

const options = [
  { value: 'default', label: 'default', desc: 'User group', ratio: 1 },
  { value: 'vip', label: 'vip', desc: 'Priority group', ratio: 3 },
]

/**
 * The group picker is shared between the keys page, where the group is the
 * subject of the form, and the organization edit drawers, where it is one field
 * among several. Only the second wants a short picker, so the default has to
 * stay tall: a regression here silently redraws the keys form.
 */
function renderCombobox(size?: 'default' | 'compact') {
  return render(
    <I18nextProvider i18n={i18n}>
      <ApiKeyGroupCombobox
        options={options}
        value='vip'
        onValueChange={() => undefined}
        size={size}
      />
    </I18nextProvider>
  )
}

describe('API key group combobox sizing', () => {
  test('defaults to the tall trigger that shows the selected group description', () => {
    renderCombobox()

    const trigger = screen.getByRole('combobox')
    expect(trigger).toHaveAttribute('data-size', 'default')
    expect(trigger).toHaveClass('min-h-14')
    expect(trigger).not.toHaveClass('h-8')
    expect(screen.getByText('Priority group')).toBeInTheDocument()
  })

  test('compact drops the trigger to input height and to a single line', () => {
    renderCombobox('compact')

    const trigger = screen.getByRole('combobox')
    expect(trigger).toHaveAttribute('data-size', 'compact')
    // The same height as the form inputs the picker sits among.
    expect(trigger).toHaveClass('h-8')
    expect(trigger).not.toHaveClass('min-h-14')
    // The label still names the field; only the description is dropped, because
    // it would be clipped at this height.
    expect(trigger).toHaveTextContent('vip')
    expect(screen.queryByText('Priority group')).not.toBeInTheDocument()
  })
})
