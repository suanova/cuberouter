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
import { SendIcon } from 'lucide-react'
import { type KeyboardEvent, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

type DshComposerProps = {
  disabled: boolean
  isRunning: boolean
  onSubmit: (prompt: string) => void
}

export function DshComposer(props: DshComposerProps) {
  const { t } = useTranslation()
  const [text, setText] = useState('')
  // 输入法组词期间按回车是「确认候选」，不是发送；组一结束就恢复。
  const [isComposing, setIsComposing] = useState(false)

  const canSend = !props.disabled && !props.isRunning && text.trim() !== ''

  const submit = () => {
    if (!canSend) {
      return
    }
    props.onSubmit(text.trim())
    setText('')
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
          <p className='text-muted-foreground text-xs'>
            {props.isRunning
              ? t('Waiting for the current turn to finish…')
              : t('Enter to send, Shift + Enter for a new line.')}
          </p>
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
        </div>
      </div>
    </div>
  )
}
