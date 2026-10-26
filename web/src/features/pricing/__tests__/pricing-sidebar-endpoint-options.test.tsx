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

function renderSidebar(models: PricingModel[]) {
  return render(
    <PricingSidebar
      quotaTypeFilter='all'
      endpointTypeFilter='all'
      vendorFilter='all'
      groupFilter='all'
      tagFilter='all'
      onQuotaTypeChange={vi.fn()}
      onEndpointTypeChange={vi.fn()}
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
  test('offers only endpoint types that the listed models can use', () => {
    renderSidebar([chatOnlyModel])

    expect(screen.getByText('Chat')).toBeInTheDocument()
    expect(screen.queryByText('Embeddings')).not.toBeInTheDocument()
    expect(screen.queryByText('Image')).not.toBeInTheDocument()
  })
})
