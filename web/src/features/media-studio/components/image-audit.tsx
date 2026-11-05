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
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Main } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

interface ImageAuditRow {
  job_id: string
  user_id: number
  model: string
  user_prompt: string
  model_prompt: string
  parameters: string
  input_hashes: string
  output_hash: string
  review: string
  channel_id: number
  state: string
  error?: string
  created_at: number
}

export function ImageAudit() {
  const { t } = useTranslation()
  const userID = useAuthStore((state) => state.auth.user?.id)
  const role = useAuthStore((state) => state.auth.user?.role)
  const canView = (role ?? 0) >= ROLE.SUPER_ADMIN
  const [cursors, setCursors] = useState<{ time: number; id: string }[]>([])
  const cursor = cursors.at(-1)
  const [user, setUser] = useState('')
  const validUser =
    user === '' ||
    (/^[1-9]\d*$/.test(user) && Number.isSafeInteger(Number(user)))
  const rows = useQuery({
    queryKey: ['studio-image-audit', userID, cursor, user],
    enabled: canView && validUser,
    queryFn: async () =>
      (
        await api.get<{ data: ImageAuditRow[]; has_more?: boolean }>(
          '/api/image-studio-audit',
          {
            params: {
              before: cursor?.time,
              before_job_id: cursor?.id,
              user_id: user || undefined,
            },
          }
        )
      ).data,
  })
  const records = rows.data?.data ?? []
  const hasOlder = rows.data?.has_more ?? records.length === 100
  if (!canView) return null
  return (
    <Main className='space-y-4 overflow-y-auto p-4 sm:p-6'>
      <h1 className='text-2xl font-semibold'>{t('Image output audit')}</h1>
      <p className='text-muted-foreground'>
        {t(
          'Prompts, parameters, image hashes and review decisions are retained for 30 days. Image bytes are excluded.'
        )}
      </p>
      <label className='block max-w-xs'>
        {t('User ID')}
        <Input
          type='number'
          min={1}
          step={1}
          aria-invalid={!validUser}
          value={user}
          onChange={(e) => {
            setUser(e.target.value)
            setCursors([])
          }}
        />
      </label>
      {!validUser && (
        <p role='alert'>{t('Enter a positive whole-number User ID.')}</p>
      )}
      {rows.isError && (
        <p role='alert'>
          {t('Image auditing is unavailable. Try again later.')}
        </p>
      )}
      {validUser && rows.isPending && <p role='status'>{t('Loading...')}</p>}
      {rows.data && !records.length && <p>{t('No image audit records.')}</p>}
      {records.map((row) => (
        <details key={row.job_id} className='rounded-xl border p-4'>
          <summary className='cursor-pointer break-all'>
            {new Date(row.created_at).toLocaleString()} / {row.model} /{' '}
            {row.state} / #{row.user_id} / {row.job_id}
          </summary>
          <dl className='mt-3 grid gap-2 text-sm'>
            {(
              [
                ['User instruction', row.user_prompt],
                ['Model instruction', row.model_prompt],
                ['Parameters', row.parameters],
                ['Input hashes', row.input_hashes],
                ['Output hash', row.output_hash],
                ['Output review', row.review],
                ['Error', row.error ?? ''],
                ['Channel ID', String(row.channel_id)],
              ] as const
            ).map(([label, value]) => (
              <div key={label}>
                <dt className='font-medium'>{t(label)}</dt>
                <dd className='text-muted-foreground break-all whitespace-pre-wrap'>
                  {value}
                </dd>
              </div>
            ))}
          </dl>
        </details>
      ))}
      <div className='flex flex-wrap gap-2'>
        <Button
          variant='outline'
          disabled={!validUser || rows.isFetching}
          onClick={() => void rows.refetch()}
        >
          {t('Refresh')}
        </Button>
        <Button
          variant='outline'
          disabled={!validUser || !cursor}
          onClick={() => setCursors((pages) => pages.slice(0, -1))}
        >
          {t('Newer records')}
        </Button>
        <Button
          variant='outline'
          disabled={!validUser || !hasOlder}
          onClick={() => {
            const last = records.at(-1)
            if (last) {
              setCursors((pages) => [
                ...pages,
                { time: last.created_at, id: last.job_id },
              ])
            }
          }}
        >
          {t('Older records')}
        </Button>
      </div>
    </Main>
  )
}
