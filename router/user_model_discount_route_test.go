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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// 路由顺序回归：/api/user/:id/model-discounts（GET 读接口、PUT 写接口）必须命中
// 各自的 handler，不能被 /api/user/:id 通配截获（Gin radix tree 对两者共存处理
// 正确，两种注册顺序均断言正确路由；生产注册顺序见 api-router.go 的
// userReadRoute / adminRoute）。
func TestUserModelDiscountRouteOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)

	run := func(t *testing.T, idFirst bool) {
		r := gin.New()
		user := r.Group("/api/user")
		idHandler := func(c *gin.Context) { c.String(200, "id="+c.Param("id")) }
		getDiscounts := func(c *gin.Context) { c.String(200, "discounts="+c.Param("id")) }
		putDiscounts := func(c *gin.Context) { c.String(200, "update="+c.Param("id")) }
		if idFirst {
			user.GET("/:id", idHandler)
			user.GET("/:id/model-discounts", getDiscounts)
			user.PUT("/:id/model-discounts", putDiscounts)
		} else {
			user.GET("/:id/model-discounts", getDiscounts)
			user.PUT("/:id/model-discounts", putDiscounts)
			user.GET("/:id", idHandler)
		}

		cases := []struct {
			method string
			path   string
			want   string
		}{
			{http.MethodGet, "/api/user/7", "id=7"},
			{http.MethodGet, "/api/user/7/model-discounts", "discounts=7"},
			{http.MethodPut, "/api/user/7/model-discounts", "update=7"},
		}
		for _, tc := range cases {
			recorder := httptest.NewRecorder()
			r.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
			assert.Equal(t, http.StatusOK, recorder.Code, "%s %s", tc.method, tc.path)
			assert.Equal(t, tc.want, recorder.Body.String(), "%s %s", tc.method, tc.path)
		}
	}

	t.Run("id-first (other registration order)", func(t *testing.T) { run(t, true) })
	t.Run("subroute-first (production order)", func(t *testing.T) { run(t, false) })
}

// 生产路由表回归：SetApiRouter 挂载 /api、/api/v1、/api/v2 三前缀，用户模型折扣
// 的读写端点必须在三处都注册成功（注册本身同时也是 Gin radix tree 的冲突检查，
// 冲突会 panic），且不得被 /:id 通配吞掉。
func TestUserModelDiscountRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}

	for _, prefix := range []string{"/api", "/api/v1", "/api/v2"} {
		assert.Containsf(t, routes, http.MethodGet+" "+prefix+"/user/:id/model-discounts",
			"读取用户模型折扣的路由缺失: GET %s/user/:id/model-discounts", prefix)
		assert.Containsf(t, routes, http.MethodPut+" "+prefix+"/user/:id/model-discounts",
			"更新用户模型折扣的路由缺失: PUT %s/user/:id/model-discounts", prefix)
	}
}
