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
import { useTranslation } from 'react-i18next'

import type { PluginEvent } from '../../types'

type PluginExecutionHintsProps = {
  events: PluginEvent[]
  pluginNameBySlug?: Record<string, string>
}

// Renders plugin process hints (interim assistant text, completed MCP tool
// calls) as muted lines above the final answer.
export function PluginExecutionHints({
  events,
  pluginNameBySlug,
}: PluginExecutionHintsProps) {
  const { t } = useTranslation()
  const firstToolPluginSlugs = new Set<string>()
  for (const event of events) {
    if (event.type === 'tool_call' && !firstToolPluginSlugs.has(event.plugin)) {
      firstToolPluginSlugs.add(event.plugin)
    }
  }

  return (
    <div className='text-muted-foreground mb-1 space-y-1 text-xs'>
      {events.map((event) => {
        if (event.type === 'interim') {
          return (
            <div className='leading-relaxed' key={event.id}>
              {event.text}
            </div>
          )
        }
        const name = pluginNameBySlug?.[event.plugin] ?? event.plugin
        const showSkill = firstToolPluginSlugs.has(event.plugin)

        return (
          <div className='flex items-baseline gap-1.5' key={event.id}>
            {showSkill && (
              <span className='text-foreground/70 font-medium'>
                {t('Used skill {{name}}', { name })}
              </span>
            )}
            <span>{t('Called tool {{tool}}', { tool: event.tool })}</span>
            {event.args && (
              <code className='text-muted-foreground/60 max-w-[240px] truncate font-mono'>
                {event.args}
              </code>
            )}
          </div>
        )
      })}
    </div>
  )
}
