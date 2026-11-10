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
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/security_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSwaggerAPIIPWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)

	setting := security_setting.GetSecuritySetting()
	original := setting.SwaggerAPIIPWhitelist
	t.Cleanup(func() { setting.SwaggerAPIIPWhitelist = original })

	tests := []struct {
		name           string
		trustedProxies string
		whitelist      []string
		remoteAddr     string
		forwardedFor   string
		wantStatus     int
	}{
		{
			name:         "empty whitelist allows any client",
			whitelist:    []string{},
			remoteAddr:   "203.0.113.7:1234",
			forwardedFor: "198.51.100.9",
			wantStatus:   http.StatusOK,
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
		{
			name:       "direct private client still passes without forwarding",
			whitelist:  []string{"10.1.2.3"},
			remoteAddr: "10.1.2.3:1234",
			wantStatus: http.StatusOK,
		},
		{
			name:         "forwarded IP is rejected under compatibility defaults",
			whitelist:    []string{"203.0.113.7"},
			remoteAddr:   "10.1.2.3:1234",
			forwardedFor: "203.0.113.7",
			wantStatus:   http.StatusForbidden,
		},
		{
			name:           "forwarded IP is honored with explicit trusted proxies",
			trustedProxies: "10.0.0.0/8",
			whitelist:      []string{"203.0.113.7"},
			remoteAddr:     "10.1.2.3:1234",
			forwardedFor:   "203.0.113.7",
			wantStatus:     http.StatusOK,
		},
		{
			name:           "TRUSTED_PROXIES=none ignores forwarded IPs",
			trustedProxies: "none",
			whitelist:      []string{"203.0.113.7"},
			remoteAddr:     "10.1.2.3:1234",
			forwardedFor:   "203.0.113.7",
			wantStatus:     http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TRUSTED_PROXIES", tt.trustedProxies)
			setting.SwaggerAPIIPWhitelist = tt.whitelist

			trustedProxies, _, err := common.ResolveTrustedProxies(os.Getenv("TRUSTED_PROXIES"))
			require.NoError(t, err)
			engine := gin.New()
			require.NoError(t, engine.SetTrustedProxies(trustedProxies))
			engine.GET("/probe", SwaggerAPIIPWhitelist(), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			request := httptest.NewRequest(http.MethodGet, "/probe", nil)
			request.RemoteAddr = tt.remoteAddr
			if tt.forwardedFor != "" {
				request.Header.Set("X-Forwarded-For", tt.forwardedFor)
			}
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)

			assert.Equal(t, tt.wantStatus, recorder.Code)
		})
	}
}
