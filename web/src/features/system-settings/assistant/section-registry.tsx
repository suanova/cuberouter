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
import type { AccountAssistantSettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'
import { AccountAssistantSettingsSection } from './account-assistant-settings-section'

const ASSISTANT_SECTIONS = [
  {
    id: 'general',
    titleKey: 'General Settings',
    build: (settings: AccountAssistantSettings) => (
      <AccountAssistantSettingsSection defaultValues={settings} />
    ),
  },
] as const

export type AssistantSectionId = (typeof ASSISTANT_SECTIONS)[number]['id']

const assistantRegistry = createSectionRegistry<
  AssistantSectionId,
  AccountAssistantSettings
>({
  sections: ASSISTANT_SECTIONS,
  defaultSection: 'general',
  basePath: '/system-settings/assistant',
  urlStyle: 'path',
})

export const ASSISTANT_SECTION_IDS = assistantRegistry.sectionIds
export const ASSISTANT_DEFAULT_SECTION = assistantRegistry.defaultSection
export const getAssistantSectionNavItems = assistantRegistry.getSectionNavItems
export const getAssistantSectionContent = assistantRegistry.getSectionContent
export const getAssistantSectionMeta = assistantRegistry.getSectionMeta
