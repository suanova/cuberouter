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
      textToImageModels={['image-model']}
      imageToImageModels={['image-edit']}
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
test('the composer offers quality tiers instead of model-specific numeric settings', () => {
  render(<Composer generate={vi.fn()} />)
  expect(
    screen.queryByLabelText('Send model-specific advanced settings')
  ).not.toBeInTheDocument()
  expect(screen.queryByLabelText('Steps')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('CFG')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('Seed')).not.toBeInTheDocument()
  expect(screen.getByRole('radio', { name: 'Standard' })).toBeChecked()
})
test('selecting a quality tier marks the chosen tier and keeps generation enabled', async () => {
  const generate = vi.fn()
  render(<Composer generate={generate} />)
  fireEvent.click(screen.getByRole('radio', { name: 'High' }))
  expect(screen.getByRole('radio', { name: 'High' })).toBeChecked()
  expect(screen.getByRole('radio', { name: 'Standard' })).not.toBeChecked()
  fireEvent.click(screen.getByRole('button', { name: 'Generate image' }))
  await waitFor(() => expect(generate).toHaveBeenCalledTimes(1))
})
test('edit mode lists only image-to-image models and requires a reference', () => {
  render(<Composer generate={vi.fn()} />)
  fireEvent.click(screen.getByRole('button', { name: 'Image to image' }))
  expect(screen.getByLabelText('Model')).toHaveValue('image-edit')
  expect(
    screen.queryByRole('option', { name: 'image-model' })
  ).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Generate image' })).toBeDisabled()
})
test('switching modes picks a model from the list the new mode allows', () => {
  // 两个列表必须各自独立：切换模式时草稿里还留着另一模式的模型，回退值只能取
  // 新模式自己的列表，否则会把仅编辑模型带进文生图。
  render(<Composer generate={vi.fn()} />)
  expect(screen.getByLabelText('Model')).toHaveValue('image-model')
  fireEvent.click(screen.getByRole('button', { name: 'Image to image' }))
  expect(screen.getByLabelText('Model')).toHaveValue('image-edit')
  fireEvent.click(screen.getByRole('button', { name: 'Text to image' }))
  expect(screen.getByLabelText('Model')).toHaveValue('image-model')
})
test('unconfigured uploads explain disabled editing while text-to-image stays available', () => {
  render(<Composer generate={vi.fn()} config={{ upload_enabled: false }} />)
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
