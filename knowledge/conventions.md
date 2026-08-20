# Fork Conventions: cuberouter vs. upstream (new-api)

This document explains how the **cuberouter** fork diverges from its upstream
([QuantumNous/new-api](https://github.com/QuantumNous/new-api)) and the
conventions that govern that divergence.

## Summary

**cuberouter** is a fork of `new-api`. All changes made by ourselves fall into
two buckets:

1. **Original features** built on top of new-api's infrastructure: the Plugin
   Market (playground `@slug` agentic loop) and the CubeRouter rebrand / main
   page.
2. **Ported features** taken from the private **searouter-isuanova** project
   (`glab.isuanova.com/cuberouter/searouter-isuanova`): user & invitation
   management, the campaign system, the ops role, the aggregated third-party
   API, the Alipay gateway, CSV channel-model import, billing reports,
   configurable email templates, and several relay/billing fixes.

The Go module path was deliberately kept as `github.com/QuantumNous/new-api`
to keep imports stable and simplify upstream merges — every ported file that
originally imported `github.com/searouter/searouter/...` was rewritten to the
new-api module path.

## Upstreams and remotes

| Role | Remote | Repository | Purpose |
|---|---|---|---|
| Origin | `origin` | `github.com/suanova/cuberouter` | This fork (mainline) |
| Upstream | (sync only) | `github.com/QuantumNous/new-api` | Pulled weekly by `upstream-sync.yml` |
| Port source | `gitlab` | `glab.isuanova.com/cuberouter/searouter-isuanova` | Read-only source for feature ports |

There is no long-lived `upstream` git remote to new-api; upstream integration
is handled entirely by the automated sync workflow (see **Upstream sync**).

## Changes by ourselves

### Overview

| Feature | Origin | Area | Added |
|---|---|---|---|
| Plugin Market | Original | Playground / plugins | 2026-07-27 |
| CubeRouter main page & rebrand | Original | Frontend / branding | 2026-08-05 |
| Campaign System | searouter | Marketing / redemption | 2026-08-06 |
| Ops Role System | searouter | Roles / auth / ops | 2026-08-10 |
| Admin User Management | searouter | User admin | 2026-08-12 |
| Invite-group inheritance, phone masking, custom user columns, dashboard dialog, password strength, username 20→50, bilingual email | searouter | User admin | porting guide 1 |
| CSV channel-model import | searouter | Channel admin | porting guide 2 |
| Swagger annotation system | searouter | Docs | porting guide 2 |
| Alipay official payment gateway | searouter | Billing / payments | porting guide 3 |
| Aggregated API (v1 → v2) | searouter | Third-party API | porting guides 4 + 5 |
| ops Token consumption columns | searouter | Ops | porting guide 4 |
| Billing reports (user / reconciliation / ops) | searouter | Billing / ops | porting guide 5 |
| Configurable password-reset email templates | searouter | Email | porting guide 5 |
| Streaming-billing fix + raw-quota display + `/swagger` dev proxy | searouter (bugfix / UI) | Relay / frontend | porting guide 5 |

### Branding & main page (original)

- System name, page titles, startup logs, Electron window/tray labels renamed
  from "New API" to **CubeRouter**; `favicon.ico` dropped, `head.png` added,
  `logo.png` / `logo.tsx` updated.
- Home page is a single `Landing` component (`landing/landing.tsx` +
  `landing.css`) ported from CubeRouter, with an interactive WebGL globe
  (`landing/globe.tsx`) powered by the new `cobe` dependency.
- Typography: UI sans switched to Bricolage Grotesque (CJK fallbacks PingFang
  SC / Microsoft YaHei); JetBrains Mono added.
- Brand accent: CubeRouter orange `#FB6415` with black ink, applied as the
  primary / sidebar-primary color across every theme preset.
- **i18n locales reduced**: fr, ru, ja, vi were dropped; the supported set is
  now `en` (source language), `zh`, and `zh-TW` only.

### Plugin Market (original)

- New `plugins` table (`model/plugin.go`) + admin CRUD under `/api/plugin/`
  (`GET/POST/PUT`, `DELETE /:id`, `POST /:id/refresh`, `POST /test`), plus
  `GET /api/plugin/enabled` for logged-in users (autocomplete; exposes only
  slug/name/description).
- A plugin bundles a remote MCP server URL (streamable HTTP/SSE, no auth) with
  a skill markdown fetched server-side from GitHub (10s timeout, 256 KiB cap,
  cached in DB; a failed fetch never blocks saving).
- New `pkg/mcp` package: minimal JSON-RPC 2.0 client (`initialize` handshake,
  `Mcp-Session-Id`, 60s tool-list cache, timeouts 5s/10s/30s, 1 MiB response
  cap).
- Playground: typing `@` opens an autocomplete; a message mentioning `@slug`
  triggers a server-side agentic tool-call loop (≤3 plugins, ≤10 rounds) that
  runs non-streaming relay rounds through the standard pipeline (billing,
  channel selection, retries, logging all normal), then streams the final
  answer. Failure modes: unknown slug → plain text; unreachable MCP → tools
  skipped, skill still injected; tool errors → fed back as tool messages;
  loop cap → forced tool-less round.
- Slug rule: `^[a-z0-9][a-z0-9-]{1,63}$`, unique, immutable after creation.

### User management & invitation domain (ported from searouter)

- **Invite-group inheritance**: registering via an invite code inherits the
  inviter's `group`; admins cannot change the group of a user who already has
  invitees (`model.HasInvitees`, `MsgUserGroupModifyForbidden`).
- **Phone masking**: `common.MaskPhone` (keep first 3 / last 4) applied on the
  ops invitee endpoints; the admin invitee endpoint intentionally returns the
  raw phone (admins outrank the data owner).
- **Custom user columns**: `controller/user_columns.go` + `/api/user/columns`
  column metadata; the users table persists column visibility via
  `columnVisibilityStorageKey` (channels/ops-users precedent).
- **User dashboard dialog**: `UserDashboardDialog` (VChart, 7/14/30-day)
  backed by the existing `GET /api/user/:id/quota-dates`; clickable username
  opens it, gated by role hierarchy.
- **Password strength**: `passwordStrength` validator (`common/validate.go`)
  — uppercase + lowercase + digit, applied to the `Password` field
  (`min=8,max=20`); empty passwords pass through (no-change flows unaffected).
- **Username length 20 → 50** (`validate:"max=50"`), frontend schemas synced.
- **Bilingual email**: `common.WrapBilingualSubject` / `WrapBilingualContent`
  (English first) used by registration-verification and password-reset emails.
- **Admin user management** (`/api/user/...`, `AdminAuth`):
  - `GET /:id/invitees` — invitee history of any user as `dto.InviteeBrief`
    (8 fields, raw phone), registered before the `/:id` wildcard with a route-
    order test.
  - `POST /export` — eager pre-fetch of all batches before writing (a DB error
    fails the request, no partial CSV), batch 200, capped at
    `maxExportIds = 10000` ids / `maxExportRows = 50000` rows, UTF-8 BOM, ASCII
    filename, 14 i18n-resolved headers, `csvSafeCell` formula-injection guard.
  - `GET /:id/quota-dates` — quota/usage dashboard proxy gated by
    `canViewUserDashboard` (`myRole > targetRole`, root 100 exempt), max
    1-month range (`adminQuotaDatesMaxRange = 2592000`).

### Campaign System (ported from searouter)

- New `campaigns`, `campaign_participants`, `campaign_rewards` tables
  (`model/campaign.go`); a `Phone` field was added to the `User` model so the
  `phone_filled` trigger exists.
- Two campaign types: `phone_filled` (user first fills a phone) and
  `invitation` (registration with an `aff` inviter). Statuses Draft/Active/
  Paused/Ended; reward statuses Pending/Dispatched/Failed/Cancelled.
- Admin CRUD + stats + participants + rewards + reward-email resend under
  `/api/campaign/` (`controller/campaign.go`). Types are immutable after
  creation.
- `service/campaign.go` engine: triggers run asynchronously via `gopool`;
  `GetActiveCampaignsByType` enforces the `[start_at, end_at)` window at query
  time; invitation dispatch is **atomic** (`DispatchRedemptionToUser` uses
  `lockForUpdate(tx)` + compare-and-swap `enabled → used` update so concurrent
  triggers cannot double-issue); the code pool is topped up incrementally to
  `code_count` (≤1000) on activation; pool empty → participant slot released.
- Invitation reward email is a credit receipt (codes auto-redeemed at
  dispatch). Ops tier deliberately skipped in the initial port (see next).

### Ops Role System (ported from searouter)

Completes the ops tier the campaign port deferred.

- `RoleOpsUser = 5` (`common/constants.go`), inserted between common (1) and
  admin (10); `IsValidateRole` accepts exactly guest/common/ops/admin/root.
- `OpsAuth` middleware (`minRole = 5`) through the shared `authHelper`, so
  ops/admin/root pass and common/guest are rejected. Note: the source's
  `New-Api-User` anti-CSRF header check does **not** exist in this repo's auth
  and was not ported.
- `common.MaskPhone` + table tests.
- Read-only invitee endpoints under `/api/ops/user/` (list / search / columns /
  CSV export), every query scoped `WHERE inviter_id = ?` with
  `Omit("password")`; export batched at 200, UTF-8 BOM, foreign ids filtered
  out; phones masked.
- Read-only campaign views under `/api/ops/campaign/` scoped to campaigns
  whose `config_json.invitee_user_id == self`; `GetCampaignsByInviteeUserId`
  / `SearchCampaignsByInviteeUserId` use a `config_json LIKE` pre-filter plus a
  precise Go-side parse to exclude LIKE false positives.
- `ManageUser` `promote_ops` / `demote_ops` actions (admin/root only, behind
  `canManageTargetRole`); error messages are i18n keys (`ops.*`), not
  hardcoded strings.
- Frontend: two ops pages, ops sidebar section gated at `requiredRole:
  ROLE.OPS`, promote/demote row actions on the admin Users page.

### Aggregated API (ported from searouter)

- `controller/aggregated_api.go` — a third-party-facing collection API behind
  `AdminAuth`, uniform `{status, ...}` responses (HTTP 200), with role
  hierarchy checks.
- Endpoints (all registered with the `""` + `"/"` dual path to avoid Gin 307
  trailing-slash redirects): `POST /users`, `POST /plans`, and per-user
  `suspend` / `reactivate` / `reset-password` / `adjust-quota` /
  `bind-subscription` / `delete`, plus `GET /users/:user_id/status`.
- Registered under a **triple prefix mirror** — `/api`, `/api/v1`, `/api/v2` —
  with swagger `@BasePath /api/v2`. This is the stable external contract; do
  not move it.
- New users are created **enabled** (the source's later decision; post-insert
  re-assert of status included). `delete` is a **hard** delete
  (`HardDeleteUserById`, `Unscoped().Delete`); `adjust-quota` forces a sync
  DB write (`Increase/DecreaseUserQuota(..., true)`).
- Source's `model.RecordManageLog` audit calls are **omitted** — this repo's
  `AdminAuth` already auto-audits write operations in the auth chain.

### Alipay official payment gateway (ported from searouter)

- Direct official Alipay (PC web payment, `smartwalle/alipay/v3`) — distinct
  from EPay's pass-through `alipay` type.
- `setting/payment_alipay.go` (6 config vars) registered in `model/option.go`;
  `controller/topup_alipay.go` (`GetAlipayClient`, `RequestAlipayPay`,
  `RequestAlipayAmount`, `AlipayNotify`).
- Constants: `PaymentMethodAlipay = "alipay_official"`,
  `PaymentProviderAlipay = "alipay"`.
- Routes: `POST /api/alipay/notify` (unauthenticated, plain-text
  `success`/`fail` responses — never JSON, or Alipay retries forever),
  `POST /api/user/alipay/pay`, `POST /api/user/alipay/amount` (self, with
  `CriticalRateLimit`).
- Reuses this repo's existing EPay infrastructure: `LockOrder`/`UnlockOrder`,
  `getPayMoney`, webhook-enablement guards (`isAlipayTopUpEnabled` /
  `isAlipayWebhookEnabled`), `GetTopUpInfo` injection of the
  `alipay_official` method + `enable_alipay_topup` / `alipay_min_topup`.
- The source's `QuotaDisplayType` column was **not** ported: the callback
  settles as `Amount × QuotaPerUnit`, identical to the existing EPay callback
  (the recommended minimal path).

### CSV channel-model import (ported from searouter)

- `service/channel_csv_import.go` (main flow + 14 pure functions),
  `controller/channel_csv_import.go`, `dto/channel_csv_import.go`; new model
  helpers `GetModelByName` + `(Model) UpdateMetaFields` (`model/model_meta.go`).
- Route `POST /api/channel/:id/import_models_csv` (permission-table driven,
  `authz.ChannelWrite`), per-handler 10 MB cap.
- Constraints: fixed 8-column header (count checked, not names), full UTF-8
  validation, ≤50000 data rows, `unit == "Million"` participates in price
  conversion; sentinel errors mapped to 400 (`ErrBadHeader` / `ErrInvalidCsvRow`
  / `ErrTooManyRows` / `ErrBadEncoding` / `ErrChannelNotFound`).
- Ratio write-back red lines preserved: `ratioWriteMu` serializes writes,
  marshal failure does **not** call `UpdateOption` (avoids clearing the global
  ratio map), and `UpdateXxxByJSONString` is forbidden (its replace semantics
  would wipe global state). Idempotent re-import (dedupe by trimmed model
  name, existing order wins).

### Billing reports (ported from searouter)

- New `controller/billing_report.go` (`GetUserBillingReport`,
  `GetReconciliationReport`, `GetOpsBillingReport`), `model/log_billing.go`,
  `dto/user_billing.go` — ported together as a block (they share
  `parseBillingDateRange` / `quotaToDisplayAmount`).
- Routes: `/api/data/billing`, `/api/data/reconciliation` (AdminAuth) and
  `/api/ops/data/billing` (OpsAuth); ranges capped at 31 days.
- `GetOpsBillingReport` restricts ops callers to users they invited
  (`user.InviterId == callerId` unless the caller is admin+).

### Email templates (ported from searouter)

- `common/email_template.go` gained `RenderPasswordResetEmail` /
  `RenderPasswordResetSuccessEmail` (Go `text/template`, `{{.SystemName}}` /
  `{{.Link}}` / `{{.ValidMinutes}}` / `{{.NewPassword}}`) with default
  templates and `renderTemplate` fallback.
- 8 configurable options (`PasswordResetEmailSubjectEn/Zh`, `...ContentEn/Zh`,
  `PasswordResetSuccess...`) registered in `model/option.go` and surfaced in
  the SMTP settings section.
- `SendPasswordResetEmail` / `ResetPassword` / the aggregated API's
  `reset-password` render via these functions. **Keep this repo's
  `url.QueryEscape` + `html.EscapeString` handling** — it is more secure than
  the source's bare concatenation.

### Streaming-billing fix (ported from searouter)

- `relay/helper/stream_scanner.go` returns a structured
  `StreamScannerResult` (completed / idle_timeout / client_disconnected /
  scanner_error / data_handler_error); idle timeout no longer fabricates a
  synthetic success usage.
- On error, `postConsumeQuota()` and `RecordConsumeLog()` are skipped; new
  error codes in `types/error.go` (`stream_idle_timeout`,
  `stream_client_disconnected`, `stream_scanner_error`,
  `stream_data_handler_error`, `stream_incomplete`).
- Test files ported alongside (`stream_scanner_test.go`,
  `relay_openai_stream_test.go`, `compatible_handler_test.go`).

### Swagger (ported from searouter)

- swaggo dependencies (`swag`, `gin-swagger`, `swag/files`), `controller/
  swagger.go` global annotations, `make swag` target, `/swagger/*any` mount,
  and gate tests under `docs/swagger_gate_test.go`.
- `@BasePath /api/v2`; internal user annotations are removed at the source so
  `swag init` (no `--tags` filtering) does not regenerate internal paths into
  the public spec.

## i18n conventions

- **Frontend**: `i18next`, English source strings are the keys in
  `web/src/i18n/locales/en.json`; only **en / zh / zh-TW** are shipped
  (fr/ru/ja/vi were dropped with the rebrand). New text is added to `en.json`
  first, then translated; run `bun run i18n:sync`.
- **Backend**: `i18n/keys.go` + `i18n/locales/{en,zh-CN,zh-TW}.yaml`. Ported
  features use `noun.verb` key names with `{{.X}}` template args
  (e.g. `ops.campaign_not_accessible`, `admin.quota_dates_range_exceeded`),
  and reuse existing keys (`common.invalid_id`, `user.not_exists`,
  `common.invalid_params`) where the message matches.
- Ported controllers never hardcode user-facing strings from the source's
  Chinese literals; they are converted to i18n keys.

## Upstream sync

An automated GitHub Action (`.github/workflows/upstream-sync.yml`) runs weekly
(cron `0 22 * * 0` — Sunday 22:00 UTC, i.e. Monday 06:00 China time; the
in-file comment "Monday at 6am UTC" is the same instant mislabelled as UTC) and
uses `suanova/upstream-semantic-sync@v1` (risk cap `max_risk: medium`) to open
a PR merging upstream `new-api` changes. Sync state is tracked in
`knowledge/mappings.yaml` (`sync_state` → upstream commit hash).

Because the Go module path is unchanged, most upstream merges apply cleanly.
The conflict surface is any file we added or modified — the ported feature
files listed above — and any upstream file that overlaps a ported feature's
dependencies. Resolve conflicts in favour of cuberouter's features unless the
upstream change is the authoritative fix (e.g. shared relay/billing helpers).

## Conventions for future changes

1. **Keep the Go module path** `github.com/QuantumNous/new-api`. Never rename
   it, and when porting code always rewrite `github.com/searouter/searouter/...`
   imports to the new-api module path.
2. **Ported features are adapted, not copied.** Mirror the source backend
   structure, but follow this repo's conventions: `common.Marshal/Unmarshal`
   for all JSON, GORM-only cross-DB code (SQLite/MySQL/PostgreSQL),
   `lockForUpdate(tx)` for row locks, i18n keys instead of hardcoded strings,
   `require`/`assert` testify tests with deterministic tables.
3. **Never copy the source's Semi UI JSX.** The frontend is rewritten in this
   repo's stack (TanStack Router + TypeScript + Base UI + Tailwind + VChart);
   reuse the existing `features/` patterns (`channels/`, `redemption-codes/`,
   `users/`, `wallet/`).
4. **Port the source's tests and add the ones it lacked.** The searouter
   project shipped several features with zero tests (ops, campaign); every
   port added backend + frontend test coverage. Do not regress this.
5. **Do not port HKBN-exclusive or repo-incompatible pieces**: the wordmark /
   colour commits, the ops read-only guest scenario, the `New-Api-User`
   anti-CSRF header check, and the `phone_region` column (this repo has none).
   Also skip source features already available (e.g. `QuotaDisplayType`
   column — settle like EPay instead).
6. **Preserve the triple-prefix contract** (`/api`, `/api/v1`, `/api/v2`,
   `@BasePath /api/v2`) for the aggregated API. New third-party-facing
   endpoints belong in `registerApiRoutes` and must keep the `""` + `"/"`
   dual registration to avoid 307 redirects.
7. **Watch the billing invariants.** New billing paths (Alipay, CSV ratio
   write-back, aggregated `adjust-quota`, streaming settlement) must keep
   every charge non-negative, use the centralized quota helpers in
   `common/quota_math.go`, and never reintroduce bare `int(...)` casts or
   `UpdateXxxByJSONString` ratio replacement.
8. **Keep `docs/superpowers/` in sync.** Every added or ported feature has a
   design spec, an implementation plan, and a changelog entry (newest first);
   operator guides live in `docs/`. Update them when behaviour changes.
9. **When merging upstream**, watch files touched by the ports. The sync
   workflow caps risk at `medium`; high-risk upstream changes should be
   reviewed manually.
