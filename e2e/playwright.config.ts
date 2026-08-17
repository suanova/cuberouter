// Playwright runs this config directly (it transpiles TS natively), so no
// build step or local toolchain install is required.
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './specs',
  // The suite drives a single fresh deployment: system setup can only happen
  // once, so specs run serially on one worker.
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: [['list'], ['html', { open: 'never' }]],
  timeout: 60_000,
  expect: { timeout: 10_000 },
  use: {
    baseURL: process.env.E2E_BASE_URL ?? 'http://127.0.0.1:3000',
    // Pin the UI language so role/label selectors match the English source
    // strings deterministically instead of depending on runner locale.
    locale: 'en-US',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
