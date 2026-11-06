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
import { useNavigate } from '@tanstack/react-router'
import i18n from 'i18next'

import {
  getSavedLanguage,
  sanitizeAuthRedirect,
} from '@/features/auth/lib/auth-redirect'
import {
  hasAdministrableOrganization,
  refreshAccountContexts,
} from '@/lib/account-context'
import { applyAuthBundle } from '@/lib/api'
import type { AuthBundle } from '@/stores/auth-store'

/**
 * Hook for handling authentication redirects and user data management
 */
export function useAuthRedirect() {
  const navigate = useNavigate()

  /**
   * Handle successful login
   * @param userData - Optional user data from login response
   * @param redirectTo - Redirect path after login
   */
  const handleLoginSuccess = async (
    bundle: AuthBundle,
    redirectTo?: string
  ) => {
    applyAuthBundle(bundle)
    const savedLang = getSavedLanguage(bundle.user)
    if (savedLang && savedLang !== i18n.language) {
      await i18n.changeLanguage(savedLang)
    }

    // An organization owner/administrator lands on the organization center so
    // the sidebar's Organizations entry is the focused one. The account context
    // is deliberately untouched: this is a landing page, not a workspace switch.
    let focusOrganizations = false
    try {
      const contexts = await refreshAccountContexts()
      focusOrganizations = hasAdministrableOrganization(contexts.contexts)
    } catch {
      // The landing choice is an enhancement; a failed context read must not
      // block login.
    }

    const targetPath =
      sanitizeAuthRedirect(redirectTo, window.location.origin) ??
      (focusOrganizations ? '/organizations' : '/dashboard')
    navigate({ href: targetPath, replace: true })
  }

  /**
   * Redirect to 2FA page
   */
  const redirectTo2FA = () => {
    navigate({ to: '/otp', replace: true })
  }

  /**
   * Redirect to login page
   */
  const redirectToLogin = () => {
    navigate({ to: '/sign-in', replace: true })
  }

  /**
   * Redirect to register page
   */
  const redirectToRegister = () => {
    navigate({ to: '/sign-up', replace: true })
  }

  return {
    handleLoginSuccess,
    redirectTo2FA,
    redirectToLogin,
    redirectToRegister,
  }
}
