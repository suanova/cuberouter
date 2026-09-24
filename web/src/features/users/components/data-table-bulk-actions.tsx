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
import type { Table } from '@tanstack/react-table'
import { Download } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { isAdmin } from '@/lib/role-guards'
import { useAuthStore } from '@/stores/auth-store'

import { exportUsers } from '../api'
import { buildExportPayload } from '../lib/export-utils'
import type { User } from '../types'

interface DataTableBulkActionsProps {
  table: Table<User>
}

export function DataTableBulkActions({ table }: DataTableBulkActionsProps) {
  const { t } = useTranslation()
  const [isExporting, setIsExporting] = useState(false)
  const viewerRole = useAuthStore((state) => state.auth.user?.role)

  // ops 只读访问（0fb2d5d）：隐藏批量操作（导出所选）
  if (!isAdmin(viewerRole)) {
    return null
  }

  const handleExportSelected = async () => {
    const selectedIds = table
      .getSelectedRowModel()
      .rows.map((row) => row.original.id)
    if (selectedIds.length === 0) {
      return
    }
    setIsExporting(true)
    try {
      const payload = buildExportPayload(selectedIds, {
        keyword: '',
        group: '',
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
    <BulkActionsToolbar table={table} entityName='user'>
      <Button
        size='sm'
        variant='outline'
        onClick={handleExportSelected}
        disabled={isExporting}
      >
        <Download aria-hidden='true' className='h-4 w-4' />
        {t('Export Selected')}
      </Button>
    </BulkActionsToolbar>
  )
}
