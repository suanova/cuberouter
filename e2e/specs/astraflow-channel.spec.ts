/*
AstraFlow (channel type 59, Seedance video tasks) through the admin UI:
create the channel in the "Create Channel" drawer, then verify it shows up in
the channel table with its built-in Seedance model list, and that the backend
persisted it as type 59 with an empty base_url (so the built-in default
https://api.modelverse.cn from constant.ChannelBaseURLs[59] applies at relay
time, via model.Channel.GetBaseURL).

Running it locally requires a serving deployment (global setup in
e2e/global-setup.ts initializes root). No upstream calls are made, so MockLLM
is not required for this spec.
*/
import { expect, test, type APIRequestContext } from '@playwright/test'

const ADMIN = { username: 'root', password: 'correct-horse-battery' }

const RUN_SUFFIX = Date.now()
const CHANNEL_NAME = `AstraFlow E2E ${RUN_SUFFIX}`
const SEEDANCE_MODELS = ['doubao-seedance-1-5-pro', 'doubao-seedance-2-0-260128']

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

test.describe('AstraFlow channel (UI)', () => {
  test('UI — create an AstraFlow channel and see its Seedance models', async ({
    page,
    request,
  }) => {
    await page.goto('/sign-in')
    await page.getByLabel('Username or Email').fill(ADMIN.username)
    await page.getByLabel('Password', { exact: true }).fill(ADMIN.password)
    await page.getByRole('button', { name: 'Sign in' }).click()
    await page.waitForURL('**/dashboard**')

    await page.goto('/channels')
    await page.getByRole('button', { name: 'Create Channel' }).click()

    // The drawer is a Sheet (Base UI Dialog); scope everything to it.
    const drawer = page.getByRole('dialog')
    await expect(drawer).toBeVisible()

    // Channel type is a searchable combobox. Clicking focuses and opens it
    // (pointer-focus also opens), typing filters by option label.
    const typeInput = drawer.getByPlaceholder('Search channel type...')
    await typeInput.click()
    await typeInput.fill('AstraFlow')
    await drawer
      .getByRole('option', { name: 'AstraFlow' })
      .click()

    await drawer
      .getByPlaceholder('e.g., OpenAI GPT-4 Production')
      .fill(CHANNEL_NAME)
    await drawer
      .getByPlaceholder('Enter API key for this channel')
      .fill('sk-astraflow-e2e')

    // Base URL stays empty: type 59 is an official channel with a built-in
    // address, so the field must accept (and keep) the empty default.
    await expect(drawer.getByPlaceholder('Leave empty to use default')).toBeVisible()

    // The model list is a chip MultiSelect with inline custom creation: type
    // the model name and press Enter to turn each Seedance model into a chip.
    // Locate it by its stable accessible name (aria-label): the placeholder
    // attribute disappears once the first chip exists, so getByPlaceholder
    // would go stale after the first model is added.
    const modelsInput = drawer.getByRole('combobox', {
      name: 'Select models or add custom ones',
    })
    for (const model of SEEDANCE_MODELS) {
      await modelsInput.click()
      await modelsInput.fill(model)
      await modelsInput.press('Enter')
    }
    for (const model of SEEDANCE_MODELS) {
      await expect(
        drawer.getByText(model, { exact: true }).first()
      ).toBeVisible()
    }

    await drawer.getByRole('button', { name: 'Save changes' }).click()
    await expect(drawer).toBeHidden()

    // Other specs leave channels behind; filter by the unique per-run name so
    // the new row is always on the current page.
    await page
      .getByPlaceholder('Filter by name, ID, or key...')
      .fill(CHANNEL_NAME)

    // The channels list defaults to card view (card rows don't expose
    // role="row" or the model chips), so switch to the table view before
    // asserting on the row. The "Table view" toggle is the second icon button
    // in the "View mode" group; only click it when table view isn't active.
    const viewModeGroup = page.getByRole('group', { name: 'View mode' })
    const tableViewToggle = viewModeGroup.getByRole('button').nth(1)
    if ((await tableViewToggle.getAttribute('aria-pressed')) !== 'true') {
      await tableViewToggle.click()
    }

    const row = page.getByRole('row', { name: new RegExp(CHANNEL_NAME) })
    await expect(row).toBeVisible()
    await expect(
      row.getByText('AstraFlow', { exact: true })
    ).toBeVisible()
    // The channels table has no Models column; the model list is verified
    // above via the drawer chips and below via the persisted channel record.

    // Cross-check via the dashboard API: type 59 persisted, base_url empty
    // (built-in default applies), models stored, channel enabled.
    const rootToken = await login(request, ADMIN.username, ADMIN.password)
    const search = await request.get(
      `/api/channel/search?keyword=${encodeURIComponent(CHANNEL_NAME)}`,
      { headers: { Authorization: `Bearer ${rootToken}` } }
    )
    const items = (await search.json()).data.items as {
      id: number
      name: string
      type: number
      base_url: string | null
      models: string
      status: number
    }[]
    const channel = items.find((c) => c.name === CHANNEL_NAME)
    expect(channel).toBeTruthy()
    expect(channel!.type).toBe(59)
    expect(channel!.base_url || '').toBe('')
    for (const model of SEEDANCE_MODELS) {
      expect(channel!.models).toContain(model)
    }
    expect(channel!.status).toBe(1)
  })
})
