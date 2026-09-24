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
import { useCallback, useEffect, useState } from 'react'

import { api } from '@/lib/api'

interface SubscriptionBrief {
  id: number
  plan_id: number
  amount_total: number
  amount_used: number
  end_time: number
  status: string
}

interface SubscriptionSummaryItem {
  subscription?: SubscriptionBrief
}

interface SelfSubscriptionsResponse {
  success: boolean
  data?: {
    billing_preference?: string
    subscriptions?: SubscriptionSummaryItem[]
    all_subscriptions?: SubscriptionSummaryItem[]
  }
}

interface AdminSubscriptionsResponse {
  success: boolean
  data?: SubscriptionSummaryItem[]
}

/**
 * Compute native quota sums over ACTIVE subscriptions for the dashboard.
 *
 * - 当前余额 = sum of max(0, amount_total - amount_used) over active
 *   subscriptions with a finite quota. If any active subscription is
 *   unlimited (amount_total <= 0), `hasUnlimited` is true so the caller
 *   can render an "unlimited" label instead of a number.
 * - 历史消耗 = sum of amount_used over active subscriptions.
 *
 * @param userId - when provided (admin viewing another user), fetches via
 *   the admin API; otherwise fetches the current user's own subscriptions.
 */
export function useSubscriptionNativeQuota(userId?: number) {
  const [remainQuota, setRemainQuota] = useState(0)
  const [usedQuota, setUsedQuota] = useState(0)
  const [hasUnlimited, setHasUnlimited] = useState(false)
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      let list: SubscriptionSummaryItem[] = []
      if (userId) {
        // Admin viewing a specific user: returns all subscriptions,
        // filter active client-side.
        const res = await api.get<AdminSubscriptionsResponse>(
          `/api/subscription/admin/users/${userId}/subscriptions`
        )
        if (res.data.success) {
          list = res.data.data || []
        }
      } else {
        // Current user: backend already returns only active subs,
        // but filter defensively anyway.
        const res = await api.get<SelfSubscriptionsResponse>(
          '/api/subscription/self'
        )
        if (res.data.success) {
          list = res.data.data?.subscriptions || []
        }
      }

      const now = Date.now() / 1000
      let remain = 0
      let used = 0
      let unlimited = false
      for (const item of list) {
        const sub = item?.subscription
        if (!sub) continue
        const isActive = sub.status === 'active' && (sub.end_time || 0) > now
        if (!isActive) continue
        const total = Number(sub.amount_total || 0)
        const subUsed = Number(sub.amount_used || 0)
        used += subUsed
        if (total > 0) {
          remain += Math.max(0, total - subUsed)
        } else {
          unlimited = true
        }
      }
      setRemainQuota(remain)
      setUsedQuota(used)
      setHasUnlimited(unlimited)
    } catch {
      setRemainQuota(0)
      setUsedQuota(0)
      setHasUnlimited(false)
    } finally {
      setLoading(false)
    }
  }, [userId])

  useEffect(() => {
    void load()
  }, [load])

  return { remainQuota, usedQuota, hasUnlimited, loading, reload: load }
}
