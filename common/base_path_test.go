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
package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeBasePath(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "empty is the site root", raw: "", want: ""},
		{name: "a lone slash is the site root", raw: "/", want: ""},
		{name: "a bare name gains a leading slash", raw: "cuberouter", want: "/cuberouter"},
		{name: "a trailing slash is trimmed", raw: "/cuberouter/", want: "/cuberouter"},
		{name: "an already normalised value is unchanged", raw: "/cuberouter", want: "/cuberouter"},
		{name: "multiple segments are kept", raw: "/g/w1", want: "/g/w1"},
		{name: "dots dashes and underscores are allowed", raw: "/my.app-v2_x", want: "/my.app-v2_x"},
		{name: "sub-delims are allowed", raw: "/team+1", want: "/team+1"},
		{name: "a complete percent escape is allowed", raw: "/my%20site", want: "/my%20site"},

		{name: "an absolute URL is rejected", raw: "https://host/cuberouter", wantErr: true},
		{name: "a protocol-relative URL is rejected", raw: "//host/cuberouter", wantErr: true},
		{name: "a leading double slash is rejected", raw: "///cuberouter///", wantErr: true},
		// The prefix is interpolated into the served HTML, so a value that can
		// close the inline <script> element must never reach it.
		{name: "markup is rejected", raw: "/</script><script>alert(1)</script>", wantErr: true},
		{name: "a bare angle bracket is rejected", raw: "/a<b", wantErr: true},
		{name: "a quote is rejected", raw: `/a"b`, wantErr: true},
		{name: "a backslash is rejected", raw: `/a\b`, wantErr: true},
		{name: "a space is rejected", raw: "/a b", wantErr: true},
		{name: "a query is rejected", raw: "/a?b=1", wantErr: true},
		{name: "a fragment is rejected", raw: "/a#b", wantErr: true},
		{name: "a line break is rejected", raw: "/a\nb", wantErr: true},
		{name: "an empty segment is rejected", raw: "/a//b", wantErr: true},
		{name: "an incomplete percent escape is rejected", raw: "/a%2", wantErr: true},
		{name: "a non-hex percent escape is rejected", raw: "/a%zz", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeBasePath(tt.raw)
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, got, "a rejected value must not produce a prefix")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInitBasePath(t *testing.T) {
	t.Cleanup(func() { basePath = "" })

	t.Run("a valid prefix is normalised and stored", func(t *testing.T) {
		t.Setenv("BASE_PATH", " /cuberouter/ ")
		InitBasePath()
		assert.Equal(t, "/cuberouter", BasePath())
	})

	t.Run("an empty value means the site root", func(t *testing.T) {
		basePath = "/stale"
		t.Setenv("BASE_PATH", "")
		InitBasePath()
		assert.Empty(t, BasePath())
	})

	t.Run("input that would break the served HTML falls back to the site root", func(t *testing.T) {
		basePath = "/stale"
		t.Setenv("BASE_PATH", "/</script><script>alert(1)</script>")
		InitBasePath()
		assert.Empty(t, BasePath())
	})
}

func TestWithBasePath(t *testing.T) {
	t.Cleanup(func() { basePath = "" })

	basePath = ""
	assert.Equal(t, "/api/status", WithBasePath("/api/status"),
		"with no prefix configured the path is returned unchanged")

	basePath = "/g/w1"
	assert.Equal(t, "/g/w1/api/status", WithBasePath("/api/status"),
		"multi-segment prefixes are applied whole")
	assert.Equal(t, "/g/w1/", WithBasePath("/"))
}

func TestStripBasePath(t *testing.T) {
	t.Cleanup(func() { basePath = "" })

	tests := []struct {
		name        string
		prefix      string
		target      string
		wantPath    string
		wantRawPath string
		wantURI     string
	}{
		{
			name:     "a relay call loses the prefix",
			prefix:   "/cuberouter",
			target:   "/cuberouter/v1/chat/completions",
			wantPath: "/v1/chat/completions",
			wantURI:  "/v1/chat/completions",
		},
		{
			name:     "the query string survives",
			prefix:   "/cuberouter",
			target:   "/cuberouter/v1/models?limit=10&after=abc",
			wantPath: "/v1/models",
			wantURI:  "/v1/models?limit=10&after=abc",
		},
		{
			name:     "a nested prefix is removed whole",
			prefix:   "/g/w1",
			target:   "/g/w1/api/status",
			wantPath: "/api/status",
			wantURI:  "/api/status",
		},
		{
			name:     "the bare prefix becomes the root route",
			prefix:   "/cuberouter",
			target:   "/cuberouter",
			wantPath: "/",
			wantURI:  "/",
		},
		{
			name:     "a trailing slash after the prefix also becomes the root route",
			prefix:   "/cuberouter",
			target:   "/cuberouter/",
			wantPath: "/",
			wantURI:  "/",
		},
		{
			name:     "the dashboard's static assets lose the prefix too",
			prefix:   "/cuberouter",
			target:   "/cuberouter/static/js/index.js",
			wantPath: "/static/js/index.js",
			wantURI:  "/static/js/index.js",
		},
		{
			// The boundary is the whole point: a sibling path that merely starts
			// with the prefix's characters is not ours to rewrite.
			name:     "a path that only starts with the prefix's characters is left alone",
			prefix:   "/cuberouter",
			target:   "/cuberouterfoo/v1/models",
			wantPath: "/cuberouterfoo/v1/models",
			wantURI:  "/cuberouterfoo/v1/models",
		},
		{
			// Tolerant by design: something upstream may have stripped already,
			// and the app must still serve that request.
			name:     "an already root-mounted request passes through",
			prefix:   "/cuberouter",
			target:   "/v1/chat/completions",
			wantPath: "/v1/chat/completions",
			wantURI:  "/v1/chat/completions",
		},
		{
			name:     "an unrelated path passes through",
			prefix:   "/cuberouter",
			target:   "/api/status",
			wantPath: "/api/status",
			wantURI:  "/api/status",
		},
		{
			name:     "the root passes through",
			prefix:   "/cuberouter",
			target:   "/",
			wantPath: "/",
			wantURI:  "/",
		},
		{
			name:     "with no prefix configured nothing is rewritten",
			prefix:   "",
			target:   "/cuberouter/v1/models",
			wantPath: "/cuberouter/v1/models",
			wantURI:  "/cuberouter/v1/models",
		},
		{
			// %2F decodes to a slash in Path but must survive in RawPath, or the
			// segment structure of the relay path changes under the client.
			name:        "an escaped slash keeps its escaping",
			prefix:      "/cuberouter",
			target:      "/cuberouter/v1/a%2Fb",
			wantPath:    "/v1/a/b",
			wantRawPath: "/v1/a%2Fb",
			wantURI:     "/v1/a%2Fb",
		},
		{
			// A prefix carrying an escape is configured escaped but compared
			// decoded, because that is the form Path is in. RawPath stays empty
			// because the decoded path's default escaping is already the raw one.
			name:     "a prefix containing an escape is matched decoded",
			prefix:   "/my%20site",
			target:   "/my%20site/api/status",
			wantPath: "/api/status",
			wantURI:  "/api/status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			basePath = tt.prefix

			var got *http.Request
			handler := StripBasePath(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r
			}))

			req := httptest.NewRequest(http.MethodPost, tt.target, nil)
			handler.ServeHTTP(httptest.NewRecorder(), req)

			require.NotNil(t, got, "the wrapped handler must be reached")
			assert.Equal(t, tt.wantPath, got.URL.Path)
			assert.Equal(t, tt.wantRawPath, got.URL.RawPath)
			assert.Equal(t, tt.wantURI, got.RequestURI)
			assert.Equal(t, http.MethodPost, got.Method, "only the path is rewritten")

			assert.Equal(t, tt.target, req.RequestURI,
				"the caller's request must not be mutated in place")
		})
	}
}
