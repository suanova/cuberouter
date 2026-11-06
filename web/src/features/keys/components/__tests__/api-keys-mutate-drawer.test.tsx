/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import type { ApiKey } from '../../types'

// The group picker ships hidden (SHOW_KEY_GROUP_SELECTOR = false). The edit form
// still carries it, so it is forced on here to keep covering the Auto-order
// behaviour that returns when the flag is flipped back.
vi.mock('../../constants', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../constants')>()),
  SHOW_KEY_GROUP_SELECTOR: true,
}))

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { api } = await import('@/lib/api')
const { useAuthStore } = await import('@/stores/auth-store')
const { ApiKeysProvider } = await import('../api-keys-provider')
const { ApiKeysMutateDrawer } = await import('../api-keys-mutate-drawer')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

type ApiRequestConfig = {
  headers?: Record<string, string>
  skipAccountContext?: boolean
}
type ApiMethod = (
  url: string,
  data?: unknown,
  config?: ApiRequestConfig
) => Promise<{ data: unknown }>
type MockableApi = {
  get: ApiMethod
  post: ApiMethod
  put: ApiMethod
}
type RenderedDrawer = {
  queryClient: InstanceType<typeof QueryClient>
}
type CapturedRequest = {
  url: string
  data: unknown
  config?: ApiRequestConfig
}
type OrganizationFixture = Record<string, unknown>

const apiClient = api as unknown as MockableApi
const originalGet = apiClient.get
const originalPost = apiClient.post
const originalPut = apiClient.put
let renderedDrawer: RenderedDrawer | null = null
let postedRequests: CapturedRequest[] = []
let putRequests: CapturedRequest[] = []

function organizationFixture(
  overrides: OrganizationFixture
): OrganizationFixture {
  return {
    id: 1,
    name: 'Organization',
    slug: 'organization',
    status: 'active',
    quota: 0,
    used_quota: 0,
    role: 'member',
    capabilities: { can_manage_all_tokens: false },
    ...overrides,
  }
}

function setPersonalQuota(quota: number) {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'tester',
    role: 1,
    quota,
  })
}

function installApiFixtures(options: {
  organizations?: OrganizationFixture[]
  apiKey?: Record<string, unknown>
} = {}) {
  apiClient.get = async (url) => {
    switch (url) {
      case '/api/status':
        return { data: { data: { default_use_auto_group: true } } }
      case '/api/user/models':
        return { data: { success: true, data: [] } }
      case '/api/user/self/groups':
        return {
          data: {
            success: true,
            data: {
              auto: { desc: 'Automatic routing', ratio: 'auto' },
              default: { desc: 'Standard access', ratio: 1 },
              vip: { desc: 'Priority access', ratio: 2 },
            },
          },
        }
      case '/api/token/auto-groups':
        return {
          data: {
            success: true,
            data: { groups: ['vip', 'default'], max_count: 3 },
          },
        }
      case '/api/user/self':
        return {
          data: {
            success: true,
            data: useAuthStore.getState().auth.user,
          },
        }
      case '/api/organizations':
        return { data: { success: true, data: options.organizations ?? [] } }
      case '/api/token/1':
        return { data: { success: true, data: options.apiKey ?? {} } }
      default:
        throw new Error(`Unexpected GET ${url}`)
    }
  }
  apiClient.post = async (url, data, config) => {
    postedRequests.push({ url, data, config })
    return { data: { success: true, data: {} } }
  }
  apiClient.put = async (url, data, config) => {
    putRequests.push({ url, data, config })
    return { data: { success: true, data: {} } }
  }
}

async function renderDrawer(
  currentRow?: ApiKey,
  waitForEnabled = true
): Promise<void> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  renderedDrawer = { queryClient }

  render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <ApiKeysProvider>
          <ApiKeysMutateDrawer
            open
            onOpenChange={() => undefined}
            currentRow={currentRow}
          />
        </ApiKeysProvider>
      </I18nextProvider>
    </QueryClientProvider>
  )
  if (waitForEnabled) {
    await waitFor(
      () => {
        const submitButton = findButton(currentRow ? 'Save changes' : 'Create')
        expect(submitButton).toBeEnabled()
      },
      { timeout: 1500 }
    )
    return
  }

  await waitFor(() => expect(screen.getByText('Billing Account')).toBeTruthy())
}

function findButton(text: string): HTMLButtonElement {
  const button = screen
    .queryAllByRole<HTMLButtonElement>('button')
    .find((candidate) => candidate.textContent?.includes(text))
  if (!button) {
    throw new Error(`Expected button containing "${text}"`)
  }
  return button
}

function getControlByLabel(labelText: string): HTMLElement {
  const label = [...document.querySelectorAll<HTMLLabelElement>('label')].find(
    (candidate) => candidate.textContent?.trim() === labelText
  )
  if (!label) {
    throw new Error(`Expected label "${labelText}"`)
  }

  const control =
    label.control ??
    label
      .closest('[data-slot="form-item"]')
      ?.querySelector<HTMLElement>(
        '[data-slot="form-control"], input, textarea, button[role="combobox"], [role="group"]'
      )
  if (!control) {
    throw new Error(`Expected control for label "${labelText}"`)
  }
  return control
}

function changeInput(input: HTMLElement, value: string): void {
  fireEvent.input(input, { target: { value } })
}

function selectComboboxOption(
  trigger: HTMLElement,
  optionDescription: string
): void {
  fireEvent.click(trigger)
  const option = [
    ...document.querySelectorAll<HTMLElement>('[data-slot="command-item"]'),
  ].find((candidate) => candidate.textContent?.includes(optionDescription))
  if (!option) {
    throw new Error(`Expected option containing "${optionDescription}"`)
  }
  fireEvent.click(option)
}

function findOrganizationRadio(): HTMLElement | null {
  const control = document.querySelector('#api-key-billing-organization')
  return control instanceof HTMLElement ? control : null
}

afterEach(() => {
  apiClient.get = originalGet
  apiClient.post = originalPost
  apiClient.put = originalPut
  useAuthStore.getState().auth.setUser(null)
  postedRequests = []
  putRequests = []
  localStorage.clear()
  if (renderedDrawer) {
    renderedDrawer.queryClient.clear()
    renderedDrawer = null
  }
})

describe('API keys create drawer billing account', () => {
  test('creates a personal key with the default settings when only a name is entered', async () => {
    setPersonalQuota(1_000_000)
    installApiFixtures({
      organizations: [organizationFixture({ id: 2, name: 'Team' })],
    })
    await renderDrawer()

    expect(screen.getByText('Billing Account')).toBeTruthy()
    expect(screen.getByRole('radio', { name: 'Personal' })).toBeChecked()
    changeInput(getControlByLabel('Name'), 'personal-key')
    fireEvent.click(findButton('Create'))

    await waitFor(() => expect(postedRequests).toHaveLength(1))
    expect(postedRequests[0].url).toBe('/api/token/')
    expect(postedRequests[0].data).toMatchObject({
      name: 'personal-key',
      remain_quota: 0,
      expired_time: -1,
      unlimited_quota: true,
      model_limits_enabled: false,
      model_limits: '',
      allow_ips: '',
      group: 'auto',
      auto_groups: [],
      cross_group_retry: true,
    })
  })

  test('defaults to the organization with the most remaining quota when the personal balance is empty', async () => {
    setPersonalQuota(0)
    installApiFixtures({
      organizations: [
        organizationFixture({
          id: 1,
          name: 'Small',
          quota: 100,
          used_quota: 0,
        }),
        organizationFixture({
          id: 2,
          name: 'Large',
          quota: 500,
          used_quota: 0,
        }),
        organizationFixture({
          id: 3,
          name: 'Owned',
          quota: 9_000,
          used_quota: 0,
          role: 'owner',
          capabilities: { can_manage_all_tokens: true },
        }),
      ],
    })
    await renderDrawer()

    expect(screen.getByRole('radio', { name: 'Organization' })).toBeChecked()
    expect(
      screen.getByRole('combobox', { name: 'Select an organization' }).textContent
    ).toContain('Large')

    changeInput(getControlByLabel('Name'), 'org-key')
    fireEvent.click(findButton('Create'))

    await waitFor(() => expect(postedRequests).toHaveLength(1))
    expect(postedRequests[0].url).toBe('/api/organizations/2/tokens')
    expect(postedRequests[0].data).toEqual({ name: 'org-key' })
    expect(postedRequests[0].config).toMatchObject({
      skipAccountContext: true,
      headers: {
        'X-Account-Context-Type': 'organization',
        'X-Account-Context-Id': '2',
      },
    })
  })

  test('blocks personal creation and explains why when the balance is empty', async () => {
    setPersonalQuota(0)
    installApiFixtures({
      organizations: [organizationFixture({ id: 2, name: 'Team' })],
    })
    await renderDrawer()

    // An empty balance defaults to organization, so the form stays usable.
    expect(screen.getByRole('radio', { name: 'Organization' })).toBeChecked()
    expect(findButton('Create').disabled).toBe(false)

    fireEvent.click(screen.getByRole('radio', { name: 'Personal' }))
    expect(
      screen.getByText(
        'Insufficient balance. Please top up or contact an administrator.'
      )
    ).toBeTruthy()
    expect(findButton('Create').disabled).toBe(true)

    fireEvent.click(screen.getByRole('radio', { name: 'Organization' }))
    expect(findButton('Create').disabled).toBe(false)
  })

  test('blocks personal creation when there is no organization to fall back to', async () => {
    setPersonalQuota(0)
    installApiFixtures({ organizations: [] })
    await renderDrawer(undefined, false)

    expect(screen.getByRole('radio', { name: 'Personal' })).toBeChecked()
    expect(
      screen.getByText(
        'Insufficient balance. Please top up or contact an administrator.'
      )
    ).toBeTruthy()
    expect(findButton('Create').disabled).toBe(true)
  })

  test('offers no organization when the caller only manages organizations', async () => {
    setPersonalQuota(1_000_000)
    installApiFixtures({
      organizations: [
        organizationFixture({
          id: 3,
          name: 'Owned',
          role: 'owner',
          capabilities: { can_manage_all_tokens: true },
        }),
      ],
    })
    await renderDrawer()

    expect(findOrganizationRadio()).toBeNull()
    changeInput(getControlByLabel('Name'), 'personal-fallback')
    fireEvent.click(findButton('Create'))

    await waitFor(() => expect(postedRequests).toHaveLength(1))
    expect(postedRequests[0].url).toBe('/api/token/')
  })
})

describe('API keys edit drawer Auto group integration', () => {
  test('preserves an unsaved custom order after Auto to ordinary to Auto changes', async () => {
    installApiFixtures({
      apiKey: {
        id: 1,
        name: 'existing',
        status: 1,
        remain_quota: 0,
        used_quota: 0,
        unlimited_quota: true,
        expired_time: -1,
        created_time: 0,
        accessed_time: 0,
        group: 'auto',
        auto_groups: ['vip'],
        cross_group_retry: true,
        model_limits_enabled: false,
        model_limits: '',
        allow_ips: '',
      },
    })
    await renderDrawer({ id: 1 } as unknown as ApiKey)

    getControlByLabel('Auto group order')
    expect(
      document.body.textContent?.includes('1 / 3 groups selected')
    ).toBe(true)

    const groupTrigger = getControlByLabel('Group')
    selectComboboxOption(groupTrigger, 'Standard access')
    expect(
      document.querySelector('button[aria-label="Remove vip"]')
    ).toBeNull()
    selectComboboxOption(groupTrigger, 'Automatic routing')

    expect(
      document.querySelector('button[aria-label="Remove vip"]')
    ).toBeTruthy()
    expect(
      document.body.textContent?.includes('1 / 3 groups selected')
    ).toBe(true)

    fireEvent.click(findButton('Save changes'))
    await waitFor(() => expect(putRequests).toHaveLength(1))
    expect(putRequests[0].data).toMatchObject({
      id: 1,
      name: 'existing',
      group: 'auto',
      auto_groups: ['vip'],
      cross_group_retry: true,
    })
  })
})
