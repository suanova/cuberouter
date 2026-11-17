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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { createCubyApp } from '../api'
import { isValidCubyAppId } from '../lib/cuby-app-id'

type CubyAppCreateDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** An app was registered; the list behind the dialog is stale. */
  onCreated: () => void
}

/**
 * Registers an app so it can be given to accounts.
 *
 * The id is the one thing that has to be right: Cuby matches on it, and it
 * cannot be changed afterwards. A registered app is given to nobody yet.
 *
 * The form is mounted per opening, so every opening starts with empty fields.
 */
export function CubyAppCreateDialog(props: CubyAppCreateDialogProps) {
  if (!props.open) return null
  return (
    <CubyAppCreateForm
      onOpenChange={props.onOpenChange}
      onCreated={props.onCreated}
    />
  )
}

function CubyAppCreateForm(props: Omit<CubyAppCreateDialogProps, 'open'>) {
  const { t } = useTranslation()
  const [appId, setAppId] = useState('')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const isIdValid = isValidCubyAppId(appId)
  const showIdError = appId.trim().length > 0 && !isIdValid

  const handleConfirm = async () => {
    if (!isIdValid) return
    setIsSubmitting(true)
    try {
      const result = await createCubyApp({
        app_id: appId.trim(),
        name: name.trim(),
        description: description.trim(),
      })
      if (!result.success) {
        toast.error(result.message || t('Failed to add the app'))
        return
      }
      toast.success(t('App added'))
      props.onOpenChange(false)
      props.onCreated()
    } catch {
      // Reported by the interceptor. The dialog stays open with what was typed.
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <ConfirmDialog
      open
      onOpenChange={(value) => !value && props.onOpenChange(false)}
      title={t('Add app')}
      desc={t(
        'Register a Cuby app so it can be given to users and organizations. Adding it gives it to nobody yet.'
      )}
      confirmText={isSubmitting ? t('Saving...') : t('Add app')}
      disabled={!isIdValid}
      isLoading={isSubmitting}
      handleConfirm={() => void handleConfirm()}
    >
      <div className='flex flex-col gap-3'>
        <div className='flex flex-col gap-2'>
          <Label htmlFor='cuby-app-id'>{t('App ID')}</Label>
          <Input
            id='cuby-app-id'
            value={appId}
            aria-invalid={showIdError}
            aria-describedby='cuby-app-id-hint'
            onChange={(event) => setAppId(event.target.value)}
            placeholder='ai-company'
          />
          <p
            id='cuby-app-id-hint'
            className={
              showIdError
                ? 'text-destructive text-xs'
                : 'text-muted-foreground text-xs'
            }
          >
            {t(
              "The id in the app's cuby-app.json: 2 to 40 lower-case letters, digits or hyphens. It cannot be changed later."
            )}
          </p>
        </div>
        <div className='flex flex-col gap-2'>
          <Label htmlFor='cuby-app-name'>{t('Name')}</Label>
          <Input
            id='cuby-app-name'
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder={t('Shown on this page. Defaults to the app ID.')}
          />
        </div>
        <div className='flex flex-col gap-2'>
          <Label htmlFor='cuby-app-description'>{t('Description')}</Label>
          <Input
            id='cuby-app-description'
            value={description}
            onChange={(event) => setDescription(event.target.value)}
          />
        </div>
      </div>
    </ConfirmDialog>
  )
}
