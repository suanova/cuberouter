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
	"fmt"
	"os"
	"strings"
)

// basePath is the URL path prefix the dashboard is published under, normalised
// to either "" (served from the site root) or "/prefix" with no trailing slash.
//
// It comes from BASE_PATH and exists because a reverse proxy may publish this
// server under a subpath. That proxy strips the prefix before forwarding, so
// the HTTP server never sees it: c.Request.URL.Path is identical to a
// root-mounted deployment, and no routing, middleware or rate-limit logic has
// to change. Only the two things the server says to the browser are affected --
// the HTML it serves and the Path attribute of the refresh cookie -- because
// those are evaluated by the browser against the prefixed URL.
var basePath string

// InitBasePath reads and validates BASE_PATH and stores the normalised prefix.
// An empty value or "/" means the app is served from the site root, which is
// the default and makes BasePath and WithBasePath no-ops.
//
// An unusable value (an absolute URL, a query, a line break) is rejected rather
// than guessed at: silently accepting "https://host/prefix" would produce
// double-prefixed URLs that are far harder to diagnose than a startup warning.
func InitBasePath() {
	raw := strings.TrimSpace(os.Getenv("BASE_PATH"))
	normalized, err := normalizeBasePath(raw)
	if err != nil {
		SysError(fmt.Sprintf("BASE_PATH %q is invalid (%s); serving from the site root instead", raw, err.Error()))
		normalized = ""
	}
	if normalized != "" {
		SysLog("dashboard is published under the URL prefix " + normalized)
	}
	basePath = normalized
}

func normalizeBasePath(raw string) (string, error) {
	if raw == "" || raw == "/" {
		return "", nil
	}
	if strings.Contains(raw, "://") || strings.HasPrefix(raw, "//") {
		return "", fmt.Errorf("expected a URL path such as /gateway, not an absolute URL")
	}
	if strings.ContainsAny(raw, "?#\r\n") {
		return "", fmt.Errorf("must not contain a query, fragment or line break")
	}
	trimmed := strings.Trim(raw, "/")
	if trimmed == "" {
		return "", nil
	}
	return "/" + trimmed, nil
}

// BasePath returns the normalised prefix, or "" when the app is served from the
// site root.
func BasePath() string {
	return basePath
}

// WithBasePath prefixes a root-absolute path with the configured BASE_PATH,
// returning it unchanged when no prefix is configured. Callers that may already
// hold a prefixed value must not use this: it does not detect double prefixing.
func WithBasePath(path string) string {
	if basePath == "" {
		return path
	}
	return basePath + path
}
