/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { OrganizationTokenFormDialog } from '../organization-token-form-dialog'

vi.mock(
  '@/features/organization/hooks/use-organization-member-options',
  () => ({
    useOrganizationTokenResponsibleOptions: () => ({
      options: [{ value: 42, label: 'alice' }],
      isLoading: false,
    }),
  })
)

/**
 * The text of the dialog's pickers.
 *
 * A select resolves its trigger text from the items its root is given, not from
 * what it renders once the popup opens. Left without them the trigger falls
 * back to the raw value, which is how the holder used to read as `42` and an
 * untouched token group as the sentinel word `none` — neither of which is a
 * thing an operator can act on.
 *
 * The trigger is found through its label, the way a person reaches the field,
 * so the assertions stay tied to what the form actually shows.
 */
function triggerText(labelText: string): string {
  const label = [...document.querySelectorAll<HTMLLabelElement>('label')].find(
    (candidate) => candidate.textContent?.trim() === labelText
  )
  if (!label) {
    throw new Error(`Expected label "${labelText}"`)
  }

  const trigger = label.control ?? null
  if (!trigger) {
    throw new Error(`Expected a control for label "${labelText}"`)
  }
  return trigger.textContent ?? ''
}

function renderCreateDialog(): void {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <OrganizationTokenFormDialog
        organizationId={7}
        open
        onOpenChange={() => undefined}
        editing={null}
        canManageAllTokens
        currentUserId={42}
        isOrganizationMember
        groupOptions={[{ value: 'vip', label: 'VIP', desc: 'Priority access' }]}
        onSaved={() => undefined}
      />
    </QueryClientProvider>
  )
}

test('names the responsible member instead of showing their user id', () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: [] },
  } as never)

  renderCreateDialog()

  expect(triggerText('Responsible user')).toBe('alice')
  expect(triggerText('Responsible user')).not.toContain('42')
})

test("words an empty token group as the organization's group, not its sentinel", () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: [] },
  } as never)

  renderCreateDialog()

  expect(triggerText('Token group')).toBe("The organization's group")
  expect(triggerText('Token group')).not.toBe('none')
})
