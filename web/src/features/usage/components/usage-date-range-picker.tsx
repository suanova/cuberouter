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
import { Calendar as CalendarIcon } from 'lucide-react'
import { useState } from 'react'
import { enUS, fr, ja, ru, vi, zhCN } from 'react-day-picker/locale'
import type { DateRange } from 'react-day-picker'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import dayjs from '@/lib/dayjs'

const calendarLocales = {
  en: enUS,
  zh: zhCN,
  fr,
  ru,
  ja,
  vi,
} as const

interface UsageDateRangePickerProps {
  start: Date
  end: Date
  onChange: (range: { start: Date; end: Date }) => void
}

const MAX_RANGE_DAYS = 31

export function UsageDateRangePicker(props: UsageDateRangePickerProps) {
  const { t, i18n } = useTranslation()
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState<DateRange | undefined>({
    from: props.start,
    to: props.end,
  })
  const calendarLocale =
    calendarLocales[i18n.language as keyof typeof calendarLocales] ?? enUS

  const handleSelect = (range: DateRange | undefined) => {
    setDraft(range)
    if (!range?.from || !range.to) return
    if (dayjs(range.to).diff(range.from, 'day') > MAX_RANGE_DAYS) return
    props.onChange({ start: range.from, end: range.to })
    setOpen(false)
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <Button variant='outline' className='justify-start font-normal' />
        }
      >
        <CalendarIcon className='me-2 size-4 opacity-60' />
        {dayjs(props.start).format('YYYY-MM-DD')} -{' '}
        {dayjs(props.end).format('YYYY-MM-DD')}
      </PopoverTrigger>
      <PopoverContent className='w-auto p-0' align='start'>
        <Calendar
          mode='range'
          captionLayout='dropdown'
          selected={draft}
          onSelect={handleSelect}
          locale={calendarLocale}
          numberOfMonths={1}
          disabled={(date: Date) =>
            date > new Date() ||
            (draft?.from
              ? dayjs(date).diff(draft.from, 'day') > MAX_RANGE_DAYS
              : false)
          }
        />
        <p className='text-muted-foreground border-t px-3 py-2 text-xs'>
          {t('The range cannot exceed 31 days.')}
        </p>
      </PopoverContent>
    </Popover>
  )
}
