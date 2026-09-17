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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { useAuthStore } from '@/stores/auth-store'

import {
  createAccessToken,
  getAccessTokenStatus,
  revokeAccessToken,
} from '../api'

export function useAccessToken() {
  const { t } = useTranslation()
  const client = useQueryClient()
  const userId = useAuthStore((state) => state.auth.user?.id)
  const statusKey = ['security', 'access-token', 'status', userId] as const
  const [token, setToken] = useState('')
  const status = useQuery({
    queryKey: statusKey,
    queryFn: getAccessTokenStatus,
    retry: false,
  })
  const refresh = () => client.invalidateQueries({ queryKey: statusKey })
  const generate = useMutation({
    // Keep plaintext out of the query/mutation cache and persistent storage.
    mutationFn: async () => {
      setToken(await createAccessToken())
    },
    onSuccess: refresh,
    onError: () => {
      toast.error(t('Failed to generate token'))
      void refresh()
    },
  })
  const revoke = useMutation({
    mutationFn: revokeAccessToken,
    onSuccess: () => {
      setToken('')
      toast.success(t('Access token revoked'))
      return refresh()
    },
    onError: () => {
      toast.error(t('Failed to revoke token'))
      void refresh()
    },
  })
  return { status, token, clearToken: () => setToken(''), generate, revoke }
}
