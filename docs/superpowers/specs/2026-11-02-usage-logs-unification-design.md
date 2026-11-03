# Usage Logs Unification — Design

**Date:** 2026-11-02
**Status:** Implemented — root/admin, personal page, organization Logs, and task logs

## Goal

Merge "Usage Logs" (`/usage-logs`) and "Organization → Logs" into one page whose
data scope is chosen explicitly. The page must show each caller exactly the rows
they are allowed to read, with scope-aware columns and details, and without
duplicating the two current implementations.

## Background

The two pages read the same `logs` table but different billing scopes:

- Usage Logs / All: `GET /api/log/` (`OpsAuth`) — every log, personal and
  organization-billed.
- Usage Logs / Only Mine: `GET /api/log/self` — personal scope only
  (`scope_type=personal`, `user_id=me`).
- Organization → Logs: `GET /api/organizations/:id/logs` — organization billing
  account only; members without wide data are pinned to
  `responsible_user_id = self`.

Platform roles and organization roles are orthogonal. Personal and organization
billing are mutually exclusive per request; a row appears in exactly one of them.

## Role → scope model

| Caller | Unified-page scopes | Row range |
|---|---|---|
| root (100) | all, personal | all logs; `root_info` kept |
| admin (10) | all, personal | all logs; `root_info` stripped |
| ops (5) | all (common only), personal | all common logs; drawing/task self only |
| org owner/admin | personal (plus the organization detail Logs tab) | personal page: personal + own responsible org rows; org tab: all members (wide data) |
| org member | personal | personal + own responsible org rows |
| user | personal | personal + own responsible org rows |

- `all` includes both personal and organization-billed rows.
- Organization-wide logs are read through the organization detail Logs tab, not
  through a scope in the unified page. The personal page's organization filter
  narrows to the caller's own responsible rows only.
- A regular member explicitly sees only their own responsible rows in an
  organization (decided 2026-11-02; privacy first). This is already enforced by
  the backend.
- Ops is an operations role (read-only, invitee-scoped on ops pages). It is not
  an organization administrator.

## Ops role (confirmed 2026-11-02)

Ops keeps its current behavior; the unified page does not need a code change
for this role beyond the root/admin work already implemented.

- **Scopes:** `all` (common logs only) and `personal`. On common logs ops is the
  `admin` view tier (`isAdminView=true`), so All is the default.
- **Drawing/task logs:** pinned to `self` because those read endpoints are
  `AdminAuth`; the All / Only Mine switch is hidden there.
- **Columns and filters:** ops sees the same common-log columns as the admin
  view, including Channel, User, Billing Account, and Responsible user, and can
  use the username, channel ID, billing type, and organization ID filters. This
  is intentional: `/api/log/` and `/api/log/stat` are `OpsAuth` and already
  return the full list.
- **Details:** platform admin diagnostics are visible (`admin_info`), but
  `root_info` is stripped because ops is not root.
- **Organization logs:** a platform ops role does not grant access to an
  organization's full log records. If the ops user is also an organization
  owner/admin, the organization detail Logs tab applies; the unified page has
  no organization-wide scope. The personal page's organization filter only
  narrows to the ops user's own responsible rows.
- **Write access:** none. Ops remains read-only on logs.
- The frontend tier is locked by `web/src/features/usage-logs/__tests__/access.test.ts`;
  the backend endpoints are already `OpsAuth`, so the billing filters added in
  the root/admin phase apply to ops without extra changes.

## Personal page (all users)

This is the Only Mine view for platform roles and the only logs view for
ordinary users. Organization role does not change it: an organization
owner/admin sees exactly the same personal page as a member; the organization's
full log remains in the organization detail Logs tab.

- **Data:** personal-billed rows (`user_id=me`, personal scope) plus
  organization-billed rows whose `responsible_user_id` is the caller. This is a
  backend union, not a client-side merge of two paginated lists.
- **Columns:** Time (+ log-type badge), Billing Account (personal / organization
  name + ID), Token, Model, Stream, Tokens, Cost, Timing, Details. No Channel,
  User, or Responsible column.
- **Stats:** personal spend, organization spend, and RPM/TPM. Organization spend
  never rolls into the personal quota figure.
- **Filters**, in date → select → input order:
  1. date range
  2. log type (select)
  3. billing account: all / personal / organization (select)
  4. organization: one of the caller's organizations; filters to the caller's
     own rows in it, never to the organization's full log (select)
  5. Model (input)
  6. Token Name (input)
  7. Request ID (input)
  8. Upstream Request ID (input)

  There is no Group filter on this page. The platform All page keeps it. There
  are no Username, Channel ID, Organization ID, or Responsible filters.
- **Details:** the shared dialog with platform diagnostics off
  (`isAdmin=false`). Show Billing Account; keep the request content for the
  caller's own organization-billed requests; no platform diagnostics.
- **Backend (implemented):** `/api/log/self` and `/api/log/self/stat` accept
  `include_organization=true`, `billing_account_type`, and `organization_id`
  (the latter always scoped to `responsible_user_id=me`); stats return split
  personal/organization quota.
- **Organization tab (implemented):** the organization detail Logs tab is
  hidden for plain members. Owner/admin keeps it for the organization-wide view.

## Organization Logs tab (implemented 2026-11-02)

The organization detail Logs tab remains the organization-wide view for
owner/admin; plain members no longer see it.

- **Details:** the organization-specific dialog was removed. The tab now uses
  the shared `DetailsDialog` with `isAdmin=false`, `showResponsible` for wide
  data, `showRequestDetails` (path, model, time, stream), and `showGroup=false`.
  This adds the shared billing/token/subscription breakdown without exposing
  platform diagnostics or the internal group.
- **Filters:** log type, responsible member (a picker backed by the member
  roster), upstream request ID, plus the existing time/model/key/request filters.
  Group remains deliberately absent.
- **Columns:** caller (`username`) added next to the responsible user; the
  caller and the billing responsible person can differ.
- **Presentation:** mobile card list (shared layout), error/refund row tints,
  and a logs-specific default page size of 50.
- **Backend:** `OrganizationLogListRequest` accepts `upstream_request_id`; the
  existing type/responsible filters are used by the UI.

## Task logs (implemented 2026-11-03)

Task logs follow the same role model as common logs, applied to the `tasks`
table.

- **Personal task page:** personal-billed tasks plus organization-billed tasks
  whose `responsible_user_id` is the caller (`include_organization=true` on
  `/api/task/self`). Columns add Billing Account, Token, and Cost. Filters:
  date, status, action/type, billing account, organization (self), plus task ID
  and channel/organization ID for the platform All view.
- **Platform All:** `/api/task` gains `billing_account_type` / `organization_id`
  filters; the admin table adds Billing Account and Responsible user columns.
  Ops remains self-only because the task read endpoint is `AdminAuth`.
- **Organization Tasks tab:** owner/admin keeps the organization-wide view;
  plain members no longer see the tab. The tab is aligned with the task page's
  shared columns/details, adds a platform filter and mobile layout, and its
  Excel export is removed.
- **Details:** the shared task details dialog shows Cost and Billing Account for
  all viewers; platform-only fields remain behind the admin section.
- **Backend:** task DTOs expose token/billing/organization/responsible fields;
  `userAsyncTaskScopeQuery` implements the union and `applyAsyncTaskBillingFilter`
  handles the billing filters. Task statistics are deferred.
- **Out of scope:** task statistics, server-side sorting, and Midjourney/drawing
  logs (the platform has no Midjourney upstream).

## Root/admin phase (this change)

### In scope

1. Keep the existing All / Only Mine toggle as the scope switch; All is the
   default for ops and above.
2. Make organization billing visible in the All table:
   - New column **Billing Account**: `Personal` or the organization name + ID.
   - New column **Responsible user**: the log's responsible user.
   - The existing **User** column keeps its meaning: the calling user. A caller
     and the billing responsible user may differ for organization keys.
   - No links from either new column.
3. Extend the shared log details dialog with **Billing Account** and
   **Responsible user** rows for platform views.
4. Remove the log export entry (the Export button and its now-unused
   implementation). Log pages keep no export option.
5. Add admin-wide billing filters: billing type (personal / organization) and
   organization ID. Both apply to the list and to the stats bar. Legacy rows
   with an empty `billing_account_type` count as personal.
6. Leave drawing/task logs untouched. They use separate tables and endpoints
   (`/api/mj`, `/api/task`) and are not part of the organization log scope.

### Out of scope (next phases)

- Server-side sorting on the organization log table.
- A direct-route guard for members who bookmark the organization Logs tab; the
  tab itself is hidden for them.

## Decisions

- The page is one page with a scope switch, not a merged stream of personal and
  organization rows.
- No export from logs.
- No click-through to organization details from log rows.
- Root and admin share the same page; the only difference remains `root_info`
  visibility in details.
- Ops keeps read-only access to all common logs and the personal scope, with the
  admin-view columns/filters; drawing/task stay pinned to self. No change. This
  was confirmed on 2026-11-02.
- There is no organization-wide scope in the unified page. Organization-wide
  logs stay in the organization detail Logs tab. The personal page's
  organization filter only narrows to the caller's own rows.
- The personal page is identical for organization owners/admins and ordinary
  members; organization role only controls whether the organization detail Logs
  tab is visible.
- Details use one dialog implementation. Organization-scope reads will later
  force platform diagnostics off (`isAdmin=false`), preserving the current
  organization dialog boundary.

## Implementation notes (this phase)

- `web/src/features/usage-logs/data/schema.ts`: add the billing/responsible
  fields already returned by the API.
- `web/src/features/usage-logs/lib/format.ts`: helpers that normalize legacy
  empty billing types to personal and resolve the responsible display name.
- `common-logs-columns.tsx`: add the two admin-only columns.
- `details-dialog.tsx`: add the two rows for platform views.
- `common-logs-filter-bar.tsx`: admin billing-type and organization-ID filters,
  carried through the URL search state.
- `controller/log.go` and `model/log.go`: `billing_account_type` /
  `organization_id` filters on `GET /api/log/` and `/api/log/stat`.
- Remove `common-logs-export-button.tsx`, `lib/logs-export.ts`, and the export
  button tests; drop the Export button from the filter bar.
- `model/log.go`: `userLogScopeQuery` (personal + responsible organization
  rows), `GetUserLogs`/`GetUserLogStats` union and split stats; `controller/log.go`
  reads `include_organization`, `billing_account_type`, `organization_id`.
- `common-logs-columns.tsx`: Billing Account is visible in the personal page;
  Channel/User/Responsible remain admin-only.
- `common-logs-filter-bar.tsx`: billing account select for everyone; organization
  select from `GET /api/organizations` for the personal page; Group input only on
  the platform All page.
- `common-logs-stats.tsx`: Personal / Organization / RPM / TPM on the personal
  page; Usage / RPM / TPM on All.
- `organization-center.ts`: the Logs tab requires wide-data access, so plain
  members no longer see it.
- `organization-logs-section.tsx`: shared details dialog, type/upstream/responsible
  filters, caller column, mobile list, row tints, logs page size.
- `organization-log.ts`: `organizationLogToUsageLog` adapts an organization row
  to the shared dialog shape.
- `use-organization-member-options.ts`: member filter options for the
  responsible picker.
- `service/organization_log.go` + `controller/organization_log.go`:
  `upstream_request_id` filter.

## Verification

- `bun run typecheck`, oxlint on changed files, `bun run build`.
- Unit tests for the billing/responsible helpers (legacy empty type,
  organization name fallback, responsible fallback).
- Go test for admin billing filters, including legacy empty types and stats.
- Go test for the personal union: responsible organization rows, caller vs
  responsible, legacy personal rows, filters, and split stats.
- Frontend test for self-view params: `include_organization`, billing filters,
  and the admin-wide endpoint not asking for a union.
- Organization tests: log section filters (type/upstream/responsible, no group),
  shared details dialog org view, and the organization-row mapper.
- Manual check with the local SQLite test site as root and admin: All view shows
  organization rows labeled; Only Mine shows personal plus own responsible
  organization rows; details show billing/responsible fields; no Export button
  appears.
