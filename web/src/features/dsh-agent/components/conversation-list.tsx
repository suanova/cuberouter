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
import { MessageSquareIcon, PlusIcon, Trash2Icon } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import type { DshConversation } from '../types'

type ConversationListProps = {
  conversations: DshConversation[]
  activeId: string | null
  onSelect: (id: string) => void
  onCreate: () => void
  onDelete: (id: string) => void
}

export function ConversationList(props: ConversationListProps) {
  const { t } = useTranslation()

  return (
    <aside className='bg-muted/20 hidden w-64 shrink-0 flex-col border-r md:flex'>
      <div className='flex items-center justify-between gap-2 border-b p-3'>
        <span className='text-sm font-medium'>{t('Conversations')}</span>
        <Button
          aria-label={t('New conversation')}
          onClick={props.onCreate}
          size='icon-sm'
          variant='ghost'
        >
          <PlusIcon aria-hidden='true' />
        </Button>
      </div>
      <div className='min-h-0 flex-1 space-y-1 overflow-y-auto p-2'>
        {props.conversations.map((conversation) => (
          <div
            className={cn(
              'group flex items-center gap-1 rounded-md',
              conversation.id === props.activeId ? 'bg-accent' : undefined
            )}
            key={conversation.id}
          >
            <button
              className='flex min-w-0 flex-1 items-center gap-2 rounded-md px-2 py-2 text-left text-sm'
              onClick={() => props.onSelect(conversation.id)}
              type='button'
            >
              <MessageSquareIcon
                aria-hidden='true'
                className='text-muted-foreground size-4 shrink-0'
              />
              <span className='truncate'>
                {conversation.title || t('New conversation')}
              </span>
            </button>
            <Button
              aria-label={t('Delete conversation')}
              className='opacity-0 group-hover:opacity-100 focus-visible:opacity-100'
              onClick={() => props.onDelete(conversation.id)}
              size='icon-sm'
              variant='ghost'
            >
              <Trash2Icon aria-hidden='true' />
            </Button>
          </div>
        ))}
      </div>
    </aside>
  )
}
