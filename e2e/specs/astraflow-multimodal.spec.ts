/*
AstraFlow (channel type 59) multi-model support: the same channel that runs
Seedance video tasks also relays OpenAI-compatible non-video modes through the
non-task adaptor. This spec creates a type-59 channel whose base_url points at
MockLLM (instead of the built-in https://api.modelverse.cn), then relays a chat
completion through it and asserts the deterministic MockLLM response — proving
a single AstraFlow channel now serves non-video models end to end, with the
channel API key sent as the upstream Bearer token.

Requires the MockLLM upstream and an initialized deployment. See the header of
channel.spec.ts for how to run MockLLM locally (MOCKLLM_BASE_URL is honored,
defaulting to http://127.0.0.1:18000). No video task is submitted in this spec.
*/
import { expect, test, type APIRequestContext } from '@playwright/test'

const ADMIN = { username: 'root', password: 'correct-horse-battery' }

const MOCKLLM_BASE_URL =
  process.env.MOCKLLM_BASE_URL ?? 'http://127.0.0.1:18000'
const RUN_SUFFIX = Date.now()
const CHANNEL_NAME = `AstraFlow Multi ${RUN_SUFFIX}`
const USERNAME = `astraflow.mock.${RUN_SUFFIX}@example.com`
const USER_PASSWORD = 'AstraMock123'
const MODEL = `astraflow-mock-${RUN_SUFFIX}`

// Same deterministic response as channel.spec.ts (e2e/mockllm/responses.yml).
const PONG = 'pong from MockLLM'

let rootToken = ''
let channelId = 0
let userId = 0
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
  // Same relay prerequisites as channel.spec.ts: self-use mode (default model
  // pricing) and the host performance monitor disabled (avoid 503s on busy
  // runners). Idempotent.
  const option = await request.put('/api/option/', {
    headers: adminHeaders(),
    data: { key: 'SelfUseModeEnabled', value: 'true' },
  })
  expect((await option.json()).success).toBe(true)

  const perf = await request.put('/api/option/', {
    headers: adminHeaders(),
    data: { key: 'performance_setting.monitor_enabled', value: 'false' },
  })
  expect((await perf.json()).success).toBe(true)

  // The channel base_url points at MockLLM over plain HTTP (locally
  // http://127.0.0.1:18000, in CI http://mockllm:18000). The AstraFlow
  // HTTPS-upstream enforcement (security_setting.require_https_channel_base_url,
  // default on) rejects cleartext non-loopback upstreams, so disable it for
  // this test deployment; production keeps the secure default.
  const sec = await request.put('/api/option/', {
    headers: adminHeaders(),
    data: { key: 'security_setting.require_https_channel_base_url', value: 'false' },
  })
  expect((await sec.json()).success).toBe(true)
})

test.describe('AstraFlow multi-model relay', () => {
  test('channel — create a type-59 AstraFlow channel pointing at MockLLM', async ({
    request,
  }) => {
    const res = await request.post('/api/channel/', {
      headers: adminHeaders(),
      data: {
        mode: 'single',
        channel: {
          type: 59, // constant.ChannelTypeAstraFlow
          name: CHANNEL_NAME,
          key: 'mock-upstream-key',
          base_url: MOCKLLM_BASE_URL,
          models: MODEL,
          group: 'default',
        },
      },
    })
    expect((await res.json()).success).toBe(true)

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
    expect(channel!.type).toBe(59)
    expect(channel!.base_url).toBe(MOCKLLM_BASE_URL)
    expect(channel!.models).toBe(MODEL)
    expect(channel!.status).toBe(1) // enabled
  })

  test('api key — create user, quota, and token for the relay call', async ({
    request,
  }) => {
    const create = await request.post('/api/user/', {
      headers: adminHeaders(),
      data: {
        username: USERNAME,
        password: USER_PASSWORD,
        display_name: 'AstraFlow Multi User',
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
      data: { name: 'astraflow-mock-token', unlimited_quota: true },
    })
    expect((await tokenRes.json()).success).toBe(true)

    const tokens = await request.get('/api/token/', {
      headers: { Authorization: `Bearer ${userToken}` },
    })
    const tokenItems = (await tokens.json()).data.items as {
      id: number
      name: string
    }[]
    const token = tokenItems.find((t) => t.name === 'astraflow-mock-token')
    expect(token).toBeTruthy()

    const keyRes = await request.post(`/api/token/${token!.id}/key`, {
      headers: { Authorization: `Bearer ${userToken}` },
    })
    const keyBody = await keyRes.json()
    expect(keyBody.success).toBe(true)
    tokenKey = keyBody.data.key as string
    expect(tokenKey.length).toBeGreaterThan(20)
  })

  test('relay — chat completion through the AstraFlow channel returns the mock response', async ({
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

    // The non-task relay bills the user like any other OpenAI-compatible mode.
    const self = await request.get(`/api/user/${userId}`, {
      headers: adminHeaders(),
    })
    expect((await self.json()).data.used_quota).toBeGreaterThan(0)
  })

  test('relay — streaming through the AstraFlow channel returns the same content', async ({
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
    let streamed = ''
    for (const line of body.split('\n')) {
      if (!line.startsWith('data: ') || line.includes('[DONE]')) continue
      const chunk = JSON.parse(line.slice('data: '.length))
      const content = chunk.choices?.[0]?.delta?.content
      if (content) streamed += content
    }
    expect(streamed).toBe(PONG)
  })
})
