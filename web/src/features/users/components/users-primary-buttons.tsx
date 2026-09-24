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
import { getRouteApi } from '@tanstack/react-router'
import { Download, Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { isAdmin } from '@/lib/role-guards'
import { useAuthStore } from '@/stores/auth-store'

import { exportUsers } from '../api'
import { buildExportPayload } from '../lib/export-utils'
import { useUsers } from './users-provider'

const route = getRouteApi('/_authenticated/users/')

export function UsersPrimaryButtons() {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow } = useUsers()
  const search = route.useSearch()
  const [isExporting, setIsExporting] = useState(false)
  const viewerRole = useAuthStore((state) => state.auth.user?.role)

  // ops 只读访问（0fb2d5d）：隐藏 添加用户 / 导出按钮
  if (!isAdmin(viewerRole)) {
    return null
  }

  const handleCreate = () => {
    setCurrentRow(null)
    setOpen('create')
  }

  // 导出全部: send the current table filter (keyword/group from the URL
  // search); an empty filter means the backend exports all users.
  const handleExportAll = async () => {
    setIsExporting(true)
    try {
      const payload = buildExportPayload([], {
        keyword: search.filter ?? '',
        group: search.group ?? '',
      })
      const { blob, filename } = await exportUsers(payload)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = filename || 'users.csv'
      link.click()
      URL.revokeObjectURL(url)
    } catch (error) {
      // exportUsers throws the backend's error envelope message, so the admin
      // sees the real reason (e.g. unsupported format, too many rows).
      const message = error instanceof Error ? error.message : ''
      toast.error(message || t('Failed to export users'))
    } finally {
      setIsExporting(false)
    }
  }

  return (
    <div className='flex gap-2'>
      <Button
        size='sm'
        variant='outline'
        onClick={handleExportAll}
        disabled={isExporting}
      >
        <Download aria-hidden='true' className='h-4 w-4' />
        {t('Export All')}
      </Button>
      <Button size='sm' onClick={handleCreate}>
        <Plus className='h-4 w-4' />
        {t('Add User')}
      </Button>
    </div>
  )
}
