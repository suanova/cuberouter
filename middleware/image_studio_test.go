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
package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestImageStudioUserBudgetIsSeparatePerAccount(t *testing.T) {
	previous := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previous })
	owner := time.Now().UnixNano()
	// These are verified IDs injected by the test; production uses UserAuth.
	router := gin.New()
	router.Use(func(c *gin.Context) { id, _ := strconv.Atoi(c.GetHeader("Test-User")); c.Set("id", id) })
	router.Use(ImageStudioUserRateLimit())
	router.GET("/pools", func(c *gin.Context) { c.Status(http.StatusOK) })
	call := func(owner string) int {
		req := httptest.NewRequest("GET", "/pools", nil)
		req.Header.Set("Test-User", owner)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response.Code
	}
	for i := 0; i < 120; i++ {
		require.Equal(t, 200, call(strconv.FormatInt(owner, 10)))
	}
	require.Equal(t, 429, call(strconv.FormatInt(owner, 10)))
	require.Equal(t, 200, call(strconv.FormatInt(owner+1, 10)))
	require.Equal(t, 401, call("0"))
}
