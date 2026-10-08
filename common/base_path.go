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
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// basePath is the URL path prefix the dashboard is published under, normalised
// to either "" (served from the site root) or "/prefix" with no trailing slash.
//
// It comes from BASE_PATH and exists because this server may be published under
// a subpath. A reverse proxy can strip the prefix before forwarding it, and
// StripBasePath does the same in-process, so in both cases the router,
// middleware and rate-limit logic see the root-mounted path and none of them
// has to change. For the same reason the prefix never reaches them, the things
// the server says to the browser are what the prefix is for -- the HTML it
// serves and the Path attribute of the refresh cookie -- because those are
// evaluated by the browser against the prefixed URL.
var basePath string

// InitBasePath reads and validates BASE_PATH and stores the normalised prefix.
// An empty value or "/" means the app is served from the site root, which is
// the default and makes BasePath and WithBasePath no-ops.
//
// An unusable value is rejected rather than guessed at. Silently accepting
// "https://host/prefix" would produce double-prefixed URLs that are far harder
// to diagnose than a startup warning, and accepting markup has a second,
// sharper consequence: the prefix is interpolated into the served HTML, so a
// value containing "</script>" would break out of the inline script that
// carries it. Only URL path characters are allowed for that reason.
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

// normalizeBasePath turns BASE_PATH into either "" (site root) or "/prefix".
//
// The character check is not cosmetic: the normalised value ends up verbatim in
// the dashboard's HTML, in the Path attribute of a cookie, and in every URL the
// SPA builds. Restricting it to the characters that can legitimately appear in a
// path keeps all three honest -- a value carrying "<" or a space would otherwise
// be injected into the page or silently produce URLs the browser rewrites.
func normalizeBasePath(raw string) (string, error) {
	if raw == "" || raw == "/" {
		return "", nil
	}
	if strings.Contains(raw, "://") || strings.HasPrefix(raw, "//") {
		return "", fmt.Errorf("expected a URL path such as /gateway, not an absolute URL")
	}
	trimmed := strings.Trim(raw, "/")
	if trimmed == "" {
		return "", nil
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if !isValidBasePathSegment(segment) {
			return "", fmt.Errorf("%q is not a valid path segment; use only URL path characters such as /gateway or /tenant-1", segment)
		}
	}
	return "/" + trimmed, nil
}

// isValidBasePathSegment reports whether segment can appear verbatim in a URL
// path. The accepted set is RFC 3986 pchar -- unreserved, sub-delims, ":" and
// "@" -- with "%" allowed only as a complete %XX escape. Everything else is
// refused, notably "<", ">", '"', "\", whitespace and control characters.
//
// Empty segments are refused too, so "a//b" cannot sneak past as an ambiguous
// prefix that a browser or proxy might collapse differently than we do.
func isValidBasePathSegment(segment string) bool {
	if segment == "" {
		return false
	}
	for i := 0; i < len(segment); i++ {
		c := segment[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '-', c == '.', c == '_', c == '~':
		case c == '!', c == '$', c == '&', c == '\'', c == '(', c == ')':
		case c == '*', c == '+', c == ',', c == ';', c == '=', c == ':', c == '@':
		case c == '%':
			if i+2 >= len(segment) || !isHexDigit(segment[i+1]) || !isHexDigit(segment[i+2]) {
				return false
			}
			i += 2
		default:
			return false
		}
	}
	return true
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
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

// StripBasePath wraps h so a request carrying the prefix is rewritten to look
// root-mounted before h sees it: URL.Path, URL.RawPath and RequestURI all lose
// the prefix. The server can therefore be mounted under BASE_PATH without the
// prefix reaching the router, the middleware or the rate limiter -- the same
// property a stripping reverse proxy provides, which is why none of them need
// to know the prefix exists.
//
// A request that is not under the prefix passes through untouched. That
// tolerance means the deployment works whether or not something upstream also
// strips, so the app serves /cuberouter/... and /... alike. With no prefix
// configured this returns h unchanged.
func StripBasePath(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := basePath
		if prefix == "" {
			h.ServeHTTP(w, r)
			return
		}
		// Path is decoded, so it is compared against the decoded prefix;
		// RawPath keeps the escapes and matches the configured value verbatim.
		decodedPrefix := prefix
		if strings.Contains(prefix, "%") {
			decoded, err := url.PathUnescape(prefix)
			if err != nil {
				// normalizeBasePath admits complete escapes only, so this is
				// unreachable; serving from the root still works.
				h.ServeHTTP(w, r)
				return
			}
			decodedPrefix = decoded
		}
		path, ok := prefixRemainder(decodedPrefix, r.URL.Path)
		if !ok {
			h.ServeHTTP(w, r)
			return
		}
		// Copy before rewriting: the original request belongs to the caller,
		// and Clone keeps Header, URL and the context intact.
		stripped := r.Clone(r.Context())
		stripped.URL.Path = path
		if r.URL.RawPath != "" {
			// An empty RawPath is valid and makes the URL re-escape Path, which
			// is the right fallback if the escaped form does not line up.
			stripped.URL.RawPath = ""
			if raw, ok := prefixRemainder(prefix, r.URL.RawPath); ok {
				stripped.URL.RawPath = raw
			}
		}
		// RequestURI has to carry the escaped path, not the decoded one: a relay
		// path such as /v1/a%2Fb must not arrive as /v1/a/b.
		stripped.RequestURI = stripped.URL.EscapedPath()
		if stripped.URL.RawQuery != "" {
			stripped.RequestURI = stripped.RequestURI + "?" + stripped.URL.RawQuery
		}
		// Only a stripped request can produce a Location the browser would
		// resolve against the wrong root; see locationPrefixWriter.
		h.ServeHTTP(&locationPrefixWriter{ResponseWriter: w, prefix: prefix}, stripped)
	})
}

// locationPrefixWriter re-applies the deployment prefix to the redirect
// Locations the wrapped handler writes.
//
// The handler sees a root-mounted request, so a redirect it builds -- gin's
// trailing-slash redirect is the one that bites, turning /api/channel into
// /api/channel/ -- is correct for the router and wrong for the browser, which
// resolves it against the origin root and leaves the prefix behind. A stripping
// reverse proxy normally repairs this on the way out; stripping in-process means
// there is no proxy left to do it, so it happens here instead.
//
// It implements the optional interfaces gin asserts on without checking --
// Flush, Hijack, CloseNotify -- because the assertion panics when they are
// missing, which would take SSE streaming down with it. Unwrap covers the rest,
// letting http.ResponseController find anything else the caller asks for.
type locationPrefixWriter struct {
	http.ResponseWriter
	prefix string
}

func (w *locationPrefixWriter) WriteHeader(status int) {
	if status >= http.StatusMultipleChoices && status < http.StatusBadRequest {
		if location := w.Header().Get("Location"); location != "" {
			w.Header().Set("Location", prefixLocation(w.prefix, location))
		}
	}
	w.ResponseWriter.WriteHeader(status)
}

// prefixLocation prefixes a Location that points at this deployment, leaving
// anything the browser must resolve elsewhere exactly as the handler wrote it.
func prefixLocation(prefix, location string) string {
	// An absolute or protocol-relative URL belongs to another origin, and a
	// location outside the prefix is one the handler deliberately aimed at the
	// site root; neither is ours to rewrite.
	if !strings.HasPrefix(location, "/") || strings.HasPrefix(location, "//") {
		return location
	}
	// Handlers such as the docs router call WithBasePath themselves, so the
	// prefix may already be there. This check cannot be exact -- a path that
	// merely starts with the prefix's characters is indistinguishable from a
	// prefixed one -- but on the way out it fails closed: the worst case leaves
	// the value the handler chose rather than doubling the prefix.
	if location == prefix || strings.HasPrefix(location, prefix+"/") {
		return location
	}
	return prefix + location
}

func (w *locationPrefixWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *locationPrefixWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *locationPrefixWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// CloseNotify satisfies the deprecated interface gin still asserts on.
//
//nolint:staticcheck // required by gin's direct type assertion
func (w *locationPrefixWriter) CloseNotify() <-chan bool {
	if notifier, ok := w.ResponseWriter.(http.CloseNotifier); ok {
		return notifier.CloseNotify()
	}
	// Nothing can report a closure here, so hand back a channel that never
	// fires rather than a nil one the caller would block on.
	return make(chan bool)
}

// prefixRemainder returns the part of p below prefix, or ok=false when p is not
// under it. The boundary check is what keeps /cuberouterfoo out of
// /cuberouter's scope, and a bare prefix maps to "/" so the root route matches.
func prefixRemainder(prefix, p string) (string, bool) {
	rest, found := strings.CutPrefix(p, prefix)
	if !found || (rest != "" && !strings.HasPrefix(rest, "/")) {
		return "", false
	}
	if rest == "" {
		return "/", true
	}
	return rest, true
}
