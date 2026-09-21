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

import { useState } from 'react'
import { expect, test, vi } from 'vitest'

import { TemplateGallery } from '../components/template-gallery'
import { WorkflowComposer } from '../components/workflow-composer'
import { initialDraft } from '../lib/workflow'
import type { WorkflowConfig } from '../workflow-types'

const config: WorkflowConfig = {
  upload_enabled: true,
  edit_models: ['image-edit'],
}
function Composer(props: { generate: () => void; config?: WorkflowConfig }) {
  const [draft, setDraft] = useState({
    ...initialDraft,
    model: 'image-model',
    prompt: 'Cat',
  })
  return (
    <WorkflowComposer
      draft={draft}
      models={['image-model', 'image-edit']}
      config={props.config ?? config}
      busy={false}
      loading={false}
      onChange={setDraft}
      onUpload={vi.fn()}
      onGenerate={props.generate}
      onReset={vi.fn()}
    />
  )
}
test('clearing a numeric field keeps it empty and prevents a request until repaired', async () => {
  const generate = vi.fn()
  render(<Composer generate={generate} />)
  fireEvent.click(
    screen.getByLabelText('Send model-specific advanced settings')
  )
  fireEvent.change(screen.getByLabelText('Seed'), { target: { value: '' } })
  fireEvent.change(screen.getByLabelText('CFG'), { target: { value: '2' } })
  expect(screen.getByLabelText('Seed')).toHaveValue(null)
  expect(screen.getByLabelText('Seed')).toHaveAttribute('aria-invalid', 'true')
  expect(screen.getByRole('button', { name: 'Generate image' })).toBeDisabled()
  expect(generate).not.toHaveBeenCalled()
  fireEvent.change(screen.getByLabelText('Seed'), { target: { value: '0' } })
  fireEvent.click(screen.getByRole('button', { name: 'Generate image' }))
  await waitFor(() => expect(generate).toHaveBeenCalledTimes(1))
})
test('out-of-range advanced steps disable generation without silently restoring old values', () => {
  render(<Composer generate={vi.fn()} />)
  fireEvent.click(
    screen.getByLabelText('Send model-specific advanced settings')
  )
  fireEvent.change(screen.getByLabelText('Steps'), { target: { value: '101' } })
  expect(screen.getByLabelText('Steps')).toHaveValue(101)
  expect(screen.getByRole('button', { name: 'Generate image' })).toBeDisabled()
})
test('edit mode lists only operator-confirmed edit models and requires a reference', () => {
  render(<Composer generate={vi.fn()} />)
  fireEvent.click(screen.getByRole('button', { name: 'Image to image' }))
  expect(screen.getByLabelText('Model')).toHaveValue('image-edit')
  expect(
    screen.queryByRole('option', { name: 'image-model' })
  ).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Generate image' })).toBeDisabled()
})
test('unconfigured uploads explain disabled editing while text-to-image stays available', () => {
  render(
    <Composer
      generate={vi.fn()}
      config={{ upload_enabled: false, edit_models: [] }}
    />
  )
  expect(screen.getByRole('button', { name: 'Generate image' })).toBeEnabled()
  fireEvent.click(screen.getByRole('button', { name: 'Image to image' }))
  expect(
    screen.getByText('Reference uploads are not configured.')
  ).toBeInTheDocument()
  expect(screen.getByLabelText('Upload reference images')).toBeDisabled()
})
test('animal template browsing does not submit a generation', () => {
  const apply = vi.fn()
  render(<TemplateGallery disabled={false} onApply={apply} />)
  fireEvent.click(screen.getByRole('button', { name: 'Animals' }))
  expect(screen.getByAltText('Pet comic strip')).toBeInTheDocument()
  expect(screen.queryByAltText('Studio product photo')).not.toBeInTheDocument()
  expect(apply).not.toHaveBeenCalled()
})
