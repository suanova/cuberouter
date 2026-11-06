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
import { BotIcon, PlusIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

import { PRESET_OPTIONS } from '../constants'
import type { DshModelOption } from '../types'

type DshHeaderProps = {
  models: DshModelOption[]
  model: string
  preset: string
  onModelChange: (model: string) => void
  onPresetChange: (preset: string) => void
  onNewConversation: () => void
}

export function DshHeader(props: DshHeaderProps) {
  const { t } = useTranslation()

  return (
    <header className='flex flex-wrap items-center gap-2 border-b px-4 py-2'>
      <div className='flex items-center gap-2'>
        <BotIcon aria-hidden='true' className='size-5' />
        <span className='text-sm font-semibold'>{t('DSH Agent')}</span>
      </div>
      <div className='ml-auto flex flex-wrap items-center gap-2'>
        <NativeSelect
          aria-label={t('Model')}
          onChange={(event) => props.onModelChange(event.target.value)}
          size='sm'
          value={props.model}
        >
          {props.models.length === 0 && (
            <NativeSelectOption value=''>
              {t('No models available')}
            </NativeSelectOption>
          )}
          {props.models.map((model) => (
            <NativeSelectOption key={model.value} value={model.value}>
              {model.label}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        <NativeSelect
          aria-label={t('Preset')}
          onChange={(event) => props.onPresetChange(event.target.value)}
          size='sm'
          value={props.preset}
        >
          {PRESET_OPTIONS.map((preset) => (
            <NativeSelectOption key={preset.value} value={preset.value}>
              {t(preset.label)}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        <Button onClick={props.onNewConversation} size='sm' variant='outline'>
          <PlusIcon aria-hidden='true' />
          {t('New conversation')}
        </Button>
      </div>
    </header>
  )
}
