# Admin Log Filters & Task Log Filter Parity — Design

**Date:** 2026-11-23
**Status:** Proposed

## Goal

Two changes to the usage-log surfaces:

1. The billing-attribution filter (`billing_account_type`: All / Personal /
   Organization, labeled 全部账单归属) is removed from **admin views only**.
   Personal views (ordinary users and an admin's "Only Mine") keep it.
2. Task logs gain the trace filters usage logs already have, so an admin can
   search tasks the same way they search common logs.

## Why

- Platform admins read the whole-platform stream; billing attribution is a
  per-account distinction already visible in the Billing Account column, and the
  organization filter covers the org slice at row level. The product owner
  decided (2026-11-23) the extra selector is noise for admins.
- Task logs currently expose only task ID / status / action / channel /
  organization, while usage logs expose model, token, user, group, and request
  ID. Task troubleshooting needs the same handles.

## Decisions from the conversation

- Remove the billing filter for admins on `/usage-logs/common`,
  `/usage-logs/task`, and the site view of `/usage`. Keep it on all
  personal/self views; the organization usage tab already hides it.
- The admin's organization filter remains (organization ID input on log pages;
  organization picker on the usage page — see the call below).
- Backend keeps accepting `billing_account_type`; only admin UI/params drop it.
- Task filters to add: model name, token name, request ID for everyone;
  username and group for admins; platform (text input) as the task-specific
  extra.
- Upstream Request ID is not added: the `tasks` table has no such column (the
  upstream *task* ID lives in `private_data` JSON and is out of scope).
- No schema change: the model name is read from `properties` JSON with
  per-dialect SQL, following `GetByUpstreamTaskId`.

## Approaches considered

**A. Query-only (chosen).** Add the filters to the existing task queries; model
name via JSON extraction; username resolved against `users`.

- Pros: no migration, no backfill, low risk.
- Cons: model filter is unindexed (bounded by the submit-time window);
  username costs one extra subquery.

**B. Materialized columns (rejected).** Add `model_name` (and possibly
`username`) columns to `tasks` with an index and a backfill.

- Pros: indexed filters, clean SQL.
- Cons: three-database migration + backfill + idempotency verification;
  disproportionate for this need.

## Implementation

### Backend

`model/task.go`

- `SyncTaskQueryParams`: add `ModelName`, `TokenName`, `Group`, `RequestID`,
  `Username` (`Platform` already exists).
- Extract `applySyncTaskFilters(db, params)` from the duplicated bodies of
  `TaskGetAllTasks` / `TaskCountAllTasks` / `TaskGetAllUserTask` /
  `TaskCountAllUserTask` so list and count can never drift. Apply the caller's
  scope query first.
- Model filter (`properties.origin_model_name`) with dialect branches:
  - PostgreSQL: `properties->>'origin_model_name'`
  - SQLite: `json_extract(properties, '$.origin_model_name')`
  - MySQL (default): `JSON_UNQUOTE(JSON_EXTRACT(properties, '$.origin_model_name'))`
  - Exact match; if the value contains `%`, reuse `sanitizeLikePattern` with
    `LIKE ? ESCAPE '!'`, matching the common-log model filter.
- Username filter: subquery on `users` (`Unscoped`, so tasks of deleted users
  stay findable) → `user_id IN (?)`. Same `%` semantics as the common-log
  username filter. No matching user yields an empty page.
- Token name, group, request ID: exact-match `WHERE` on their columns, matching
  the common-log filter semantics (token name is exact there too).
- Platform: exact match, moved into the shared helper.
- `GetUserTask` accepts model, token name, group, request ID, and platform;
  username and channel stay admin-only.

`controller/task.go`

- `GetAllTask`: parse `model_name`, `token_name`, `group`, `username`,
  `request_id`.
- `GetUserTask`: parse `model_name`, `token_name`, `group`, `request_id`,
  `platform`.

### Frontend

`web/src/features/usage-logs/`

- `types.ts`: extend `TaskLogFilters` and `GetTaskLogsParams`.
- `lib/filter.ts`: carry the new task keys through `buildSearchParams`.
- `lib/utils.ts`: forward them in `fetchLogsByCategory`; send username/group
  only when `isAdmin`; never send `billing_account_type` when `isAdmin`
  (common and task).
- `components/common-logs-filter-bar.tsx`: hide the billing selector when
  `isAdmin`; ignore an inherited `billingAccountType` in that view (draft
  state, apply, counts). The organization ID input stays.
- `components/task-logs-filter-bar.tsx`: same billing removal; add
  model/token/request-ID inputs for all viewers, username/group for admins,
  and a platform input; update advanced/mobile counts.
- `routes/_authenticated/usage-logs/$section.tsx`: add `platform` to the
  search schema and clear it when leaving the task section.

`web/src/features/usage/` (site view)

- `components/usage-filter-bar.tsx`: hide the billing selector when
  `variant === 'site'`; keep the organization picker visible for site (it
  previously only appeared under billing = organization — removing billing
  must not silently remove it). Personal keeps both.
- `index.tsx`: do not send `billing_account_type` (page params, export params,
  initial filters) when `scope === 'all'`.

### Copy

No new English strings: Model Name, Token Name, Username, Group, Request ID,
and Platform already exist in the locale files.

## Non-goals

- No task statistics bar and no server-side sorting.
- No backend removal of `billing_account_type` from `/api/log`, `/api/task`,
  or the usage endpoints.
- No new columns or migrations; no backfill.
- No upstream request/task ID filter.

## Verification

- Go: model test covering each new task filter on both list and count,
  including the JSON model name across SQLite, MySQL, and PostgreSQL
  (`TEST_MYSQL_DSN` / `TEST_POSTGRES_DSN` external-test pattern) and username
  resolution including a deleted user.
- Frontend: filter-param tests (admin drops billing, personal keeps it; task
  filters map to the right query params); usage site params omit billing.
- `bun run typecheck`, oxlint on changed files, `bun run build`.
- Full Go test suite (CI-equivalent command) covering service/model/router.
