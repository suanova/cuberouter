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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { refreshAccountContexts } from '@/lib/account-context'

import { acceptOrganizationInvite, getOrganizationInvite } from '../api'
import {
  alignOrganizationAccountContext,
  organizationInviteAcceptErrorMessage,
  organizationInviteCanAccept,
  organizationInviteLandingErrorMessage,
  organizationInviteLandingRows,
} from '../lib'

const route = getRouteApi('/_authenticated/organization/invite/$token')

/**
 * The page an emailed invitation link lands on.
 *
 * The message the backend mails points here, and the recipient is usually
 * somebody who has no account yet, so the route sits behind the authenticated
 * layout: its guard is what sends them to sign in and brings them back to this
 * exact URL. That is also why the invitation is re-read with the session
 * attached — `email_matched` is the server's verdict on *this* account, and only
 * it can say whether the Accept button means anything.
 */
export function OrganizationInviteLanding() {
  const { token } = route.useParams()
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [isAccepting, setIsAccepting] = useState(false)

  const inviteQuery = useQuery({
    queryKey: ['organization-invite', token],
    // A link is opened once; a bad token is a bad token on the second try too.
    retry: false,
    queryFn: () => getOrganizationInvite(token, { silent: true }),
  })

  const invite = inviteQuery.data?.success
    ? (inviteQuery.data.data ?? null)
    : null

  // Both ways a read can fail end up here: the interceptor was silenced, so a
  // refusal is reported on this card rather than in a toast that cannot name
  // the invitation it was about.
  let refusal: unknown = null
  if (inviteQuery.isError) {
    refusal = inviteQuery.error
  } else if (inviteQuery.data && !inviteQuery.data.success) {
    refusal = inviteQuery.data
  }
  const loadError = organizationInviteLandingErrorMessage(
    refusal,
    t('Failed to load the invitation')
  )

  const handleAccept = async () => {
    if (!invite) return
    setIsAccepting(true)
    try {
      const result = await acceptOrganizationInvite(token, { silent: true })
      if (!result.success) {
        toast.error(
          t(
            organizationInviteAcceptErrorMessage(
              result,
              t('Failed to accept the invitation')
            )
          )
        )
        return
      }

      // Membership is what puts the organization into the account-context list,
      // and the organization's own endpoints answer only to a request that
      // carries that context — so the list is refreshed and the context aligned
      // before navigating, or the workspace would load in the personal context
      // and be refused.
      await refreshAccountContexts()
      await alignOrganizationAccountContext(invite.organization_id, queryClient)
      toast.success(t('Invitation accepted'))
      await navigate({
        to: '/organizations/$organizationId/$section',
        params: {
          organizationId: String(invite.organization_id),
          section: 'overview',
        },
      })
    } catch (error) {
      toast.error(
        t(
          organizationInviteAcceptErrorMessage(
            error,
            t('Failed to accept the invitation')
          )
        )
      )
    } finally {
      setIsAccepting(false)
    }
  }

  const canAccept = organizationInviteCanAccept(invite)

  return (
    <div className='flex min-h-[calc(100vh-8rem)] items-start justify-center px-4 py-8 sm:items-center sm:py-12'>
      <Card className='w-full max-w-xl'>
        <CardHeader className='border-border/60 border-b'>
          <CardTitle>{t('Organization invitation')}</CardTitle>
          <CardDescription>
            {t('Review this invitation before you join the organization.')}
          </CardDescription>
        </CardHeader>

        <CardContent className='space-y-4'>
          {inviteQuery.isPending ? (
            <div className='space-y-3'>
              <Skeleton className='h-5 w-3/4' />
              <Skeleton className='h-5 w-2/3' />
              <Skeleton className='h-5 w-1/2' />
            </div>
          ) : null}

          {!inviteQuery.isPending && !invite ? (
            <Alert variant='destructive'>
              <AlertDescription>{loadError}</AlertDescription>
            </Alert>
          ) : null}

          {invite ? (
            <>
              {invite.status !== 'pending' ? (
                <Alert>
                  <AlertDescription>
                    {t(
                      'This invitation is no longer available. Ask an administrator for a new one.'
                    )}
                  </AlertDescription>
                </Alert>
              ) : null}
              {!invite.email_matched ? (
                <Alert variant='destructive'>
                  <AlertDescription>
                    {t(
                      'This invitation was sent to a different email address. Sign in with that address to accept it.'
                    )}
                  </AlertDescription>
                </Alert>
              ) : null}

              <dl className='space-y-3'>
                {organizationInviteLandingRows(invite, t).map((row) => (
                  <div
                    key={row.key}
                    className='grid grid-cols-[7rem_1fr] items-start gap-4 border-b pb-3 last:border-b-0 last:pb-0 sm:grid-cols-[8rem_1fr]'
                  >
                    <dt className='text-muted-foreground text-sm'>
                      {row.label}
                    </dt>
                    <dd className='min-w-0 text-sm font-medium break-words'>
                      {row.value}
                    </dd>
                  </div>
                ))}
              </dl>
            </>
          ) : null}
        </CardContent>

        {invite ? (
          <CardFooter className='flex-col-reverse gap-3 sm:flex-row sm:justify-end'>
            <Button
              className='w-full sm:w-auto'
              variant='outline'
              onClick={() => void navigate({ to: '/organizations' })}
            >
              {t('Cancel')}
            </Button>
            <Button
              className='w-full sm:w-auto'
              disabled={!canAccept || isAccepting}
              onClick={() => void handleAccept()}
            >
              {isAccepting ? <Loader2 className='animate-spin' /> : null}
              {t('Accept invitation')}
            </Button>
          </CardFooter>
        ) : null}
      </Card>
    </div>
  )
}
