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
import { SettingsPage } from '../components/settings-page'
import type { AccountAssistantSettings } from '../types'
import {
  ASSISTANT_DEFAULT_SECTION,
  getAssistantSectionContent,
  getAssistantSectionMeta,
} from './section-registry.tsx'

const defaultAssistantSettings: AccountAssistantSettings = {
  'account_assistant.enabled': false,
  'account_assistant.model': '',
  'account_assistant.billing_user_id': 0,
  'account_assistant.group': '',
  'account_assistant.max_tokens': 1024,
  'account_assistant.max_tool_calls': 3,
  'account_assistant.max_rounds': 2,
  'account_assistant.hourly_limit': 10,
  'account_assistant.daily_limit': 50,
  'account_assistant.daily_token_budget': 200000,
  'account_assistant.faq_json': '[]',
  'account_assistant.support_url': '',
}

export function AssistantSettings() {
  return (
    <SettingsPage
      routePath='/_authenticated/system-settings/assistant/$section'
      defaultSettings={defaultAssistantSettings}
      defaultSection={ASSISTANT_DEFAULT_SECTION}
      getSectionContent={getAssistantSectionContent}
      getSectionMeta={getAssistantSectionMeta}
      loadingMessage='Loading assistant settings...'
    />
  )
}
