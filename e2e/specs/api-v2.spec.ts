/*
Aggregated (onboarding) API tests for the /api/v2 prefix — the
CubeRouter onboarding surface from the deck "API Onboarding Specification
v4.5" (POST /api/v2/plans, /api/v2/users, .../suspend, .../reactivate,
.../adjust-quota, .../bind-subscription, .../delete, GET .../status).

The same handlers are also mounted on /api/v1; that contract surface is
covered separately by specs/api-v1.spec.ts, so each prefix keeps its own
coverage and report. Requires an initialized deployment (e2e/global-setup.ts).
*/
import { defineAggregatedApiSuite } from './lib/aggregated-api-suite'

defineAggregatedApiSuite('/api/v2')
