/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package router

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOrganizationCanonicalRestfulRoutesAreRegistered(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NotPanics(t, func() {
		SetApiRouter(r)
	})

	routes := registeredRouteSet(r)

	require.Contains(t, routes, "GET /api/account-contexts")
	require.Contains(t, routes, "PUT /api/account-contexts/current")
	require.Contains(t, routes, "GET /api/organizations")
	require.Contains(t, routes, "POST /api/organizations")
	require.Contains(t, routes, "GET /api/organizations/self")
	require.Contains(t, routes, "GET /api/organizations/:id")
	require.Contains(t, routes, "PATCH /api/organizations/:id")
	require.Contains(t, routes, "DELETE /api/organizations/:id")
	require.Contains(t, routes, "PUT /api/organizations/:id/owner")
	require.Contains(t, routes, "GET /api/organizations/:id/members")
	require.Contains(t, routes, "PATCH /api/organizations/:id/members/:userId")
	require.Contains(t, routes, "DELETE /api/organizations/:id/members/me")
	require.Contains(t, routes, "DELETE /api/organizations/:id/members/:userId")
	require.Contains(t, routes, "GET /api/organizations/:id/invitations")
	require.Contains(t, routes, "POST /api/organizations/:id/invitations")
	require.Contains(t, routes, "DELETE /api/organizations/:id/invitations/:invitationId")
	require.Contains(t, routes, "GET /api/organization-invitations/:token")
	require.Contains(t, routes, "PATCH /api/organization-invitations/:token")
	require.Contains(t, routes, "GET /api/organizations/:id/tokens")
	require.Contains(t, routes, "POST /api/organizations/:id/tokens")
	require.Contains(t, routes, "POST /api/organizations/:id/token-batches")
	require.Contains(t, routes, "POST /api/organizations/:id/token-deletions")
	require.Contains(t, routes, "GET /api/organizations/:id/tokens/:tokenId")
	require.Contains(t, routes, "PATCH /api/organizations/:id/tokens/:tokenId")
	require.Contains(t, routes, "DELETE /api/organizations/:id/tokens/:tokenId")
	require.Contains(t, routes, "PATCH /api/organizations/:id/tokens/:tokenId/responsible-user")
	require.Contains(t, routes, "GET /api/organizations/:id/quota-data")
	require.Contains(t, routes, "GET /api/organizations/:id/logs")
	require.Contains(t, routes, "GET /api/organizations/:id/logs/stats")
	require.Contains(t, routes, "GET /api/organizations/:id/midjourney-tasks")
	require.Contains(t, routes, "GET /api/organizations/:id/billing/summary")
	require.Contains(t, routes, "GET /api/organizations/:id/billing/members/me")
	require.Contains(t, routes, "GET /api/organizations/:id/billing/user-summaries")
	require.Contains(t, routes, "GET /api/organizations/:id/billing/monthly-summaries")
	require.Contains(t, routes, "GET /api/organizations/:id/billing/records")
	require.Contains(t, routes, "GET /api/organizations/:id/audit-logs")
	require.Contains(t, routes, "GET /api/admin/organizations")
	require.Contains(t, routes, "GET /api/admin/organizations/:id")
	require.Contains(t, routes, "PATCH /api/admin/organizations/:id")
	require.Contains(t, routes, "PATCH /api/admin/organizations/:id/status")
	require.Contains(t, routes, "DELETE /api/admin/organizations/:id")
	require.Contains(t, routes, "PUT /api/admin/organizations/:id/owner")
	require.Contains(t, routes, "POST /api/admin/organizations/:id/quota-adjustments")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/members")
	require.Contains(t, routes, "POST /api/admin/organizations/:id/members")
	require.Contains(t, routes, "PATCH /api/admin/organizations/:id/members/:userId")
	require.Contains(t, routes, "DELETE /api/admin/organizations/:id/members/:userId")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/tokens")
	require.NotContains(t, routes, "POST /api/admin/organizations/:id/tokens")
	require.NotContains(t, routes, "POST /api/admin/organizations/:id/token-batches")
	require.Contains(t, routes, "POST /api/admin/organizations/:id/token-deletions")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/tokens/:tokenId")
	require.Contains(t, routes, "PATCH /api/admin/organizations/:id/tokens/:tokenId")
	require.Contains(t, routes, "DELETE /api/admin/organizations/:id/tokens/:tokenId")
	require.Contains(t, routes, "PATCH /api/admin/organizations/:id/tokens/:tokenId/responsible-user")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/logs")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/logs/stats")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/tasks")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/midjourney-tasks")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/quota-data")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/billing/summary")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/billing/user-summaries")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/billing/monthly-summaries")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/billing/records")
	require.Contains(t, routes, "GET /api/admin/organizations/:id/audit-logs")
}

func TestOrganizationLegacyRoutesAreRemoved(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NotPanics(t, func() {
		SetApiRouter(r)
	})

	routes := registeredRouteSet(r)

	for route := range routes {
		method, path, _ := strings.Cut(route, " ")
		if strings.HasPrefix(path, "/api/organization-invites") {
			t.Fatalf("legacy organization invite route restored: %s", route)
		}
		if strings.HasPrefix(path, "/api/organizations/history") {
			t.Fatalf("legacy organization history route restored: %s", route)
		}
		if path == "/api/organization-invitations/:token" && method == "POST" {
			t.Fatalf("legacy invitation acceptance method restored: %s", route)
		}
		if !isOrganizationRoutePath(path) {
			continue
		}
		for _, fragment := range []string{"/disable", "/enable", "/owner-transfer", "/leave", "/resend", "/acceptances", "/audits"} {
			require.NotContains(t, path, fragment, "legacy organization route restored: %s", route)
		}
	}

	require.NotContains(t, routes, "GET /api/account-contexts/")
	require.NotContains(t, routes, "GET /api/organizations/")
	require.NotContains(t, routes, "POST /api/organizations/")
	require.NotContains(t, routes, "GET /api/admin/organizations/")
	require.NotContains(t, routes, "PUT /api/organizations/:id")
	require.NotContains(t, routes, "POST /api/organizations/:id/members")
	require.NotContains(t, routes, "POST /api/organizations/:id/tokens/batch")
	require.NotContains(t, routes, "POST /api/organizations/:id/tokens/batch-deletions")
	require.NotContains(t, routes, "PATCH /api/organizations/:id/tokens/:tokenId/responsibility")
	require.NotContains(t, routes, "PUT /api/organizations/:id/tokens/:tokenId")
	require.NotContains(t, routes, "GET /api/organizations/:id/data")
	require.NotContains(t, routes, "GET /api/organizations/:id/log-stats")
	require.NotContains(t, routes, "GET /api/organizations/:id/billing-summary")
	require.NotContains(t, routes, "GET /api/organizations/:id/member-billing/me")
	require.NotContains(t, routes, "GET /api/organizations/:id/billing/details")
	require.NotContains(t, routes, "GET /api/organizations/:id/audit")
	require.NotContains(t, routes, "GET /api/organizations/:id/invites")
	require.NotContains(t, routes, "POST /api/organizations/:id/invites")
	require.NotContains(t, routes, "DELETE /api/organizations/:id/invites/:inviteId")
}

func isOrganizationRoutePath(path string) bool {
	return strings.HasPrefix(path, "/api/organizations") ||
		strings.HasPrefix(path, "/api/organization-invitations") ||
		strings.HasPrefix(path, "/api/admin/organizations")
}

func registeredRouteSet(r *gin.Engine) map[string]struct{} {
	routes := make(map[string]struct{})
	for _, route := range r.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	return routes
}
