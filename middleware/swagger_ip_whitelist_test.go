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

	"github.com/QuantumNous/new-api/setting/security_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSwaggerAPIIPWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)

	setting := security_setting.GetSecuritySetting()
	original := setting.SwaggerAPIIPWhitelist
	defer func() { setting.SwaggerAPIIPWhitelist = original }()

	tests := []struct {
		name       string
		whitelist  []string
		remoteAddr string
		wantStatus int
	}{
		{
			name:       "empty whitelist allows any client",
			whitelist:  []string{},
			remoteAddr: "203.0.113.7:1234",
			wantStatus: http.StatusOK,
		},
		{
			name:       "listed IP passes",
			whitelist:  []string{"203.0.113.7"},
			remoteAddr: "203.0.113.7:1234",
			wantStatus: http.StatusOK,
		},
		{
			name:       "IP inside listed CIDR passes",
			whitelist:  []string{"203.0.113.0/24"},
			remoteAddr: "203.0.113.9:1234",
			wantStatus: http.StatusOK,
		},
		{
			name:       "IP outside whitelist is rejected",
			whitelist:  []string{"198.51.100.0/24", "203.0.113.7"},
			remoteAddr: "203.0.113.8:1234",
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting.SwaggerAPIIPWhitelist = tt.whitelist

			engine := gin.New()
			require.NoError(t, engine.SetTrustedProxies(nil))
			engine.GET("/probe", SwaggerAPIIPWhitelist(), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/probe", nil)
			request.RemoteAddr = tt.remoteAddr
			engine.ServeHTTP(recorder, request)

			assert.Equal(t, tt.wantStatus, recorder.Code)
		})
	}
}
