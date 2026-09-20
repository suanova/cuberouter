# Organization Porting Notes

> Ported from `searouter-isuanova` branch `feature/organization-management` (3538f7ed..064fb4d51).
> Design: [2026-09-20-organization-management-port-design.md](superpowers/specs/2026-09-20-organization-management-port-design.md).

## Baseline (branch `organization` @ `a1939af90`, 2026-09-20, before any port work)

| Gate | Command | Result |
|---|---|---|
| Backend build | `GOWORK=off go build ./...` | pass (no output) |
| Backend vet | `GOWORK=off go vet ./...` | pass (0 diagnostics) |
| Backend tests | `GOWORK=off go test -count=1 <root packages>` | 47 packages ok, 0 FAIL |
| Frontend typecheck | `cd web && bun run typecheck` | pass |
| Frontend tests | `cd web && bun run test` | 105 files, 684 tests, all pass |

Any `go vet` diagnostic or failing package appearing later is attributable to the port, since the
baseline is clean.

## Phase 1 — data model + migrations

New `model/` files: `organization.go`, `organization_member.go`, `organization_invite.go`,
`organization_audit_log.go`, `organization_quota_adjustment.go`, `organization_foundation.go`
(11 tables), `async_task_scope.go`, `organization_scope_migration.go`,
`organization_name_migration.go`, `organization_member_migration.go`.

Extended: `token.go`, `task.go`, `midjourney.go`, `usedata.go`, `log.go`, `main.go`
(both `migrateDB` and `migrateDBFast`, `migrateLOGDB`, the ClickHouse path and the hand-written
`clickHouseLogCreateTableSQL`).

### Deliberate divergences from the source

| # | Divergence | Why |
|---|---|---|
| 1 | `quota_data` gets the six scope columns **and** a unique index `idx_quota_data_account_context` over all 14 identity columns — but the index is created by `ensureQuotaDataAccountContextIndex`, not by a `uniqueIndex` struct tag, and legacy duplicate rows are merged before it is built. | Two separate problems. (a) Row identity: cuberouter's `quota_data` bucket already includes `use_group`/`token_id`/`channel_id`/`node_name`, so the source's 10-column index would silently merge distinct buckets. The port threads the full 14-column identity through the cache key and `ON CONFLICT` target instead, with `quotaDataAccountContextColumnNames` as the single source for both the index and the conflict target. (b) Table rebuild: a `uniqueIndex` tag makes GORM's AutoMigrate rebuild the whole table on SQLite on every startup (see divergence 4). (c) The index cannot simply be created blind: the pre-port write path was read-then-insert with no constraint, so a multi-node deployment can hold two rows that collapse onto one index key — `CREATE UNIQUE INDEX` then fails and `ensureOrganizationBillingIndexes`'s error stops the master node from starting. `ensureQuotaDataAccountContextIndex` merges such rows first (summing `count`/`quota`/`token_used` into the lowest id, inside a transaction) and rebuilds, so the dashboard totals are preserved and the upgrade completes. |
| 2 | `Task.TokenKey` / `Midjourney.TokenKey` are `varchar(128)`, not the source's `char(48)`. | cuberouter token keys are `varchar(128)`; `char(48)` would truncate. |
| 3 | The ClickHouse log path is extended: 14 new columns in `clickHouseLogCreateTableSQL` plus an idempotent `addClickHouseLogScopeColumns()` (`ALTER TABLE … ADD COLUMN IF NOT EXISTS`). | The source repo has no ClickHouse log database. `CREATE TABLE IF NOT EXISTS` never alters an existing table, so without the explicit `ALTER`s an upgraded deployment would silently lose every new log column. |
| 4 | `logs.billing_event_key`'s unique index is created explicitly by `ensureModelUniqueIndex` instead of a `uniqueIndex` struct tag. | A `uniqueIndex` tag makes GORM's AutoMigrate rebuild the **entire table** on SQLite on every startup (verified against `main`: `tokens` already behaves this way because of `Token.Key`). Migrations run on every master-node boot and `logs` is the largest table in the system, so the tag would turn a startup into a full table copy. The explicit index keeps the exactly-once contract and is idempotent via `HasIndex`. |
| 5 | `Token.CacheScopeVersion` (`gorm:"-"`) is written into the Redis token hash and required by `cacheGetTokenByKey`. | The cached token hash predates the scope columns. A hash written before this upgrade reads back with empty `ScopeType`/`OrganizationId`, which would make an organization key look like a personal key and charge the personal wallet. Rejecting the stale hash forces a database read instead; the hash is rebuilt when it expires naturally (`cacheInitToken` deliberately never overwrites a live hash). |
| 6 | `Token.Update`'s `Select` list covers every new column **except** `user_id`. | `user_id` only changes in the Phase 3 member-exit key-transfer flow; adding it now would let unrelated `Token.Update` callers silently reassign token ownership. |

### Test gate

Ported: `organization_scope_migration_test.go`, `organization_name_migration_test.go`,
`organization_member_migration_test.go` (the source's `_external_` variants are folded into the
configured-database subtests here). They reuse the existing `migrationSQLRecorder`
(`user_session_migration_test.go`) and follow cuberouter's `TEST_MYSQL_DSN` / `TEST_POSTGRES_DSN`
convention, self-skipping when unset.

Coverage: legacy-upgrade backfill, resume-safety (a corrupted scope row is repaired, no schema DDL
is replayed), fresh-database no-op, `migrateDB`/`migrateDBFast` end-to-end on a fresh SQLite
database including the name-contract recheck after the unique index is created, the Redis token
cache scope round-trip and stale-hash rejection, and the member disable-source attribution matrix.

**Honest status: SQLite is verified; MySQL and PostgreSQL are not.** The configured-database tests
are written and compile, but no local MySQL/PostgreSQL instance is reachable in this environment
(the Docker daemon proxies through a SOCKS5 proxy at `127.0.0.1:1080`, which is down, and neither
`psql` nor a MySQL client is installed). Run them with:

```bash
TEST_MYSQL_DSN='…' TEST_POSTGRES_DSN='…' go test -count=1 -run 'ConfiguredDatabases' ./model/
```

Each uses a throwaway database (MySQL) or schema (PostgreSQL) so a shared test DSN is not damaged.
Until that run happens, three-database compatibility for Phase 1 is **unverified**, not proven.

### Result

`go build ./...` clean, `go vet ./...` clean (0 diagnostics), `cd relaykit && GOWORK=off go build
./...` clean, `go test -count=1 ./...` → 47 packages ok, 0 FAIL — identical to the Phase 0 baseline.

---

## Phase 6 — swagger, i18n, docs

### Swagger: the org routes are deliberately **not** annotated

The ported controllers arrived carrying the source's 67 `@Router` annotations
(`/organizations/…`, `/account-contexts/…`, `/admin/organizations/…`). They were **stripped**, and
`make swag` now regenerates byte-identical output — `docs/{docs.go,swagger.json,swagger.yaml}` are
unchanged from the pre-port commit.

The reason is that cuberouter already converged its swagger source on purpose. Commit `1628bf59f`
("Swagger source convergence: strip annotations from 10 internal controllers, drop makefile `--tags`
filter, BasePath -> /api/v2, add `TestSpec_NoInternalUserPaths` gate") reduced the published spec to
the **public third-party aggregated API** (`/users`, `/plans`, both `AdminAuth`-gated but designed as
the external contract) plus one documented exception (`/channel/{id}/import_models_csv`, protected by
`TestSpec_HasImportCsvPath`). `docs/swagger_gate_test.go` states the rule — "internal controllers
carry no annotations" — and `router/main.go` mounts the UI only when `DEBUG=true`
("生产默认关闭,避免匿名暴露完整 API 规格").

Re-adding 67 annotations would have grown the published spec from 10 paths to 62 and documented
session/`AccountContext`-authenticated routes under the spec's `ApiKeyAuth` security definition,
which is the opposite of what the source is describing. The organization API is an internal product
surface, not part of the aggregated contract, so the annotations were removed rather than published.
`docs/swagger_gate_test.go` passes.

### i18n

Only one backend string is localized: `organization.quota_data_time_span_too_long`
(`controller/organization_data.go` → `common.ApiErrorI18n`). Everything else the organization API
rejects comes back as a stable `error_code` plus an English message — the frontend maps the code to
its own copy — so there are no further `organization.*` keys to add. The key was already present in
all three locales; `i18n/organization_messages_test.go` now pins the exact per-locale text and the
unknown-locale fallback, mirroring `i18n/ops_messages_test.go`.

### Tests added in this phase

- `controller/organization_error_test.go` — pins `writeOrganizationError`'s status/code mapping
  (410 dissolved, 409 conflicts, 403 forbidden), the invite-delivery failure shape, and that SMTP
  errors carrying the recipient address never reach the response body.
- `controller/organization_invite_test.go` — invite request binding (`target_email` preferred,
  legacy `email` fallback, `force_rotate`), empty-body accept, and the unavailable code for a revoked
  invite.
- `controller/organization_test.go` — malformed-JSON member removal is rejected, an invalid exit
  transfer target leaves the membership intact, and dissolve records the body confirmation and reason
  in the audit log.
- `model/usedata_scope_test.go` — the legacy-duplicate upgrade path (see divergence 1c) and a drift
  guard tying the merge predicate to `quotaDataAccountContextColumnNames`.

Skipped as duplicates: `controller/organization_billing_test.go` (member billing visibility is
already covered at service level in `service/organization_billing_summary_test.go`) and
`controller/organization_async_task_test.go` (video-task settlement, already covered by
`service/organization_async_billing_test.go` and built on a task path that differs in cuberouter).

### Result

`go build ./...` clean, `go vet ./...` clean (0 diagnostics), `go test -count=1 ./...` → all packages
ok, 0 FAIL; `docs/` unchanged by `make swag`.

---

## Cross-phase divergences

| # | Divergence | Why |
|---|---|---|
| A | Organization API error responses are `{"success":false,"message":…,"code":…}` with English messages and no localization. | Ported as-is from the source: the frontend branches on `code`, and localizing the message backend-side would have to be redone for every client. Only `quota_data_time_span_too_long` is translated because that one is rendered directly. |
| B | `GetUserBillingAgg` (`model/log_billing.go`) is left **unscoped** — organization spend attributed to a responsible user still appears in that user's billing report. | Both callers are admin/ops-gated (`/api/data/billing` behind `AdminAuth`, `/api/ops/data/billing` behind `OpsAuth`), and the plan's rule was that admin queries keep their existing semantics. It does mean an organization's spend can be counted once in the org report and once in the responsible user's admin report; flagged rather than silently changed. |
| C | `TokenAuth`'s organization branches leave cuberouter's `OpsAuth` untouched. | `OpsAuth` is a cuberouter-only feature mounted on the three `/ops/*` groups with no counterpart in the source repo. |


