# Organization System (组织管理)

> Last Updated: 2026-09-20
> Ported from `searouter-isuanova` branch `feature/organization-management` — see
> [ORGANIZATION_PORTING_NOTES.md](ORGANIZATION_PORTING_NOTES.md) for the divergences,
> [ORGANIZATION_TECHNICAL_DESIGN.md](ORGANIZATION_TECHNICAL_DESIGN.md) for the full design and
> [ORGANIZATION_PRD.md](ORGANIZATION_PRD.md) for the acceptance criteria.

## 1. Feature Description

An **organization** is a shared account: members use *organization API keys*, and the quota those
keys consume is charged to the organization's pool, not to the caller's personal wallet. It is a
collaboration boundary, a permission boundary and a billing boundary at the same time.

A user always acts in exactly one **account context** — `personal` or `organization:<id>` — and that
context decides which keys exist, which quota is spent, what the dashboards show and what the
pricing group resolves to. Personal behaviour is unchanged when the context is `personal`.

### Lifecycle

| `Organization.Status` | Meaning |
|---|---|
| `active` | Normal operation. |
| `disabled` | Suspended by the platform (or by itself). Members keep their data; organization keys stop working. |
| `dissolved` | Terminal. Kicked off by a member, or by a platform admin; the slug and name stay reserved. |

### Roles inside an organization

| `OrganizationMember.Role` | Meaning |
|---|---|
| `owner` | Exactly one per organization. Can transfer ownership and dissolve. |
| `admin` | Manages members, invitations, keys and settings. |
| `member` | Uses the organization's keys; sees its usage within the limits of its access mode. |

`OrganizationMember.Status` is `active`, `disabled`, `exited` or `removed`. A member disabled by the
organization has `DisabledSource = organization`; one disabled by a platform admin has
`DisabledSource = platform`. The distinction matters because re-enabling is only allowed for the
source that disabled them.

### Invitations

`organization_invitations` rows are `pending` → `accepted`, or `expired` / `revoked`. Only the email
address the invitation was sent to may accept it, and acceptance is what creates the
`OrganizationMember` row. `force_rotate` re-sends an invitation whose delivery previously failed.
Invitation tokens are stored hashed (`HashOrganizationInviteToken`) — the raw token exists only in
the email.

### Access modes

Every organization request is evaluated under one access mode, which is what decides the capability
set:

| Mode | Used by | Notes |
|---|---|---|
| `workspace` | Members acting inside their own organization. | Members see their own usage in full and the rest of the organization's usage in aggregate. |
| `management` | Owner/admin self-service writes (`PATCH /:id`, member and key management). | Requires the corresponding capability on the caller's own role. |
| `read_only` | Read paths that must not expose member-level detail. | |
| `admin` | Platform admins under `/api/admin/organizations`. | The `platform_admin` / `platform_root` policy roles. Platform admins are *not* members and do not need a membership row. |

### Capabilities

`service/organization_policy.go` defines 20 capability strings (`view_organization`,
`manage_members`, `adjust_organization_quota`, `view_organization_audit_logs`, …). A request is
allowed when its access mode grants the required capabilities; `GetOrganizationPolicyDecisionForUser`
returns a decision object carrying both `Allowed` and the granted capability set, and the middleware
checks each capability the route declared. Refusals come back as
`{"success":false,"code":"organization_access_denied",…}` (or
`organization_operation_blocked` when the refusal has named blockers).

### Organization API keys

Keys live in the existing `tokens` table with the scope columns populated
(`scope_type=organization`, `scope_id=<org id>`, `organization_id`, `responsible_user_id`,
`creator_user_id`, `visibility`). A key may have a **responsible user** — the member whose name the
usage is attributed to — and responsibility can be moved with
`PATCH /:id/tokens/:tokenId/responsible-user`; the previous responsible user's keys are re-checked
when a membership ends.

Because an organization key must stop working the moment its owner, its responsible user or the
organization itself is disabled, there is a separate **system blocker** table
(`organization_token_system_blockers`). `ReconcileOrganizationTokenBlockers` re-derives the blockers
from the current state of organizations, memberships and users, and restores each key's previous
status when the last blocker clears. A blocker carries a clearance level, so a key disabled for a
platform-level reason is not re-enabled just because an organization-level reason went away.

### Billing

Organization usage runs through cuberouter's existing `BillingSession`, not a parallel system:
`service/organization_funding.go` implements the `FundingSource` interface (`Source` / `PreConsume` /
`Settle` / `Refund`), selected by `NewBillingSession` when the request carries an organization scope.
`Source()` returns `BillingSourceOrganization`, so the wallet and subscription sources are never
touched on that path.

Around it sits an organization ledger:

- `organization_billing_sessions` — one row per `request_id`, holding the pre-consumed amount, the
  relay lease and a status (`pre_consumed` → `settled` / `refunded`, or `repairing` / `failed`).
- `organization_billing_records` — the append-only rows (`pre_consume`, `settle`, `refund`,
  `adjustment`) that the member / monthly / detail reports are built from.
- `StartOrganizationBillingSessionHeartbeat` renews the session lease while a stream is open
  (interval = half the TTL); `StartOrganizationBillingRepairTask` sweeps every minute for sessions
  whose lease expired — a crashed node's pre-consumed quota is settled or refunded rather than
  leaking.

Every billing event also carries a `logs.billing_event_key`; its unique index makes "one ledger row
per event" exactly-once even when a repair pass races a late settle.

Personal quota, subscription and wallet are never moved on an organization request, and organization
spend is excluded from the personal dashboards, logs and task lists.

---

## 2. Related Code and Code Logic

### Data model (`model/`)

Eleven new tables:

| File | Tables |
|---|---|
| `organization.go` | `organizations` |
| `organization_member.go` | `organization_members` |
| `organization_invite.go` | `organization_invitations` (explicit `TableName()`) |
| `organization_audit_log.go` | `organization_audit_logs` |
| `organization_quota_adjustment.go` | `organization_quota_adjustments` |
| `organization_foundation.go` | `user_account_contexts`, `organization_disable_records`, `organization_token_system_blockers`, `organization_idempotency_records`, `organization_billing_sessions`, `organization_billing_records` |

Plus scope columns and scope-aware queries on `tokens`, `tasks`, `midjourneys`, `quota_data` and
`logs`, and `model/async_task_scope.go` (the shared scope predicate for task/Midjourney lookups).

Migrations (`model/main.go`, called from both `migrateDB` and `migrateDBFast`):

| Migration | Purpose |
|---|---|
| `organization_scope_migration.go` | Adds the scope/billing columns and backfills legacy rows to personal scope in batches. Runs **before** `AutoMigrate`, so the columns exist by the time the rows are written. |
| `organization_name_migration.go` | Backfills `name_normalized` and enforces name uniqueness (MySQL `utf8mb4_bin`). |
| `organization_member_migration.go` | Backfills `disabled_source` from the audit trail. |
| `prepareLogBillingEventKeyMigration` | `logs.billing_event_key` and its unique index. |
| `ensureOrganizationBillingIndexes` | The multi-column indexes AutoMigrate will not create — including `idx_quota_data_account_context`, which merges legacy duplicate rows first. |

All of them are idempotent and cheap on the second run: each has a marker `Option` row, an
existence fast path, and a `lockForUpdate` re-check.

### Auth flow (`middleware/`)

1. `UserAuth()` — session / dashboard authentication, as for every other `/api/*` route.
2. `middleware/account_context.go` — reads `X-Account-Context-Type` and `X-Account-Context-Id` (or
   falls back to the user's stored context), resolves it through
   `service.ResolveCurrentAccountContext`, writes it into the gin context, and rejects a request
   whose header contradicts the `:id` in the path with
   `403 organization_context_mismatch`. A context that no longer exists falls back to personal.
3. `middleware/organization_management.go` — five middlewares over one policy core:
   `OrganizationManagementAuth` (`management`), `OrganizationAccountContextAuth` (`workspace`, also
   verifies the path id matches the context), `OrganizationReadOnlyAuth` (`read_only`),
   `OrganizationAdminAuth` (`admin`) and `OrganizationReadAccessAuth` (reads that must work for a
   caller with no writable role).

`middleware/auth.go`'s `TokenAuth` gained the relay-side counterpart: for a token carrying an
organization scope it calls `service.ValidateTokenScopeForRelay`, which checks that the
organization, its membership and the responsible user are all still active, and splits
`accountGroup` (which pricing group applies) from `usingGroup` (which group the key was issued for).
`OpsAuth` is untouched.

### Policy engine (`service/organization_policy.go`)

Pure functions over the caller's relation to the organization — no HTTP, no gin. They resolve the
role (`owner` / `admin` / `member`, or `platform_admin` / `platform_root`), select the capability set
for the access mode, and report a decision. `service/organization_access.go` resolves the less
frequently asked questions (who is the owner, is this member active, which organizations does this
user belong to).

`organization_disable_records` records why an organization was disabled; `loadOrganizationPolicyDisableState`
reads it, which is how a disabled organization makes every request under it fail with
`organization_disabled` instead of silently falling back to personal.

### Other services

| File | Responsibility |
|---|---|
| `organization.go`, `_detail`, `_management`, `_data`, `_group` | Lifecycle, detail view, owner transfer / dissolve / status changes, dashboards, pricing group. |
| `organization_member.go` | Membership changes, exit, key hand-over on exit. |
| `organization_invite.go` | Invitation creation, delivery, acceptance, revocation. |
| `organization_token.go` | Organization key CRUD and batching. |
| `organization_token_auth.go`, `organization_token_blocker.go` | Relay-time scope validation; the multi-source force-disable engine. |
| `organization_idempotency.go` | `Idempotency-Key` handling for the write endpoints. |
| `organization_audit.go`, `organization_audit_query.go` | Audit writes and the audit query surface. |
| `organization_billing.go`, `_summary`, `_heartbeat`, `_repair_task` | The ledger and its workers. |
| `account_context.go` | List / set / resolve the caller's account context. |
| `quota.go`, `text_quota.go`, `midjourney.go`, `violation_fee.go` | Organization branches of the existing consumption paths. |

### Routes (`router/organization-router.go`)

67 endpoints in four groups, all registered from `setOrganizationApiRoutes(apiRouter)`:

- `/api/organizations` — member self-service behind `UserAuth`; writes additionally behind
  `AccountContext()` and the workspace/manage/read policies.
- `/api/organization-invitations/:token` — public invitation view and accept.
- `/api/admin/organizations` and `/api/admin/organization-audit-logs` — platform admin, behind the
  existing `middleware.AdminAuth()` plus `OrganizationAdminAuth`.
- `/api/account-contexts` — list and switch the caller's context.

### Error mapping (`controller/organization.go`)

`writeOrganizationError` is the single place an organization error becomes an HTTP response: it maps
sentinel errors and message substrings onto a status plus a `types.ErrorCode` value
(`organization_dissolved` → 410, conflicts → 409, permission failures → 403), and it deliberately
never echoes a delivery error's cause (an SMTP rejection carries the recipient's address).
`types/organization_error.go` holds the codes.

### Swagger

The organization endpoints are **not** annotated. cuberouter's published spec is deliberately limited
to the public aggregated API plus the CSV import endpoint; see "Swagger: the org routes are
deliberately not annotated" in the porting notes.

### Frontend

**Not yet ported.** The organization UI is Phase 7 of the porting plan
(`docs/superpowers/specs/2026-09-20-organization-management-port-design.md`) and will be rewritten in
cuberouter's own stack (TanStack Router + Base UI + Tailwind + vitest) rather than vendoring the
source's Semi Design components.

---

## 3. Tests

Backend only at this point (`go test -count=1 ./...`):

| Area | Files |
|---|---|
| Migrations | `model/organization_scope_migration_test.go`, `organization_name_migration_test.go`, `organization_member_migration_test.go` — fresh DB, upgraded DB, resumed run, configured MySQL/PostgreSQL (self-skipping when `TEST_MYSQL_DSN` / `TEST_POSTGRES_DSN` are unset). |
| Scope isolation | `model/usedata_scope_test.go`, `model/async_task_scope_test.go`, `model/log_scope_test.go`, `model/token_scope_test.go` — organization rows never appear in a personal dashboard, log or task list, and the legacy-duplicate upgrade path merges instead of blocking startup. |
| Policy & access | `service/organization_policy_test.go`, `organization_access_test.go`, `router/organization_policy_router_test.go` — the capability matrix per role and access mode. |
| Routing | `router/organization_api_router_test.go` — the four route groups and their middleware. |
| Middleware | `middleware/account_context_test.go`, `organization_management_test.go`. |
| Lifecycle | `service/organization_test.go`, `organization_detail_test.go`, `organization_management_test.go`, `organization_member_test.go`, `organization_invite_test.go`, `organization_token_test.go`, `organization_token_auth_test.go`, `organization_group_test.go`, `organization_data_test.go`, `organization_log_test.go`, `organization_audit_snapshot_test.go`, `organization_audit_query_test.go`. |
| Billing | `service/organization_billing_test.go`, `organization_billing_gap_test.go`, `organization_billing_summary_test.go`, `organization_async_billing_test.go`, `organization_concurrency_external_test.go`. |
| Controller contracts | `controller/organization_error_test.go`, `organization_invite_test.go`, `organization_test.go`, `pricing_account_group_test.go`. |
| i18n | `i18n/organization_messages_test.go`. |

---

## 4. Known Limitations

- **MySQL and PostgreSQL are unverified for this feature.** The configured-database tests are written
  and compile, but no MySQL/PostgreSQL instance is reachable in the development environment used for
  the port, so every result above is SQLite-only. Run them with
  `TEST_MYSQL_DSN='…' TEST_POSTGRES_DSN='…' go test -count=1 -run 'ConfiguredDatabases' ./model/ ./service/`.
- **Organization spend can be double-counted in admin reports.** `GetUserBillingAgg`
  (`model/log_billing.go`) is not scope-filtered, so organization spend attributed to a responsible
  user still shows up in that user's `/api/data/billing` and `/api/ops/data/billing` report as well as
  in the organization's own report. Both routes are admin/ops-gated, and the port deliberately left
  the existing admin query semantics alone rather than changing them silently.
- **Rolling upgrades have a narrow window.** During a deployment that mixes old and new nodes, an old
  node's read-then-insert `SaveQuotaDataCache` can conflict with the new unique index; that flush is
  logged and dropped rather than corrupting the bucket. The window closes once every node runs the
  new code.
- **The frontend does not exist yet**, so the feature is reachable only through the API today.
