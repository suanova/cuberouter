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
import { useTranslation } from 'react-i18next'

import { formatTimestamp } from '@/lib/format'

import { SettingsSection } from '../components/settings-section'

type MaintenanceInfoSectionProps = {
  currentVersion?: string | null
  startTime?: number | null
}

/**
 * Read-only maintenance overview: the running build's version and uptime.
 */
export function MaintenanceInfoSection(props: MaintenanceInfoSectionProps) {
  const { t } = useTranslation()
  const uptime = props.startTime
    ? formatTimestamp(props.startTime)
    : t('Unknown')
  const version = props.currentVersion || t('Unknown')

  return (
    <SettingsSection title={t('System maintenance')}>
      <div className='grid gap-4 md:grid-cols-2'>
        <div className='rounded-lg border p-4'>
          <div className='text-muted-foreground text-sm'>
            {t('Current version')}
          </div>
          <div className='text-lg font-semibold'>{version}</div>
        </div>
        <div className='rounded-lg border p-4'>
          <div className='text-muted-foreground text-sm'>
            {t('Uptime since')}
          </div>
          <div className='text-lg font-semibold'>{uptime}</div>
        </div>
      </div>
    </SettingsSection>
  )
}
