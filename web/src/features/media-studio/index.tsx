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
import { Sparkles } from 'lucide-react'

import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { useAuthStore } from '@/stores/auth-store'

import { VideoStudio } from './video-studio'
import { WorkflowStudio } from './workflow-studio'

type StudioTab = 'images' | 'video'

const STUDIO_TABS = [
  { id: 'images', label: 'Images' },
  { id: 'video', label: 'Video' },
] as const

export function MediaStudio() {
  const { t } = useTranslation()
  const owner = useAuthStore((state) => state.auth.user?.id)
  const [tab, setTab] = useState<StudioTab>('images')
  if (!owner) return <p role='status'>{t('Sign in to use Media Studio.')}</p>
  return (
    <div className='flex min-h-0 flex-1 flex-col overflow-y-auto'>
      <div className='mx-auto flex w-full max-w-[1500px] flex-col gap-6 p-4 sm:p-6'>
        <header className='flex items-center gap-3'>
          <Sparkles className='text-primary size-6' aria-hidden='true' />
          <div>
            <h1 className='text-xl font-semibold'>{t('Media Studio')}</h1>
            <p className='text-muted-foreground text-xs'>
              {t('Create, refine and keep every version.')}
            </p>
          </div>
        </header>
        <nav className='flex flex-wrap gap-1' aria-label={t('Media types')}>
          {STUDIO_TABS.map((item) => (
            <Button
              key={item.id}
              size='sm'
              variant={tab === item.id ? 'secondary' : 'ghost'}
              aria-pressed={tab === item.id}
              onClick={() => setTab(item.id)}
            >
              {t(item.label)}
            </Button>
          ))}
        </nav>
        {tab === 'images' ? (
          <WorkflowStudio key={owner} owner={owner} />
        ) : (
          <VideoStudio key={owner} owner={owner} />
        )}
      </div>
    </div>
  )
}
