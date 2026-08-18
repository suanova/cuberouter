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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { LanguageSwitcher } from '@/components/language-switcher'
import { ThemeSwitch } from '@/components/theme-switch'

import { useSystemConfig } from '@/hooks/use-system-config'

type AuthLayoutProps = {
  children: React.ReactNode
}

/**
 * Centered single-card auth shell (nbility-style):
 *   - top-right: theme (light/dark/system) + language switchers
 *   - center: logo lockup above one prominent card holding the page content
 */
export function AuthLayout({ children }: AuthLayoutProps) {
  const { t } = useTranslation()
  const { systemName } = useSystemConfig()

  return (
    <div className='relative grid min-h-svh place-items-center overflow-hidden px-4 py-14 sm:py-20'>
      {/* Top-right: theme + language */}
      <div className='absolute top-4 right-4 z-10 flex items-center gap-2 sm:top-6 sm:right-6'>
        <ThemeSwitch />
        <LanguageSwitcher />
      </div>

      {/* Centered shell: logo above the card */}
      <div className='flex w-full max-w-md flex-col items-center gap-6'>
        <Link
          to='/'
          aria-label={systemName || t('CubeRouter')}
          className='transition-opacity hover:opacity-80'
        >
          <img
            src='/head.png'
            alt={systemName || t('CubeRouter')}
            className='h-10 w-auto sm:h-12'
          />
        </Link>

        {/* Prominent card holding the auth form content */}
        <div className='bg-card text-card-foreground w-full rounded-2xl border p-6 shadow-sm sm:p-8'>
          {children}
        </div>
      </div>
    </div>
  )
}
