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
import { type ModelsSectionId, MODELS_SECTION_IDS } from '../section-registry'

/**
 * Sections shown as tabs on the Models page. The deployments tab requires the
 * model deployment service (System Settings → Models & Routing → Model
 * Deployment) to be enabled, so it is hidden when the service is disabled.
 */
export function getVisibleModelsSectionIds(
  deploymentsAvailable: boolean
): ModelsSectionId[] {
  if (deploymentsAvailable) return [...MODELS_SECTION_IDS]
  return MODELS_SECTION_IDS.filter((id) => id !== 'deployments')
}
