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
import { Cancel01Icon, SparklesIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { getAccountAssistantConfig } from '../api'
import { AssistantChatPanel } from './assistant-chat-panel'

export function AccountAssistant() {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const launcherRef = useRef<HTMLButtonElement | null>(null)
  const configQuery = useQuery({
    queryKey: ['account-assistant', 'config'],
    queryFn: getAccountAssistantConfig,
    staleTime: 5 * 60 * 1000,
    retry: false,
  })

  if (!configQuery.data?.enabled) {
    return null
  }

  const closeAndReturnFocus = () => {
    setOpen(false)
    // Move focus back to the launcher after closing so keyboard users do not
    // lose their place (the panel may hold the focused textarea).
    requestAnimationFrame(() => launcherRef.current?.focus())
  }

  return (
    <>
      {/* Kept mounted across close/reopen so the transcript survives and an
          in-flight stream is not aborted when the panel is merely hidden. */}
      <AssistantChatPanel
        supportUrl={configQuery.data.support_url}
        open={open}
        onClose={closeAndReturnFocus}
      />
      <Button
        ref={launcherRef}
        type='button'
        size='icon-lg'
        aria-label={
          open ? t('Close account assistant') : t('Open account assistant')
        }
        aria-expanded={open}
        onClick={() => setOpen((previous) => !previous)}
        className='fixed right-4 bottom-4 z-50 size-12 rounded-full shadow-lg sm:right-6'
      >
        <HugeiconsIcon
          icon={open ? Cancel01Icon : SparklesIcon}
          className='size-5'
        />
      </Button>
    </>
  )
}
