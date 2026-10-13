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
package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 前端页头的站点切换入口读取 /api/status 的 site_counterpart_enabled/url/label,
// 这三个字段必须与后台配置的站点互跳设置一致：开关决定显示与否,地址与文案决定指向与文字,
// 任一字段漏下发都会让切换入口要么不显示要么指向错误地址。
func TestGetStatusExposesSiteCounterpartSettings(t *testing.T) {
	settings := system_setting.GetSiteSettings()
	previousEnabled := settings.CounterpartEnabled
	previousURL := settings.CounterpartURL
	previousLabel := settings.CounterpartLabel
	t.Cleanup(func() {
		settings.CounterpartEnabled = previousEnabled
		settings.CounterpartURL = previousURL
		settings.CounterpartLabel = previousLabel
	})

	for _, testCase := range []struct {
		name    string
		enabled bool
	}{
		{name: "enabled", enabled: true},
		{name: "disabled", enabled: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			settings.CounterpartEnabled = testCase.enabled
			settings.CounterpartURL = "https://cuberouter.com"
			settings.CounterpartLabel = "香港 · 国际"

			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)

			GetStatus(context)

			var payload struct {
				Success bool           `json:"success"`
				Data    map[string]any `json:"data"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
			require.True(t, payload.Success)
			assert.Equal(t, testCase.enabled, payload.Data["site_counterpart_enabled"])
			assert.Equal(t, "https://cuberouter.com", payload.Data["site_counterpart_url"])
			assert.Equal(t, "香港 · 国际", payload.Data["site_counterpart_label"])
		})
	}
}
