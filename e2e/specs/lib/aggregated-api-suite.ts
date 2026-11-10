/*
Shared aggregated (onboarding) API suite, run by specs/api-v2.spec.ts. The
aggregated surface (controller/aggregated_api.go) is registered only under
/api/v2 so every documented endpoint is covered by the same IP whitelist.
*/
import { expect, test, type APIRequestContext } from '@playwright/test'

const ADMIN = { username: 'root', password: 'correct-horse-battery' }

export function defineAggregatedApiSuite(prefix: string) {
  test.describe.configure({ mode: 'serial', retries: 0 })

  test.describe(`aggregated API (${prefix})`, () => {
    let adminToken = ''
    // Per-run unique identifiers so v1/v2 specs and re-runs never collide.
    const runSuffix = Date.now()
    const username = `agg.user.${runSuffix}@example.com`
    const password = 'AggUser123'
    const planTitle = `Agg Plan ${runSuffix}`
    // The plan's total_amount; the status API reports it as plan_raw_quota.
    const planTotalAmount = 400000000

    let planId = 0
    let userId = 0

    function adminHeaders(): Record<string, string> {
      return { Authorization: `Bearer ${adminToken}` }
    }

    async function login(
      request: APIRequestContext,
      name: string,
      pass: string
    ): Promise<string> {
      const res = await request.post('/api/user/login', {
        data: { username: name, password: pass },
      })
      const body = await res.json()
      expect(body.success).toBe(true)
      return body.data.access_token as string
    }

    test.beforeAll(async ({ request }) => {
      adminToken = await login(request, ADMIN.username, ADMIN.password)
    })

    test('create plan (API#1) returns plan_id', async ({ request }) => {
      const res = await request.post(`${prefix}/plans`, {
        headers: adminHeaders(),
        data: {
          plan: {
            title: planTitle,
            price_amount: 1288,
            duration_unit: 'month',
            duration_value: 3,
            total_amount: planTotalAmount,
            quota_reset_period: 'No Reset',
            sort_order: 30,
            enabled: true,
          },
        },
      })
      const body = await res.json()
      expect(body.status).toBe('success')
      planId = body.plan_id
      expect(planId).toBeGreaterThan(0)
    })

    test('create user (API#3) returns user_id', async ({ request }) => {
      const res = await request.post(`${prefix}/users`, {
        headers: adminHeaders(),
        data: {
          email: username,
          username,
          password,
          // deck slide 6 note 2: inviter_id = the operator account (root).
          inviter_id: 1,
        },
      })
      const body = await res.json()
      expect(body.status).toBe('success')
      userId = Number(body.user_id)
      expect(userId).toBeGreaterThan(0)
    })

    test('suspend user (API#7) disables the account', async ({ request }) => {
      const res = await request.post(`${prefix}/users/${userId}/suspend`, {
        headers: adminHeaders(),
      })
      const body = await res.json()
      expect(body.status).toBe('success')
      expect(body.status_code).toBe(2000)

      const status = await request.get(`${prefix}/users/${userId}/status`, {
        headers: adminHeaders(),
      })
      expect((await status.json()).user_status).toBe('disabled')

      const loginRes = await request.post('/api/user/login', {
        data: { username, password },
      })
      expect((await loginRes.json()).success).toBe(false)
    })

    test('reactivate user (API#5) restores the account', async ({
      request,
    }) => {
      const res = await request.post(
        `${prefix}/users/${userId}/reactivate`,
        { headers: adminHeaders() }
      )
      const body = await res.json()
      expect(body.status).toBe('success')
      expect(body.status_code).toBe(2000)

      const status = await request.get(`${prefix}/users/${userId}/status`, {
        headers: adminHeaders(),
      })
      expect((await status.json()).user_status).toBe('enabled')

      await login(request, username, password)
    })

    test('adjust quota (API#2) adds quota and reports totals', async ({
      request,
    }) => {
      const res = await request.post(
        `${prefix}/users/${userId}/adjust-quota`,
        {
          headers: adminHeaders(),
          data: { added_quota: 1500000 },
        }
      )
      const body = await res.json()
      expect(body.status).toBe('success')
      expect(body.status_code).toBe(2000)
      expect(body.total_quota - body.current_quota).toBe(1500000)
    })

    test('bind subscription (API#6) attaches the plan', async ({ request }) => {
      const res = await request.post(
        `${prefix}/users/${userId}/bind-subscription`,
        {
          headers: adminHeaders(),
          data: { plan_id: planId },
        }
      )
      const body = await res.json()
      expect(body.status, JSON.stringify(body)).toBe('success')
      expect(body.status_code).toBe(2000)

      const status = await request.get(`${prefix}/users/${userId}/status`, {
        headers: adminHeaders(),
      })
      const statusBody = await status.json()
      expect(statusBody.status).toBe('success')
      const plan = (statusBody.plans ?? []).find(
        (s: { plan_id: number }) => s.plan_id === planId
      )
      expect(plan).toBeTruthy()
      expect(plan.status).toBe('active')
    })

    test('get user status reports plan status, validity and quota', async ({
      request,
    }) => {
      const res = await request.get(`${prefix}/users/${userId}/status`, {
        headers: adminHeaders(),
      })
      const body = await res.json()
      expect(body.status).toBe('success')

      const plan = (body.plans ?? []).find(
        (s: { plan_id: number }) => s.plan_id === planId
      )
      expect(plan).toBeTruthy()

      // The bound subscription is active, and nothing has been consumed since
      // binding, so the remaining quota equals the plan's full raw quota.
      expect(plan.status).toBe('active')
      expect(plan.plan_raw_quota).toBe(planTotalAmount)
      expect(plan.plan_remaining_quota).toBe(planTotalAmount)

      // The validity window must match the plan's 3-month duration:
      // validity_end_at == validity_start_at + 3 months.
      const start = new Date(plan.validity_start_at.replace(' ', 'T'))
      const end = new Date(plan.validity_end_at.replace(' ', 'T'))
      expect(Number.isNaN(start.getTime())).toBe(false)
      expect(Number.isNaN(end.getTime())).toBe(false)
      const expectedEnd = new Date(start)
      expectedEnd.setMonth(expectedEnd.getMonth() + 3)
      expect(end.getTime()).toBe(expectedEnd.getTime())
    })

    test('guards: root and missing users cannot be suspended', async ({
      request,
    }) => {
      // root cannot be disabled (id 1 is the admin created by global setup).
      const rootSuspend = await request.post(`${prefix}/users/1/suspend`, {
        headers: adminHeaders(),
      })
      expect((await rootSuspend.json()).status).toBe('fail')

      const missingSuspend = await request.post(
        `${prefix}/users/99999999/suspend`,
        { headers: adminHeaders() }
      )
      expect((await missingSuspend.json()).status).toBe('fail')

      const badId = await request.post(`${prefix}/users/0/suspend`, {
        headers: adminHeaders(),
      })
      expect((await badId.json()).status).toBe('fail')
    })

    test('delete user (API#9) removes the account', async ({ request }) => {
      const res = await request.post(`${prefix}/users/${userId}/delete`, {
        headers: adminHeaders(),
      })
      const body = await res.json()
      expect(body.status).toBe('success')
      expect(body.status_code).toBe(2000)

      // The deleted user can no longer sign in and the status API reports it.
      const loginRes = await request.post('/api/user/login', {
        data: { username, password },
      })
      expect((await loginRes.json()).success).toBe(false)

      const status = await request.get(`${prefix}/users/${userId}/status`, {
        headers: adminHeaders(),
      })
      expect((await status.json()).status).toBe('fail')
    })
  })
}
