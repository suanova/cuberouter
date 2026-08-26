/*
Admin docs link (`general_setting.admin_docs_link`) and the placement of the
docs-link settings.

Covers the two features shipped together:
1. The user-facing "Documentation Link" setting was moved from
   Billing -> Quota Settings into System Settings -> Site & Branding ->
   System Information (next to Logo URL), and a new "Admin Documentation
   Link" setting was added next to it. Both are persisted with the
   flattened `general_setting.*` keys and exposed by `/api/status`.
2. The top-nav "Docs" link is routed by role: admins get the admin docs
   link (falling back to the user docs link when it is unset); regular
   users and guests get the user docs link.

Deployment initialization is owned by the global setup
(e2e/global-setup.ts), which creates root/correct-horse-battery, so this
file only signs in and drives the deployment like a real operator. Every
run creates its own regular user, so the file is repeatable against the
same deployment.
*/
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

// Must match the admin account the global setup (e2e/global-setup.ts) creates.
const ADMIN = { username: 'root', password: 'correct-horse-battery' }

// Distinct URLs so the assertions prove which setting produced the href.
const USER_DOCS = 'https://user-docs.example.com'
const ADMIN_DOCS = 'https://admin-docs.example.com'

// Timestamp suffix keeps every run isolated: the username column is unique,
// so re-running against an already-used deployment never collides.
const USERNAME = `docs.user.${Date.now()}@example.com`
const PASSWORD = 'DocsUser123'

let rootToken = ''

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

// Set one option the way the settings UI saves it: the flattened
// `general_setting.<field>` key against PUT /api/option/.
async function setOption(
  request: APIRequestContext,
  key: string,
  value: string
): Promise<void> {
  const res = await request.put('/api/option/', {
    headers: adminHeaders(),
    data: { key, value },
  })
  const body = await res.json()
  expect(body.success, `setOption(${key}, ${value}) failed: ${body.message}`).toBe(
    true
  )
}

// Top-nav "Docs" link. The desktop header renders it as a plain external
// anchor; the mobile dropdown is not in the DOM until opened, so this is the
// only "Docs" link on the page.
async function docsLinkHref(page: Page): Promise<string | null> {
  const link = page.getByRole('link', { name: 'Docs', exact: true })
  await expect(link).toHaveCount(1)
  await expect(link).toBeVisible()
  return (await link.getAttribute('href')) as string | null
}

async function freshSignIn(page: Page, username: string, password: string) {
  // Navigate to a real origin page first: a fresh page starts at about:blank,
  // where reading localStorage throws a SecurityError.
  await page.goto('/sign-in')
  // The app keeps the session token client-side and caches /api/status in
  // localStorage; clear both so the new session refetches auth + status
  // instead of inheriting the previous role's view.
  await page.evaluate(() => localStorage.clear())
  await page.context().clearCookies()
  await page.getByLabel('Username or Email').fill(username)
  await page.getByLabel('Password', { exact: true }).fill(password)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await page.waitForURL('**/dashboard**')
}

test.describe.configure({ mode: 'serial', retries: 0 })

test.beforeAll(async ({ request }) => {
  rootToken = await login(request, ADMIN.username, ADMIN.password)
})

test.describe('docs link settings and role-based top-nav routing', () => {
  test('admin configures both docs links; /api/status exposes them', async ({
    request,
  }) => {
    await setOption(request, 'general_setting.docs_link', USER_DOCS)
    await setOption(request, 'general_setting.admin_docs_link', ADMIN_DOCS)

    const res = await request.get('/api/status')
    const body = await res.json()
    expect(body.success).toBe(true)
    expect(body.data.docs_link).toBe(USER_DOCS)
    expect(body.data.admin_docs_link).toBe(ADMIN_DOCS)
  })

  test('admin creates a regular (role 1) user', async ({ request }) => {
    const create = await request.post('/api/user/', {
      headers: adminHeaders(),
      data: {
        username: USERNAME,
        password: PASSWORD,
        display_name: 'Docs Test User',
      },
    })
    expect((await create.json()).success).toBe(true)

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

    const detail = await request.get(`/api/user/${created!.id}`, {
      headers: adminHeaders(),
    })
    const user = (await detail.json()).data as { role: number }
    // RoleCommonUser — a regular user, so the top-nav routing is exercised
    // against a non-admin account.
    expect(user.role).toBe(1)
  })

  test('admin UI: top-nav Docs → admin link; both fields on System Information, gone from Billing → Quota', async ({
    page,
  }) => {
    await freshSignIn(page, ADMIN.username, ADMIN.password)

    await page.goto('/')
    expect(await docsLinkHref(page)).toBe(ADMIN_DOCS)

    // The migrated settings live on System Information, next to Logo URL,
    // and are pre-filled with the saved values.
    await page.goto('/system-settings/site/system-info')
    await expect(
      page.getByLabel('Documentation Link', { exact: true })
    ).toHaveValue(USER_DOCS)
    await expect(
      page.getByLabel('Admin Documentation Link', { exact: true })
    ).toHaveValue(ADMIN_DOCS)

    // And they are gone from Billing -> Quota Settings (Top-Up Link is still
    // there, proving the section rendered).
    await page.goto('/system-settings/billing/quota')
    await expect(
      page.getByText('Documentation Link', { exact: true })
    ).toHaveCount(0)
    await expect(page.getByText('Top-Up Link', { exact: true })).toBeVisible()
  })

  test('regular user UI: top-nav Docs → user docs link', async ({ page }) => {
    await freshSignIn(page, USERNAME, PASSWORD)

    await page.goto('/')
    expect(await docsLinkHref(page)).toBe(USER_DOCS)
  })

  test('guest UI: top-nav Docs → user docs link', async ({ page }) => {
    await page.goto('/')
    await page.evaluate(() => localStorage.clear())
    await page.context().clearCookies()
    await page.reload()

    expect(await docsLinkHref(page)).toBe(USER_DOCS)
  })

  test('admin UI: empty admin docs link falls back to the user docs link', async ({
    page,
    request,
  }) => {
    await setOption(request, 'general_setting.admin_docs_link', '')
    await freshSignIn(page, ADMIN.username, ADMIN.password)

    await page.goto('/')
    expect(await docsLinkHref(page)).toBe(USER_DOCS)
  })

  test.afterAll(async ({ request }) => {
    // Leave the deployment with the admin docs link configured again, so a
    // partial re-run of this suite still sees a consistent state.
    await setOption(request, 'general_setting.admin_docs_link', ADMIN_DOCS)
  })
})
