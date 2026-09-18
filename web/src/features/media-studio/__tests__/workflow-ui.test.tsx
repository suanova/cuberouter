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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { TemplateGallery } from '../components/template-gallery'
import { WorkflowResults } from '../components/workflow-results'
import { WorkflowStudio } from '../workflow-studio'
import type { WorkflowConfig, WorkflowJob } from '../workflow-types'

const http = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  delete: vi.fn(),
}))
vi.mock('@/lib/api', () => ({ api: http }))
const config: WorkflowConfig = {
  models: {
    create: 'qwen-image-2512',
    edit: 'qwen-image-edit-2511',
    regional: 'qwen-image-edit-2511',
  },
  health: { create: 'ready', edit: 'ready', tools: 'ready' },
  retention_days: 30,
}

beforeEach(() => {
  vi.clearAllMocks()
  http.get.mockResolvedValue({ data: [] })
})
describe('Integrated image studio', () => {
  test('shows real animal templates and filters editing examples without submitting a GPU request', () => {
    const apply = vi.fn()
    render(<TemplateGallery disabled={false} onApply={apply} />)
    fireEvent.click(screen.getByRole('button', { name: 'Animals' }))
    expect(screen.getByAltText('Pet comic strip')).toBeInTheDocument()
    expect(
      screen.queryByAltText('Studio product photo')
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Animals' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    expect(apply).not.toHaveBeenCalled()
    expect(http.post).not.toHaveBeenCalled()
  })
  test('requires a reference in edit mode and switches the displayed model', async () => {
    const query = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })
    render(
      <QueryClientProvider client={query}>
        <WorkflowStudio config={config} />
      </QueryClientProvider>
    )
    fireEvent.click(
      screen.getAllByRole('button', { name: 'Image to image' })[0]
    )
    fireEvent.change(screen.getByLabelText('Describe your changes'), {
      target: { value: 'Turn the coat blue' },
    })
    expect(screen.getByText('qwen-image-edit-2511')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Generate image' })
    ).toBeDisabled()
    expect(screen.getByLabelText('Upload reference images')).toHaveAttribute(
      'accept',
      'image/png,image/jpeg,image/webp'
    )
    await waitFor(() =>
      expect(http.get).toHaveBeenCalledWith(
        '/api/v1/media-studio/jobs',
        expect.anything()
      )
    )
    query.clear()
  })
  test('informational OCR keeps a successful no-text image separate from a generation failure', () => {
    const job: WorkflowJob = {
      id: 'job',
      mode: 'create',
      state: 'completed',
      stage: 'Completed',
      request: { prompt: 'A forest, no text' },
      billing: 'relay_completed',
      created_at: 1,
      expires_at: 100,
      result: { images: [], comparisons: [], text_quality: 'informational' },
    }
    render(
      <WorkflowResults
        job={job}
        busy={false}
        onEdit={vi.fn()}
        onTools={vi.fn()}
      />
    )
    expect(
      screen.getByText('Text check is informational; generation is complete.')
    ).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(
      screen.queryByText('Image generated. Please review the requested text.')
    ).not.toBeInTheDocument()
  })
  test('restored running work shows server progress and disables a second generation', async () => {
    http.get.mockResolvedValue({
      data: [
        {
          id: 'pending',
          state: 'running',
          stage: 'Denoising',
          mode: 'create',
          request: { prompt: 'Cat' },
          created_at: 1,
          expires_at: 100,
          billing: 'pending',
          completed_steps: 3,
          total_steps: 20,
        },
      ],
    })
    const query = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    render(
      <QueryClientProvider client={query}>
        <WorkflowStudio config={config} />
      </QueryClientProvider>
    )
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Generation in progress…' })
      ).toBeDisabled()
    )
    fireEvent.click(screen.getByRole('button', { name: 'Result' }))
    expect(
      screen.getByRole('progressbar', { name: 'Generation progress' })
    ).toHaveAttribute('value', '3')
    expect(
      screen.getByText(
        'You can refresh this page. The submitted job continues on the server.'
      )
    ).toBeInTheDocument()
    query.clear()
  })
})
