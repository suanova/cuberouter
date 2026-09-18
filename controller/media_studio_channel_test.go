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
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudioChannelReadiness(t *testing.T) {
	ready := `{"models":{"create":"qwen-image-2512","edit":"qwen-image-edit-2511"},"health":{"create":"ready","edit":"ready","tools":"ready"}}`
	cases := []struct {
		name, modelName, endpoint, body string
		status                          int
		wantError                       string
	}{
		{name: "default create checks readiness without generating", body: ready, status: 200},
		{name: "edit checks readiness without uploading a reference", modelName: "qwen-image-edit-2511", body: ready, status: 200},
		{name: "image endpoint checks readiness", endpoint: "image-generation", body: ready, status: 200},
		{name: "wrong channel key fails", body: `{"error":"unauthorized"}`, status: 401, wantError: "HTTP 401"},
		{name: "chat endpoint gives actionable error", endpoint: "openai", body: ready, status: 200, wantError: "Select Auto detect"},
		{name: "missing model fails", body: `{"models":{},"health":{"tools":"ready"}}`, status: 200, wantError: "not available"},
		{name: "offline model fails", body: strings.Replace(ready, `"create":"ready"`, `"create":"unavailable"`, 1), status: 200, wantError: "not ready"},
		{name: "offline tools fail", body: strings.Replace(ready, `"tools":"ready"`, `"tools":"unavailable"`, 1), status: 200, wantError: "not ready"},
		{name: "malformed response fails", body: `<html>proxy error</html>`, status: 200, wantError: "invalid readiness"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/studio/7/config", r.URL.Path)
				assert.Equal(t, "Bearer selected-channel-key", r.Header.Get("Authorization"))
				assert.Equal(t, int64(0), r.ContentLength)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer upstream.Close()
			t.Setenv("MEDIA_STUDIO_BRIDGE_URL", upstream.URL)
			t.Setenv("MEDIA_STUDIO_BRIDGE_KEY", "different-workflow-key-must-not-be-used")
			channel := &model.Channel{Id: 9, Type: constant.ChannelTypeOpenAI, Key: "selected-channel-key", BaseURL: common.GetPointer(upstream.URL), Models: "qwen-image-2512,qwen-image-edit-2511"}
			result := testChannel(context.Background(), channel, 7, tc.modelName, tc.endpoint, false)
			assert.Equal(t, "studio-readiness", result.testMode)
			require.NotNil(t, result.context)
			if tc.wantError != "" {
				require.ErrorContains(t, result.localErr, tc.wantError)
				require.NotNil(t, result.newAPIError, "automatic health checks must also see failures")
			} else {
				require.NoError(t, result.localErr)
				require.Nil(t, result.newAPIError)
			}
			expectedCalls := int32(1)
			if tc.endpoint == "openai" {
				expectedCalls = 0
			}
			assert.Equal(t, expectedCalls, calls.Load())
		})
	}
}

func TestStudioChannelReadinessDoesNotFollowRedirects(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	t.Setenv("MEDIA_STUDIO_BRIDGE_URL", origin.URL)
	channel := &model.Channel{Type: constant.ChannelTypeOpenAI, Key: "private", BaseURL: common.GetPointer(origin.URL), Models: "qwen-image-2512"}
	result := testChannel(context.Background(), channel, 7, "", "", false)
	require.ErrorContains(t, result.localErr, "HTTP 307")
	assert.Zero(t, redirected.Load())
}
