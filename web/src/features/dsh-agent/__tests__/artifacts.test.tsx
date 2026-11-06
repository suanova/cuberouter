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
import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { fetchTaskArtifacts } from '../api'
import { TurnArtifacts } from '../components/turn-artifacts'

vi.mock('../api', () => ({
  fetchTaskArtifacts: vi.fn(),
}))

const fetchArtifactsMock = vi.mocked(fetchTaskArtifacts)

const CONTENT_URL =
  'http://127.0.0.1:3101/v1/tasks/task-1/artifacts/medical-llms.pptx/content?access=token'

function renderArtifacts(taskId: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <TurnArtifacts taskId={taskId} />
    </QueryClientProvider>
  )
}

describe('TurnArtifacts', () => {
  beforeEach(() => {
    fetchArtifactsMock.mockReset()
  })

  test('links every produced file to its signed download URL', async () => {
    fetchArtifactsMock.mockResolvedValue({
      artifacts: [
        {
          key: 'medical-llms.pptx',
          type: 'file',
          mime_type:
            'application/vnd.openxmlformats-officedocument.presentationml.presentation',
          content_url: CONTENT_URL,
        },
        {
          key: 'cover.png',
          type: 'image',
          mime_type: 'image/png',
          content_url: CONTENT_URL,
        },
      ],
    })

    renderArtifacts('task-1')

    const deck = await screen.findByRole('link', {
      name: 'Download medical-llms.pptx',
    })
    expect(deck).toHaveAttribute('href', CONTENT_URL)
    expect(deck).toHaveAttribute('download', 'medical-llms.pptx')
    expect(
      screen.getByRole('link', { name: 'Download cover.png' })
    ).toBeInTheDocument()
    expect(fetchArtifactsMock).toHaveBeenCalledWith('task-1')
  })

  test('prefers the reported filename over the artifact key', async () => {
    fetchArtifactsMock.mockResolvedValue({
      artifacts: [
        {
          key: 'pptx',
          type: 'file',
          name: '大模型入门-程序员版.pptx',
          content_url: CONTENT_URL,
        },
      ],
    })

    renderArtifacts('task-1')

    const deck = await screen.findByRole('link', {
      name: 'Download 大模型入门-程序员版.pptx',
    })
    expect(deck).toHaveAttribute('download', '大模型入门-程序员版.pptx')
    expect(deck).toHaveTextContent('大模型入门-程序员版.pptx')
  })

  test('renders nothing when the turn produced no files', async () => {
    fetchArtifactsMock.mockResolvedValue({ artifacts: [] })

    const view = renderArtifacts('task-1')

    await waitFor(() => expect(fetchArtifactsMock).toHaveBeenCalled())
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    expect(
      view.queryByLabelText('Files produced by this turn')
    ).not.toBeInTheDocument()
  })

  test('stays silent when the artifact projection fails', async () => {
    fetchArtifactsMock.mockRejectedValue(new Error('artifact_plugin_error'))

    const view = renderArtifacts('task-1')

    await waitFor(() => expect(fetchArtifactsMock).toHaveBeenCalled())
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    expect(view.container).toBeEmptyDOMElement()
  })

  test('does not query without a task id', () => {
    const view = renderArtifacts('')

    expect(fetchArtifactsMock).not.toHaveBeenCalled()
    expect(view.container).toBeEmptyDOMElement()
  })
})
