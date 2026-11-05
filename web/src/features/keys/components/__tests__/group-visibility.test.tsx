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
import { afterEach, describe, expect, test } from 'vitest'

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

type ApiMethod = (url: string, data?: unknown) => Promise<{ data: unknown }>
type MockableApi = {
  get: ApiMethod
  post: ApiMethod
}
type RenderedDrawer = {
  queryClient: InstanceType<typeof QueryClient>
}

const apiClient = api as unknown as MockableApi
const originalGet = apiClient.get
const originalPost = apiClient.post
let renderedDrawer: RenderedDrawer | null = null

// The Auto default is on and the backend offers an Auto group, so the drawer's
// form value really is "auto". That is the case where the hidden controls would
// otherwise render, which is what makes it the interesting fixture.
function installApiFixtures(createdPayloads: Array<Record<string, unknown>>) {
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
            },
          },
        }
      case '/api/token/auto-groups':
        return {
          data: { success: true, data: { groups: ['default'], max_count: 3 } },
        }
      case '/api/user/self':
        return {
          data: {
            success: true,
            data: useAuthStore.getState().auth.user,
          },
        }
      case '/api/organizations':
        return { data: { success: true, data: [] } }
      default:
        throw new Error(`Unexpected GET ${url}`)
    }
  }
  apiClient.post = async (url, data) => {
    expect(url).toBe('/api/token/')
    createdPayloads.push(data as Record<string, unknown>)
    return { data: { success: true, data: {} } }
  }
}

async function renderCreateDrawer(): Promise<void> {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'tester',
    role: 1,
    quota: 1_000_000,
  })
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const freshAt = Date.now() + 60_000
  queryClient.setQueryData(
    ['status'],
    { default_use_auto_group: true },
    { updatedAt: freshAt }
  )
  queryClient.setQueryData(
    ['user-models'],
    { success: true, data: [] },
    { updatedAt: freshAt }
  )
  queryClient.setQueryData(
    ['user-groups'],
    {
      success: true,
      data: {
        auto: { desc: 'Automatic routing', ratio: 'auto' },
        default: { desc: 'Standard access', ratio: 1 },
      },
    },
    { updatedAt: freshAt }
  )
  queryClient.setQueryData(
    ['token-auto-groups'],
    { success: true, data: { groups: ['default'], max_count: 3 } },
    { updatedAt: freshAt }
  )
  renderedDrawer = { queryClient }

  render(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <ApiKeysProvider>
          <ApiKeysMutateDrawer open onOpenChange={() => undefined} />
        </ApiKeysProvider>
      </I18nextProvider>
    </QueryClientProvider>
  )
  await waitFor(
    () => {
      expect(findButton('Create')).toBeEnabled()
    },
    { timeout: 1500 }
  )
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

function hasLabel(labelText: string): boolean {
  return [...document.querySelectorAll('label')].some(
    (label) => label.textContent?.trim() === labelText
  )
}

function getControlByLabel(labelText: string): HTMLInputElement {
  const label = [...document.querySelectorAll<HTMLLabelElement>('label')].find(
    (candidate) => candidate.textContent?.trim() === labelText
  )
  const control = label?.control
  if (!control) {
    throw new Error(`Expected input for label "${labelText}"`)
  }
  return control as HTMLInputElement
}

afterEach(() => {
  apiClient.get = originalGet
  apiClient.post = originalPost
  useAuthStore.getState().auth.setUser(null)
  localStorage.clear()
  if (renderedDrawer) {
    renderedDrawer.queryClient.clear()
    renderedDrawer = null
  }
})

describe('API key drawer group visibility', () => {
  test('hides the group picker and the Auto-only controls even when the default group is Auto', async () => {
    installApiFixtures([])
    await renderCreateDrawer()

    expect(hasLabel('Group')).toBe(false)
    expect(hasLabel('Auto group order')).toBe(false)
    expect(hasLabel('Cross-group retry')).toBe(false)
    expect(document.body.textContent?.includes('Select a group')).toBe(false)
  })

  test('still submits the default group when the picker is hidden', async () => {
    const createdPayloads: Array<Record<string, unknown>> = []
    installApiFixtures(createdPayloads)
    await renderCreateDrawer()

    fireEvent.input(getControlByLabel('Name'), {
      target: { value: 'hidden-group' },
    })
    fireEvent.click(findButton('Create'))
    await waitFor(() => expect(createdPayloads).toHaveLength(1))

    // The untouched default survives: the key keeps following the configured
    // Auto default exactly as it did when the picker was on screen.
    expect(createdPayloads[0]?.name).toBe('hidden-group')
    expect(createdPayloads[0]?.group).toBe('auto')
    expect(createdPayloads[0]?.cross_group_retry).toBe(true)
  })
})
