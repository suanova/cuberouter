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
import {
  Cancel01Icon,
  SentIcon,
  SparklesIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'
import { withBasePath } from '@/lib/base-path'

import { streamAccountAssistantChat } from '../api'
import { TOOL_STATUS_KEYS } from '../constants'
import type {
  AccountAssistantChatMessage,
  AccountAssistantHistoryMessage,
  AccountAssistantSseEvent,
} from '../types'
import { AssistantMessage } from './assistant-message'

type AssistantChatPanelProps = {
  supportUrl: string
  open: boolean
  onClose: () => void
}

const MAX_MESSAGE_LENGTH = 2000

function nextMessageId(): string {
  if (
    typeof crypto !== 'undefined' &&
    typeof crypto.randomUUID === 'function'
  ) {
    return crypto.randomUUID()
  }
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`
}

export function AssistantChatPanel(props: AssistantChatPanelProps) {
  const { t } = useTranslation()
  const { open } = props
  const [messages, setMessages] = useState<AccountAssistantChatMessage[]>([])
  const [input, setInput] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [activeTool, setActiveTool] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)
  const inputRef = useRef<HTMLTextAreaElement | null>(null)
  const scrollRef = useRef<HTMLDivElement | null>(null)

  // Focus the composer whenever the panel becomes visible. The panel stays
  // mounted (so the transcript and in-flight stream survive close/reopen),
  // so focus only on the open transition.
  const wasOpenRef = useRef(open)
  useEffect(() => {
    if (open && !wasOpenRef.current) {
      inputRef.current?.focus()
    }
    wasOpenRef.current = open
  }, [open])

  useEffect(() => {
    const container = scrollRef.current
    if (container) {
      container.scrollTop = container.scrollHeight
    }
  }, [messages, streaming, activeTool])

  const openSupport = () => {
    const configured = props.supportUrl.trim()
    const target = /^(https?:\/\/|mailto:)/i.test(configured)
      ? configured
      : withBasePath('/about')
    window.open(target, '_blank', 'noopener,noreferrer')
  }

  const handleEvent = (
    event: AccountAssistantSseEvent,
    addAssistant: (
      message: Omit<AccountAssistantChatMessage, 'id' | 'role'>
    ) => void
  ) => {
    if (event.type === 'tool') {
      setActiveTool(event.tool)
      return
    }
    if (event.type === 'final') {
      addAssistant({ content: event.content, kind: 'text' })
      return
    }
    if (event.type === 'refusal') {
      addAssistant({ content: '', kind: 'refusal', code: event.code })
      return
    }
    if (event.type === 'error') {
      addAssistant({
        content: event.code ? '' : (event.message ?? ''),
        kind: 'error',
        code: event.code,
      })
    }
  }

  const send = async () => {
    const text = input.trim()
    if (!text || streaming) return

    const history: AccountAssistantHistoryMessage[] = messages
      .filter(
        (message) =>
          (message.kind === 'text' || message.kind === undefined) &&
          message.content.trim() !== ''
      )
      .map((message) => ({ role: message.role, content: message.content }))

    setMessages((previous) => [
      ...previous,
      { id: nextMessageId(), role: 'user', content: text },
    ])
    setInput('')
    setStreaming(true)
    setActiveTool(null)

    const controller = new AbortController()
    abortRef.current = controller
    let assistantAdded = false
    const addAssistant = (
      message: Omit<AccountAssistantChatMessage, 'id' | 'role'>
    ) => {
      assistantAdded = true
      setMessages((previous) => [
        ...previous,
        { id: nextMessageId(), role: 'assistant', ...message },
      ])
    }

    try {
      await streamAccountAssistantChat({
        message: text,
        history,
        signal: controller.signal,
        onEvent: (event) => handleEvent(event, addAssistant),
      })
      if (!assistantAdded) {
        addAssistant({ kind: 'error', code: 'unavailable', content: '' })
      }
    } catch (error) {
      if (!controller.signal.aborted) {
        addAssistant({
          kind: 'error',
          code: 'unavailable',
          content: error instanceof Error ? error.message : '',
        })
      }
    } finally {
      setStreaming(false)
      setActiveTool(null)
      abortRef.current = null
    }
  }

  const handleInputKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (
      event.key === 'Enter' &&
      !event.shiftKey &&
      !event.nativeEvent.isComposing
    ) {
      event.preventDefault()
      void send()
    }
  }

  const handlePanelKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      props.onClose()
    }
  }

  const toolStatus = activeTool
    ? t(TOOL_STATUS_KEYS[activeTool] ?? 'Querying your data…')
    : t('Thinking…')

  return (
    <div
      role='dialog'
      aria-label={t('Account Assistant')}
      aria-hidden={!open}
      hidden={!open}
      onKeyDown={handlePanelKeyDown}
      className='bg-background fixed right-4 bottom-24 z-50 flex h-[min(560px,calc(100svh-8rem))] w-[min(380px,calc(100vw-2rem))] flex-col overflow-hidden rounded-2xl border shadow-2xl sm:right-6'
    >
      <div className='flex h-12 shrink-0 items-center justify-between border-b px-3'>
        <div className='flex items-center gap-2 text-sm font-medium'>
          <HugeiconsIcon icon={SparklesIcon} className='text-primary size-4' />
          {t('Account Assistant')}
        </div>
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          aria-label={t('Close account assistant')}
          onClick={props.onClose}
        >
          <HugeiconsIcon icon={Cancel01Icon} />
        </Button>
      </div>

      <div
        ref={scrollRef}
        role='log'
        aria-live='polite'
        aria-label={t('Account Assistant conversation')}
        className='min-h-0 flex-1 space-y-3 overflow-y-auto px-3 py-3'
      >
        {messages.length === 0 && <EmptyState />}

        {messages.map((message) =>
          message.role === 'user' ? (
            <div
              key={message.id}
              className='bg-primary text-primary-foreground ml-auto max-w-[85%] rounded-2xl px-3 py-2 text-sm whitespace-pre-wrap'
            >
              {message.content}
            </div>
          ) : (
            <AssistantMessage
              key={message.id}
              message={message}
              onContact={openSupport}
            />
          )
        )}

        {streaming && (
          <div
            role='status'
            className='text-muted-foreground flex items-center gap-2 px-1 text-xs'
          >
            <Spinner className='size-3.5' />
            {toolStatus}
          </div>
        )}
      </div>

      <form
        className='flex shrink-0 items-end gap-2 border-t p-2'
        onSubmit={(event) => {
          event.preventDefault()
          void send()
        }}
      >
        <Textarea
          ref={inputRef}
          value={input}
          onChange={(event) => setInput(event.target.value)}
          onKeyDown={handleInputKeyDown}
          placeholder={t('Ask about your account…')}
          aria-label={t('Ask about your account…')}
          maxLength={MAX_MESSAGE_LENGTH}
          disabled={streaming}
          className='max-h-28 min-h-9 resize-none py-1.5'
        />
        {streaming ? (
          <Button
            type='button'
            variant='outline'
            size='icon'
            aria-label={t('Stop')}
            onClick={() => abortRef.current?.abort()}
          >
            <HugeiconsIcon icon={Cancel01Icon} />
          </Button>
        ) : (
          <Button
            type='submit'
            size='icon'
            aria-label={t('Send')}
            disabled={input.trim() === ''}
          >
            <HugeiconsIcon icon={SentIcon} />
          </Button>
        )}
      </form>
    </div>
  )
}

function EmptyState() {
  const { t } = useTranslation()
  return (
    <div className='text-muted-foreground px-2 py-6 text-center text-sm'>
      <p className='text-foreground font-medium'>{t('Account Assistant')}</p>
      <p className='mt-1'>
        {t('Ask about your balance, usage, keys, tasks, or help docs.')}
      </p>
      <p className='mt-2 text-xs'>
        {t(
          'The assistant only answers from your own account data and help docs.'
        )}
      </p>
    </div>
  )
}
