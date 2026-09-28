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

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestImageStudioPreservesUnicodePromptBody(t *testing.T) {
	prompt := "將花變成藍色，加入「中秋快樂」與 \"Hello\"。\n保留 {{原文}}、\\、🌕。\n" + strings.Repeat("尾段不可遺失", 1000)
	body, err := common.Marshal(map[string]any{"model": "2.1", "mode": "edit", "prompt": prompt, "images": []string{"first", "second"}})
	require.NoError(t, err)
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Error(readErr)
		}
		received <- data
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("{}"))
	}))
	t.Cleanup(server.Close)
	t.Setenv("IMAGE_STUDIO_DISPATCHER_URL", server.URL)
	t.Setenv("IMAGE_STUDIO_DISPATCHER_TOKEN", strings.Repeat("x", 32))
	require.Equal(t, 202, studioRequest("POST", "/jobs", string(body), 7).Code)
	select {
	case actual := <-received:
		require.Equal(t, body, actual)
	default:
		t.Fatal("request did not reach dispatcher")
	}
}

func studioRequest(method, path, body string, owner int) *httptest.ResponseRecorder {
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("id", owner) })
	router.Any("/studio/*path", ImageStudio)
	request := httptest.NewRequest(method, "/studio"+path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer browser-secret-must-not-forward")
	request.Header.Set("Cookie", "session=private")
	request.Header.Set("X-Image-Owner", "cuberouter-999")
	request.Header.Set("Idempotency-Key", "same-request-123456")
	request.Header.Set("X-Trial-Epoch", "test-epoch")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestImageStudioVerifiedIdentityAndBoundaries(t *testing.T) {
	const secret = "dispatcher-secret-at-least-32-characters"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Assertions occur on a server goroutine, so avoid FailNow here.
		if r.Header.Get("Authorization") != "Bearer "+secret || r.Header.Get("X-Image-Owner") != "cuberouter-7" || r.Header.Get("Cookie") != "" {
			t.Error("browser credentials or unverified owner reached dispatcher")
		}
		if r.Header.Get("Idempotency-Key") != "same-request-123456" || r.Header.Get("X-Trial-Epoch") != "test-epoch" {
			t.Error("recovery headers lost")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/jobs" {
			data, err := io.ReadAll(r.Body)
			if err != nil || string(data) != "{}" {
				t.Error("request body changed")
			}
			w.WriteHeader(http.StatusAccepted)
		}
		_, _ = w.Write([]byte("{\"ok\":true}"))
	}))
	t.Cleanup(upstream.Close)
	t.Setenv("IMAGE_STUDIO_DISPATCHER_URL", upstream.URL)
	t.Setenv("IMAGE_STUDIO_DISPATCHER_TOKEN", secret)
	for _, tc := range []struct {
		name, method, path, body string
		owner, status            int
	}{
		{"session", "GET", "/session", "", 7, 200},
		{"submit", "POST", "/jobs", "{}", 7, 202},
		{"anonymous", "GET", "/session", "", 0, 401},
		{"no-admin-proxy", "GET", "/admin", "", 7, 404},
		{"no-query-owner", "GET", "/session?owner=cuberouter-999", "", 7, 404},
		{"no-arbitrary-job", "GET", "/jobs/../session", "", 7, 404},
		{"oversized-body", "POST", "/jobs", strings.Repeat("a", imageStudioBodyLimit+1), 7, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			response := studioRequest(tc.method, tc.path, tc.body, tc.owner)
			require.Equal(t, tc.status, response.Code)
			require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
		})
	}
}

func TestImageStudioDisabledByDefault(t *testing.T) {
	t.Setenv("IMAGE_STUDIO_DISPATCHER_URL", "")
	t.Setenv("IMAGE_STUDIO_DISPATCHER_TOKEN", "")
	require.Equal(t, 503, studioRequest("GET", "/session", "", 7).Code)
}

func TestImageStudioRejectsRedirectAndPrivateError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/session" {
			w.Header().Set("Location", "http://private-infra/secret")
			w.WriteHeader(302)
		} else {
			w.WriteHeader(500)
		}
		_, _ = w.Write([]byte("private-infra-token"))
	}))
	defer server.Close()
	t.Setenv("IMAGE_STUDIO_DISPATCHER_URL", server.URL)
	t.Setenv("IMAGE_STUDIO_DISPATCHER_TOKEN", strings.Repeat("x", 32))
	for _, path := range []string{"/session", "/pools"} {
		result := studioRequest("GET", path, "", 7)
		require.Equal(t, 502, result.Code)
		require.NotContains(t, result.Body.String(), "private-infra")
	}
}
