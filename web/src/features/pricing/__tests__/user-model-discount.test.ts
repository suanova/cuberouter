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
import assert from 'node:assert/strict'

import { render, screen } from '@testing-library/react'
import { createElement } from 'react'
import { describe, expect, test, vi } from 'vitest'

import { ModelCard } from '../components/model-card'
import {
  formatModelDiscountPercent,
  getViewerModelDiscount,
} from '../lib/model-helpers'
import {
  formatFixedPrice,
  formatGroupPrice,
  formatPrice,
  formatRequestPrice,
} from '../lib/price'
import type { PricingModel } from '../types'

// @lobehub/icons transitively imports @emoji-mart JSON assets that vitest's
// externalized ESM loader rejects. Icon rendering is irrelevant to the price
// summary contracts under test, so the icon loader boundary is stubbed.
vi.mock('@/lib/lobe-icon', () => ({
  getLobeIcon: () => null,
}))

function pricingModel(overrides: Partial<PricingModel>): PricingModel {
  return {
    id: 1,
    model_name: 'test-model',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 2,
    enable_groups: ['default'],
    ...overrides,
  }
}

function renderCard(
  model: PricingModel,
  userModelDiscounts?: Record<string, number>
) {
  render(
    createElement(ModelCard, {
      model,
      onClick: () => undefined,
      userModelDiscounts,
    })
  )
}

const tokenModel = pricingModel({ group_ratio: { default: 1 } })
const groupRatio = { default: 0.5 }

describe('getViewerModelDiscount', () => {
  test('returns the discount the pricing response stores for the model', () => {
    assert.equal(
      getViewerModelDiscount({ 'test-model': 0.8 }, 'test-model'),
      0.8
    )
  })

  test('treats an absent discount map, empty map, or other model as no discount', () => {
    assert.equal(getViewerModelDiscount(undefined, 'test-model'), 1)
    assert.equal(getViewerModelDiscount({}, 'test-model'), 1)
    assert.equal(getViewerModelDiscount({ other: 0.8 }, 'test-model'), 1)
  })

  test('treats a discount outside 0-1 exclusive as no discount', () => {
    assert.equal(getViewerModelDiscount({ 'test-model': 1 }, 'test-model'), 1)
    assert.equal(getViewerModelDiscount({ 'test-model': 0 }, 'test-model'), 1)
    assert.equal(
      getViewerModelDiscount({ 'test-model': -0.5 }, 'test-model'),
      1
    )
    assert.equal(getViewerModelDiscount({ 'test-model': 1.5 }, 'test-model'), 1)
    assert.equal(
      getViewerModelDiscount({ 'test-model': Number.NaN }, 'test-model'),
      1
    )
  })
})

describe('formatModelDiscountPercent', () => {
  test('renders the paid percent without trailing zeros', () => {
    assert.equal(formatModelDiscountPercent(0.8), '80')
    assert.equal(formatModelDiscountPercent(0.8513), '85.13')
    assert.equal(formatModelDiscountPercent(0.005), '0.5')
  })
})

describe('discounted token prices', () => {
  test('omitting the discount keeps the previous group price exactly', () => {
    assert.equal(
      formatGroupPrice(tokenModel, 'default', 'input', 'M', false, 1, 1, {
        default: 0.5,
      }),
      '$1'
    )
    assert.equal(
      formatGroupPrice(tokenModel, 'default', 'input', 'M', false, 1, 1, {
        default: 0.5,
      }),
      formatGroupPrice(
        tokenModel,
        'default',
        'input',
        'M',
        false,
        1,
        1,
        { default: 0.5 },
        1
      )
    )
  })

  test('multiplies the group token price by the viewer discount', () => {
    assert.equal(
      formatGroupPrice(
        tokenModel,
        'default',
        'input',
        'M',
        false,
        1,
        1,
        groupRatio,
        0.8
      ),
      '$0.8'
    )
    assert.equal(
      formatGroupPrice(
        tokenModel,
        'default',
        'output',
        'M',
        false,
        1,
        1,
        groupRatio,
        0.8
      ),
      '$1.6'
    )
  })

  test('multiplies the display-ratio price the card and list show', () => {
    assert.equal(formatPrice(tokenModel, 'input', 'M'), '$2')
    assert.equal(
      formatPrice(tokenModel, 'input', 'M', false, 1, 1, undefined, 0.5),
      '$1'
    )
  })
})

describe('discounted per-request prices', () => {
  const requestModel = pricingModel({ quota_type: 1, model_price: 4 })

  test('omitting the discount keeps the previous per-request price exactly', () => {
    assert.equal(formatRequestPrice(requestModel, false, 1, 1), '$4')
    assert.equal(
      formatFixedPrice(requestModel, 'default', false, 1, 1, { default: 1 }),
      '$4'
    )
  })

  test('multiplies the per-request price by the viewer discount', () => {
    assert.equal(
      formatRequestPrice(requestModel, false, 1, 1, undefined, 0.5),
      '$2'
    )
    assert.equal(
      formatFixedPrice(
        requestModel,
        'default',
        false,
        1,
        1,
        { default: 1 },
        0.5
      ),
      '$2'
    )
  })
})

describe('ModelCard viewer discount', () => {
  test('shows the discounted price summary and a discount badge', () => {
    renderCard(tokenModel, { 'test-model': 0.8 })

    // input = 1 * 2 * 1 * 0.8, output = input * 2 per 1M tokens
    expect(screen.getByText('$1.6')).toBeInTheDocument()
    expect(screen.getByText('$3.2')).toBeInTheDocument()
    expect(screen.getByText('Discount 80%')).toBeInTheDocument()
  })

  test('leaves the price summary and badge unchanged without a discount', () => {
    renderCard(tokenModel)

    expect(screen.getByText('$2')).toBeInTheDocument()
    expect(screen.getByText('$4')).toBeInTheDocument()
    expect(screen.queryByText(/Discount/)).not.toBeInTheDocument()
  })

  test('hides the badge on a video summary that keeps the base price', () => {
    renderCard(
      pricingModel({
        video_prices: {
          rows: [
            { resolution: '480p', normal_price: 0.05, off_peak_price: 0.03 },
            { resolution: '720p', normal_price: 0.1, off_peak_price: 0.07 },
          ],
        },
      }),
      { 'test-model': 0.8 }
    )

    // The card keeps summarizing in base prices, so the badge would read as if
    // these numbers were already discounted; the detail panel is where the
    // discount and the badge live.
    expect(screen.getByText(/From \$0\.05\/s/)).toBeInTheDocument()
    expect(screen.queryByText(/Discount/)).not.toBeInTheDocument()
  })

  test('hides the badge on an image summary that keeps the base price', () => {
    renderCard(
      pricingModel({
        image_prices: {
          rows: [
            { tier: 'standard', price: 0.04 },
            { tier: 'high', price: 0.08 },
          ],
        },
      }),
      { 'test-model': 0.8 }
    )

    expect(screen.getByText('$0.08/image')).toBeInTheDocument()
    expect(screen.queryByText(/Discount/)).not.toBeInTheDocument()
  })
})
