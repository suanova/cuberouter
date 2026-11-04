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

import {
  usageCalendarLocale,
  usageRangeWithinLimit,
} from '../lib/date-range'

interface UsageDateRangePickerProps {
  start: Date
  end: Date
  onChange: (range: { start: Date; end: Date }) => void
}

export function UsageDateRangePicker(props: UsageDateRangePickerProps) {
  const { t, i18n } = useTranslation()
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState<DateRange | undefined>({
    from: props.start,
    to: props.end,
  })
  const calendarLocale = usageCalendarLocale(i18n.language)

  const handleSelect = (range: DateRange | undefined) => {
    setDraft(range)
    if (!range?.from || !range.to) return
    if (!usageRangeWithinLimit(range.from, range.to)) return
    props.onChange({ start: range.from, end: range.to })
    setOpen(false)
  }

  const handleOpenChange = (nextOpen: boolean) => {
    // 每次打开都以当前已应用区间为准；URL 变化后不会残留旧草稿。
    if (nextOpen) setDraft({ from: props.start, to: props.end })
    setOpen(nextOpen)
  }

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
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
            (draft?.from ? !usageRangeWithinLimit(draft.from, date) : false)
          }
        />
        <div className='flex items-center justify-between gap-3 border-t px-3 py-2'>
          <p className='text-muted-foreground text-xs'>
            {t('The range cannot exceed 31 days.')}
          </p>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            onClick={() => setDraft(undefined)}
          >
            {t('Reset')}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}
