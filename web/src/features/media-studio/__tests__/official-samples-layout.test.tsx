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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'

import { OfficialSamples } from '../components/official-samples'

function renderSamples() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <OfficialSamples disabled={false} onApply={vi.fn()} />
    </QueryClientProvider>
  )
}

test('official examples keep the same tile size for different image proportions', () => {
  renderSamples()
  const [research, travel] = screen.getAllByRole('article')
  expect(research.parentElement).toHaveClass(
    'auto-rows-fr',
    'grid-cols-2',
    '@3xl/gallery:grid-cols-5'
  )
  for (const card of [research, travel]) {
    const preview = within(card).getByRole('button', { name: /Enlarge/ })
    expect(preview).toHaveClass('aspect-[4/3]')
    expect(preview.querySelector('img')).toHaveClass('object-cover')
  }
})

test('the first and last official examples are in the same scroll area', () => {
  renderSamples()
  const viewport = screen.getByRole('region', {
    name: 'Official Qwen examples',
  })
  expect(viewport).toHaveClass('overflow-y-auto')
  expect(
    within(viewport).getByRole('heading', { name: 'Research figure' })
  ).toBeInTheDocument()
  expect(
    within(viewport).getByRole('heading', { name: 'Separate reference mask' })
  ).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'Next page' })).toBeNull()
})

test('opening a thumbnail with the keyboard shows the full image and Escape returns focus', async () => {
  const user = userEvent.setup()
  renderSamples()
  const preview = screen.getByRole('button', {
    name: 'Enlarge Travel app design',
  })

  preview.focus()
  await user.keyboard('{Enter}')
  const dialog = await screen.findByRole('dialog', {
    name: 'Travel app design',
  })
  const image = within(dialog).getByRole('img', { name: 'Travel app design' })
  expect(image).toHaveClass('object-contain')
  expect(image).toHaveAttribute(
    'src',
    preview.querySelector('img')?.getAttribute('src')
  )

  await user.keyboard('{Escape}')
  await waitFor(() => expect(dialog).not.toBeInTheDocument())
  expect(preview).toHaveFocus()
})
