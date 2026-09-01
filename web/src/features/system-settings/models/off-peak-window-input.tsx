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
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { FieldGroup } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import type { OffPeakWindow } from '@/features/pricing/types'

import {
  draftsToWindow,
  hourDraftRegex,
  isOffPeakDisabled,
  isValidHourDraft,
  windowToDrafts,
  type OffPeakWindowDraft,
} from './off-peak-window-drafts'

export type OffPeakWindowInputProps = {
  value: OffPeakWindow
  /** 有效窗口回调窗口对象;草稿无效(空/越界小时)时回调 null,让表单拒绝保存 */
  onChange: (window: OffPeakWindow | null) => void
}

/** Stable serialization used to detect externally supplied value changes. */
const serializeWindow = (window: OffPeakWindow) =>
  `${window.start_hour}:${window.end_hour}:${window.timezone}`

export function OffPeakWindowInput({
  value,
  onChange,
}: OffPeakWindowInputProps) {
  const { t } = useTranslation()
  const [drafts, setDrafts] = useState<OffPeakWindowDraft>(() =>
    windowToDrafts(value)
  )
  const lastEmittedRef = useRef(serializeWindow(value))

  // Adopt externally supplied values (form reset after refetch) without
  // clobbering in-progress typing: only when the value differs from the last
  // one this component emitted.
  useEffect(() => {
    const nextSerialized = serializeWindow(value)
    if (nextSerialized !== lastEmittedRef.current) {
      setDrafts(windowToDrafts(value))
      lastEmittedRef.current = nextSerialized
    }
  }, [value])

  const handleChange = useCallback(
    (patch: Partial<OffPeakWindowDraft>) => {
      const nextDrafts = { ...drafts, ...patch }
      setDrafts(nextDrafts)
      const nextWindow = draftsToWindow(nextDrafts)
      if (nextWindow) {
        lastEmittedRef.current = serializeWindow(nextWindow)
        onChange(nextWindow)
      } else {
        // 草稿无效:显式上报,避免表单静默保留上一次的合法值
        onChange(null)
      }
    },
    [drafts, onChange]
  )

  const hourError = (draft: string) => {
    if (!draft) return t('Value is required')
    if (!isValidHourDraft(draft)) return t('Hour must be between 0 and 23')
    return undefined
  }
  const startHourError = hourError(drafts.startHour)
  const endHourError = hourError(drafts.endHour)

  const window = draftsToWindow(drafts)
  const showDisabledWarning = window !== null && isOffPeakDisabled(window)

  return (
    <FieldGroup className='gap-4'>
      <div className='grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1.5fr)] items-start gap-2'>
        <div className='space-y-2'>
          <span className='text-muted-foreground text-xs'>
            {t('Start hour')}
          </span>
          <Input
            inputMode='numeric'
            value={drafts.startHour}
            placeholder='22'
            aria-invalid={startHourError !== undefined}
            onChange={(event) => {
              const next = event.target.value
              if (hourDraftRegex.test(next)) {
                handleChange({ startHour: next })
              }
            }}
          />
          {startHourError !== undefined && (
            <p className='text-destructive text-xs'>{startHourError}</p>
          )}
        </div>
        <div className='space-y-2'>
          <span className='text-muted-foreground text-xs'>{t('End hour')}</span>
          <Input
            inputMode='numeric'
            value={drafts.endHour}
            placeholder='8'
            aria-invalid={endHourError !== undefined}
            onChange={(event) => {
              const next = event.target.value
              if (hourDraftRegex.test(next)) {
                handleChange({ endHour: next })
              }
            }}
          />
          {endHourError !== undefined && (
            <p className='text-destructive text-xs'>{endHourError}</p>
          )}
        </div>
        <div className='space-y-2'>
          <span className='text-muted-foreground text-xs'>
            {t('Timezone')}
          </span>
          <Input
            value={drafts.timezone}
            placeholder='Asia/Shanghai'
            onChange={(event) => handleChange({ timezone: event.target.value })}
          />
        </div>
      </div>
      {showDisabledWarning && (
        <p className='text-amber-600 text-xs dark:text-amber-400'>
          {t('Start and end hours are equal, so off-peak pricing is disabled.')}
        </p>
      )}
    </FieldGroup>
  )
}
