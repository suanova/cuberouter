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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { LongText } from '@/components/long-text'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useAuthStore } from '@/stores/auth-store'

import { USER_ROLE } from '../constants'
import type { User } from '../types'
import { UserDashboardDialog } from './user-dashboard-dialog'

// UsernameCell renders the username plus the remark/display-name secondary
// lines. The username is clickable (opening the data dashboard dialog) only
// when the current user can manage the target user: ROOT, or a strictly
// higher role.
function UsernameCell({ user }: { user: User }) {
  const { t } = useTranslation()
  const currentUserRole = useAuthStore((s) => s.auth.user?.role) ?? 0
  const [dashboardOpen, setDashboardOpen] = useState(false)
  const canOpen =
    currentUserRole === USER_ROLE.ROOT || currentUserRole > user.role
  const displayName = user.display_name
  const remark = user.remark

  return (
    <div className='flex min-w-[160px] flex-col gap-1'>
      <div className='flex items-center gap-2'>
        <Button
          variant='link'
          className='h-auto max-w-[140px] p-0'
          disabled={!canOpen}
          title={t('Data Dashboard')}
          onClick={() => setDashboardOpen(true)}
        >
          <LongText className='font-medium'>{user.username}</LongText>
        </Button>
        {remark && (
          <Tooltip>
            <TooltipTrigger
              render={<StatusBadge variant='success' copyable={false} />}
            >
              <LongText className='max-w-[80px]'>{remark}</LongText>
            </TooltipTrigger>
            <TooltipContent>
              <p className='text-xs'>{remark}</p>
            </TooltipContent>
          </Tooltip>
        )}
      </div>
      {displayName && displayName !== user.username && (
        <LongText className='text-muted-foreground max-w-[180px] text-xs'>
          {displayName}
        </LongText>
      )}
      <UserDashboardDialog
        open={dashboardOpen}
        onOpenChange={setDashboardOpen}
        userId={user.id}
        username={user.username}
      />
    </div>
  )
}

export default UsernameCell
