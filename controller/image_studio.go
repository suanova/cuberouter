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
	"bytes"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const imageStudioBodyLimit = 16 * 1024 * 1024

var imageStudioJobID = regexp.MustCompile(`^[0-9a-f]{32}$`)
var imageStudioRequestKey = regexp.MustCompile(`^[a-zA-Z0-9_-]{16,80}$`)
var imageStudioHTTP = &http.Client{
	Timeout:       45 * time.Second,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
}

// ImageStudio forwards only the documented job API. UserAuth must run first.
// The dispatcher token and owner are constructed here, never copied from a browser.
func ImageStudio(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	fail := func(status int, code string) { c.JSON(status, gin.H{"error": code}) }
	owner := c.GetInt("id")
	if owner <= 0 {
		fail(http.StatusUnauthorized, "login_required")
		return
	}
	base, token := os.Getenv("IMAGE_STUDIO_DISPATCHER_URL"), os.Getenv("IMAGE_STUDIO_DISPATCHER_TOKEN")
	if base == "" || len(token) < 32 {
		fail(http.StatusServiceUnavailable, "image_studio_unavailable")
		return
	}
	target, err := url.Parse(base)
	if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") || target.User != nil || target.RawQuery != "" || target.Fragment != "" {
		fail(http.StatusServiceUnavailable, "image_studio_unavailable")
		return
	}
	path := c.Param("path")
	valid := c.Request.Method == http.MethodGet && (path == "/session" || path == "/pools")
	valid = valid || (c.Request.Method == http.MethodPost && path == "/jobs")
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	valid = valid || (c.Request.Method == http.MethodGet && len(parts) == 2 && parts[0] == "requests" && imageStudioRequestKey.MatchString(parts[1]))
	if len(parts) >= 2 && parts[0] == "jobs" && imageStudioJobID.MatchString(parts[1]) {
		valid = valid || (c.Request.Method == http.MethodGet && (len(parts) == 2 || (len(parts) == 3 && parts[2] == "result")))
		valid = valid || (c.Request.Method == http.MethodPost && len(parts) == 3 && (parts[2] == "ack" || parts[2] == "cancel"))
	}
	if !valid || c.Request.URL.RawQuery != "" {
		fail(http.StatusNotFound, "unknown_route")
		return
	}
	var body []byte
	if c.Request.Method == http.MethodPost {
		kind, _, parseErr := mime.ParseMediaType(c.ContentType())
		if parseErr != nil || kind != "application/json" {
			fail(http.StatusUnsupportedMediaType, "json_required")
			return
		}
		// Read directly into bounded RAM; do not use the reusable-body disk cache.
		body, err = io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, imageStudioBodyLimit))
		if err != nil {
			fail(http.StatusRequestEntityTooLarge, "body_limit")
			return
		}
	}
	target.Path = strings.TrimRight(target.Path, "/") + path
	target.RawPath = ""
	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, target.String(), bytes.NewReader(body))
	if err != nil {
		fail(http.StatusServiceUnavailable, "image_studio_unavailable")
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Image-Owner", "cuberouter-"+strconv.Itoa(owner))
	req.Header.Set("Content-Type", "application/json")
	for _, header := range []string{"Idempotency-Key", "X-Trial-Epoch"} {
		req.Header.Set(header, c.GetHeader(header))
	}
	resp, err := imageStudioHTTP.Do(req)
	if err != nil {
		fail(http.StatusBadGateway, "dispatcher_connection_unavailable")
		return
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 24*1024*1024+1))
	if err != nil || len(data) > 24*1024*1024 || resp.StatusCode >= 500 || resp.StatusCode < 200 || (resp.StatusCode >= 300 && resp.StatusCode < 400) {
		fail(http.StatusBadGateway, "dispatcher_response_unavailable")
		return
	}
	kind, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if kind != "application/json" && kind != "image/png" {
		fail(http.StatusBadGateway, "dispatcher_response_unavailable")
		return
	}
	if model.ImageStudioAuditEnabled() && c.Request.Method == http.MethodPost && path == "/jobs" && resp.StatusCode < 300 {
		var job struct {
			ID    string `json:"id"`
			State string `json:"state"`
		}
		var parameters studioParameters
		if common.Unmarshal(data, &job) != nil || !studioJobID.MatchString(job.ID) || common.Unmarshal(body, &parameters) != nil ||
			studioAudit(job.ID, owner, parameters, job.State) != nil {
			fail(http.StatusServiceUnavailable, "audit_unavailable")
			return
		}
	}
	if path == "/session" && resp.StatusCode == 200 {
		var session map[string]any
		if common.Unmarshal(data, &session) == nil {
			session["audit_retention_days"] = 0
			if model.ImageStudioAuditEnabled() {
				session["audit_retention_days"] = 30
			}
			data, _ = common.Marshal(session)
		}
	}
	if model.ImageStudioAuditEnabled() && resp.StatusCode < 300 && len(parts) >= 2 && parts[0] == "jobs" && kind == "application/json" {
		var status struct {
			ID    string `json:"id"`
			State string `json:"state"`
		}
		if common.Unmarshal(data, &status) == nil && studioJobID.MatchString(status.ID) &&
			(status.State == "cancelled" || status.State == "expired" || status.State == "failed") {
			// Keep a completed generation's audit even if the queue later discards
			// its bytes (delivery state and model completion are different).
			rows, findErr := model.FindImageStudioAudits(status.ID, "", owner)
			if findErr == nil && len(rows) == 1 && rows[0].OutputHash == "" {
				_ = model.UpdateImageStudioAudit(status.ID, owner, map[string]any{"state": status.State})
			}
		}
	}
	c.Data(resp.StatusCode, kind, data)
}
