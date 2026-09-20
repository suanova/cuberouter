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
| 1 | `quota_data` gets the six scope columns but **not** the source's `idx_quota_data_account_context` unique index. | cuberouter's `quota_data` row identity also includes `use_group`/`token_id`/`channel_id`/`node_name`. Importing the 10-column unique index would either fail to build on an upgraded database that already holds duplicate scope tuples, or silently merge distinct rows. Phase 5 threads the scope dimension through the existing cache key and `increaseQuotaData` `WHERE` clauses instead, preserving cuberouter's bucket identity. |
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

