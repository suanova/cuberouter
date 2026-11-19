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

/** PaymentAsia top-up log (type=1) carrying the optional admin audit snapshot. */
function topupLog(adminInfo: Record<string, unknown>): UsageLog {
  return {
    id: 2,
    user_id: 42,
    created_at: 1_700_000_000,
    type: 1,
    content: 'PaymentAsia top-up succeeded',
    username: 'alice',
    token_name: '',
    model_name: '',
    quota: 49_200_000,
    prompt_tokens: 0,
    completion_tokens: 0,
    use_time: 0,
    is_stream: false,
    channel: 0,
    channel_name: '',
    token_id: 0,
    group: '',
    ip: '203.0.113.10',
    other: JSON.stringify({ admin_info: adminInfo }),
    request_id: '',
    upstream_request_id: '',
    scope_type: '',
    scope_id: 0,
    billing_account_type: '',
    billing_account_id: 0,
    organization_id: 0,
    organization_name: '',
    actor_user_id: 0,
    creator_user_id: 0,
    creator_name: '',
    responsible_user_id: 0,
    responsible_name: '',
    responsible_username: '',
    responsible_display_name: '',
  }
}

function renderTopupLog(adminInfo: Record<string, unknown>) {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <DetailsDialog
        log={topupLog(adminInfo)}
        isAdmin
        isRoot={false}
        open
        onOpenChange={() => undefined}
      />
    </QueryClientProvider>
  )
}

describe('top-up audit service fee fields', () => {
  test('shows the PaymentAsia fee snapshot, including a zero fixed fee', () => {
    renderTopupLog({
      payment_method: 'paymentasia_alipayhk',
      trade_no: 'PA-1',
      currency: 'HKD',
      fee: 1.6,
      fee_rate: 0.016,
      fee_fixed: 0,
      net_amount: 98.4,
      net_quota: 49_200_000,
    })

    expect(screen.getByText('Service Fee').parentElement).toHaveTextContent(
      'HKD 1.6000'
    )
    expect(screen.getByText('Fee Rate').parentElement).toHaveTextContent(
      '1.60%'
    )
    expect(screen.getByText('Fixed Fee').parentElement).toHaveTextContent(
      'HKD 0.0000'
    )
    expect(
      screen.getByText('Net Amount (display units)').parentElement
    ).toHaveTextContent('98.4000')
    expect(screen.getByText('Net Quota').parentElement).toHaveTextContent(
      '49200000'
    )
  })

  test('hides the fee rows for orders without a fee snapshot', () => {
    renderTopupLog({
      payment_method: 'paymentasia_alipayhk',
      trade_no: 'PA-2',
    })

    expect(screen.queryByText('Service Fee')).not.toBeInTheDocument()
    expect(screen.queryByText('Fee Rate')).not.toBeInTheDocument()
    expect(screen.queryByText('Fixed Fee')).not.toBeInTheDocument()
    expect(
      screen.queryByText('Net Amount (display units)')
    ).not.toBeInTheDocument()
    expect(screen.queryByText('Net Quota')).not.toBeInTheDocument()
  })
})
