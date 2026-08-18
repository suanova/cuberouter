/*
Global setup: runs once before every project and spec. It owns the one-time
deployment initialization (root admin creation) that the whole suite depends
on — this is the "setup first" step, moved out of journey.spec.ts so no spec
file ever has to initialize (or assert freshness of) the deployment.

Initialization is a one-way gate, and the deployment is a shared resource: if
it is already initialized (e.g. a re-run against the same deployment), setup
is skipped. Because this runs for every invocation — regardless of spec file,
project, or run order — individual specs can rely on an initialized system.
*/
import { request } from '@playwright/test'
import type { FullConfig } from '@playwright/test'

// Must match the admin account every spec signs in with.
const ADMIN = { username: 'root', password: 'correct-horse-battery' }

export default async function globalSetup(_config: FullConfig): Promise<void> {
  const baseURL = process.env.E2E_BASE_URL ?? 'http://127.0.0.1:3000'
  const api = await request.newContext({ baseURL })
  try {
    const setupRes = await api.get('/api/setup')
    const setupBody = await setupRes.json()
    if (setupBody.success && setupBody.data?.status) {
      return // already initialized (re-run)
    }

    // Same one-way wizard journey.spec.ts used to drive, called directly via
    // its API so no browser session is needed before tests start.
    const initRes = await api.post('/api/setup', {
      data: {
        username: ADMIN.username,
        password: ADMIN.password,
        confirmPassword: ADMIN.password,
        SelfUseModeEnabled: false,
        DemoSiteEnabled: false,
      },
    })
    const initBody = await initRes.json()
    if (!initBody.success) {
      throw new Error(
        `global setup failed to initialize the deployment: ${initBody.message ?? 'unknown error'}`
      )
    }
  } finally {
    await api.dispose()
  }
}
