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
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import {
  createOrganizationJoinRules,
  deleteOrganizationJoinRule,
  listOrganizationJoinRules,
  type OrganizationJoinRule,
} from '@/features/organization/api'

import { PlatformOrganizationJoinRules } from '../platform-organization-join-rules'

const { toastError, toastSuccess } = vi.hoisted(() => ({
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
}))

vi.mock('sonner', () => ({
  toast: { error: toastError, success: toastSuccess },
}))

vi.mock('@/features/organization/api', () => ({
  createOrganizationJoinRules: vi.fn(),
  deleteOrganizationJoinRule: vi.fn(),
  listOrganizationJoinRules: vi.fn(),
}))

const listMock = vi.mocked(listOrganizationJoinRules)
const createMock = vi.mocked(createOrganizationJoinRules)
const deleteMock = vi.mocked(deleteOrganizationJoinRule)

const ORGANIZATION_ID = 7
const EMPTY_STATE =
  'No join rules yet. Nobody joins this organization automatically.'

function rule(overrides: Partial<OrganizationJoinRule> = {}): OrganizationJoinRule {
  return {
    id: 31,
    organization_id: ORGANIZATION_ID,
    match_type: 'domain',
    pattern: '*.enterprise.com',
    pattern_normalized: '*.enterprise.com',
    created_by: 1,
    creator_username: 'root',
    creator_display_name: 'Root',
    created_at: 1_760_000_000,
    updated_at: 1_760_000_000,
    ...overrides,
  }
}

const queryClients: QueryClient[] = []

function renderSection(props: { readOnly?: boolean } = {}): void {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Number.POSITIVE_INFINITY },
    },
  })
  queryClients.push(queryClient)

  render(
    <QueryClientProvider client={queryClient}>
      <PlatformOrganizationJoinRules
        organizationId={ORGANIZATION_ID}
        readOnly={props.readOnly ?? false}
      />
    </QueryClientProvider>
  )
}

/** The two add-form fields are reached by their labels: the placeholder is only
 * a hint, and it disappears as soon as the operator types. */
function patternsField(): HTMLTextAreaElement {
  return screen.getByLabelText('Patterns') as HTMLTextAreaElement
}

function reasonField(): HTMLInputElement {
  return screen.getByLabelText('Reason')
}

function saveButton(): HTMLElement {
  return screen.getByRole('button', { name: 'Save rules' })
}

beforeEach(() => {
  listMock.mockReset()
  createMock.mockReset()
  deleteMock.mockReset()
  toastError.mockReset()
  toastSuccess.mockReset()
  listMock.mockResolvedValue({ success: true, data: [] })
})

afterEach(() => {
  for (const queryClient of queryClients) queryClient.clear()
  queryClients.length = 0
})

describe('platform organization join rules', () => {
  test('says nobody joins automatically when there are no rules', async () => {
    renderSection()

    expect(await screen.findByText(EMPTY_STATE)).toBeInTheDocument()
    expect(listMock).toHaveBeenCalledWith(ORGANIZATION_ID)
  })

  test('previews what each pattern will match before it is saved', async () => {
    renderSection()
    await screen.findByText(EMPTY_STATE)

    fireEvent.input(patternsField(), {
      target: {
        value: '*.enterprise.com\nuser-a@enterprise.com\n*.enterprise.com',
      },
    })

    // The apex on its own reads as "only enterprise.com" to most people, so the
    // preview has to spell out the subdomain the wildcard also swallows. The
    // wildcard is pasted twice and previewed once: the repeated line is the same
    // rule, and the backend keeps only one of them.
    expect(
      screen.getAllByText(
        'Will match enterprise.com and any subdomain, e.g. someone@mail.enterprise.com'
      )
    ).toHaveLength(1)
    expect(
      screen.getByText('Will match only user-a@enterprise.com')
    ).toBeInTheDocument()
  })

  test('sends the pasted lines without blanks, keeping duplicates for the backend', async () => {
    createMock.mockResolvedValue({
      success: true,
      data: { rules: [], notices: [] },
    })
    renderSection()
    await screen.findByText(EMPTY_STATE)

    fireEvent.input(patternsField(), {
      target: {
        value: '\n*.enterprise.com\n\nuser-a@enterprise.com\n*.enterprise.com\n\n',
      },
    })
    fireEvent.input(reasonField(), {
      target: { value: '  Onboarding the enterprise pilot  ' },
    })
    fireEvent.click(saveButton())

    await waitFor(() => expect(createMock).toHaveBeenCalled())

    // Blank lines carry no rule and are dropped; the duplicate is left in — the
    // backend de-duplicates within the batch, and dropping it here would hide
    // the fact that the operator pasted the same line twice.
    expect(createMock).toHaveBeenCalledWith(ORGANIZATION_ID, {
      patterns: ['*.enterprise.com', 'user-a@enterprise.com', '*.enterprise.com'],
      reason: 'Onboarding the enterprise pilot',
    })
  })

  test('reports per-line reasons in place and keeps the pasted lines to fix', async () => {
    createMock.mockRejectedValue({
      response: {
        data: {
          success: false,
          message: 'organization join rule validation failed',
          code: 'organization_join_rule_invalid',
          line_errors: [
            {
              line: 2,
              pattern: '*.enterprise.com',
              kind: 'conflict',
              message: 'pattern conflicts with an existing join rule',
              organization_name: 'Globex Industries',
            },
            {
              line: 3,
              pattern: 'not a domain',
              kind: 'invalid_pattern',
              message: 'invalid join rule pattern',
            },
          ],
          notices: [],
        },
      },
    })
    renderSection()
    await screen.findByText(EMPTY_STATE)

    const pasted = 'user-a@enterprise.com\n*.enterprise.com\nnot a domain'
    fireEvent.input(patternsField(), { target: { value: pasted } })
    fireEvent.input(reasonField(), { target: { value: 'Onboarding' } })
    fireEvent.click(saveButton())

    expect(await screen.findByText('These lines cannot be saved')).toBeInTheDocument()
    expect(
      screen.getByText(/Line 2: \*\.enterprise\.com — It conflicts with a rule of Globex Industries\./)
    ).toBeInTheDocument()
    expect(
      screen.getByText(/Line 3: not a domain — It is not a usable domain or email address\./)
    ).toBeInTheDocument()
    // The batch was refused as a whole, so the request is a failure the operator
    // fixes in place: the lines they pasted are still there to correct.
    expect(patternsField()).toHaveValue(pasted)
    // Line reasons are the message. A generic toast on top of them would name no
    // line and read as a second, unrelated failure.
    expect(toastError).not.toHaveBeenCalled()
  })

  test('localizes the refusal from its stable code when no line is named', async () => {
    // The backend answers with a stable code plus English text, and the copy the
    // operator reads comes from the frontend's own code table — that is how the
    // "email verification is disabled" refusal reaches a Chinese operator in
    // Chinese instead of raw English. It carries its own code rather than the
    // per-line one, because no line is at fault: the fix is a platform setting.
    createMock.mockRejectedValue({
      response: {
        data: {
          success: false,
          message: 'email verification is disabled, join rules will never take effect',
          code: 'organization_join_rule_email_verification_disabled',
        },
      },
    })
    renderSection()
    await screen.findByText(EMPTY_STATE)

    fireEvent.input(patternsField(), { target: { value: '*.enterprise.com' } })
    fireEvent.input(reasonField(), { target: { value: 'Onboarding' } })
    fireEvent.click(saveButton())

    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith(
        'Email verification is disabled, so join rules cannot take effect. A platform administrator has to enable it first.'
      )
    )
    // The backend's English message is not what the operator is shown.
    expect(toastError).not.toHaveBeenCalledWith(
      'email verification is disabled, join rules will never take effect'
    )
  })

  test('falls back to a toast when the refusal names no line', async () => {
    createMock.mockRejectedValue({
      response: {
        data: {
          success: false,
          message: 'unexpected end of JSON input',
        },
      },
    })
    renderSection()
    await screen.findByText(EMPTY_STATE)

    fireEvent.input(patternsField(), { target: { value: '*.enterprise.com' } })
    fireEvent.input(reasonField(), { target: { value: 'Onboarding' } })
    fireEvent.click(saveButton())

    // A refusal with no code from this subsystem has no local copy, so the
    // backend's own message is all there is to show.
    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith('unexpected end of JSON input')
    )
  })

  test('shows the note the backend returned for a batch it accepted', async () => {
    createMock.mockResolvedValue({
      success: true,
      data: {
        rules: [rule()],
        notices: [
          {
            pattern: 'ceo@example.com',
            kind: 'covered_by_other_organization',
            organization_name: 'Globex Industries',
            pattern_conflict: '*.globex.com',
          },
        ],
      },
    })
    renderSection()
    await screen.findByText(EMPTY_STATE)

    fireEvent.input(patternsField(), { target: { value: 'ceo@example.com' } })
    fireEvent.input(reasonField(), { target: { value: 'Executive onboarding' } })
    fireEvent.click(saveButton())

    expect(await screen.findByText('Saved, with a note')).toBeInTheDocument()
    expect(
      screen.getByText(
        'ceo@example.com overlaps Globex Industries rule *.globex.com.'
      )
    ).toBeInTheDocument()
  })

  test('renders the mailbox note with its provider instead of the overlap sentence', async () => {
    createMock.mockResolvedValue({
      success: true,
      data: {
        rules: [rule({ id: 33, pattern: '163.com', pattern_normalized: '163.com' })],
        notices: [
          {
            pattern: '163.com',
            kind: 'public_mailbox_provider',
            organization_name: '',
            pattern_conflict: '163.com',
          },
        ],
      },
    })
    renderSection()
    await screen.findByText(EMPTY_STATE)

    fireEvent.input(patternsField(), { target: { value: '163.com' } })
    fireEvent.input(reasonField(), { target: { value: 'Onboarding' } })
    fireEvent.click(saveButton())

    expect(await screen.findByText('Saved, with a note')).toBeInTheDocument()
    // `kind` is the whole contract between this branch and the backend's notice
    // constant (service/organization_join_rule.go): the two strings are linked by
    // nothing but their text, so a drifted literal would quietly fall through to
    // the overlap sentence — which has no organization to name here and would read
    // as "163.com overlaps rule 163.com.".
    expect(
      screen.getByText(
        '163.com admits anyone who registers with a 163.com mailbox. List exact addresses instead if you only mean specific people.'
      )
    ).toBeInTheDocument()
    expect(screen.queryByText(/overlaps/)).not.toBeInTheDocument()
  })

  test('deletes a rule only once a reason is written', async () => {
    deleteMock.mockResolvedValue({ success: true })
    listMock.mockResolvedValue({ success: true, data: [rule()] })
    renderSection()

    fireEvent.click(await screen.findByRole('button', { name: 'Delete' }))

    const dialog = await screen.findByRole('alertdialog')
    const confirm = within(dialog).getByRole('button', { name: 'Delete' })
    expect(confirm).toBeDisabled()

    fireEvent.input(within(dialog).getByLabelText('Reason'), {
      target: { value: '  Pilot ended  ' },
    })
    expect(confirm).toBeEnabled()
    fireEvent.click(confirm)

    await waitFor(() =>
      expect(deleteMock).toHaveBeenCalledWith(ORGANIZATION_ID, 31, 'Pilot ended')
    )
    await waitFor(() =>
      expect(toastSuccess).toHaveBeenCalledWith('Join rule deleted')
    )
  })

  test('does not carry a cancelled reason over to the next rule', async () => {
    listMock.mockResolvedValue({
      success: true,
      data: [
        rule({ id: 31, pattern: '*.enterprise.com' }),
        rule({ id: 32, pattern: 'user-a@enterprise.com' }),
      ],
    })
    renderSection()

    fireEvent.click(
      (await screen.findAllByRole('button', { name: 'Delete' }))[0]
    )
    const dialog = await screen.findByRole('alertdialog')
    fireEvent.input(within(dialog).getByLabelText('Reason'), {
      target: { value: 'Pilot ended' },
    })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )

    fireEvent.click(screen.getAllByRole('button', { name: 'Delete' })[1])

    // The reason written for the first rule is gone with its dialog: it would
    // otherwise be recorded in the audit trail as the reason the *second* rule
    // was removed.
    const secondDialog = await screen.findByRole('alertdialog')
    expect(within(secondDialog).getByLabelText('Reason')).toHaveValue('')
    expect(
      within(secondDialog).getByRole('button', { name: 'Delete' })
    ).toBeDisabled()
    expect(deleteMock).not.toHaveBeenCalled()
  })

  test('offers no way to change the rules on a read-only organization', async () => {
    listMock.mockResolvedValue({ success: true, data: [rule()] })
    renderSection({ readOnly: true })

    await screen.findByRole('button', { name: 'Delete' })

    // A dissolved organization keeps its rules readable and every control that
    // would change them switched off.
    expect(patternsField()).toBeDisabled()
    expect(reasonField()).toBeDisabled()
    expect(saveButton()).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled()
  })
})
