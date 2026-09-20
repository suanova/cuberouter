# Organization Management — Port Design (searouter-isuanova → cuberouter)

**Date:** 2026-09-20
**Status:** Approved
**Source:** `searouter-isuanova`, branch `feature/organization-management`
**Source commit range:** `3538f7ed` (port design spec, the branch's first org commit) → `064fb4d51` (branch head, 18 commits)
**Target:** `cuberouter`, branch `organization` (derived from `main` @ `a1939af90`)
**Domain docs:** [`docs/ORGANIZATION_PRD.md`](../../ORGANIZATION_PRD.md), [`docs/ORGANIZATION_TECHNICAL_DESIGN.md`](../../ORGANIZATION_TECHNICAL_DESIGN.md)

## 1. Goal

Port the complete organization management feature — organization lifecycle, members and invitations,
organization API keys, organization billing isolation, logs/tasks/usage/audit, the platform admin
surface, and account-context switching — into cuberouter, with full backend + frontend parity.

Personal-account behavior (personal keys, charging, logs, subscriptions, wallet) must be completely
unaffected.

## 2. Why this is a semantic port, not a merge

The two repositories have **unrelated git histories**: `git merge-base main gitlab/main` exits 1.
searouter-isuanova is a squashed 491-commit fork; cuberouter is a 6247-commit derivative of
`QuantumNous/new-api`. Cherry-pick, rebase and three-way merge are all unavailable — there is no
common base blob.

The source branch's own port (recorded in its `docs/ORGANIZATION_PORTING_NOTES.md`) faced the same
problem against a *different* upstream and solved it with `git merge-file` against a common ancestor.
We do not even have that. So every shared file is re-integrated **semantically**: read the source
change, understand its intent, and apply it to cuberouter's current shape.

Divergence of the shared files, measured as changed lines against the source's base version:

| File | base lines | divergence |
|---|---|---|
| `controller/user.go` | 1662 | 1346 |
| `model/user.go` | 1285 | 1345 |
| `relay/relay_task.go` | 514 | 859 |
| `relay/channel/openai/relay-openai.go` | 715 | 662 |
| `controller/task.go` | 276 | 612 |
| `model/log.go` | 459 | 511 |
| `middleware/auth.go` | 390 | 380 |
| `model/main.go` | 692 | 522 |
| `relay/common/relay_info.go` | 783 | 582 |

Files close enough to take nearly verbatim: `common/email-outlook-auth.go` (0 lines of divergence),
`service/funding_source.go` (16), `service/billing.go` (23), `.dockerignore` (26),
`constant/context_key.go` (28), `service/violation_fee.go` (28), `controller/log.go` (35),
`model/midjourney.go` (42), `controller/pricing.go` (45).

## 3. Scope

Porting unit is a **file**, in one of three classes:

1. **New file** — take the source file, adapt module path and project conventions, add the
   two-line license header.
2. **Shared file** — read the source diff, apply the org-relevant hunks to cuberouter's current
   version, keeping cuberouter's own customizations. Never overwrite wholesale.
3. **Excluded** — leave cuberouter untouched.

Excluded files are skipped; the *behavior* they carry in the source repo is re-implemented at
cuberouter's equivalent location. Named cases:

- `controller/task_video.go` — does not exist here; video task billing lives in
  `relay/relay_task.go` + `controller/relay.go`.
- `dto/user_settings.go` — does not exist here; the two account-context settings fields land in
  cuberouter's equivalent settings struct.
- `web/pnpm-lock.yaml`, `web/.eslintrc.cjs`, `web/i18next.config.js` — cuberouter uses Bun,
  oxlint/oxfmt and vitest.

## 4. Exclusions (not ported at all)

`helm/`, `docs-site/`, the upstream account-center refactor (issue-9987), the Playground/Landing/
Dashboard rewrites, `design-tokens.css` and the Semi override layer, brand-string renames, and the
source repo's language-pack reduction (`fr`/`ja`/`ru`/`vi` are retained here and rely on i18next
fallback).

## 5. Adaptation rules

1. **Module path** — `github.com/searouter/searouter` → `github.com/QuantumNous/new-api`.
2. **License header** — every new file begins with both lines, in this order:
   `Copyright (C) 2023-2026 QuantumNous` then `Copyright (C) 2026 CubeRouter`. Frontend files get it
   via `bun run copyright`.
3. **Database test environment** — the source uses `ORGANIZATION_TEST_DB_TYPE` /
   `ORGANIZATION_TEST_DSN` / `ORGANIZATION_TEST_ALLOW_DESTRUCTIVE`. Use cuberouter's existing
   convention instead: `TEST_MYSQL_DSN` / `TEST_POSTGRES_DSN`, self-skipping when unset (see
   `model/user_session_migration_test.go`). Follow AGENTS.md:94 — real SQLite, MySQL and PostgreSQL.
4. **`relaykit` independence** — nothing under `relaykit/` may import root-module org code. Verify
   with `cd relaykit && GOWORK=off go build ./...` (AGENTS.md:78).
5. **JSON** — `common.Marshal` / `common.Unmarshal` / `common.DecodeJson`, never `encoding/json`
   calls directly (AGENTS.md:81). The source already made this change in `model/user.go`; bring it.
6. **Quota arithmetic** — `common.QuotaFromFloat` / `QuotaRound` / `QuotaFromDecimal` and their
   `*Checked` variants, surfacing `*common.QuotaClamp` through `attachQuotaSaturation`
   (AGENTS.md:124-125). The source predates this rule in places; audit each org billing computation
   rather than copying it.
7. **Row locks** — `lockForUpdate(tx)`, never an inline `clause.Locking` (AGENTS.md:100).
8. **Reserved words and booleans in raw SQL** — `commonGroupCol` / `commonKeyCol` /
   `commonTrueVal` / `commonFalseVal` from `model/main.go`.
9. **New frontend code** follows `web/AGENTS.md` and `AGENTS.md`'s frontend section: English source
   strings as i18n keys, tests in `__tests__/` directories, Bun as the package manager.

## 6. cuberouter-specific hazards

These are places where cuberouter's shape differs in a way that silently breaks an otherwise
correct port:

- **`Token.Update` uses an explicit `Select(...)` list** (`model/token.go`). New token columns must
  be added there or they never persist.
- **`model/token_cache.go`'s `cacheInitToken` Lua script** enumerates hash fields and ARGV literals.
  New token columns must be added to both, or cached reads return zero values.
- **`model/usedata.go`'s in-memory cache key** is a literal `\x00`-joined format string. The org
  dimension must be added to the key *and* to the `WHERE` in `increaseQuotaData`.
- **ClickHouse log path** (`migrateClickHouseLogDB`, `clickHouseLogCreateTableSQL`) uses
  hand-written SQL rather than AutoMigrate. New `logs` columns must be added there explicitly.
- **`migrateDBFast`** carries its own model list, separate from `migrateDB`. New models go in both.
- **`OpsAuth`** is a cuberouter-only role mounted on the three `/ops/*` route groups. The `TokenAuth`
  rewrite must not disturb it.
- **`docs/swagger_gate_test.go`** forbids `/user/*` paths, paths containing `video`, and `password`
  in response schemas. Org endpoint annotations must not trip it.

## 7. Integration design

Two seams carry the feature, both of which already exist in cuberouter:

- **Billing** — `OrganizationFunding` implements the existing four-method `FundingSource` interface
  (`service/funding_source.go`) and is selected inside `NewBillingSession`. cuberouter's
  `BillingSession` (two-phase settle, `Reserve`, refund retry, trust logic) is **reused, not
  replaced**. On top of it we port the org ledger (`organization_billing_sessions` +
  `organization_billing_records`) and the 60-second lease/heartbeat/repair worker, which recover org
  charges abandoned by a crash or a long-running async task.
- **Authorization** — `service/organization_policy.go` is ported as the engine for org-internal
  capabilities (roles owner/admin/member × access modes workspace/management/read_only). The
  platform-admin surface under `/api/admin/organizations` is gated by the existing
  `middleware.AdminAuth()`. cuberouter's `authz` permission catalog is deliberately left untouched.

## 8. Domain summary

- **Organization** is simultaneously a collaboration, permission, resource and billing boundary.
  Status: `active` → `disabled` (reversible) → `dissolved` (terminal). Writes against a dissolved
  organization return HTTP 410 `organization_dissolved`.
- **Roles:** `owner` | `admin` | `member`; exactly one active owner, transferable.
- **Membership statuses:** `active` | `disabled` | `exited` | `removed`; disable source is
  `organization` or `platform`.
- **Invitations:** email strictly normalized (trim + lowercase), 7-day expiry, no plaintext token
  ever persisted (only `TokenHash`), resend rotates the token, persist-then-deliver so a mail
  failure does not lose the invite.
- **Org API keys:** scoped to the organization rather than a user, with `private`/`public`
  visibility and a responsible member; force-disabled while any blocker applies.
- **Account contexts:** one personal context plus N organization contexts per user, carried on
  `X-Account-Context-Type` / `X-Account-Context-Id`; a body id disagreeing with the path id yields
  403 `organization_context_mismatch`.
- **Billing:** closed pre-consume → settle/refund loop with an immutable ledger; refunds idempotent;
  org traffic never touches personal quota.
- **Idempotency:** batch token create/delete, quota adjustment, invite resend, member removal/exit
  key transfer and owner transfer accept an `Idempotency-Key` plus a request hash.
- **Audit:** append-only with before/after snapshots, secrets masked before persistence.

## 9. Data model

Eleven new tables: `organizations`, `organization_members`, `organization_invitations`,
`user_account_contexts`, `organization_disable_records`, `organization_token_system_blockers`,
`organization_idempotency_records`, `organization_billing_sessions`, `organization_billing_records`,
`organization_audit_logs`, `organization_quota_adjustments`.

Five extended tables: `tokens`, `tasks`, `midjourneys`, `quota_data`, `logs` gain a scope/billing
column subset (`scope_type`, `scope_id`, `billing_account_type`, `billing_account_id`,
`organization_id`, `actor_user_id`, `creator_user_id`, `responsible_user_id`; `tokens` also gains
`visibility` and the system-disable fields; `logs` also gains `billing_event_key`, the unique index
that makes settle/refund exactly-once).

Legacy rows are backfilled to the personal scope (`scope_type = 'personal'`, `scope_id = user_id`),
so existing behavior is unchanged.

## 10. Delivery

Phased commits, backend before frontend, each phase building and testing green before the next:

| Phase | Content |
|---|---|
| 0 | Port spec + baseline harness + domain docs |
| 1 | Data model + migrations |
| 2 | Auth, scope, middleware |
| 3 | Org core services, controllers, routes |
| 4 | Billing integration |
| 5 | Scoped reads, pricing, user status |
| 6 | Swagger annotations, backend i18n, docs |
| 7 | Frontend rewrite in cuberouter's stack |
| 8 | End-to-end verification |

## 11. Frontend

The source UI is ~13.7k lines of Semi Design JSX (25 components importing `@douyinfe/semi-ui`,
react-router-dom v6, react-toastify) across 88 files. cuberouter's web app is a different stack
(React 19 + Rsbuild + TanStack Router + Base UI + Tailwind + Zustand + vitest + Bun, with flat
English-source-string i18n keys). None of the JSX transfers.

The **behavior and API contract** transfer. The rewrite produces three feature modules under
`web/src/features/`: `organization/` (org center), `organization-admin/` (platform management), and
`account-context/`. Structural conventions come from `web/src/features/users/`; tables compose
`useTableUrlState` + `useQuery` + `useDataTable` + `DataTablePage`.

Cross-cutting work: account-context header injection in `web/src/lib/http-client.ts`; a Zustand
store for the active context; routes under `src/routes/_authenticated/`; sidebar/command-menu
registration; conversion of 632 dotted i18n keys into cuberouter's flat English-source-string
convention followed by `bun run i18n:sync`.

## 12. Verification and regression

Automated: `go build ./...`, `go vet ./...` (compared against the phase-0 baseline — cuberouter's
baseline is clean, so any diagnostic is new), `go test -count=1 ./...` on SQLite with the org core
suites additionally on PostgreSQL and MySQL, `cd relaykit && GOWORK=off go build ./...`, fresh-DB
and upgraded-DB migrations executed twice to prove idempotency, and the frontend gate list
(`typecheck`, `lint`, `format:check`, `test`, `build`).

Manual smoke: create organization → invite → accept (strict email match) → create org key → relay
call charges only the organization → personal quota/subscription/wallet unchanged → logs, tasks,
usage and audit attribute correctly to organization and responsible user → platform admin
list/detail/quota-adjust/disable/dissolve → organization self-disable and recovery → account-context
switch and fall back.

Regression checklist: personal key create/call/charge/log; subscription and top-up; campaign and
redemption codes; ops user list/export (`OpsAuth`); CSV channel import; model marketplace group
filtering; Redis enabled and disabled.

Database verification results are reported honestly. If MySQL cannot be exercised locally, that is
stated rather than claimed as verified.

## 13. Risks

| Risk | Mitigation |
|---|---|
| Billing integration corrupts personal accounting | `OrganizationFunding` plugs into the existing tested `BillingSession`; personal-scope guards on every `UpdateUser*` call; explicit smoke assertion that org traffic never moves personal numbers |
| Migration silently loses data (ClickHouse SQL, `Token.Update` Select list, token-cache Lua fields, `migrateDBFast`) | Enumerated as named hazards in §6; migration tests run twice on fresh and upgraded databases |
| `TokenAuth` rewrite breaks `OpsAuth` group inheritance or pricing filters | `OpsAuth` untouched; explicit phase-2 regression |
| Swagger gate failure from new endpoints | `docs/swagger_gate_test.go` run on every regeneration |
| Frontend rewrite drops features present in the Semi original | Behavior inventory taken from the PRD and the source tests before each module is written |
| Scale — 114 new backend files, 42 modified, 88 frontend files | Phased delivery with a green gate per phase; each phase independently reviewable and revertible |
