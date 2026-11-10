/*
Aggregated (onboarding) API tests for the /api/v2 prefix — the
CubeRouter onboarding surface from the deck "API Onboarding Specification
v4.5" (POST /api/v2/plans, /api/v2/users, .../suspend, .../reactivate,
.../adjust-quota, .../bind-subscription, .../delete, GET .../status).

The aggregated handlers are mounted only under /api/v2: /api and /api/v1 no
longer expose the same paths, so callers cannot bypass the /api/v2 IP
whitelist. Requires an initialized deployment (e2e/global-setup.ts).
*/
import { defineAggregatedApiSuite } from './lib/aggregated-api-suite'

defineAggregatedApiSuite('/api/v2')
