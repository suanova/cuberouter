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

import { CopyButton } from '@/components/copy-button'

interface DebugPanelProps {
  requestBody: unknown
  rawResponse: unknown
}

const MAX_DEBUG_STRING_LEN = 200

function toJson(value: unknown): string {
  try {
    return (
      JSON.stringify(
        value,
        (_key: string, val: unknown) => {
          if (typeof val === 'string' && val.length > MAX_DEBUG_STRING_LEN) {
            return `${val.slice(0, MAX_DEBUG_STRING_LEN)}…(${val.length} chars total)`
          }
          return val
        },
        2
      ) ?? ''
    )
  } catch {
    return String(value)
  }
}

export function DebugPanel({
  requestBody,
  rawResponse,
}: DebugPanelProps) {
  const { t } = useTranslation()

  if (requestBody === null && rawResponse === null) {
    return null
  }

  return (
    <section
      aria-label={t('Request and response')}
      className='flex flex-col gap-2'
    >
      <h3 className='text-xs font-semibold uppercase tracking-wide text-muted-foreground'>
        {t('Request and response')}
      </h3>
      {requestBody !== null ? (
        <div className='relative'>
          <pre className='max-h-48 overflow-auto rounded-lg border border-border bg-muted/40 p-3 text-[11px] leading-relaxed'>
            {toJson(requestBody)}
          </pre>
          <CopyButton
            value={toJson(requestBody)}
            className='absolute right-2 top-2'
            size='icon'
            tooltip={t('Copy')}
          />
        </div>
      ) : null}
      {rawResponse !== null && rawResponse !== undefined ? (
        <div className='relative'>
          <pre className='max-h-48 overflow-auto rounded-lg border border-border bg-muted/40 p-3 text-[11px] leading-relaxed'>
            {toJson(rawResponse)}
          </pre>
          <CopyButton
            value={toJson(rawResponse)}
            className='absolute right-2 top-2'
            size='icon'
            tooltip={t('Copy')}
          />
        </div>
      ) : null}
    </section>
  )
}
