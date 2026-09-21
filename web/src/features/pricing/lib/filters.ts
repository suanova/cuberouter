/*
Copyright (C) 2023-2026 QuantumNous

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
import {
  SORT_OPTIONS,
  FILTER_ALL,
  QUOTA_TYPES,
  QUOTA_TYPE_VALUES,
  ENDPOINT_TYPES,
} from '../constants'
import type { PricingModel } from '../types'
import { hasTaskUsageSchema } from './dynamic-price'

// ----------------------------------------------------------------------------
// Filter Utilities
// ----------------------------------------------------------------------------

/**
 * Filter models by search query
 */
export function filterBySearch(
  models: PricingModel[],
  query: string
): PricingModel[] {
  if (!query) return models

  const lowerQuery = query.toLowerCase()
  return models.filter(
    (m) =>
      m.model_name?.toLowerCase().includes(lowerQuery) ||
      m.description?.toLowerCase().includes(lowerQuery) ||
      m.tags?.toLowerCase().includes(lowerQuery) ||
      m.vendor_name?.toLowerCase().includes(lowerQuery)
  )
}

/**
 * Filter models by vendor
 */
export function filterByVendor(
  models: PricingModel[],
  vendor: string
): PricingModel[] {
  if (vendor === FILTER_ALL) return models
  return models.filter((m) => m.vendor_name === vendor)
}

/**
 * Filter models by group
 */
export function filterByGroup(
  models: PricingModel[],
  group: string
): PricingModel[] {
  if (group === FILTER_ALL) return models
  return models.filter((m) => m.enable_groups?.includes(group))
}

/**
 * Filter models by quota type
 */
export function filterByQuotaType(
  models: PricingModel[],
  quotaType: string
): PricingModel[] {
  if (quotaType === QUOTA_TYPES.ALL) return models
  // Task-usage models form their own bucket, disjoint from token/request.
  if (quotaType === QUOTA_TYPES.TASK) {
    return models.filter((m) => hasTaskUsageSchema(m))
  }
  const targetType =
    quotaType === QUOTA_TYPES.TOKEN
      ? QUOTA_TYPE_VALUES.TOKEN
      : QUOTA_TYPE_VALUES.REQUEST
  return models.filter(
    (m) => m.quota_type === targetType && !hasTaskUsageSchema(m)
  )
}

/**
 * Whether a model is served by the given endpoint filter value.
 *
 * ENDPOINT_TYPES.VIDEO is the one value that is not carried by any model: it
 * stands for both video styles, so it matches a model served through either the
 * OpenAI-style or the Ark-style video endpoint. It also matches any model with
 * a per-second video price table: task-platform video models (e.g. MiniMax,
 * Vidu) declare no raw video endpoint type, yet the model card already labels
 * them 视频 from `video_prices` (see getModelEndpointLabels), so the filter
 * follows the same condition.
 */
export function matchesEndpointType(
  model: PricingModel,
  endpointType: string
): boolean {
  const wanted =
    endpointType === ENDPOINT_TYPES.VIDEO
      ? [ENDPOINT_TYPES.OPENAI_VIDEO, ENDPOINT_TYPES.ARK_VIDEO]
      : [endpointType]
  if (endpointType === ENDPOINT_TYPES.VIDEO && model.video_prices) {
    return true
  }
  return wanted.some((type) => model.supported_endpoint_types?.includes(type))
}

/**
 * Filter models by endpoint type
 */
export function filterByEndpointType(
  models: PricingModel[],
  endpointType: string
): PricingModel[] {
  if (endpointType === ENDPOINT_TYPES.ALL) return models
  return models.filter((model) => matchesEndpointType(model, endpointType))
}

/**
 * Sort key for the price options.
 *
 * TODO(pricing-sort): the price sort is hidden (SHOW_PRICE_SORT in
 * ../constants) until this key is reworked — it does not describe the price the
 * card shows, for four reasons:
 *
 * 1. Units are mixed. A token model's `model_ratio` (a dimensionless
 *    multiplier) is compared against a per-request model's `model_price`
 *    (USD/request) as if the two shared a scale.
 * 2. The group ratio is ignored. Cards price with `getDisplayGroupRatio`
 *    (lib/price.ts), while this reads the raw ratio, so with a group selected
 *    the order no longer follows the numbers on screen.
 * 3. Dynamic billing never reaches the comparator. tiered_expr (including the
 *    peak/off-peak expressions), per-second video (`video_prices`) and
 *    task-usage models are all sorted by `model_ratio`, which is the 37.5
 *    placeholder whenever the model has no configured ratio
 *    (setting/ratio_setting/model_ratio.go:388).
 * 4. Unpriced models therefore tie with each other instead of sitting in their
 *    own bucket.
 *
 * Agreed direction: sort by the lowest visible unit price of each billing type
 * — input $/1M for token models, $/request for per-request models, the normal
 * (peak) $/s for video, the first tier for expressions, the primary field for
 * task usage — and put models with no price last. Peak/off-peak uses the normal
 * price unless a separate toggle is asked for.
 */
function getModelPrice(model: PricingModel): number {
  return model.quota_type === 0 ? model.model_ratio : model.model_price || 0
}

/**
 * Sort models by specified option
 */
export function sortModels(
  models: PricingModel[],
  sortBy: string
): PricingModel[] {
  const sorted = [...models]

  switch (sortBy) {
    case SORT_OPTIONS.NAME:
      sorted.sort((a, b) =>
        (a.model_name || '').localeCompare(b.model_name || '')
      )
      break
    case SORT_OPTIONS.PRICE_LOW:
      sorted.sort((a, b) => getModelPrice(a) - getModelPrice(b))
      break
    case SORT_OPTIONS.PRICE_HIGH:
      sorted.sort((a, b) => getModelPrice(b) - getModelPrice(a))
      break
  }

  return sorted
}

/**
 * Apply all filters and sorting to models
 */
export function filterAndSortModels(
  models: PricingModel[],
  filters: {
    search: string
    vendor: string
    group: string
    quotaType: string
    endpointType: string
    tag: string
    sortBy: string
  }
): PricingModel[] {
  let result = filterBySearch(models, filters.search)
  result = filterByVendor(result, filters.vendor)
  result = filterByGroup(result, filters.group)
  result = filterByQuotaType(result, filters.quotaType)
  result = filterByEndpointType(result, filters.endpointType)
  result = filterByTag(result, filters.tag)
  result = sortModels(result, filters.sortBy)

  return result
}

/**
 * Parse tags from comma-separated string
 */
export function parseTags(tagsString?: string): string[] {
  if (!tagsString) return []
  return tagsString
    .split(/[,;|\s]+/)
    .map((t) => t.trim())
    .filter(Boolean)
}

/**
 * Extract all unique tags from models
 */
export function extractAllTags(models: PricingModel[]): string[] {
  const tagSet = new Set<string>()

  models.forEach((model) => {
    if (model.tags) {
      const tags = parseTags(model.tags)
      tags.forEach((tag) => {
        tagSet.add(tag.toLowerCase())
      })
    }
  })

  return [...tagSet].sort((a, b) => a.localeCompare(b))
}

/**
 * Filter models by tag
 */
export function filterByTag(
  models: PricingModel[],
  tag: string
): PricingModel[] {
  if (tag === FILTER_ALL) return models

  const tagLower = tag.toLowerCase()
  return models.filter((m) => {
    if (!m.tags) return false
    const modelTags = parseTags(m.tags).map((t) => t.toLowerCase())
    return modelTags.includes(tagLower)
  })
}
