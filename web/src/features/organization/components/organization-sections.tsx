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
import { useTranslation } from 'react-i18next'

import type { OrganizationDetailTabKey } from '../constants'
import type { OrganizationDetail } from '../types'
import { OrganizationAuditSection } from './organization-audit-section'
import { OrganizationBillingSection } from './organization-billing-section'
import { OrganizationInvitesSection } from './organization-invites-section'
import { OrganizationLogsSection } from './organization-logs-section'
import { OrganizationMembersSection } from './organization-members-section'
import { OrganizationOverviewSection } from './organization-overview-section'
import { OrganizationSectionEmpty } from './organization-section'
import { OrganizationSettingsSection } from './organization-settings-section'
import { OrganizationTasksSection } from './organization-tasks-section'
import { OrganizationTokensSection } from './organization-tokens-section'
import { useOrganizationSurface } from './organization-page-provider'

type OrganizationSectionsProps = {
  detail: OrganizationDetail
  tab: OrganizationDetailTabKey
  /** The caller may write to this organization; see `getOrganizationReadOnlyState`. */
  readOnly: boolean
  /** The backend refused a section read; the page is stale and should leave. */
  onForbidden: () => void
  /** Reload the detail payload after a write that changed it. */
  onUpdated: () => Promise<unknown> | unknown
  /**
   * The caller left the organization of their own accord.
   *
   * Kept separate from `onForbidden` because the two are not the same event:
   * leaving is deliberate and already reported to the caller, while a refusal
   * means access was taken away and has to be explained.
   */
  onLeftOrganization: () => Promise<unknown> | unknown
}

/**
 * The body of the selected section.
 *
 * The tab strip and the section bodies are driven by the same capability set, so
 * a body only ever renders for a caller the backend has already allowed to read
 * it. The sections are switched here rather than routed individually because
 * they share the page's header, its organization payload and its account
 * context; a nested route per section would re-fetch all three.
 *
 * Two of the bodies behave differently for an administrator standing outside the
 * organization, because two things genuinely are different there: the role
 * hierarchy that decides which member rows are editable does not apply to
 * someone with no role, and key creation happens under the caller's own account
 * context, which an administrator does not have here. Both arrive as flags
 * rather than as a second implementation of these sections.
 */
export function OrganizationSections(props: OrganizationSectionsProps) {
  const surface = useOrganizationSurface()
  const { organization, actor } = props.detail
  const capabilities = actor.capabilities
  const isPlatformSurface = surface === 'admin'

  switch (props.tab) {
    case 'overview':
      return (
        <OrganizationOverviewSection
          organization={organization}
          canViewUsage={capabilities.can_view_organization_usage}
          onForbidden={props.onForbidden}
        />
      )
    case 'settings':
      return (
        <OrganizationSettingsSection
          organization={organization}
          canUpdate={capabilities.can_update_organization}
          readOnly={props.readOnly}
          onUpdated={props.onUpdated}
        />
      )
    case 'members':
      return (
        <OrganizationMembersSection
          organizationId={organization.id}
          actorRole={actor.organization_role || actor.role}
          currentUserId={actor.user_id}
          canView={capabilities.can_view_organization}
          canManageMembers={capabilities.can_manage_members}
          canAddMembersDirectly={capabilities.can_add_members_directly}
          canExitOrganization={capabilities.can_exit_organization}
          isPlatformAdministrator={isPlatformSurface && actor.is_platform_admin}
          readOnly={props.readOnly}
          onForbidden={props.onForbidden}
          onLeftOrganization={props.onLeftOrganization}
        />
      )
    case 'invitations':
      return (
        <OrganizationInvitesSection
          organizationId={organization.id}
          canViewInvites={capabilities.can_view_invites}
          canCreateInvites={capabilities.can_create_invites}
          canRevokeInvites={capabilities.can_revoke_invites}
          readOnly={props.readOnly}
          onForbidden={props.onForbidden}
        />
      )
    case 'tokens':
      return (
        <OrganizationTokensSection
          organizationId={organization.id}
          organizationGroup={organization.group ?? ''}
          canView={capabilities.can_view_organization_tokens}
          canManageAllTokens={capabilities.can_manage_all_tokens}
          canCreate={!isPlatformSurface}
          currentUserId={actor.user_id}
          isOrganizationMember={actor.is_organization_member}
          readOnly={props.readOnly}
          onForbidden={props.onForbidden}
        />
      )
    case 'logs':
      return (
        <OrganizationLogsSection
          organizationId={organization.id}
          canView={capabilities.can_view_organization_logs}
          canViewWideData={capabilities.can_view_organization_wide_data}
          onForbidden={props.onForbidden}
        />
      )
    case 'usage':
      return (
        <OrganizationBillingSection
          organizationId={organization.id}
          canView={capabilities.can_view_organization_usage}
          canViewWideData={capabilities.can_view_organization_wide_data}
          onForbidden={props.onForbidden}
        />
      )
    case 'tasks':
      return (
        <OrganizationTasksSection
          organizationId={organization.id}
          canView={capabilities.can_view_organization_logs}
          canViewWideData={capabilities.can_view_organization_wide_data}
          onForbidden={props.onForbidden}
        />
      )
    case 'audit-logs':
      return (
        <OrganizationAuditSection
          organizationId={organization.id}
          canViewAudit={capabilities.can_view_audit}
          onForbidden={props.onForbidden}
        />
      )
    default:
      return <OrganizationSectionPlaceholder tab={props.tab} />
  }
}

/**
 * A section that has not been ported yet.
 *
 * It renders through the same shell as the real sections so the page keeps its
 * shape, and says plainly that it is unfinished rather than showing an empty
 * list that reads as "there is no data".
 */
function OrganizationSectionPlaceholder({
  tab,
}: {
  tab: OrganizationDetailTabKey
}) {
  const { t } = useTranslation()
  return (
    <OrganizationSectionEmpty
      icon={tab}
      title={t('Organization')}
      message={t('This section is not available yet.')}
    />
  )
}
