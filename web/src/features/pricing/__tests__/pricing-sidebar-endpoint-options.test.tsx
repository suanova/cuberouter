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
import { describe, expect, test, vi } from 'vitest'

import { PricingSidebar } from '../components/pricing-sidebar'
import type { PricingModel } from '../types'

vi.mock('@/lib/lobe-icon', () => ({
  getLobeIcon: () => null,
}))

const chatOnlyModel: PricingModel = {
  id: 1,
  model_name: 'chat-only-model',
  quota_type: 0,
  model_ratio: 1,
  completion_ratio: 2,
  enable_groups: ['yeebo'],
  supported_endpoint_types: ['openai'],
}

function renderSidebar(
  models: PricingModel[],
  endpointTypeFilter = 'all',
  onEndpointTypeChange = vi.fn()
) {
  return render(
    <PricingSidebar
      quotaTypeFilter='all'
      endpointTypeFilter={endpointTypeFilter}
      vendorFilter='all'
      groupFilter='all'
      tagFilter='all'
      onQuotaTypeChange={vi.fn()}
      onEndpointTypeChange={onEndpointTypeChange}
      onVendorChange={vi.fn()}
      onGroupChange={vi.fn()}
      onTagChange={vi.fn()}
      vendors={[]}
      groups={[]}
      tags={[]}
      models={models}
      hasActiveFilters={false}
      onClearFilters={vi.fn()}
    />
  )
}

describe('pricing sidebar endpoint type options', () => {
  test('keeps the selected endpoint available with zero matches and allows clearing it', () => {
    const onEndpointTypeChange = vi.fn()
    renderSidebar([chatOnlyModel], 'embeddings', onEndpointTypeChange)

    expect(
      screen.getByRole('button', { name: 'Embeddings0' })
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Chat1' })).toBeInTheDocument()
    expect(screen.queryByText('Image')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'All Types1' }))
    expect(onEndpointTypeChange).toHaveBeenCalledWith('all')
  })

  test('keeps all types available when the model list is empty', () => {
    renderSidebar([])

    expect(
      screen.getByRole('button', { name: 'All Types0' })
    ).toBeInTheDocument()
    expect(screen.queryByText('Chat')).not.toBeInTheDocument()
  })

  test('offers only endpoint types that the listed models can use', () => {
    renderSidebar([chatOnlyModel])

    expect(screen.getByText('Chat')).toBeInTheDocument()
    expect(screen.queryByText('Embeddings')).not.toBeInTheDocument()
    expect(screen.queryByText('Image')).not.toBeInTheDocument()
  })
})
