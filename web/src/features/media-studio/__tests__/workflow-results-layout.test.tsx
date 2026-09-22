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
import { expect, test, vi } from 'vitest'

import { WorkflowResults } from '../components/workflow-results'
import type { WorkflowJob } from '../workflow-types'

const job: WorkflowJob = {
  id: 'job-1',
  created_at: 0,
  request: {
    mode: 'create',
    model: 'image-model',
    prompt: 'A cat',
    size: '1024x1024',
    count: 1,
    quality: 'standard',
    references: [],
  },
  images: [{ id: 'asset-1', url: 'https://media.test/a.png', mime: 'image/png' }],
  elapsed_ms: 1200,
}

function renderResult() {
  render(
    <WorkflowResults
      job={job}
      busy={false}
      count={1}
      elapsed={1200}
      onEdit={vi.fn()}
    />
  )
}

// 两个动作并排放在同一行，尺寸必须来自同一个 Button size，否则高度与内边距不同、
// 底边对不齐。这里只固定决定对齐的尺寸契约，不锁定具体配色。
test('result actions share the button small size so they sit on the same baseline', () => {
  renderResult()
  const actions = [
    screen.getByText('Download').closest('[data-slot="button"]'),
    screen.getByText('Continue editing').closest('[data-slot="button"]'),
  ]
  for (const action of actions) {
    expect(action).not.toBeNull()
    expect(action?.className.split(' ')).toEqual(
      expect.arrayContaining(['h-7', 'rounded-md', 'px-2.5'])
    )
  }
})

test('download keeps anchor semantics so the browser download attribute still applies', () => {
  renderResult()
  const download = screen.getByText('Download').closest('a')
  expect(download).toHaveAttribute('download', 'image-1.png')
  expect(download).toHaveAttribute('href', 'https://media.test/a.png')
})
