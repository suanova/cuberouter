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
import { useMutation, useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { api } from '@/lib/api'

type AuditRow = {
  id: number
  job_id: string
  username: string
  user_id: number
  created_at: number
  expires_at: number
  channel_id: number
  source: string
  state: string
  moderation_state: string
  moderation_policy: string
  user_prompt: string
  model_prompt: string
  parameters: string
  input_sha256: string
  output_sha256: string
  moderation_result: string
  token_name: string
}
type Filters = {
  user_id: string
  job_id: string
  output_sha256: string
  source: string
  moderation_state: string
}
type AuditResponse = {
  success: boolean
  message?: string
  data: AuditRow[]
  next_before_id: number
  audit_enabled: boolean
}
type PolicyResponse = {
  success: boolean
  message?: string
  data: { group: string; moderation: string }[]
  groups: string[]
}
const emptyFilters: Filters = {
  user_id: '',
  job_id: '',
  output_sha256: '',
  source: '',
  moderation_state: '',
}

export function ImageStudioAudit() {
  const { t } = useTranslation()
  const [draft, setDraft] = useState(emptyFilters)
  const [filters, setFilters] = useState(emptyFilters)
  const [cursor, setCursor] = useState(0)
  const [group, setGroup] = useState('')
  const [moderation, setModeration] = useState('external')
  const audits = useQuery({
    queryKey: ['image-audits', filters, cursor],
    queryFn: async () => {
      const { data } = await api.get<AuditResponse>('/api/image-studio-audit', {
        params: {
          ...filters,
          user_id: filters.user_id || undefined,
          before_id: cursor || undefined,
        },
      })
      if (!data.success) {
        throw new Error(data.message || 'Image audit lookup failed.')
      }
      return data
    },
    retry: false,
  })
  const policies = useQuery({
    queryKey: ['image-api-policies'],
    queryFn: async () => {
      const { data } = await api.get<PolicyResponse>('/api/image-api-policy')
      if (!data.success) {
        throw new Error(data.message || 'Image API policy lookup failed.')
      }
      return data.groups.map((group) => ({
        group,
        moderation:
          data.data?.find((p) => p.group === group)?.moderation || 'external',
      }))
    },
    retry: false,
  })
  const save = useMutation({
    mutationFn: async () => {
      const { data } = await api.put<{ success: boolean; message?: string }>(
        '/api/image-api-policy',
        { group, moderation }
      )
      if (!data.success) {
        throw new Error(data.message || 'Image API policy could not be saved.')
      }
    },
    onSuccess: () => {
      void policies.refetch()
    },
  })
  return (
    <main className='space-y-6 p-4 md:p-6'>
      <h1 className='text-2xl font-semibold'>{t('Image audit')}</h1>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Prompts, settings, image hashes and review scores are retained for 30 days. Images are not stored in this audit.'
        )}
      </p>
      {audits.data?.audit_enabled === false && (
        <p role='alert'>
          {t('Image audit recording is disabled on this server.')}
        </p>
      )}
      <section
        className='bg-card space-y-3 rounded-xl border p-4'
        aria-label={t('Image API review policy')}
      >
        <h2 className='font-medium'>{t('Image API review policy')}</h2>
        <p className='text-muted-foreground text-sm'>
          {t(
            'API review defaults to external. Required review releases only approved images. Browser Studio review is configured separately through moderation channels.'
          )}
        </p>
        <div className='flex flex-wrap items-end gap-3'>
          <label className='space-y-1 text-sm'>
            {t('Account group')}
            <select
              className='bg-background block rounded-md border p-2'
              value={group}
              onChange={(e) => {
                setGroup(e.target.value)
                setModeration(
                  policies.data?.find((p) => p.group === e.target.value)
                    ?.moderation || 'external'
                )
                save.reset()
              }}
            >
              <option value=''>{t('Select account group')}</option>
              {policies.data?.map((p) => (
                <option key={p.group} value={p.group}>
                  {p.group}
                </option>
              ))}
            </select>
          </label>
          <label className='space-y-1 text-sm'>
            {t('Review policy')}
            <select
              className='bg-background block rounded-md border p-2'
              value={moderation}
              onChange={(e) => {
                setModeration(e.target.value)
                save.reset()
              }}
            >
              <option value='external'>
                {t('External review (integrator responsibility)')}
              </option>
              <option value='required'>{t('Gemma review required')}</option>
            </select>
          </label>
          <Button
            disabled={!group || save.isPending}
            onClick={() => save.mutate()}
          >
            {t('Save policy')}
          </Button>
          {save.isSuccess && <p role='status'>{t('Policy saved.')}</p>}
        </div>
      </section>
      <form
        className='flex flex-wrap items-end gap-3'
        onSubmit={(e) => {
          e.preventDefault()
          setFilters({ ...draft })
          setCursor(0)
        }}
      >
        {(['user_id', 'job_id', 'output_sha256'] as const).map((key) => (
          <label key={key} className='space-y-1 text-sm'>
            {t(
              {
                user_id: 'User ID',
                job_id: 'Job ID',
                output_sha256: 'Output SHA-256',
              }[key]
            )}
            <Input
              value={draft[key]}
              onChange={(e) => setDraft({ ...draft, [key]: e.target.value })}
            />
          </label>
        ))}
        <label className='space-y-1 text-sm'>
          {t('Source')}
          <select
            className='bg-background block rounded-md border p-2'
            value={draft.source}
            onChange={(e) => setDraft({ ...draft, source: e.target.value })}
          >
            <option value=''>{t('All')}</option>
            <option value='studio'>{t('Browser Studio')}</option>
            <option value='api'>API</option>
          </select>
        </label>
        <label className='space-y-1 text-sm'>
          {t('Review result')}
          <select
            className='bg-background block rounded-md border p-2'
            value={draft.moderation_state}
            onChange={(e) =>
              setDraft({ ...draft, moderation_state: e.target.value })
            }
          >
            <option value=''>{t('All')}</option>
            {['passed', 'blocked', 'error', 'external', 'disabled'].map(
              (state) => (
                <option key={state} value={state}>
                  {t(state)}
                </option>
              )
            )}
          </select>
        </label>
        <Button type='submit'>{t('Search')}</Button>
        <Button
          type='button'
          variant='outline'
          onClick={() => {
            setCursor(0)
            void audits.refetch()
          }}
        >
          {t('Refresh')}
        </Button>
      </form>
      {Object.entries({
        audits: audits.error,
        policies: policies.error,
        save: save.error,
      })
        .filter(([, error]) => Boolean(error))
        .map(([key, error]) => (
          <p role='alert' key={key}>
            {t(
              error instanceof Error
                ? error.message
                : 'Image audit lookup failed.'
            )}
          </p>
        ))}
      {audits.isPending && <p role='status'>{t('Loading...')}</p>}
      {audits.data?.data.length === 0 && (
        <p>{t('No image audit records found.')}</p>
      )}
      <div className='space-y-3'>
        {audits.data?.data.map((row) => (
          <details key={row.id} className='bg-card rounded-xl border p-4'>
            <summary className='cursor-pointer break-words'>
              {new Date(row.created_at * 1000).toLocaleString()} ·{' '}
              {row.username} ({row.user_id}) · {row.source || 'studio'} ·{' '}
              {t(row.state)} · {t(row.moderation_state || 'pending')}
            </summary>
            <dl className='mt-4 grid gap-3 text-sm md:grid-cols-2'>
              {Object.entries({
                'Job ID': row.job_id,
                'Channel ID': String(row.channel_id),
                'API key name': row.token_name,
                'Review policy': row.moderation_policy,
                'User prompt': row.user_prompt,
                'Model prompt': row.model_prompt,
                Parameters: row.parameters,
                'Input SHA-256': row.input_sha256,
                'Output SHA-256': row.output_sha256,
                'Review scores and thresholds': row.moderation_result,
                'Expires at': new Date(row.expires_at * 1000).toLocaleString(),
              }).map(([label, value]) => (
                <div key={label} className='min-w-0'>
                  <dt className='text-muted-foreground'>{t(label)}</dt>
                  <dd className='mt-1 break-all whitespace-pre-wrap'>
                    {value || '—'}
                  </dd>
                </div>
              ))}
            </dl>
          </details>
        ))}
      </div>
      {!!audits.data?.next_before_id && (
        <Button
          variant='outline'
          onClick={() => setCursor(audits.data?.next_before_id ?? 0)}
        >
          {t('Older records')}
        </Button>
      )}
    </main>
  )
}
