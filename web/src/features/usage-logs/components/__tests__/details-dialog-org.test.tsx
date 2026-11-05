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
import { render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import type { UsageLog } from '../../data/schema'
import { DetailsDialog } from '../dialogs/details-dialog'

vi.mock('@/lib/lobe-icon', () => ({
  getLobeIcon: () => null,
}))

function organizationLog(): UsageLog {
  return {
    id: 1,
    user_id: 42,
    created_at: 1_700_000_000,
    type: 2,
    content: '',
    username: 'alice',
    token_name: 'key-1',
    model_name: 'gpt-4o',
    quota: 1_000,
    prompt_tokens: 10,
    completion_tokens: 5,
    use_time: 1,
    is_stream: false,
    channel: 1,
    channel_name: '',
    token_id: 1,
    group: 'vip-group',
    ip: '203.0.113.10',
    other: JSON.stringify({
      request_path: '/v1/chat/completions',
      billing_mode: 'token',
      model_ratio: 1,
      group_ratio: 1,
    }),
    request_id: 'request-1',
    upstream_request_id: 'upstream-1',
    scope_type: 'organization',
    scope_id: 9,
    billing_account_type: 'organization',
    billing_account_id: 9,
    organization_id: 9,
    organization_name: 'Acme',
    actor_user_id: 42,
    creator_user_id: 42,
    creator_name: 'alice',
    responsible_user_id: 7,
    responsible_name: 'bob',
    responsible_username: 'bob',
    responsible_display_name: '',
  }
}

describe('organization details dialog', () => {
  test('shows request and responsible fields but hides the internal group', () => {
    render(
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <DetailsDialog
          log={organizationLog()}
          isAdmin={false}
          isRoot={false}
          showResponsible
          showRequestDetails
          showGroup={false}
          open
          onOpenChange={() => undefined}
        />
      </QueryClientProvider>
    )

    expect(screen.getByText('request-1')).toBeInTheDocument()
    expect(screen.getByText('/v1/chat/completions')).toBeInTheDocument()
    expect(screen.getByText('bob')).toBeInTheDocument()
    expect(screen.queryByText('Group')).not.toBeInTheDocument()
    expect(screen.queryByText('vip-group')).not.toBeInTheDocument()
  })
})
