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
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Skeleton } from '@/components/ui/skeleton'
import { listPlatformOrganizations } from '@/features/organization/api'
import { searchUsers } from '@/features/users/api'

import {
  createCubyAppGrant,
  deleteCubyAppGrant,
  listCubyAppGrants,
  type CubyApp,
  type CubyAppGrant,
  type CubyAppHolderType,
} from '../api'

const HOLDER_PAGE_SIZE = 100
const SEARCH_RESULT_LIMIT = 8

type CubyAppAccessDialogProps = {
  /** The app whose holders are shown; null keeps the dialog closed. */
  app: CubyApp | null
  onOpenChange: (open: boolean) => void
  /** A holder was added or removed; the counts on the page are stale. */
  onChanged: () => void
}

type HolderCandidate = {
  type: CubyAppHolderType
  id: number
  name: string
  detail: string
}

/**
 * Who has one app, and the place to give it or take it away.
 *
 * An app given to an organization reaches every active member of it, so an
 * organization is one row here however many people that is. Cuby picks the
 * change up the next time it asks: at sign-in, at start, or on its Apps page.
 */
export function CubyAppAccessDialog(props: CubyAppAccessDialogProps) {
  const { t } = useTranslation()
  const [holderType, setHolderType] =
    useState<CubyAppHolderType>('organization')
  const [keyword, setKeyword] = useState('')
  const [candidates, setCandidates] = useState<HolderCandidate[] | null>(null)
  const [isSearching, setIsSearching] = useState(false)
  const [busyKey, setBusyKey] = useState<string | null>(null)

  const app = props.app
  const {
    data: holders,
    isLoading,
    refetch,
  } = useQuery({
    queryKey: ['cuby-app-grants', app?.id],
    enabled: app !== null,
    queryFn: async () => {
      const result = await listCubyAppGrants(app?.id ?? 0, {
        p: 1,
        page_size: HOLDER_PAGE_SIZE,
      })
      return result.data ?? { items: [], total: 0 }
    },
  })

  if (!app) return null

  const onSearch = async () => {
    setIsSearching(true)
    try {
      if (holderType === 'organization') {
        const result = await listPlatformOrganizations({
          p: 1,
          page_size: SEARCH_RESULT_LIMIT,
          keyword: keyword.trim(),
          status: 'active,disabled',
        })
        setCandidates(
          (result.data?.items ?? []).map((organization) => ({
            type: 'organization',
            id: organization.id,
            name: organization.name,
            detail: organization.slug,
          }))
        )
        return
      }
      const result = await searchUsers({
        keyword: keyword.trim(),
        page_size: SEARCH_RESULT_LIMIT,
      })
      setCandidates(
        (result.data?.items ?? []).map((user) => ({
          type: 'user',
          id: user.id,
          name: user.username,
          detail: user.email || user.display_name || '',
        }))
      )
    } catch {
      // Reported by the interceptor.
    } finally {
      setIsSearching(false)
    }
  }

  const afterChange = async () => {
    await refetch()
    props.onChanged()
  }

  const onGive = async (candidate: HolderCandidate) => {
    setBusyKey(`give-${candidate.type}-${candidate.id}`)
    try {
      const result = await createCubyAppGrant(app.id, {
        subject_type: candidate.type,
        subject_id: candidate.id,
      })
      if (!result.success) {
        toast.error(result.message || t('Failed to give the app'))
        return
      }
      toast.success(
        t('{{name}} now has {{app}}', { name: candidate.name, app: app.name })
      )
      await afterChange()
    } catch {
      // Reported by the interceptor: most often the holder already has the app.
    } finally {
      setBusyKey(null)
    }
  }

  const onTakeAway = async (grant: CubyAppGrant) => {
    setBusyKey(`take-${grant.id}`)
    try {
      const result = await deleteCubyAppGrant(app.id, grant.id)
      if (!result.success) {
        toast.error(result.message || t('Failed to take the app away'))
        return
      }
      toast.success(t('Access taken away'))
      await afterChange()
    } catch {
      // Reported by the interceptor.
    } finally {
      setBusyKey(null)
    }
  }

  const holderName = (grant: CubyAppGrant): string => {
    if (grant.subject_type === 'organization') {
      return (
        grant.organization_name ||
        t('Organization #{{id}} (gone)', { id: grant.subject_id })
      )
    }
    return grant.username || t('User #{{id}} (gone)', { id: grant.subject_id })
  }

  const items = holders?.items ?? []
  const total = holders?.total ?? 0

  return (
    <Dialog open onOpenChange={(open) => !open && props.onOpenChange(false)}>
      <DialogContent className='sm:max-w-xl'>
        <DialogHeader>
          <DialogTitle>{t('Who has {{app}}', { app: app.name })}</DialogTitle>
          <DialogDescription>
            {t(
              'An organization passes the app to every active member. Cuby picks up a change the next time it checks: at sign-in, at start, or on its Apps page.'
            )}
          </DialogDescription>
        </DialogHeader>

        <div className='flex flex-col gap-2'>
          <Label htmlFor='cuby-app-holder-keyword'>{t('Give it to')}</Label>
          <div className='flex gap-2'>
            <NativeSelect
              className='shrink-0'
              aria-label={t('Holder type')}
              value={holderType}
              onChange={(event) => {
                setHolderType(event.target.value as CubyAppHolderType)
                setCandidates(null)
              }}
            >
              <NativeSelectOption value='organization'>
                {t('Organization')}
              </NativeSelectOption>
              <NativeSelectOption value='user'>{t('User')}</NativeSelectOption>
            </NativeSelect>
            <Input
              id='cuby-app-holder-keyword'
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') void onSearch()
              }}
              placeholder={t('Name, email or ID')}
            />
            <Button
              type='button'
              variant='outline'
              disabled={isSearching}
              onClick={() => void onSearch()}
            >
              {t('Search')}
            </Button>
          </div>
          {candidates !== null && candidates.length === 0 && (
            <p className='text-muted-foreground text-sm'>
              {t('Nothing found.')}
            </p>
          )}
          {candidates !== null && candidates.length > 0 && (
            <ul className='space-y-1.5' aria-label={t('Search results')}>
              {candidates.map((candidate) => (
                <li
                  key={`${candidate.type}-${candidate.id}`}
                  className='flex items-center justify-between gap-3 rounded-lg border px-3 py-2'
                >
                  <div className='min-w-0'>
                    <p className='truncate text-sm font-medium'>
                      {candidate.name}
                    </p>
                    <p className='text-muted-foreground truncate text-xs'>
                      {candidate.detail}
                    </p>
                  </div>
                  <Button
                    type='button'
                    size='sm'
                    disabled={busyKey !== null}
                    onClick={() => void onGive(candidate)}
                  >
                    {t('Give')}
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </div>

        <div className='flex flex-col gap-2'>
          <p className='text-sm font-medium'>{t('Has it now')}</p>
          {isLoading && <Skeleton className='h-20 w-full' />}
          {!isLoading && items.length === 0 && (
            <p className='text-muted-foreground text-sm'>
              {t('Nobody yet. No account can use this app.')}
            </p>
          )}
          {!isLoading && items.length > 0 && (
            <ul
              className='max-h-72 space-y-1.5 overflow-y-auto'
              aria-label={t('Has it now')}
            >
              {items.map((grant) => (
                <li
                  key={grant.id}
                  className='flex items-center justify-between gap-3 rounded-lg border px-3 py-2'
                >
                  <div className='min-w-0'>
                    <p className='truncate text-sm font-medium'>
                      {holderName(grant)}
                    </p>
                    <p className='text-muted-foreground truncate text-xs'>
                      {grant.subject_type === 'organization'
                        ? t('Organization')
                        : grant.email || grant.display_name || t('User')}
                    </p>
                  </div>
                  <div className='flex shrink-0 items-center gap-2'>
                    {grant.organization_status === 'disabled' && (
                      <Badge variant='outline'>
                        {t('Organization disabled')}
                      </Badge>
                    )}
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      disabled={busyKey !== null}
                      onClick={() => void onTakeAway(grant)}
                    >
                      {t('Take away')}
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          )}
          {total > items.length && (
            <p className='text-muted-foreground text-xs'>
              {t('Showing the first {{shown}} of {{total}}.', {
                shown: items.length,
                total,
              })}
            </p>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
