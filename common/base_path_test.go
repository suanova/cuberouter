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
