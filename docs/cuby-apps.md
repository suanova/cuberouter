# Cuby apps: which plug-ins an account may use

Cuby is Suanova's desktop client. Its features beyond the core arrive as apps (plug-ins): the AI company, FinanceBuddy and others. Cuby signs users in through CubeRouter, so CubeRouter is where an administrator decides which account gets which app. Cuby asks; CubeRouter answers.

This decides what Cuby shows and runs for an account. It is not copy protection: the app files are on the user's PC. An app that must be protected has to check the account on its own server.

## What an administrator does

Admin → **Cuby Apps** (`/admin/cuby-apps`, platform administrators).

1. **Add app.** Register the app by the id its `cuby-app.json` carries, for example `ai-company`. The id cannot be changed later. A registered app is given to nobody yet.
2. **Who has it.** Give the app to an organization (every active member gets it) or to a single user. Take it away the same way.
3. **Limit Cuby apps by account.** The switch at the top. See the next section before switching it on.

Each app also has its own switch: off takes the app from everyone and keeps the list of who had it. Deleting an app deletes its grants too.

Cuby picks a change up the next time it asks: after sign-in, at start, when its Apps page opens, or when the user presses "Check again" there. Nothing is pushed to Cuby.

## The switch, and why it ships off

| Switch | What Cuby is told | What Cuby does |
|---|---|---|
| Off (default) | HTTP 404: this instance does not manage apps | Limits nothing. Every app installed on the PC can be used |
| On | The account's list, possibly empty | Runs only the apps on the list |

An empty list is an answer: "this account has no apps". If the route answered with lists from the moment it was deployed, every signed-in Cuby user would lose every app until grants existed. So the order is: deploy, register the apps, give them out, then switch on. Switching on asks for confirmation and shows how many apps and grants exist.

The switch is the option `cuby_app_setting.enabled` (`setting/operation_setting/cuby_app_setting.go`), stored like every other option, so all nodes follow it.

## What Cuby reads

```
GET /api/account/apps
Authorization: Bearer <access token of the signed-in account>
```

```json
{
  "success": true,
  "message": "",
  "data": {
    "apps": [
      { "id": "ai-company", "sources": [{ "type": "organization", "id": 7, "name": "Harbour Ltd" }] },
      { "id": "financebuddy", "sources": [{ "type": "user" }] }
    ]
  }
}
```

- `apps[].id` is the registered app id.
- `apps[].sources` says why the account has it: `user` (given to the account) or `organization` (given to an organization it is an active member of). Cuby shows it; it decides nothing. The account's own grant comes first.
- The list holds enabled apps only, given to the user or to an **active** organization in which the user's membership is **active**.
- HTTP 404 while the switch is off; HTTP 401 without a valid session.

The path, the 404 and this JSON are a contract with installed Cuby clients (Cuby: `App/memmy-agent/src/apps/README.md`). `controller/cuby_app_test.go` pins them.

## Administrator routes

All behind `AdminAuth`; writes are recorded by the administrator audit that `AdminAuth` applies.

| Route | Does |
|---|---|
| `GET /api/admin/cuby-apps` | The switch and every app with its grant counts |
| `POST /api/admin/cuby-apps` | Register an app: `{ "app_id", "name", "description" }` |
| `PATCH /api/admin/cuby-apps/:id` | Change `name`, `description` or `status` (`enabled` / `disabled`) |
| `DELETE /api/admin/cuby-apps/:id` | Delete the app and its grants |
| `GET /api/admin/cuby-apps/:id/grants` | Who has it, paged |
| `POST /api/admin/cuby-apps/:id/grants` | Give it: `{ "subject_type": "user" \| "organization", "subject_id" }` |
| `DELETE /api/admin/cuby-apps/:id/grants/:grantId` | Take it away |
| `PUT /api/admin/cuby-app-setting` | The switch: `{ "enabled": true \| false }` |

404 for an unknown app, grant or holder; 409 for an app id or a grant that already exists.

## Tables

- `cuby_apps`: `app_id` (unique), `name`, `description`, `status`.
- `cuby_app_grants`: `app_id`, `subject_type`, `subject_id`, and `holder_key` (unique), which is `<app_id>/<subject_type>/<subject_id>`.

Uniqueness is declared as column constraints (`unique` tag), and the columns carry no default tags. With the SQLite driver in use, a unique index, a string default, or a unique column that also sits in an ordinary index makes `AutoMigrate` rebuild the table on every startup. `holder_key` exists for that reason: it stands in for a unique index over three columns. Declared this way a restart issues no DDL for these tables on any of the three databases, and `model/cuby_app_test.go` asserts it.

Not here yet: giving apps through a subscription plan. The answer format has room for it (one more `sources[].type`).

## Database verification

Run on 17 Nov 2026 against SQLite (the built-in driver), MySQL 8.4.13 and PostgreSQL 16.15.

**Contract test** (schema added to an existing database without touching its rows, no DDL on a second startup, both unique constraints, the account answer, grant counts, the grant list, delete with grants):

```
TEST_MYSQL_DSN='…' TEST_POSTGRES_DSN='…' go test ./model -run TestCubyAppDatabaseContract -count=1 -v
```

Result: `TestCubyAppDatabaseContractSQLite`, `…ConfiguredDatabases/mysql` and `…ConfiguredDatabases/postgres` pass.

**Startup migration with the real binary**, per engine:

1. The released `v2.0.1` binary creates the database and runs first-time setup, so the database holds an account the release wrote.
2. This branch's binary starts on that database, twice.
3. This branch's binary starts on an empty database.

Result on all three engines: every start answered `/api/status` with no fatal or panic line in its log; the account and the setup record written by the release were unchanged; `cuby_apps` and `cuby_app_grants` appeared on the first upgraded start; their schema was byte-identical after the second; the empty database got both tables with their indexes and unique constraints.

The log database is not involved: both tables live in the main database only.
