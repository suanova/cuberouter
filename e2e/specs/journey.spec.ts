/*
Journey-level smoke for an initialized cuberouter deployment: API health,
sign-in, and an authenticated session. Deployment initialization is owned by
the global setup (e2e/global-setup.ts), which runs once before any spec, so
this file never touches /api/setup and does not depend on run order or on a
fresh deployment.
*/
import { expect, test } from '@playwright/test'

const ADMIN = { username: 'root', password: 'correct-horse-battery' }

test.describe.configure({ mode: 'serial' })

test.describe('deployment journey', () => {
  test('API health is sane and bad credentials are rejected', async ({
    request,
  }) => {
    const status = await request.get('/api/status')
    expect(status.ok()).toBeTruthy()
    const statusBody = await status.json()
    expect(statusBody.success).toBe(true)

    const badLogin = await request.post('/api/user/login', {
      data: { username: ADMIN.username, password: 'definitely-wrong' },
    })
    const badLoginBody = await badLogin.json()
    expect(badLoginBody.success).toBe(false)
  })

  test('sign-in and authenticated session work end to end', async ({
    page,
    request,
  }) => {
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
  })
})
