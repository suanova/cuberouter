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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudioProxyScopesRequestsToAuthenticatedAccount(t *testing.T) {
	var seenPath, seenKey string
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath, seenKey = r.URL.Path, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "private=secret")
		_, _ = io.WriteString(w, `{"id":"example"}`)
	}))
	t.Cleanup(worker.Close)
	t.Setenv("MEDIA_STUDIO_BRIDGE_URL", worker.URL)
	t.Setenv("MEDIA_STUDIO_BRIDGE_KEY", strings.Repeat("k", 32))
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("id", 7) })
	router.GET("/api/media-studio/*resource", MediaStudio)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/media-studio/jobs", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "/studio/7/jobs", seenPath)
	assert.Equal(t, "Bearer "+strings.Repeat("k", 32), seenKey)
	assert.Empty(t, response.Header().Get("Set-Cookie"))
	assert.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
}

func TestStudioProxyRejectsPrivateOperationsAndAlternateOwners(t *testing.T) {
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("id", 7) })
	router.Any("/api/media-studio/*resource", MediaStudio)
	for _, path := range []string{"/jobs/" + strings.Repeat("a", 32) + "/settle", "/studio/8/jobs", "/jobs?owner=8", "/media/raw.png", "/regional"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/media-studio"+path, strings.NewReader(`{}`)))
			assert.Equal(t, http.StatusNotFound, response.Code)
		})
	}
}

func TestStudioRelayDisablesAutomaticRetry(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("media_studio_relay", true)
	err := types.NewErrorWithStatusCode(io.ErrUnexpectedEOF, types.ErrorCodeDoRequestFailed, http.StatusBadGateway)
	assert.False(t, shouldRetry(c, err, 3))
}
