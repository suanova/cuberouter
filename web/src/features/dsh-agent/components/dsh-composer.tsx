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
  FileIcon,
  ImageIcon,
  LoaderCircleIcon,
  PaperclipIcon,
  SendIcon,
  SquareIcon,
  XIcon,
} from 'lucide-react'
import { type KeyboardEvent, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Textarea } from '@/components/ui/textarea'

import { getRequestErrorMessage, uploadDshAttachment } from '../api'
import type { DshAttachment, DshUploadSettings } from '../types'

type PendingAttachment = {
  id: string
  name: string
  size: number
  status: 'uploading' | 'ready' | 'error'
  attachment?: DshAttachment
  error?: string
}

type DshComposerProps = {
  disabled: boolean
  isRunning: boolean
  uploads?: DshUploadSettings | null
  onCancel: () => void
  onSubmit: (prompt: string, attachments: DshAttachment[]) => void
}

function nextPendingId(): string {
  return `file-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

function formatFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return ''
  }
  if (bytes < 1024) {
    return `${bytes} B`
  }
  if (bytes < 1024 * 1024) {
    return `${Math.round(bytes / 1024)} KB`
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function PendingIcon(props: { status: PendingAttachment['status'] }) {
  if (props.status === 'uploading') {
    return (
      <LoaderCircleIcon
        aria-hidden='true'
        className='text-muted-foreground size-3.5 shrink-0 animate-spin'
      />
    )
  }
  return (
    <FileIcon
      aria-hidden='true'
      className={
        props.status === 'error'
          ? 'text-destructive size-3.5 shrink-0'
          : 'text-muted-foreground size-3.5 shrink-0'
      }
    />
  )
}

export function DshComposer(props: DshComposerProps) {
  const { t } = useTranslation()
  const [text, setText] = useState('')
  const [pending, setPending] = useState<PendingAttachment[]>([])
  // 输入法组词期间按回车是「确认候选」，不是发送；组一结束就恢复。
  const [isComposing, setIsComposing] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const photoInputRef = useRef<HTMLInputElement>(null)

  const uploads = props.uploads
  const uploading = pending.some((item) => item.status === 'uploading')
  const canAttach = Boolean(uploads?.enabled) && !props.disabled
  const canSend =
    !props.disabled && !props.isRunning && !uploading && text.trim() !== ''

  const updatePending = (
    id: string,
    updater: (item: PendingAttachment) => PendingAttachment
  ) => {
    setPending((items) =>
      items.map((item) => (item.id === id ? updater(item) : item))
    )
  }

  const statusLabel = (item: PendingAttachment): string => {
    if (item.status === 'uploading') {
      return t('Uploading…')
    }
    if (item.status === 'error') {
      return t('Upload failed')
    }
    return formatFileSize(item.size)
  }

  const startUpload = (file: File) => {
    const id = nextPendingId()
    setPending((items) => [
      ...items,
      { id, name: file.name, size: file.size, status: 'uploading' },
    ])
    void uploadDshAttachment(file)
      .then((attachment) => {
        updatePending(id, (item) => ({
          ...item,
          status: 'ready',
          name: attachment.name,
          size: attachment.size ?? item.size,
          attachment,
        }))
      })
      .catch((error: unknown) => {
        const message = getRequestErrorMessage(error)
        updatePending(id, (item) => ({
          ...item,
          status: 'error',
          error: message,
        }))
        toast.error(t('Upload failed'), { description: message })
      })
  }

  const addFiles = (files: FileList | null) => {
    if (!files || files.length === 0 || !uploads?.enabled) {
      return
    }
    const room = Math.max(0, uploads.maxFiles - pending.length)
    if (files.length > room) {
      toast.error(
        t('You can attach up to {{count}} files.', { count: uploads.maxFiles })
      )
    }
    const accepted = [...files].slice(0, room).filter((file) => {
      if (uploads.maxBytes > 0 && file.size > uploads.maxBytes) {
        toast.error(
          t('The file is too large. The limit is {{size}}.', {
            size: formatFileSize(uploads.maxBytes),
          })
        )
        return false
      }
      return true
    })
    for (const file of accepted) {
      startUpload(file)
    }
  }

  const removePending = (id: string) => {
    setPending((items) => items.filter((item) => item.id !== id))
  }

  const submit = () => {
    if (!canSend) {
      return
    }
    const attachments = pending.flatMap((item) =>
      item.status === 'ready' && item.attachment ? [item.attachment] : []
    )
    props.onSubmit(text.trim(), attachments)
    setText('')
    setPending([])
  }

  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key !== 'Enter' || event.shiftKey) {
      return
    }
    if (isComposing || event.nativeEvent.isComposing) {
      return
    }
    event.preventDefault()
    submit()
  }

  return (
    <div className='border-t'>
      <div className='mx-auto w-full max-w-4xl space-y-2 px-4 py-3'>
        {pending.length > 0 && (
          <ul aria-label={t('Attached files')} className='flex flex-wrap gap-2'>
            {pending.map((item) => (
              <li
                className='bg-muted/50 flex max-w-64 items-center gap-1.5 rounded-md border px-2 py-1 text-xs'
                key={item.id}
              >
                <PendingIcon status={item.status} />
                <span className='truncate' title={item.name}>
                  {item.name}
                </span>
                <span
                  className='text-muted-foreground shrink-0'
                  title={item.error}
                >
                  {statusLabel(item)}
                </span>
                <Button
                  aria-label={t('Remove attachment')}
                  className='text-muted-foreground hover:text-foreground -mr-1 shrink-0'
                  onClick={() => removePending(item.id)}
                  size='icon-xs'
                  variant='ghost'
                >
                  <XIcon aria-hidden='true' />
                </Button>
              </li>
            ))}
          </ul>
        )}
        <Textarea
          aria-label={t('Message the DSH agent')}
          disabled={props.disabled}
          onChange={(event) => setText(event.target.value)}
          onCompositionEnd={() => setIsComposing(false)}
          onCompositionStart={() => setIsComposing(true)}
          onKeyDown={handleKeyDown}
          placeholder={t('Describe the task for the agent…')}
          rows={3}
          value={text}
        />
        <div className='flex items-center gap-2'>
          {canAttach && (
            <>
              <DropdownMenu modal={false}>
                <DropdownMenuTrigger
                  render={
                    <Button
                      aria-label={t('Attach')}
                      size='icon-sm'
                      variant='ghost'
                    />
                  }
                >
                  <PaperclipIcon aria-hidden='true' />
                </DropdownMenuTrigger>
                <DropdownMenuContent align='start'>
                  <DropdownMenuItem
                    onClick={() => fileInputRef.current?.click()}
                  >
                    <FileIcon className='mr-2' size={16} />
                    {t('Upload file')}
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onClick={() => photoInputRef.current?.click()}
                  >
                    <ImageIcon className='mr-2' size={16} />
                    {t('Upload photo')}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
              <input
                accept={uploads?.accept.join(',') || undefined}
                aria-label={t('Upload file')}
                className='hidden'
                multiple
                onChange={(event) => {
                  addFiles(event.target.files)
                  event.target.value = ''
                }}
                ref={fileInputRef}
                type='file'
              />
              <input
                accept='image/*'
                aria-label={t('Upload photo')}
                className='hidden'
                multiple
                onChange={(event) => {
                  addFiles(event.target.files)
                  event.target.value = ''
                }}
                ref={photoInputRef}
                type='file'
              />
            </>
          )}
          <p className='text-muted-foreground text-xs'>
            {props.isRunning
              ? t('Waiting for the current turn to finish…')
              : t('Enter to send, Shift + Enter for a new line.')}
          </p>
          {props.isRunning ? (
            <Button
              className='border-destructive/25 bg-destructive/10 text-destructive hover:bg-destructive/15 ml-auto font-medium'
              onClick={props.onCancel}
              size='sm'
              variant='secondary'
            >
              <SquareIcon aria-hidden='true' className='fill-current' />
              {t('Stop waiting')}
            </Button>
          ) : (
            <Button
              aria-label={t('Send')}
              className='ml-auto'
              disabled={!canSend}
              onClick={submit}
              size='sm'
            >
              <SendIcon aria-hidden='true' />
              {t('Send')}
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}
