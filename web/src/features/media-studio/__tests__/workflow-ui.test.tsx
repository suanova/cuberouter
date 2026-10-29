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
import type { StudioAsset, WorkflowDraft } from '../workflow-types'

const reference: StudioAsset = {
  id: 'photo',
  mime: 'image/png',
  url: 'data:image/png;base64,YQ==',
}

function Composer(props: {
  generate: () => void
  draft?: Partial<WorkflowDraft>
  maskModels?: string[]
  imageToImageModels?: string[]
  probe?: boolean
}) {
  const [draft, setDraft] = useState({
    ...initialDraft,
    model: 'image-model',
    prompt: 'Cat',
    ...props.draft,
  })
  return (
    <>
      {props.probe && <p>{`boxes: ${draft.mask?.length ?? 0}`}</p>}
      <WorkflowComposer
        draft={draft}
        textToImageModels={['image-model']}
        imageToImageModels={props.imageToImageModels ?? ['image-edit']}
        maskModels={props.maskModels ?? []}
        busy={false}
        loading={false}
        onChange={setDraft}
        onUpload={vi.fn()}
        onGenerate={props.generate}
        onReset={vi.fn()}
      />
    </>
  )
}
test('the composer keeps quality tiers and adds optional machine settings', () => {
  // Steps/CFG/Seed 曾经被刻意藏起来，只能靠画质档位推导；现在作为可选覆盖暴露，
  // 但初始必须留空，请求体才继续回落到档位与机器默认值。
  render(<Composer generate={vi.fn()} />)
  expect(screen.getByText('Advanced Settings')).toBeInTheDocument()
  expect(screen.getByLabelText('Steps')).toHaveValue(null)
  expect(screen.getByLabelText('Steps')).toHaveAttribute('placeholder', '30')
  expect(screen.getByLabelText('CFG')).toHaveValue(null)
  expect(screen.getByLabelText('CFG')).toHaveAttribute('placeholder', '4')
  expect(screen.getByLabelText('Seed')).toHaveValue(null)
  expect(screen.getByLabelText('Seed')).toHaveAttribute('placeholder', '42')
  expect(screen.getByRole('radio', { name: 'Standard' })).toBeChecked()
  // 步数的占位符跟随画质档位，告诉用户留空时实际会下发多少步。
  fireEvent.click(screen.getByRole('radio', { name: 'High' }))
  expect(screen.getByLabelText('Steps')).toHaveAttribute('placeholder', '50')
})
test('advanced settings keep a typed value and drop it when cleared', () => {
  render(<Composer generate={vi.fn()} />)
  const steps = screen.getByLabelText('Steps')
  fireEvent.change(steps, { target: { value: '12' } })
  expect(steps).toHaveValue(12)
  fireEvent.change(steps, { target: { value: '' } })
  expect(steps).toHaveValue(null)
})
test('out-of-range machine settings are marked invalid and block generation', () => {
  // 超范围时草稿校验失败、按钮变灰；aria-invalid 让用户看得见是哪个输入框出了问题。
  render(<Composer generate={vi.fn()} />)
  fireEvent.change(screen.getByLabelText('Steps'), {
    target: { value: '999' },
  })
  expect(screen.getByLabelText('Steps')).toHaveAttribute('aria-invalid', 'true')
  expect(screen.getByRole('button', { name: 'Generate image' })).toBeDisabled()
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
test('reference images can be attached without any upload service configured', () => {
  // 参考图以 base64 内联下发，不再经过对象存储，编辑模式因此与 S3 是否配置无关：
  // 原先那条「未配置上传」的提示与门禁必须彻底消失，否则没配 S3 的部署图生图不可用。
  render(<Composer generate={vi.fn()} />)
  expect(screen.getByRole('button', { name: 'Generate image' })).toBeEnabled()
  fireEvent.click(screen.getByRole('button', { name: 'Image to image' }))
  expect(
    screen.queryByText('Reference uploads are not configured.')
  ).not.toBeInTheDocument()
  expect(screen.getByLabelText('Upload reference images')).toBeEnabled()
})
test('reference uploads stop once ten images are attached', () => {
  // 上限提升到 10 张后仍要挡住第 11 张：不挡住就只能等服务端按请求体大小报错。
  const references = Array.from({ length: 10 }, (_, index) => ({
    id: `ref-${index}`,
    mime: 'image/png',
    url: 'data:image/png;base64,YQ==',
  }))
  render(
    <Composer
      generate={vi.fn()}
      draft={{ mode: 'edit', model: 'image-edit', references }}
    />
  )
  expect(screen.getByLabelText('Upload reference images')).toBeDisabled()
  expect(
    screen.getByText(
      'PNG, JPEG or WebP · up to 10 MB each · maximum 10 references'
    )
  ).toBeInTheDocument()
})
test('animal template browsing does not submit a generation', () => {
  const apply = vi.fn()
  render(<TemplateGallery disabled={false} onApply={apply} />)
  fireEvent.click(screen.getByRole('button', { name: 'Animals' }))
  expect(screen.getByAltText('Pet comic strip')).toBeInTheDocument()
  expect(screen.queryByAltText('Studio product photo')).not.toBeInTheDocument()
  expect(apply).not.toHaveBeenCalled()
})

const editDraft = {
  mode: 'edit' as const,
  model: 'image-edit',
  references: [reference],
}

/** 在被测元素上模拟一次指针拖拽；jsdom 没有布局，所以先固定元素尺寸。 */
function dragBox(
  surface: HTMLElement,
  from: [number, number],
  to: [number, number]
) {
  vi.spyOn(surface, 'getBoundingClientRect').mockReturnValue({
    left: 0,
    top: 0,
    width: 200,
    height: 100,
  } as DOMRect)
  const at = (point: [number, number]) => ({
    clientX: point[0],
    clientY: point[1],
  })
  fireEvent.pointerDown(surface, at(from))
  fireEvent.pointerMove(surface, at(to))
  fireEvent.pointerUp(surface, at(to))
}

test('the box tool is offered only for models declared to support masks', () => {
  // 没声明 image-mask 的上游会丢掉 mask：宁可不显示工具，也不要给出框了区域却整图重绘的错觉。
  const view = render(<Composer generate={vi.fn()} draft={editDraft} />)
  expect(screen.queryByLabelText('Selection area')).not.toBeInTheDocument()
  view.rerender(
    <Composer
      generate={vi.fn()}
      draft={editDraft}
      maskModels={['image-edit']}
    />
  )
  expect(screen.getByLabelText('Selection area')).toBeInTheDocument()
})

test('dragging on the reference image records a box that undo and clear remove', () => {
  render(
    <Composer
      generate={vi.fn()}
      draft={editDraft}
      maskModels={['image-edit']}
      probe
    />
  )
  const surface = screen.getByLabelText('Selection area')
  dragBox(surface, [20, 10], [120, 60])
  expect(screen.getByText('boxes: 1')).toBeInTheDocument()
  expect(screen.getByText('1 boxed area(s)')).toBeInTheDocument()

  dragBox(surface, [0, 0], [40, 20])
  expect(screen.getByText('boxes: 2')).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: 'Undo box' }))
  expect(screen.getByText('boxes: 1')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Clear boxes' }))
  expect(screen.getByText('boxes: 0')).toBeInTheDocument()
  expect(screen.queryByText('boxed area(s)', { exact: false })).toBeNull()
})

test('removing the reference image drops the boxes drawn on it', () => {
  // 框选坐标相对第一张参考图：图没了，坐标就没有意义，不能留到下一张图上。
  render(
    <Composer
      generate={vi.fn()}
      draft={editDraft}
      maskModels={['image-edit']}
      probe
    />
  )
  dragBox(screen.getByLabelText('Selection area'), [20, 10], [120, 60])
  expect(screen.getByText('boxes: 1')).toBeInTheDocument()

  fireEvent.click(
    screen.getByRole('button', { name: 'Remove reference image' })
  )
  expect(screen.getByText('boxes: 0')).toBeInTheDocument()
  expect(screen.queryByLabelText('Selection area')).not.toBeInTheDocument()
})

test('keyboard users can place a box with the arrow keys and Enter', () => {
  // 只能靠键盘的用户原本能撤销、清空，却画不出框：框选区域必须可获得焦点，
  // 方向键移动光标、Enter 落框。
  render(
    <Composer
      generate={vi.fn()}
      draft={editDraft}
      maskModels={['image-edit']}
      probe
    />
  )
  const surface = screen.getByLabelText('Selection area')
  expect(surface).toHaveAttribute('tabindex', '0')
  surface.focus()
  fireEvent.keyDown(surface, { key: 'ArrowRight' })
  fireEvent.keyDown(surface, { key: 'Enter' })
  expect(screen.getByText('boxes: 1')).toBeInTheDocument()
  expect(screen.getByText('1 boxed area(s)')).toBeInTheDocument()
})

test('the editor refuses to draw more boxes than a draft may carry', () => {
  // 超过上限的框会让 draftSchema 校验失败，生成按钮跟着变灰且没有任何解释；
  // 所以在编辑阶段就挡住，并说明上限。
  const regions = Array.from({ length: 20 }, (_, index) => ({
    x: index / 40,
    y: 0.1,
    width: 0.02,
    height: 0.02,
  }))
  render(
    <Composer
      generate={vi.fn()}
      draft={{ ...editDraft, mask: regions }}
      maskModels={['image-edit']}
      probe
    />
  )
  expect(screen.getByText('boxes: 20')).toBeInTheDocument()
  dragBox(screen.getByLabelText('Selection area'), [20, 10], [120, 60])
  expect(screen.getByText('boxes: 20')).toBeInTheDocument()
  expect(screen.getByText('At most 20 boxes per image.')).toBeInTheDocument()
})

test('switching to a model without image-mask drops the boxes', () => {
  // 换到没有声明 image-mask 的模型后编辑器就隐藏了，框却还留在草稿里，
  // 生成时会把蒙版发给不认识它的上游。
  const draft = {
    ...editDraft,
    mask: [{ x: 0.1, y: 0.1, width: 0.4, height: 0.4 }],
  }
  render(
    <Composer
      generate={vi.fn()}
      draft={draft}
      maskModels={['image-edit']}
      imageToImageModels={['image-edit', 'plain-edit']}
      probe
    />
  )
  expect(screen.getByText('boxes: 1')).toBeInTheDocument()

  fireEvent.change(screen.getByLabelText('Model'), {
    target: { value: 'plain-edit' },
  })
  expect(screen.getByText('boxes: 0')).toBeInTheDocument()
  expect(screen.queryByLabelText('Selection area')).not.toBeInTheDocument()
})

test('a CFG value the schema accepts is not blocked by native step validation', async () => {
  // 草稿接受 0..10 的任意小数；输入框固定 0.1 步长会把 6.55 判成非法，
  // 生成按钮仍可点、浏览器却拦下提交。
  const generate = vi.fn()
  render(<Composer generate={generate} />)
  const cfg = screen.getByLabelText('CFG')
  expect(cfg).toHaveAttribute('step', 'any')
  fireEvent.change(cfg, { target: { value: '6.55' } })
  fireEvent.click(screen.getByRole('button', { name: 'Generate image' }))
  await waitFor(() => expect(generate).toHaveBeenCalledTimes(1))
})
