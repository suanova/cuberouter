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
import { ChevronDownIcon, WrenchIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'

import type { DshToolCall } from '../types'

type ToolCallsPanelProps = {
  toolCalls: DshToolCall[]
}

// callId 由 dsh 在多数情况下提供，但不保证每一条都有；缺省时用同名调用的
// 出现次数区分，避免退回到数组下标。
function withKeys(toolCalls: DshToolCall[]) {
  const seen = new Map<string, number>()
  return toolCalls.map((call) => {
    const base = call.callId || call.name
    const count = seen.get(base) ?? 0
    seen.set(base, count + 1)
    return { call, key: `${base}#${count}` }
  })
}

export function ToolCallsPanel(props: ToolCallsPanelProps) {
  const { t } = useTranslation()

  if (props.toolCalls.length === 0) {
    return null
  }

  const calls = withKeys(props.toolCalls)

  return (
    <Collapsible className='w-full rounded-md border'>
      <CollapsibleTrigger className='group flex w-full items-center gap-2 px-3 py-2 text-left text-xs'>
        <WrenchIcon aria-hidden='true' className='size-3.5' />
        <span>
          {t('{{count}} tool calls', { count: props.toolCalls.length })}
        </span>
        <ChevronDownIcon
          aria-hidden='true'
          className='ml-auto size-3.5 transition-transform group-data-[panel-open]:rotate-180'
        />
      </CollapsibleTrigger>
      <CollapsibleContent className='border-t px-3 py-2'>
        <ul className='space-y-1'>
          {calls.map(({ call, key }) => (
            <li
              className='text-muted-foreground flex items-center gap-2 text-xs'
              key={key}
            >
              <span className='text-foreground font-mono'>{call.name}</span>
              {call.callId && (
                <span className='truncate font-mono'>{call.callId}</span>
              )}
            </li>
          ))}
        </ul>
      </CollapsibleContent>
    </Collapsible>
  )
}
