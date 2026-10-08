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
import { render, screen } from '@testing-library/react'
import { afterAll, beforeAll, expect, test, vi } from 'vitest'

// The deployment prefix is resolved once, when the module graph loads, so it has
// to be set before the component is pulled in -- which is why it is imported
// dynamically below rather than at the top of the file.
beforeAll(() => {
  window.__BASE_PATH__ = '/cuberouter'
})

afterAll(() => {
  delete window.__BASE_PATH__
})

// Regression: these were plain <a href> links opened in a new tab, and the
// browser resolves a leading "/" against the origin root. Under BASE_PATH the
// new tab left the deployment and landed on the SPA's not-found route.
test('opens the legal documents under the deployment prefix', async () => {
  const { LegalConsent } = await import('../legal-consent')

  render(
    <LegalConsent
      status={{ user_agreement_enabled: true, privacy_policy_enabled: true }}
      checked={false}
      onCheckedChange={vi.fn()}
    />
  )

  expect(screen.getByRole('link', { name: 'User Agreement' })).toHaveAttribute(
    'href',
    '/cuberouter/user-agreement'
  )
  expect(screen.getByRole('link', { name: 'Privacy Policy' })).toHaveAttribute(
    'href',
    '/cuberouter/privacy-policy'
  )
})
