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

For Commercial licensing, please contact@quantumnous.com
*/
import { createFileRoute, redirect } from '@tanstack/react-router'

import { AssistantSettings } from '@/features/system-settings/assistant'
import {
  ASSISTANT_DEFAULT_SECTION,
  ASSISTANT_SECTION_IDS,
} from '@/features/system-settings/assistant/section-registry.tsx'

export const Route = createFileRoute(
  '/_authenticated/system-settings/assistant/$section'
)({
  beforeLoad: ({ params }) => {
    const validSections = ASSISTANT_SECTION_IDS as unknown as string[]
    if (!validSections.includes(params.section)) {
      throw redirect({
        to: '/system-settings/assistant/$section',
        params: { section: ASSISTANT_DEFAULT_SECTION },
      })
    }
  },
  component: AssistantSettings,
})
