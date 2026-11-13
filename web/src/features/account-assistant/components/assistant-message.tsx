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
import { CustomerService01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Markdown } from '@/components/ui/markdown'

import {
  ASSISTANT_ERROR_FALLBACK,
  ASSISTANT_ERROR_KEYS,
  ASSISTANT_REFUSAL_TEXT,
} from '../constants'
import type { AccountAssistantChatMessage } from '../types'

export function AssistantMessage(props: {
  message: AccountAssistantChatMessage
  onContact: () => void
}) {
  const { t } = useTranslation()
  const message = props.message
  if (message.kind === 'refusal') {
    return (
      <div className='bg-muted max-w-[90%] rounded-2xl px-3 py-2 text-sm'>
        <p>{t(ASSISTANT_REFUSAL_TEXT)}</p>
        <ContactButton
          label={t('Contact human support')}
          onClick={props.onContact}
        />
      </div>
    )
  }
  if (message.kind === 'error') {
    const errorKey = message.code
      ? ASSISTANT_ERROR_KEYS[message.code]
      : undefined
    const text = errorKey
      ? t(errorKey)
      : message.content.trim() || t(ASSISTANT_ERROR_FALLBACK)
    const needsContact =
      message.code === 'daily_limit' ||
      message.code === 'token_budget' ||
      message.code === 'empty_answer'
    return (
      <div className='border-destructive/40 bg-destructive/5 text-destructive max-w-[90%] rounded-2xl border px-3 py-2 text-sm'>
        <p>{text}</p>
        {needsContact && (
          <ContactButton
            label={t('Contact human support')}
            onClick={props.onContact}
          />
        )}
      </div>
    )
  }
  return (
    <div className='bg-muted max-w-[90%] rounded-2xl px-3 py-2'>
      <Markdown className='text-sm'>{message.content}</Markdown>
    </div>
  )
}

function ContactButton(props: { label: string; onClick: () => void }) {
  return (
    <Button
      type='button'
      variant='outline'
      size='sm'
      className='mt-2'
      onClick={props.onClick}
    >
      <HugeiconsIcon icon={CustomerService01Icon} data-icon='inline-start' />
      {props.label}
    </Button>
  )
}
