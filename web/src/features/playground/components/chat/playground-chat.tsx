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
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Conversation,
  ConversationContent,
  ConversationScrollButton,
} from '@/components/ai-elements/conversation'
import { Loader } from '@/components/ai-elements/loader'
import { Message } from '@/components/ai-elements/message'

import { MESSAGE_ROLES } from '../../constants'
import {
  getChatMessageRenderState,
  getEditingMessageContent,
  getMessageAlignment,
  getMessageContent,
  getPreviousUserMessage,
  isErrorMessage,
} from '../../lib'
import type {
  Message as MessageType,
  PlaygroundMessageLayoutMode,
} from '../../types'
import { useEnabledPlugins } from '../../hooks/use-enabled-plugins'
import { MessageActions } from '../message/message-actions'
import { MessageErrorActions } from '../message/message-error-actions'
import { PlaygroundMessageContent } from '../message/playground-message-content'
import { PlaygroundMessageEditor } from '../message/playground-message-editor'
import { PlaygroundEmptyState } from './playground-empty-state'

const MAX_RENDERED_HISTORY_MESSAGES = 24

interface PlaygroundChatProps {
  messages: MessageType[]
  onCopyMessage?: (message: MessageType) => void
  onRegenerateMessage?: (message: MessageType) => void
  onEditMessage?: (message: MessageType) => void
  onDeleteMessage?: (message: MessageType) => void
  onSelectPrompt?: (prompt: string) => void
  isGenerating?: boolean
  isLoadingMessages?: boolean
  editingKey?: string | null
  onSaveEdit?: (newContent: string) => void
  onCancelEdit?: (open: boolean) => void
  onSaveEditAndSubmit?: (newContent: string) => void
  messageLayoutMode?: PlaygroundMessageLayoutMode
}

export function PlaygroundChat({
  messages,
  onCopyMessage,
  onRegenerateMessage,
  onEditMessage,
  onDeleteMessage,
  onSelectPrompt,
  isGenerating = false,
  isLoadingMessages = false,
  editingKey,
  onSaveEdit,
  onCancelEdit,
  onSaveEditAndSubmit,
  messageLayoutMode = 'alternating',
}: PlaygroundChatProps) {
  const { t } = useTranslation()
  const { plugins } = useEnabledPlugins()
  const pluginNameBySlug = useMemo(
    () => Object.fromEntries(plugins.map((plugin) => [plugin.slug, plugin.name])),
    [plugins]
  )
  const [editText, setEditText] = useState('')
  const [originalText, setOriginalText] = useState('')
  const [sourceMessageKeys, setSourceMessageKeys] = useState<
    ReadonlySet<string>
  >(() => new Set())
  const visibleMessageOffset = Math.max(
    0,
    messages.length - MAX_RENDERED_HISTORY_MESSAGES
  )
  const visibleMessages = messages.slice(visibleMessageOffset)

  function handleToggleMessageSource(message: MessageType): void {
    setSourceMessageKeys((currentKeys) => {
      const nextKeys = new Set(currentKeys)

      if (nextKeys.has(message.key)) {
        nextKeys.delete(message.key)
      } else {
        nextKeys.add(message.key)
      }

      return nextKeys
    })
  }

  useEffect(() => {
    if (!editingKey) return
    const content = getEditingMessageContent(messages, editingKey)
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setEditText(content)

    setOriginalText(content)
  }, [editingKey, messages])

  let chatContent = visibleMessages.map((message, visibleMessageIndex) => {
    const messageIndex = visibleMessageOffset + visibleMessageIndex
    const { alwaysShowActions, content, isEditing } = getChatMessageRenderState(
      messages,
      message,
      messageIndex,
      editingKey
    )
    const isError = isErrorMessage(message)
    const previousUserMessage = getPreviousUserMessage(messages, messageIndex)
    const errorPreviousUserMessage = isError ? previousUserMessage : null
    const alignment = getMessageAlignment(message, messageLayoutMode)
    const isSourceVisible = sourceMessageKeys.has(message.key)

    let pendingLabel: string | undefined
    let pluginSlug: string | undefined
    if (message.from === MESSAGE_ROLES.ASSISTANT && previousUserMessage) {
      const slugMatch = /@([a-z0-9-]+)/.exec(
        getMessageContent(previousUserMessage)
      )
      const plugin = slugMatch
        ? plugins.find((item) => item.slug === slugMatch[1])
        : undefined
      if (plugin) {
        pendingLabel = t('Using {{slug}}…', { slug: plugin.slug })
        pluginSlug = plugin.slug
      }
    }

    return (
      <Message
        className='group flex-row-reverse py-2.5'
        from={message.from}
        key={message.key}
      >
        <div className='w-full min-w-0 flex-1 basis-full'>
          {isEditing ? (
            <PlaygroundMessageEditor
              editText={editText}
              message={message}
              onCancelEdit={onCancelEdit}
              onEditTextChange={setEditText}
              onSaveEdit={onSaveEdit}
              onSaveEditAndSubmit={onSaveEditAndSubmit}
              originalText={originalText}
            />
          ) : (
            <PlaygroundMessageContent
              alignment={alignment}
              actions={
                <MessageActions
                  message={message}
                  onCopy={onCopyMessage}
                  onRegenerate={onRegenerateMessage}
                  onToggleSource={handleToggleMessageSource}
                  onEdit={onEditMessage}
                  onDelete={onDeleteMessage}
                  isSourceVisible={isSourceVisible}
                  isGenerating={isGenerating}
                  alwaysVisible={alwaysShowActions}
                  className='mt-1.5'
                />
              }
              isSourceVisible={isSourceVisible}
              message={message}
              pendingLabel={pendingLabel}
              pluginNameBySlug={pluginNameBySlug}
              pluginSlug={pluginSlug}
              errorActions={
                isError ? (
                  <MessageErrorActions
                    disabled={isGenerating}
                    onRetry={
                      onRegenerateMessage
                        ? () => onRegenerateMessage(message)
                        : undefined
                    }
                    onEditPrompt={
                      onEditMessage && errorPreviousUserMessage
                        ? () => onEditMessage(errorPreviousUserMessage)
                        : undefined
                    }
                    onDelete={
                      onDeleteMessage
                        ? () => onDeleteMessage(message)
                        : undefined
                    }
                  />
                ) : undefined
              }
              versionContent={content}
            />
          )}
        </div>
      </Message>
    )
  })

  if (visibleMessages.length === 0 && onSelectPrompt) {
    chatContent = [
      <PlaygroundEmptyState key='empty' onSelectPrompt={onSelectPrompt} />,
    ]
  }

  if (isLoadingMessages) {
    chatContent = [
      <div
        className='text-muted-foreground flex min-h-[min(520px,calc(100svh-18rem))] items-center justify-center gap-2 text-sm'
        key='loading'
      >
        <Loader />
        <span>{t('Loading conversation...')}</span>
      </div>,
    ]
  }

  return (
    <Conversation>
      {/* Remove outer padding; apply padding to inner centered container to align with input */}
      <ConversationContent className='p-0'>
        <div className='mx-auto w-full max-w-4xl px-4 py-4'>{chatContent}</div>
      </ConversationContent>
      <ConversationScrollButton />
    </Conversation>
  )
}
