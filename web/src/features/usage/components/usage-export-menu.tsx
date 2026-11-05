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
import { ChevronDown, Download, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

import type { UsageExportGranularity } from '../types'

interface UsageExportMenuProps {
  exporting: UsageExportGranularity | null
  disabled?: boolean
  onExport: (granularity: UsageExportGranularity) => void
}

export function UsageExportMenu(props: UsageExportMenuProps) {
  const { t } = useTranslation()
  const busy = props.exporting !== null

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger
        render={
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={busy || props.disabled}
          />
        }
      >
        {busy ? (
          <Loader2 className='size-4 animate-spin' />
        ) : (
          <Download className='size-4' />
        )}
        {busy ? t('Exporting...') : t('Export')}
        <ChevronDown className='size-3.5' />
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem onClick={() => props.onExport('day')}>
          {t('Daily')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => props.onExport('hour')}>
          {t('Hourly')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
