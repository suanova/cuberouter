/*
LLM relay flows driven by a MockLLM upstream (https://github.com/StacklokLabs/mockllm):
channel management, API keys (tokens), and the playground (训练场), all verified
end to end through the cuberouter relay.

MockLLM is an OpenAI/Anthropic-compatible simulator that maps the LAST user
message EXACTLY to a deterministic response from e2e/mockllm/responses.yml, so
the suite can assert exact response content through the relay.

Running it locally (required before this spec):
    pip install mockllm
    mockllm start --host 127.0.0.1 --port 18000 \
        --responses e2e/mockllm/responses.yml
In CI the e2e deployment runs MockLLM in-cluster (e2e/k8s/mockllm.yaml) and
sets MOCKLLM_BASE_URL to the in-cluster service address
(.github/workflows/e2e.yml).

This spec requires an initialized deployment (owned by e2e/global-setup.ts)
and its own user/channel with unique names, so it never collides with other
spec files or with re-runs. Its beforeAll also enables self-use mode (default
model pricing) and disables the host performance monitor (CPU/memory/disk
thresholds would otherwise 503 on busy shared runners).
*/
import { expect, test, type APIRequestContext } from '@playwright/test'

// Must match the admin account the global setup (e2e/global-setup.ts) creates.
const ADMIN = { username: 'root', password: 'correct-horse-battery' }

// Deck-style unique identifiers keep every run isolated: the channel, user,
// and MODEL name are all per-run, so a re-run against the same deployment
// never collides (in particular, disabling "our" channel must leave no other
// channel able to serve "our" model).
const MOCKLLM_BASE_URL =
  process.env.MOCKLLM_BASE_URL ?? 'http://127.0.0.1:18000'
const RUN_SUFFIX = Date.now()
const CHANNEL_NAME = `MockLLM ${RUN_SUFFIX}`
const USERNAME = `mock.user.${RUN_SUFFIX}@example.com`
const USER_PASSWORD = 'MockUser123'
const MODEL = `mock-llm-${RUN_SUFFIX}`

// Deterministic responses from e2e/mockllm/responses.yml.
const PONG = 'pong from MockLLM'
const UNKNOWN_RESPONSE =
  "I don't know the answer to that. This is a mock response."
const PLAYGROUND_REPLY = 'Hello back from MockLLM!'
// The `models:` list in e2e/mockllm/responses.yml, served as the
// OpenAI-compatible GET /v1/models by the launcher workaround.
const MOCKLLM_MODELS = ['mock-llm', 'mock-llm-2', 'gpt-3.5-turbo', 'gpt-4']

// State flows between tests; the describe block is serial by design.
let rootToken = ''
let channelId = 0
let userId = 0
let tokenId = 0
let tokenKey = ''

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
  // Model pricing: enable self-use mode so relay requests against the mock
  // model work without configuring per-model prices (the system-setup wizard
  // leaves it off). Idempotent, and only affects billing defaults.
  const option = await request.put('/api/option/', {
    headers: adminHeaders(),
    data: { key: 'SelfUseModeEnabled', value: 'true' },
  })
  expect((await option.json()).success).toBe(true)

  // Host performance checks (CPU/memory/disk thresholds) are environment
  // concerns that flake shared runners and local dev machines; disable them
  // for the relay tests so a busy host cannot produce 503s.
  const perf = await request.put('/api/option/', {
    headers: adminHeaders(),
    data: { key: 'performance_setting.monitor_enabled', value: 'false' },
  })
  expect((await perf.json()).success).toBe(true)
})

test.describe('MockLLM relay (v4.5 demo flows)', () => {
  test('channel — admin creates an OpenAI channel pointing at MockLLM', async ({
    request,
  }) => {
    const res = await request.post('/api/channel/', {
      headers: adminHeaders(),
      data: {
        mode: 'single',
        channel: {
          type: 1, // constant.ChannelTypeOpenAI
          name: CHANNEL_NAME,
          key: 'mock-upstream-key',
          base_url: MOCKLLM_BASE_URL,
          models: MODEL,
          group: 'default',
          test_model: MODEL,
        },
      },
    })
    const body = await res.json()
    expect(body.success).toBe(true)

    const search = await request.get(
      `/api/channel/search?keyword=${encodeURIComponent(CHANNEL_NAME)}`,
      { headers: adminHeaders() }
    )
    const items = (await search.json()).data.items as {
      id: number
      name: string
      type: number
      base_url: string
      models: string
      status: number
    }[]
    const channel = items.find((c) => c.name === CHANNEL_NAME)
    expect(channel).toBeTruthy()
    channelId = channel!.id
    expect(channel!.type).toBe(1)
    expect(channel!.base_url).toBe(MOCKLLM_BASE_URL)
    expect(channel!.models).toBe(MODEL)
    expect(channel!.status).toBe(1) // enabled
  })

  test('channel — connection test against MockLLM succeeds', async ({
    request,
  }) => {
    // In CI the MockLLM pod pip-installs mockllm on first boot, so the
    // upstream may not be ready yet; poll the backend-mediated channel test
    // until it answers.
    let body: { success?: boolean } = {}
    for (let attempt = 1; attempt <= 20; attempt++) {
      const res = await request.get(`/api/channel/test/${channelId}`, {
        headers: adminHeaders(),
      })
      body = await res.json()
      if (body.success) break
      await new Promise((resolve) => setTimeout(resolve, 3000))
    }
    expect(body.success).toBe(true)
  })

  test('channel — fetch_models pulls the OpenAI-compatible model list', async ({
    request,
  }) => {
    // Upstream mockllm has no /v1/models; the launcher workaround
    // (e2e/mockllm/launcher.py) serves an OpenAI-compatible one so the
    // channel "fetch models from upstream" feature works. Exercise it on a
    // throwaway channel and delete it afterwards so it never serves relay
    // traffic (the relay tests keep their per-run model for isolation).
    const create = await request.post('/api/channel/', {
      headers: adminHeaders(),
      data: {
        mode: 'single',
        channel: {
          type: 1,
          name: `${CHANNEL_NAME} fetch`,
          key: 'mock-upstream-key',
          base_url: MOCKLLM_BASE_URL,
          models: '',
          group: 'default',
        },
      },
    })
    expect((await create.json()).success).toBe(true)

    const search = await request.get(
      `/api/channel/search?keyword=${encodeURIComponent(`${CHANNEL_NAME} fetch`)}`,
      { headers: adminHeaders() }
    )
    const items = (await search.json()).data.items as { id: number }[]
    const fetchChannelId = items[0]?.id
    expect(fetchChannelId).toBeTruthy()

    const fetch = await request.get(
      `/api/channel/fetch_models/${fetchChannelId}`,
      { headers: adminHeaders() }
    )
    const fetchBody = await fetch.json()
    expect(fetchBody.success).toBe(true)
    const models = fetchBody.data as string[]
    for (const expected of MOCKLLM_MODELS) {
      expect(models).toContain(expected)
    }

    const del = await request.delete(`/api/channel/${fetchChannelId}`, {
      headers: adminHeaders(),
    })
    expect((await del.json()).success).toBe(true)
  })

  test('api key — create user, quota, token, and fetch the key', async ({
    request,
  }) => {
    const create = await request.post('/api/user/', {
      headers: adminHeaders(),
      data: {
        username: USERNAME,
        password: USER_PASSWORD,
        display_name: 'Mock LLM User',
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
    const user = items.find((u) => u.username === USERNAME)
    expect(user).toBeTruthy()
    userId = user!.id

    const quota = await request.post('/api/user/manage', {
      headers: adminHeaders(),
      data: { id: userId, action: 'add_quota', value: 10000000, mode: 'add' },
    })
    expect((await quota.json()).success).toBe(true)

    const userToken = await login(request, USERNAME, USER_PASSWORD)
    const tokenRes = await request.post('/api/token/', {
      headers: { Authorization: `Bearer ${userToken}` },
      data: { name: 'mockllm-token', unlimited_quota: true },
    })
    expect((await tokenRes.json()).success).toBe(true)

    const tokens = await request.get('/api/token/', {
      headers: { Authorization: `Bearer ${userToken}` },
    })
    const tokenItems = (await tokens.json()).data.items as {
      id: number
      name: string
      status: number
    }[]
    const token = tokenItems.find((t) => t.name === 'mockllm-token')
    expect(token).toBeTruthy()
    tokenId = token!.id
    expect(token!.status).toBe(1)

    const keyRes = await request.post(`/api/token/${tokenId}/key`, {
      headers: { Authorization: `Bearer ${userToken}` },
    })
    const keyBody = await keyRes.json()
    expect(keyBody.success).toBe(true)
    tokenKey = keyBody.data.key as string
    expect(tokenKey.length).toBeGreaterThan(20)
  })

  test('relay — chat completion returns the deterministic mock response', async ({
    request,
  }) => {
    const res = await request.post('/v1/chat/completions', {
      headers: { Authorization: `Bearer ${tokenKey}` },
      data: {
        model: MODEL,
        messages: [{ role: 'user', content: 'ping' }],
      },
    })
    expect(res.ok()).toBeTruthy()
    const body = await res.json()
    expect(body.choices[0].message.content).toBe(PONG)
    // The relay bills the user: used quota grows after the request.
    const self = await request.get(`/api/user/${userId}`, {
      headers: adminHeaders(),
    })
    expect((await self.json()).data.used_quota).toBeGreaterThan(0)
  })

  test('relay — streaming returns the same deterministic content', async ({
    request,
  }) => {
    const res = await request.post('/v1/chat/completions', {
      headers: { Authorization: `Bearer ${tokenKey}` },
      data: {
        model: MODEL,
        messages: [{ role: 'user', content: 'ping' }],
        stream: true,
      },
    })
    expect(res.ok()).toBeTruthy()
    const body = await res.text()
    expect(body).toContain('data: [DONE]')
    // mockllm streams character by character, so the full text only exists
    // after concatenating every chunk's content delta.
    let streamed = ''
    for (const line of body.split('\n')) {
      if (!line.startsWith('data: ') || line.includes('[DONE]')) continue
      const chunk = JSON.parse(line.slice('data: '.length))
      const content = chunk.choices?.[0]?.delta?.content
      if (content) streamed += content
    }
    expect(streamed).toBe(PONG)
  })

  test('relay — unconfigured prompts fall back to the mock default', async ({
    request,
  }) => {
    const res = await request.post('/v1/chat/completions', {
      headers: { Authorization: `Bearer ${tokenKey}` },
      data: {
        model: MODEL,
        messages: [
          { role: 'user', content: 'some prompt that is not configured' },
        ],
      },
    })
    expect(res.ok()).toBeTruthy()
    const body = await res.json()
    expect(body.choices[0].message.content).toBe(UNKNOWN_RESPONSE)
  })

  test('channel — update name and models', async ({ request }) => {
    const res = await request.put('/api/channel/', {
      headers: adminHeaders(),
      data: {
        id: channelId,
        name: `${CHANNEL_NAME} updated`,
        models: `${MODEL},mock-llm-2`,
      },
    })
    expect((await res.json()).success).toBe(true)

    const channel = await request.get(`/api/channel/${channelId}`, {
      headers: adminHeaders(),
    })
    const data = (await channel.json()).data
    expect(data.name).toBe(`${CHANNEL_NAME} updated`)
    expect(data.models).toContain('mock-llm-2')
  })

  test('channel — disabled channel makes the relay fail; re-enable recovers', async ({
    request,
  }) => {
    const disable = await request.post(`/api/channel/${channelId}/status`, {
      headers: adminHeaders(),
      data: { status: 2 }, // disabled
    })
    expect((await disable.json()).success).toBe(true)

    const fail = await request.post('/v1/chat/completions', {
      headers: { Authorization: `Bearer ${tokenKey}` },
      data: { model: MODEL, messages: [{ role: 'user', content: 'ping' }] },
    })
    // Relay errors are returned as {"error": {"message": ...}} bodies (no
    // "success" field), unlike the dashboard API responses.
    const failBody = await fail.json()
    expect(failBody.error?.message ?? failBody.message).toContain(
      'No available channel'
    )

    const enable = await request.post(`/api/channel/${channelId}/status`, {
      headers: adminHeaders(),
      data: { status: 1 }, // enabled
    })
    expect((await enable.json()).success).toBe(true)

    const ok = await request.post('/v1/chat/completions', {
      headers: { Authorization: `Bearer ${tokenKey}` },
      data: { model: MODEL, messages: [{ role: 'user', content: 'ping' }] },
    })
    const okBody = await ok.json()
    expect(okBody.choices[0].message.content).toBe(PONG)
  })

  test('api key — disabled token is rejected; re-enable recovers', async ({
    request,
  }) => {
    const userToken = await login(request, USERNAME, USER_PASSWORD)
    const disable = await request.put('/api/token/?status_only=true', {
      headers: { Authorization: `Bearer ${userToken}` },
      data: { id: tokenId, status: 2 }, // disabled
    })
    expect((await disable.json()).success).toBe(true)

    const fail = await request.post('/v1/chat/completions', {
      headers: { Authorization: `Bearer ${tokenKey}` },
      data: { model: MODEL, messages: [{ role: 'user', content: 'ping' }] },
    })
    expect(fail.status()).toBe(401)

    const enable = await request.put('/api/token/?status_only=true', {
      headers: { Authorization: `Bearer ${userToken}` },
      data: { id: tokenId, status: 1 }, // enabled
    })
    expect((await enable.json()).success).toBe(true)

    const ok = await request.post('/v1/chat/completions', {
      headers: { Authorization: `Bearer ${tokenKey}` },
      data: { model: MODEL, messages: [{ role: 'user', content: 'ping' }] },
    })
    expect((await ok.json()).choices[0].message.content).toBe(PONG)
  })

  test('api key — an invalid key is rejected', async ({ request }) => {
    const res = await request.post('/v1/chat/completions', {
      headers: { Authorization: 'Bearer sk-invalid-key' },
      data: { model: MODEL, messages: [{ role: 'user', content: 'ping' }] },
    })
    expect(res.status()).toBe(401)
  })

  test('playground — user sends a question in the UI and sees the mock reply', async ({
    page,
  }) => {
    await page.goto('/sign-in')
    await page.getByLabel('Username or Email').fill(USERNAME)
    await page.getByLabel('Password', { exact: true }).fill(USER_PASSWORD)
    await page.getByRole('button', { name: 'Sign in' }).click()
    await page.waitForURL('**/dashboard**')

    await page.goto('/playground')
    const input = page.getByPlaceholder('Ask anything')
    await expect(input).toBeVisible()
    // The playground auto-selects a model, but other spec files leave
    // channels behind (e.g. the AstraFlow spec's type-59 channel), so the
    // auto-selected model may route to a non-MockLLM upstream. Explicitly
    // pick the per-run mock model to keep the reply deterministic.
    await page.getByRole('combobox').first().click()
    await page.getByPlaceholder('Search models...').fill(MODEL)
    await page.getByRole('option', { name: MODEL, exact: true }).click()
    await input.fill('Hello from the playground')
    await page.getByRole('button', { name: 'Send' }).click()
    await expect(page.getByText(PLAYGROUND_REPLY)).toBeVisible()
  })

  test('UI — usage logs show the responding channel and model', async ({
    page,
  }) => {
    // The relay requests from the earlier tests produced log entries. The
    // Channel column is admin-only, so sign in as root to verify that the
    // UI displays which channel served the request and which model.
    await page.goto('/sign-in')
    await page.getByLabel('Username or Email').fill(ADMIN.username)
    await page.getByLabel('Password', { exact: true }).fill(ADMIN.password)
    await page.getByRole('button', { name: 'Sign in' }).click()
    await page.waitForURL('**/dashboard**')

    await page.goto('/usage-logs')
    // Newest entries are at the top; every request this spec made used the
    // same per-run model and channel.
    await expect(page.getByText(MODEL).first()).toBeVisible()
    await expect(page.getByText(CHANNEL_NAME).first()).toBeVisible()
    await expect(page.getByText(`#${channelId}`).first()).toBeVisible()
  })

  test('UI — add a model on the models page and verify it is listed', async ({
    page,
  }) => {
    // The models page (模型管理) lists model metadata from the `models`
    // table, which is independent of channel configuration — a channel with
    // models does not make them appear there. Drive the real "Add Model"
    // flow and verify the row shows up in the table.
    await page.goto('/sign-in')
    await page.getByLabel('Username or Email').fill(ADMIN.username)
    await page.getByLabel('Password', { exact: true }).fill(ADMIN.password)
    await page.getByRole('button', { name: 'Sign in' }).click()
    await page.waitForURL('**/dashboard**')

    await page.goto('/models')
    await page.getByRole('button', { name: 'Add Model' }).click()
    await page
      .getByPlaceholder('gpt-4, claude-3-opus, etc.')
      .fill(MODEL)
    await page.getByRole('button', { name: 'Save changes' }).click()

    // The created model row appears in the table (per-run name is unique).
    await expect(page.getByText(MODEL).first()).toBeVisible()
  })
})
