import { Trash2 } from 'lucide-react'
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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import dayjs from '@/lib/dayjs'

import type { GeneratedImage, GenerationResult, HistoryEntry } from '../types'
import { ResultGallery } from './result-gallery'

interface HistoryListProps {
  entries: HistoryEntry[]
  loading?: boolean
  storageAvailable?: boolean
  onDelete: (id: string) => void
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

function toGenerationResult(entry: HistoryEntry): GenerationResult {
  return {
    created: Math.floor(entry.createdAt / 1000),
    images: entry.imageUrls.map((url): GeneratedImage => ({ url })),
    raw: null,
  }
}

function renderBody(
  entries: HistoryEntry[],
  storageAvailable: boolean,
  selectedId: string | null,
  onSelect: (id: string | null) => void,
  onDelete: (id: string) => void,
  t: (key: string, options?: Record<string, unknown>) => string
) {
  if (!storageAvailable) {
    return (
      <p className='border-border text-muted-foreground rounded-lg border border-dashed p-4 text-center text-xs'>
        {t('History saving is not available in this browser.')}
      </p>
    )
  }
  if (entries.length === 0) {
    return (
      <p className='border-border text-muted-foreground rounded-lg border border-dashed p-4 text-center text-xs'>
        {t('No generations in this browser yet.')}
      </p>
    )
  }
  return (
    <ul className='flex flex-col gap-1.5'>
      {entries.map((entry) => {
        const isSelected = entry.id === selectedId
        return (
          <li key={entry.id}>
            <div className='flex items-start gap-2'>
              <button
                type='button'
                onClick={() => onSelect(isSelected ? null : entry.id)}
                aria-expanded={isSelected}
                className='border-border hover:bg-muted flex min-w-0 flex-1 items-center gap-3 rounded-lg border p-2 text-left transition-colors'
              >
                {entry.imageUrls[0] ? (
                  <img
                    src={entry.imageUrls[0]}
                    alt=''
                    loading='lazy'
                    aria-hidden='true'
                    className='border-border bg-muted size-12 shrink-0 rounded-md border object-cover'
                  />
                ) : null}
                <span className='min-w-0 flex-1'>
                  <span className='block truncate text-xs font-medium'>
                    {entry.prompt}
                  </span>
                  <span className='text-muted-foreground block text-[11px]'>
                    {t(
                      '{{count}} image · {{ratio}} · {{model}} · {{elapsed}}',
                      {
                        count: entry.imageUrls.length,
                        ratio: entry.params.ratio,
                        model: entry.model,
                        elapsed: formatElapsed(entry.elapsedMs),
                      }
                    )}
                    {' · '}
                    {dayjs(entry.createdAt).format('MM-DD HH:mm')}
                  </span>
                </span>
              </button>
              <Button
                type='button'
                variant='ghost'
                size='icon-xs'
                onClick={() => onDelete(entry.id)}
                aria-label={t('Delete')}
              >
                <Trash2 aria-hidden='true' />
              </Button>
            </div>
            {isSelected ? (
              <div className='border-border bg-muted/20 mt-2 rounded-lg border p-3'>
                <ResultGallery result={toGenerationResult(entry)} />
              </div>
            ) : null}
          </li>
        )
      })}
    </ul>
  )
}

/**
 * 本地生成历史：条目点击后内联展开完整结果（含下载），
 * 图片为自包含 data URL，可跨会话还原。
 */
export function HistoryList({
  entries,
  loading = false,
  storageAvailable = true,
  onDelete,
  onClear,
}: HistoryListProps) {
  const { t } = useTranslation()
  const [selectedId, setSelectedId] = useState<string | null>(null)

  if (loading) {
    return null
  }

  return (
    <section
      aria-label={t('Recent generations')}
      className='flex flex-col gap-2'
    >
      <header className='flex items-center justify-between'>
        <h3 className='text-muted-foreground text-xs font-semibold tracking-wide uppercase'>
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

      {renderBody(
        entries,
        storageAvailable,
        selectedId,
        setSelectedId,
        onDelete,
        t
      )}
    </section>
  )
}
