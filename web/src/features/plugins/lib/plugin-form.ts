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
import type { TFunction } from 'i18next'
import { z } from 'zod'

const PLUGIN_SLUG_REGEX = /^[a-z0-9][a-z0-9-]{1,63}$/

export function getPluginFormSchema(t: TFunction) {
  return z.object({
    name: z.string().trim().min(1, t('Name is required')),
    slug: z
      .string()
      .trim()
      .regex(
        PLUGIN_SLUG_REGEX,
        t('Lowercase letters, digits and dashes, 2-64 chars, must not start with a dash')
      ),
    description: z.string(),
    mcp_url: z.string().trim().min(1, t('MCP URL is required')),
    auth_header: z.string(),
    auth_token: z.string(),
    skill_source: z.string(),
    enabled: z.boolean(),
  })
}

export type PluginFormValues = z.infer<ReturnType<typeof getPluginFormSchema>>

export const PLUGIN_FORM_DEFAULT_VALUES: PluginFormValues = {
  name: '',
  slug: '',
  description: '',
  mcp_url: '',
  auth_header: '',
  auth_token: '',
  skill_source: '',
  enabled: true,
}
