/*
Journey-level smoke for a fresh cuberouter deployment: API gates, the setup
wizard, sign-in, and an authenticated session — exercised against the real
container image running in KinD. Setup can only happen once, so the describe
block is serial and state intentionally flows between tests.
*/
import { expect, test } from '@playwright/test'

const ADMIN = { username: 'root', password: 'correct-horse-battery' }

// A fresh-deployment journey mutates global state (system setup can only
// happen once), so Playwright cannot retry it: after any partial run the
// system is no longer fresh and the pre-init assertions would fail again.
test.describe.configure({ mode: 'serial', retries: 0 })

test.describe('fresh deployment journey', () => {
  test('API health and setup gate are sane before initialization', async ({
    request,
  }) => {
    const status = await request.get('/api/status')
    expect(status.ok()).toBeTruthy()
    const statusBody = await status.json()
    expect(statusBody.success).toBe(true)

    const setup = await request.get('/api/setup')
    const setupBody = await setup.json()
    expect(setupBody.success).toBe(true)
    expect(setupBody.data.status).toBe(false)

    const badLogin = await request.post('/api/user/login', {
      data: { username: ADMIN.username, password: 'definitely-wrong' },
    })
    const badLoginBody = await badLogin.json()
    expect(badLoginBody.success).toBe(false)
  })

  test('setup wizard, sign-in, and authenticated session work end to end', async ({
    page,
    request,
  }) => {
    await page.goto('/setup')

    // Step 1: database summary (SQLite default) → Next
    await page.getByRole('button', { name: 'Next' }).click()

    // Step 2: administrator account
    await page.getByLabel('Administrator username').fill(ADMIN.username)
    await page.getByLabel('Password', { exact: true }).fill(ADMIN.password)
    await page.getByLabel('Confirm password').fill(ADMIN.password)
    await page.getByRole('button', { name: 'Next' }).click()

    // Step 3: usage mode (defaults) → Next
    await page.getByRole('button', { name: 'Next' }).click()

    // Step 4: review → initialize. A full page navigation before the POST
    // finishes would abort the request, so wait for the successful response
    // before moving on.
    const setupResponsePromise = page.waitForResponse(
      (response) =>
        response.url().includes('/api/setup') &&
        response.request().method() === 'POST'
    )
    await page.getByRole('button', { name: 'Initialize system' }).click()
    const setupResponse = await setupResponsePromise
    const setupBody = await setupResponse.json()
    expect(setupBody.success).toBe(true)

    await page.goto('/sign-in')
    await page.getByLabel('Username or Email').fill(ADMIN.username)
    await page.getByLabel('Password', { exact: true }).fill(ADMIN.password)
    await page.getByRole('button', { name: 'Sign in' }).click()
    await page.waitForURL('**/dashboard**')

    // Dashboard auth is stateless: /api/user/login returns an access token in
    // the response body and the UI sends it as an Authorization header via its
    // axios interceptor. page.request shares only cookies with the browser, so
    // log in through the API and authenticate the way the backend expects.
    const login = await request.post('/api/user/login', {
      data: { username: ADMIN.username, password: ADMIN.password },
    })
    const loginBody = await login.json()
    expect(loginBody.success).toBe(true)
    const accessToken = loginBody.data.access_token

    const self = await request.get('/api/user/self', {
      headers: { Authorization: `Bearer ${accessToken}` },
    })
    const selfBody = await self.json()
    expect(selfBody.success).toBe(true)
    expect(selfBody.data.username).toBe(ADMIN.username)

    // Setup is a one-way gate: a second initialization must be rejected.
    const again = await page.request.post('/api/setup', { data: {} })
    const againBody = await again.json()
    expect(againBody.success).toBe(false)
  })
})
