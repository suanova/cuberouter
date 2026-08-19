/*
Aggregated (onboarding) API tests for the /api/v1 prefix. /api/v1 and
/api/v2 mount the identical aggregated handlers (controller/aggregated_api.go,
see router/api-router.go), but they are distinct public contract surfaces, so
each prefix is tested by its own spec file: api-v2.spec.ts covers /api/v2,
this file covers /api/v1. Requires an initialized deployment
(e2e/global-setup.ts).
*/
import { defineAggregatedApiSuite } from './lib/aggregated-api-suite'

defineAggregatedApiSuite('/api/v1')
