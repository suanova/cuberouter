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

import { ConfirmDialog } from '@/components/confirm-dialog'
import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'

import {
  deleteCubyApp,
  listCubyApps,
  setCubyAppManagement,
  updateCubyApp,
  type CubyApp,
} from './api'
import { CubyAppAccessDialog } from './components/cuby-app-access-dialog'
import { CubyAppCreateDialog } from './components/cuby-app-create-dialog'

/**
 * Which Cuby apps exist, and who may use them.
 *
 * Cuby (the desktop client) asks this instance which of its apps the signed-in
 * account has. Three things decide the answer, and all three are on this page:
 * the switch at the top (off: Cuby is told nothing and limits nothing), each
 * app's own switch (off: nobody has it), and who each app was given to.
 *
 * Turning the top switch on is the one step that can take something away from
 * everybody at once: from then on an account has only what it was given here.
 * That is why it asks first and says how many apps and grants exist.
 */
export function CubyApps() {
  const { t } = useTranslation()
  const [createOpen, setCreateOpen] = useState(false)
  const [accessAppId, setAccessAppId] = useState<number | null>(null)
  const [pendingDelete, setPendingDelete] = useState<CubyApp | null>(null)
  // Which way the switch is about to go, kept apart from whether the question
  // is on screen: the dialog must keep its wording while it closes.
  const [managementTarget, setManagementTarget] = useState(true)
  const [managementOpen, setManagementOpen] = useState(false)
  const [isSaving, setIsSaving] = useState(false)

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['cuby-apps'],
    queryFn: async () => {
      const result = await listCubyApps()
      return result.data ?? { enabled: false, apps: [] }
    },
  })
  const apps = data?.apps ?? []
  const enabled = data?.enabled ?? false
  const grantCount = apps.reduce(
    (sum, app) => sum + app.user_grant_count + app.organization_grant_count,
    0
  )
  const accessApp = apps.find((app) => app.id === accessAppId) ?? null

  const run = async (
    action: () => Promise<{ success: boolean; message?: string }>,
    done: string
  ) => {
    setIsSaving(true)
    try {
      const result = await action()
      if (!result.success) {
        toast.error(result.message || t('Failed to save'))
        return false
      }
      toast.success(done)
      await refetch()
      return true
    } catch {
      // Reported by the interceptor.
      return false
    } finally {
      setIsSaving(false)
    }
  }

  const onConfirmManagement = async () => {
    const ok = await run(
      () => setCubyAppManagement(managementTarget),
      managementTarget
        ? t('Cuby now follows this list')
        : t('Cuby no longer limits apps')
    )
    if (ok) setManagementOpen(false)
  }

  const onConfirmDelete = async () => {
    if (!pendingDelete) return
    const ok = await run(
      () => deleteCubyApp(pendingDelete.id),
      t('App deleted')
    )
    if (ok) setPendingDelete(null)
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Cuby Apps')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button type='button' onClick={() => setCreateOpen(true)}>
            {t('Add app')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex flex-col gap-4'>
            <Alert>
              <AlertTitle className='flex items-center justify-between gap-3'>
                <span>{t('Limit Cuby apps by account')}</span>
                <Switch
                  aria-label={t('Limit Cuby apps by account')}
                  checked={enabled}
                  disabled={isLoading || isSaving}
                  onCheckedChange={(checked) => {
                    setManagementTarget(checked)
                    setManagementOpen(true)
                  }}
                />
              </AlertTitle>
              <AlertDescription>
                {enabled
                  ? t(
                      'On. In Cuby, an account can use only the apps given to it, or to an organization it belongs to, on this page.'
                    )
                  : t(
                      'Off. Cuby is told this instance does not manage apps, so every app installed on a PC can be used. Add the apps and give them out first, then switch this on.'
                    )}
              </AlertDescription>
            </Alert>

            {isLoading && <Skeleton className='h-32 w-full' />}
            {!isLoading && apps.length === 0 && (
              <p className='text-muted-foreground text-sm'>
                {t(
                  'No apps yet. Add one to start giving it to users and organizations.'
                )}
              </p>
            )}
            {!isLoading && apps.length > 0 && (
              <ul className='space-y-2' aria-label={t('Cuby Apps')}>
                {apps.map((app) => (
                  <li
                    key={app.id}
                    className='flex flex-col gap-3 rounded-xl border px-4 py-3 sm:flex-row sm:items-center sm:justify-between'
                  >
                    <div className='min-w-0'>
                      <p className='flex items-center gap-2 font-medium'>
                        <span className='truncate'>{app.name}</span>
                        {app.status === 'disabled' && (
                          <Badge variant='outline'>
                            {t('Off for everyone')}
                          </Badge>
                        )}
                      </p>
                      <p className='text-muted-foreground truncate text-xs'>
                        <code>{app.app_id}</code>
                        {app.description ? ` · ${app.description}` : ''}
                      </p>
                      <p className='text-muted-foreground text-xs'>
                        {t(
                          'Organizations: {{organizations}} · Users: {{users}}',
                          {
                            organizations: app.organization_grant_count,
                            users: app.user_grant_count,
                          }
                        )}
                      </p>
                    </div>
                    <div className='flex shrink-0 items-center gap-2'>
                      <Switch
                        aria-label={t('{{app}} available', { app: app.name })}
                        checked={app.status === 'enabled'}
                        disabled={isSaving}
                        onCheckedChange={(checked) =>
                          void run(
                            () =>
                              updateCubyApp(app.id, {
                                status: checked ? 'enabled' : 'disabled',
                              }),
                            checked
                              ? t('App switched on')
                              : t('App switched off for everyone')
                          )
                        }
                      />
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => setAccessAppId(app.id)}
                      >
                        {t('Who has it')}
                      </Button>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => setPendingDelete(app)}
                      >
                        {t('Delete')}
                      </Button>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <CubyAppCreateDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={() => void refetch()}
      />
      <CubyAppAccessDialog
        app={accessApp}
        onOpenChange={(open) => !open && setAccessAppId(null)}
        onChanged={() => void refetch()}
      />

      <ConfirmDialog
        open={managementOpen}
        onOpenChange={(open) => !open && setManagementOpen(false)}
        title={
          managementTarget
            ? t('Limit Cuby apps by account?')
            : t('Stop limiting Cuby apps?')
        }
        desc={
          managementTarget
            ? t(
                'From the next check, every Cuby account signed in to this instance can use only the apps given to it here. Right now {{apps}} apps are registered and {{grants}} grants exist. An account with no grant loses its apps in Cuby until it is given one.',
                { apps: apps.length, grants: grantCount }
              )
            : t(
                'Cuby will be told this instance does not manage apps, and every app installed on a PC can be used again. The apps and grants on this page are kept.'
              )
        }
        confirmText={managementTarget ? t('Switch on') : t('Switch off')}
        destructive={managementTarget}
        isLoading={isSaving}
        handleConfirm={() => void onConfirmManagement()}
      />

      <ConfirmDialog
        open={pendingDelete !== null}
        onOpenChange={(open) => !open && setPendingDelete(null)}
        title={t('Delete app')}
        desc={t(
          'Deleting {{app}} also removes every grant of it. To take it from everyone but keep who had it, switch it off instead.',
          { app: pendingDelete?.name ?? '' }
        )}
        confirmText={t('Delete')}
        destructive
        isLoading={isSaving}
        handleConfirm={() => void onConfirmDelete()}
      />
    </>
  )
}
