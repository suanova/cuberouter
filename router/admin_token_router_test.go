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
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminTokenRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Parallel()

	r := gin.New()
	require.NotPanics(t, func() {
		SetApiRouter(r)
	})

	routes := registeredRouteSet(r)
	for _, route := range []string{
		"GET /api/admin/token/",
		"POST /api/admin/token/:id/key",
		"PUT /api/admin/token/:id/status",
		"DELETE /api/admin/token/:id",
	} {
		require.Contains(t, routes, route)
	}
}
