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
import { useTranslation } from 'react-i18next'
import { Trash2 } from 'lucide-react'

import { Button } from '@/components/ui/button'
import dayjs from '@/lib/dayjs'

import type { HistoryEntry } from '../types'

interface HistoryListProps {
  entries: HistoryEntry[]
  disabled?: boolean
  onSelect: (entry: HistoryEntry) => void
  onClear: () => void
}

function formatElapsed(ms: number): string {
  const seconds = Math.round(ms / 1000)
  if (seconds < 60) {
    return `${seconds}s`
  }
  const minutes = Math.floor(seconds / 60)
  const rest = seconds % 60
  return `${minutes}m ${rest}s`
}

export function HistoryList({
  entries,
  disabled = false,
  onSelect,
  onClear,
}: HistoryListProps) {
  const { t } = useTranslation()

  return (
    <section
      aria-label={t('Recent generations')}
      className='flex flex-col gap-2'
    >
      <header className='flex items-center justify-between'>
        <h3 className='text-xs font-semibold uppercase tracking-wide text-muted-foreground'>
          {t('Recent generations')}
        </h3>
        {entries.length > 0 ? (
          <Button
            type='button'
            variant='ghost'
            size='xs'
            onClick={onClear}
            aria-label={t('Clear history')}
          >
            <Trash2 aria-hidden='true' />
            {t('Clear')}
          </Button>
        ) : null}
      </header>

      {entries.length === 0 ? (
        <p className='rounded-lg border border-dashed border-border p-4 text-center text-xs text-muted-foreground'>
          {t('No generations in this browser yet.')}
        </p>
      ) : (
        <ul className='flex flex-col gap-1.5'>
          {entries.map((entry) => (
            <li key={entry.id}>
              <button
                type='button'
                disabled={disabled}
                onClick={() => onSelect(entry)}
                className='flex w-full items-center gap-3 rounded-lg border border-border p-2 text-left transition-colors hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50'
              >
                {entry.imageUrls[0] ? (
                  <img
                    src={entry.imageUrls[0]}
                    alt=''
                    loading='lazy'
                    aria-hidden='true'
                    className='size-12 shrink-0 rounded-md border border-border bg-muted object-cover'
                  />
                ) : null}
                <span className='min-w-0 flex-1'>
                  <span className='block truncate text-xs font-medium'>
                    {entry.prompt}
                  </span>
                  <span className='block text-[11px] text-muted-foreground'>
                    {t('{{count}} image · {{ratio}} · {{elapsed}}', {
                      count: entry.imageUrls.length,
                      ratio: entry.params.ratio,
                      elapsed: formatElapsed(entry.elapsedMs),
                    })}
                    {' · '}
                    {dayjs(entry.createdAt).format('MM-DD HH:mm')}
                  </span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
