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

