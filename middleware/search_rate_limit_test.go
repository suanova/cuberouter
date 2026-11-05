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
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSearchRateLimitWhenQueryOnlyThrottlesSearchRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousEnable := common.SearchRateLimitEnable
	previousNum := common.SearchRateLimitNum
	previousDuration := common.SearchRateLimitDuration
	previousRedis := common.RedisEnabled
	common.RedisEnabled = false
	common.SearchRateLimitEnable = true
	common.SearchRateLimitNum = 1
	common.SearchRateLimitDuration = 60
	t.Cleanup(func() {
		common.SearchRateLimitEnable = previousEnable
		common.SearchRateLimitNum = previousNum
		common.SearchRateLimitDuration = previousDuration
		common.RedisEnabled = previousRedis
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", 987654321)
	})
	router.GET("/keys", SearchRateLimitWhenQuery(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := func(target string) int {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		return recorder.Code
	}

	// Pagination is not a search and must not consume the search budget.
	assert.Equal(t, http.StatusNoContent, request("/keys?p=1&size=20"))
	assert.Equal(t, http.StatusNoContent, request("/keys?p=2&size=20"))

	assert.Equal(t, http.StatusNoContent, request("/keys?keyword=acme"))
	assert.Equal(t, http.StatusTooManyRequests, request("/keys?token=sk-acme"))
}
