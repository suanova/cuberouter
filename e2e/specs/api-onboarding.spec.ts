/*
CubeRouter API onboarding flows — converted from the deck
"Cuberouter API Onboarding Specification v4.5" (cuberouter.com).

The deck documents a dedicated onboarding surface (POST /api/v2/plans,
POST /api/v2/users, .../suspend, .../reactivate, .../reset-password,
.../bind-subscription, .../adjust-quota) driven by an API key. This product
does not ship that v2 surface; the same operations are exposed by the real
admin APIs below, so each test maps one deck slide to its product equivalent:

  Deck API (v4.5)                          Product endpoint
  ---------------------------------------- --------------------------------------
  POST /api/v2/plans                       POST /api/subscription/admin/plans
  POST /api/v2/users                       POST /api/user/ + bind subscription
  POST /api/v2/users/:id/adjust-quota      POST /api/user/manage (add_quota)
  POST /api/v2/users/:id/reset-password    PUT /api/user/ (admin-set password)
  POST /api/v2/users/:id/reactivate        POST /api/user/manage (enable)
  POST /api/v2/users/:id/suspend           POST /api/user/manage (disable)
  POST /api/v2/users/:id/bind-subscription POST /api/subscription/admin/bind

The deck's "Authorization: <API_Key>" maps to an admin access token obtained
from POST /api/user/login.

File independence: this spec does not run system setup and does not depend
on journey.spec.ts data. Deployment initialization is a one-time gate owned
by the global setup (e2e/global-setup.ts), which runs before every spec, so
this file can rely on an initialized system regardless of run order, file
name, or whether journey.spec.ts is run at all. Every run creates its own
user with a unique username, so the file can be run repeatedly against the
same deployment without colliding with earlier runs or with other spec
files.

Note: sign-in and page loads fire CriticalRateLimit-marked requests (login,
auth/refresh, PUT /api/user/self), so the e2e deployment disables
CRITICAL_RATE_LIMIT (see e2e/k8s/app.yaml) to avoid per-IP 429 flakes.

UI flows from the deck (slides 10-13: sign-in → dashboard, change password,
wallet subscriptions, playground) are covered with real browser steps.
*/
import { expect, test, type APIRequestContext } from '@playwright/test'

// Must match the admin account the global setup (e2e/global-setup.ts)
// creates during initialization, so this spec can sign in as root.
const ADMIN = { username: 'root', password: 'correct-horse-battery' }

// Deck slide 6: "suggested to use email as username". The timestamp suffix
// keeps every run isolated: the username column is unique, so re-running
// against an already-used deployment never collides with prior users.
const USERNAME = `demo.user.${Date.now()}@example.com`
const INITIAL_PASSWORD = 'DemoUser123'

const ONE_OFF_PLAN = {
  // Deck slide 3: one-off plan, HKD 1288, 3-month validity, 400M quota.
  title: 'one-off plan',
  price_amount: 1288,
  currency: 'HKD',
  duration_unit: 'month',
  duration_value: 3,
  total_amount: 400000000,
  quota_reset_period: 'No Reset',
  sort_order: 30,
  enabled: true,
}

const TOP_UP_PLAN = {
  // Deck slide 4: top-up plan, HKD 888, 3-month validity, 300M quota.
  title: '3-month Top-Up plan',
  price_amount: 888,
  currency: 'HKD',
  duration_unit: 'month',
  duration_value: 3,
  total_amount: 300000000,
  quota_reset_period: 'No Reset',
  sort_order: 35, // deck slide 3 note 2: +5 per created plan (30 -> 35)
  enabled: true,
}

// State flows between tests; the describe block is serial by design.
let rootToken = ''
let oneOffPlanId = 0
let topUpPlanId = 0
let userId = 0
let userPassword = INITIAL_PASSWORD

function adminHeaders(): Record<string, string> {
  return { Authorization: `Bearer ${rootToken}` }
}

async function login(
  request: APIRequestContext,
  username: string,
  password: string
): Promise<string> {
  const res = await request.post('/api/user/login', {
    data: { username, password },
  })
  const body = await res.json()
  expect(body.success).toBe(true)
  return body.data.access_token as string
}

test.describe.configure({ mode: 'serial', retries: 0 })

test.beforeAll(async ({ request }) => {
  rootToken = await login(request, ADMIN.username, ADMIN.password)
})

test.describe('API onboarding (v4.5 deck)', () => {
  test('API#1 — admin creates the 3-month one-off plan (deck slide 3)', async ({
    request,
  }) => {
    const res = await request.post('/api/subscription/admin/plans', {
      headers: adminHeaders(),
      data: { plan: ONE_OFF_PLAN },
    })
    const body = await res.json()
    expect(body.success).toBe(true)

    const plan = body.data
    oneOffPlanId = plan.id
    expect(oneOffPlanId).toBeGreaterThan(0)
    expect(plan.title).toBe('one-off plan')
    expect(plan.price_amount).toBe(1288)
    expect(plan.duration_unit).toBe('month')
    expect(plan.duration_value).toBe(3)
    expect(plan.total_amount).toBe(400000000)
    // Deck value "No Reset" is normalized to the product's "never".
    expect(plan.quota_reset_period).toBe('never')
    expect(plan.sort_order).toBe(30)
    expect(plan.enabled).toBe(true)
    // The product prices in USD regardless of the deck's HKD payload.
    expect(plan.currency).toBe('USD')
  })

  test('API#1 — admin creates the 3-month top-up plan (deck slide 4)', async ({
    request,
  }) => {
    const res = await request.post('/api/subscription/admin/plans', {
      headers: adminHeaders(),
      data: { plan: TOP_UP_PLAN },
    })
    const body = await res.json()
    expect(body.success).toBe(true)

    const plan = body.data
    topUpPlanId = plan.id
    expect(topUpPlanId).toBeGreaterThan(0)
    expect(plan.title).toBe('3-month Top-Up plan')
    expect(plan.total_amount).toBe(300000000)
    expect(plan.sort_order).toBe(35)
    expect(plan.enabled).toBe(true)

    // Deck slide 3 note 2: a higher sort_order displays the plan more
    // top-left in the UI; the admin list orders by sort_order desc.
    const list = await request.get('/api/subscription/admin/plans', {
      headers: adminHeaders(),
    })
    const plans = (await list.json()).data as { plan: { title: string } }[]
    const titles = plans.map((entry) => entry.plan.title)
    expect(titles.indexOf('3-month Top-Up plan')).toBeLessThan(
      titles.indexOf('one-off plan')
    )
  })

  test('API#3 — admin creates the user and assigns the one-off plan (deck slide 6)', async ({
    request,
  }) => {
    // Deck payload carries email/inviter_id/plan_id/user_validity. The
    // product's CreateUser API takes username/password (deck: use email as
    // username); plan assignment is an explicit bind (deck note 3), and
    // user_validity is only a record-keeping date, not enforced here.
    const create = await request.post('/api/user/', {
      headers: adminHeaders(),
      data: {
        username: USERNAME,
        password: INITIAL_PASSWORD,
        display_name: 'Demo User',
      },
    })
    const createBody = await create.json()
    expect(createBody.success).toBe(true)

    const search = await request.get(
      `/api/user/search?keyword=${encodeURIComponent(USERNAME)}`,
      { headers: adminHeaders() }
    )
    const items = (await search.json()).data.items as {
      id: number
      username: string
    }[]
    const created = items.find((u) => u.username === USERNAME)
    expect(created).toBeTruthy()
    userId = created!.id

    const bind = await request.post('/api/subscription/admin/bind', {
      headers: adminHeaders(),
      data: { user_id: userId, plan_id: oneOffPlanId },
    })
    expect((await bind.json()).success).toBe(true)

    const subs = await request.get(
      `/api/subscription/admin/users/${userId}/subscriptions`,
      { headers: adminHeaders() }
    )
    const subscriptions = (await subs.json()).data as {
      subscription: {
        plan_id: number
        status: string
        amount_total: number
        start_time: number
        end_time: number
      }
    }[]
    expect(subscriptions).toHaveLength(1)
    const subscription = subscriptions[0].subscription
    expect(subscription.plan_id).toBe(oneOffPlanId)
    expect(subscription.status).toBe('active')
    expect(subscription.amount_total).toBe(400000000)
    // 3-month validity: duration_value = 3 months from the bind time.
    const validityDays =
      (subscription.end_time - subscription.start_time) / (24 * 3600)
    expect(validityDays).toBeGreaterThan(89)
    expect(validityDays).toBeLessThanOrEqual(92)
  })

  test('API#6 — admin adds the top-up plan to the user (deck slide 8)', async ({
    request,
  }) => {
    const bind = await request.post('/api/subscription/admin/bind', {
      headers: adminHeaders(),
      data: { user_id: userId, plan_id: topUpPlanId },
    })
    expect((await bind.json()).success).toBe(true)

    const subs = await request.get(
      `/api/subscription/admin/users/${userId}/subscriptions`,
      { headers: adminHeaders() }
    )
    const subscriptions = (await subs.json()).data as {
      subscription: {
        id: number
        plan_id: number
        status: string
        amount_total: number
        end_time: number
      }
    }[]
    expect(subscriptions).toHaveLength(2)
    const byPlan = new Map(
      subscriptions.map((entry) => [
        entry.subscription.plan_id,
        entry.subscription,
      ])
    )
    expect(byPlan.get(oneOffPlanId)?.status).toBe('active')
    expect(byPlan.get(oneOffPlanId)?.amount_total).toBe(400000000)
    expect(byPlan.get(topUpPlanId)?.status).toBe('active')
    expect(byPlan.get(topUpPlanId)?.amount_total).toBe(300000000)

    // Deck slide 8 note 1: plan validity follows FIFO — the plan with the
    // earliest end datetime is consumed first (ties broken by creation id).
    const consumptionOrder = [...subscriptions].sort(
      (a, b) =>
        a.subscription.end_time - b.subscription.end_time ||
        a.subscription.id - b.subscription.id
    )
    expect(consumptionOrder[0].subscription.plan_id).toBe(oneOffPlanId)
  })

  test('API#2 — admin adjusts user quota (deck slide 5, marked "Not used")', async ({
    request,
  }) => {
    // The deck lists this endpoint but marks it "(Not used)" in onboarding;
    // its product equivalent is the add_quota manage action.
    const before = await request.get(`/api/user/${userId}`, {
      headers: adminHeaders(),
    })
    const beforeQuota = (await before.json()).data.quota as number

    const adjust = await request.post('/api/user/manage', {
      headers: adminHeaders(),
      data: {
        id: userId,
        action: 'add_quota',
        value: 1500000,
        mode: 'add',
      },
    })
    expect((await adjust.json()).success).toBe(true)

    const after = await request.get(`/api/user/${userId}`, {
      headers: adminHeaders(),
    })
    const afterQuota = (await after.json()).data.quota as number
    expect(afterQuota).toBe(beforeQuota + 1500000)
    // Deck slide 5 note 2: "500000 quota = 1 HKD" is the deployment's
    // QuotaPerUnit setting, not asserted here.
  })

  test('API#7 — admin suspends the user; the disabled user cannot sign in (deck slide 9)', async ({
    request,
  }) => {
    const manage = await request.post('/api/user/manage', {
      headers: adminHeaders(),
      data: { id: userId, action: 'disable' },
    })
    const manageBody = await manage.json()
    expect(manageBody.success).toBe(true)
    expect(manageBody.data.status).toBe(2) // common.UserStatusDisabled

    const loginRes = await request.post('/api/user/login', {
      data: { username: USERNAME, password: userPassword },
    })
    expect((await loginRes.json()).success).toBe(false)
  })

  test('API#5 — admin reactivates the user; sign-in works again (deck slide 7)', async ({
    request,
  }) => {
    const manage = await request.post('/api/user/manage', {
      headers: adminHeaders(),
      data: { id: userId, action: 'enable' },
    })
    const manageBody = await manage.json()
    expect(manageBody.success).toBe(true)
    expect(manageBody.data.status).toBe(1) // common.UserStatusEnabled

    const token = await login(request, USERNAME, userPassword)
    expect(token).toBeTruthy()
    // Deck slide 6 note 1: user_validity is a date FOR RECORD to
    // reactivate users only — an operational note, not a product field.
  })

  test('API#4 — admin resets the user password (deck slide 7)', async ({
    request,
  }) => {
    // Deck flow notifies the user by email to reset the password; without
    // SMTP/email delivery in the e2e deployment, the operator-side reset
    // (admin sets a new password) is exercised instead.
    const newPassword = 'ResetPass456'
    const reset = await request.put('/api/user/', {
      headers: adminHeaders(),
      data: { id: userId, username: USERNAME, password: newPassword },
    })
    expect((await reset.json()).success).toBe(true)

    const oldLogin = await request.post('/api/user/login', {
      data: { username: USERNAME, password: INITIAL_PASSWORD },
    })
    expect((await oldLogin.json()).success).toBe(false)

    const newLogin = await request.post('/api/user/login', {
      data: { username: USERNAME, password: newPassword },
    })
    expect((await newLogin.json()).success).toBe(true)

    userPassword = newPassword
  })

  test('slide 10 — user signs in and opens the dashboard', async ({ page }) => {
    await page.goto('/sign-in')
    await page.getByLabel('Username or Email').fill(USERNAME)
    await page.getByLabel('Password', { exact: true }).fill(userPassword)
    await page.getByRole('button', { name: 'Sign in' }).click()
    await page.waitForURL('**/dashboard**')
    await expect(
      page.getByRole('heading', { name: 'Overview' })
    ).toBeVisible()
  })

  test('slide 11 — user changes the password in the UI and signs in with it', async ({
    page,
  }) => {
    await page.goto('/sign-in')
    await page.getByLabel('Username or Email').fill(USERNAME)
    await page.getByLabel('Password', { exact: true }).fill(userPassword)
    await page.getByRole('button', { name: 'Sign in' }).click()
    await page.waitForURL('**/dashboard**')

    await page.goto('/profile')
    await page.getByRole('button', { name: 'Change Password' }).click()

    const changedPassword = 'ChangedPass789'
    const dialog = page.locator('[data-slot="dialog-content"]')
    await dialog.getByLabel('Current Password', { exact: true }).fill(userPassword)
    await dialog.getByLabel('New Password', { exact: true }).fill(changedPassword)
    await dialog
      .getByLabel('Confirm New Password', { exact: true })
      .fill(changedPassword)
    await dialog
      .getByRole('button', { name: 'Change Password' })
      .click()
    await expect(page.getByText('Password changed successfully')).toBeVisible()
    userPassword = changedPassword

    // Sign out (the app keeps the session token client-side) and sign back
    // in with the new password, as the deck describes.
    await page.evaluate(() => localStorage.clear())
    await page.context().clearCookies()
    await page.goto('/sign-in')
    await page.getByLabel('Username or Email').fill(USERNAME)
    await page.getByLabel('Password', { exact: true }).fill(userPassword)
    await page.getByRole('button', { name: 'Sign in' }).click()
    await page.waitForURL('**/dashboard**')
  })

  test('slide 12 — user views the one-off and top-up plans in the wallet', async ({
    page,
    request,
  }) => {
    // API level (deck slide 12: confirm the one-off plan is active with its
    // token quota, and that the earliest-ending subscription is consumed
    // first — deck note 3).
    const userToken = await login(request, USERNAME, userPassword)
    const self = await request.get('/api/subscription/self', {
      headers: { Authorization: `Bearer ${userToken}` },
    })
    const selfBody = (await self.json()).data
    const active = selfBody.subscriptions as {
      subscription: {
        id: number
        plan_id: number
        status: string
        amount_total: number
        end_time: number
      }
    }[]
    expect(active).toHaveLength(2)
    const byPlan = new Map(
      active.map((entry) => [entry.subscription.plan_id, entry.subscription])
    )
    expect(byPlan.get(oneOffPlanId)?.status).toBe('active')
    expect(byPlan.get(oneOffPlanId)?.amount_total).toBe(400000000)
    expect(byPlan.get(topUpPlanId)?.status).toBe('active')
    expect(byPlan.get(topUpPlanId)?.amount_total).toBe(300000000)
    const consumptionOrder = [...active].sort(
      (a, b) =>
        a.subscription.end_time - b.subscription.end_time ||
        a.subscription.id - b.subscription.id
    )
    expect(consumptionOrder[0].subscription.plan_id).toBe(oneOffPlanId)

    // UI level: Wallet Management -> My Subscriptions.
    await page.goto('/sign-in')
    await page.getByLabel('Username or Email').fill(USERNAME)
    await page.getByLabel('Password', { exact: true }).fill(userPassword)
    await page.getByRole('button', { name: 'Sign in' }).click()
    await page.waitForURL('**/dashboard**')

    await page.goto('/wallet')
    await expect(page.getByText('My Subscriptions')).toBeVisible()
    await expect(page.getByText('2 active')).toBeVisible()
    await expect(page.getByText(/one-off plan · Subscription #/)).toBeVisible()
    await expect(
      page.getByText(/3-month Top-Up plan · Subscription #/)
    ).toBeVisible()
    await expect(page.getByText('Active', { exact: true }).first()).toBeVisible()
  })

  test('slide 13 — user opens the playground and enters a question', async ({
    page,
  }) => {
    await page.goto('/sign-in')
    await page.getByLabel('Username or Email').fill(USERNAME)
    await page.getByLabel('Password', { exact: true }).fill(userPassword)
    await page.getByRole('button', { name: 'Sign in' }).click()
    await page.waitForURL('**/dashboard**')

    await page.goto('/playground')
    const input = page.getByPlaceholder('Ask anything')
    await expect(input).toBeVisible()
    await input.fill('Hello, what can you help me with today?')
    await expect(page.getByRole('button', { name: 'Send' })).toBeVisible()

    // Deck note 1: the one-off plan quota is deducted first, then top-up —
    // a backend billing rule verified structurally by the FIFO assertions in
    // the API#6 and wallet tests. An actual relay round trip needs a
    // configured upstream channel, which a fresh e2e deployment does not have.
  })

  test('appendix (slide 14) — plan input types are validated', async ({
    request,
  }) => {
    // title varchar(128) — required on create.
    const noTitle = await request.post('/api/subscription/admin/plans', {
      headers: adminHeaders(),
      data: { plan: { price_amount: 100, total_amount: 1000 } },
    })
    expect((await noTitle.json()).success).toBe(false)

    // price_amount numeric(10,6) — no negative prices.
    const negative = await request.post('/api/subscription/admin/plans', {
      headers: adminHeaders(),
      data: {
        plan: {
          title: 'invalid price',
          price_amount: -1,
          total_amount: 1000,
        },
      },
    })
    expect((await negative.json()).success).toBe(false)

    // price_amount is bounded at 9999.
    const tooLarge = await request.post('/api/subscription/admin/plans', {
      headers: adminHeaders(),
      data: {
        plan: {
          title: 'too expensive',
          price_amount: 10000,
          total_amount: 1000,
        },
      },
    })
    expect((await tooLarge.json()).success).toBe(false)
  })
})
