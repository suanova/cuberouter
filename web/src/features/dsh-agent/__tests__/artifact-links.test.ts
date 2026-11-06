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
import { describe, expect, it } from 'vitest'

import type { TaskArtifact } from '@/features/usage-logs/types'

import { rewriteArtifactLinks } from '../lib/artifact-links'

const DECK_URL =
  'http://gateway/v1/tasks/task-1/artifacts/pptx/content?access=token'

const artifacts: TaskArtifact[] = [
  {
    key: 'pptx',
    type: 'file',
    name: '大模型入门-程序员版.pptx',
    content_url: DECK_URL,
  },
]

describe('rewriteArtifactLinks', () => {
  it('points workspace paths at the signed artifact URL', () => {
    const reply =
      '**文件：** [大模型入门-程序员版.pptx](/data/workspace/大模型入门-程序员版.pptx)'

    expect(rewriteArtifactLinks(reply, artifacts)).toBe(
      `**文件：** [大模型入门-程序员版.pptx](${DECK_URL})`
    )
  })

  it('matches percent-encoded paths', () => {
    const reply =
      '[下载](/%E5%A4%A7%E6%A8%A1%E5%9E%8B%E5%85%A5%E9%97%A8-%E7%A8%8B%E5%BA%8F%E5%91%98%E7%89%88.pptx)'

    expect(rewriteArtifactLinks(reply, artifacts)).toContain(`](${DECK_URL})`)
  })

  it('matches artifacts that only carry a key', () => {
    const reply = '见 [产物](pptx)'

    expect(
      rewriteArtifactLinks(reply, [
        { key: 'pptx', type: 'file', content_url: DECK_URL },
      ])
    ).toBe(`见 [产物](${DECK_URL})`)
  })

  it('leaves unrelated and absolute links untouched', () => {
    const reply = [
      '[说明](https://example.com/a.pptx)',
      '[源码](/data/workspace/build_deck.py)',
      '[锚点](#section)',
    ].join(' ')

    expect(rewriteArtifactLinks(reply, artifacts)).toBe(reply)
  })

  it('returns the reply unchanged when there are no artifacts', () => {
    const reply = '[文件](/data/workspace/a.pptx)'

    expect(rewriteArtifactLinks(reply, [])).toBe(reply)
    expect(rewriteArtifactLinks('', artifacts)).toBe('')
  })
})
