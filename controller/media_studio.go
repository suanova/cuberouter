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
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

var studioResource = regexp.MustCompile(`^/(jobs/[0-9a-f]{32}|assets/[0-9a-f]{32}/content)$`)

// This is deliberately not a general reverse proxy. In particular, clients
// cannot call the worker's relay authorization/settlement endpoints or choose
// an owner, upstream host, raw asset ID or filesystem path.
func MediaStudio(c *gin.Context) {
	path := c.Param("resource")
	if c.Request.Method == http.MethodGet && path == "/config" && service.StudioBridgeOrigin() == "" {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}
	allowed := false
	switch c.Request.Method {
	case http.MethodGet:
		allowed = path == "/config" || path == "/jobs" || studioResource.MatchString(path)
	case http.MethodPost:
		allowed = path == "/jobs" || path == "/uploads" || path == "/masks" || path == "/ocr" || path == "/text" || path == "/preview"
	case http.MethodDelete:
		allowed = studioResource.MatchString(path) && strings.HasPrefix(path, "/jobs/")
	}
	if !allowed || c.Request.URL.RawQuery != "" {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "Unknown studio operation"}})
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, service.StudioMaxBody))
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"message": "Studio request is too large"}})
		return
	}
	ownerPath := "/studio/" + strconv.Itoa(c.GetInt("id")) + path
	response, err := service.StudioBridgeRequest(c.Request.Context(), c.Request.Method, ownerPath, raw, c.GetHeader("Content-Type"))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": "Image studio worker is unavailable"}})
		return
	}
	defer response.Body.Close()
	// Keep upstream secrets, cookies, redirects, paths and headers private.
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		c.Status(http.StatusBadGateway)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Type", response.Header.Get("Content-Type"))
	c.Status(response.StatusCode)
	_, _ = io.Copy(c.Writer, io.LimitReader(response.Body, 24*1024*1024))
}

// StudioImage wraps the existing authenticated, quota-reserving image relay.
// The worker prepares a single-use job. It may publish its files only after
// this handler returns successfully from normal relay settlement/logging.
func StudioImage(c *gin.Context) {
	var body map[string]any
	if err := common.UnmarshalBodyReusable(c, &body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Invalid studio request"}})
		return
	}
	extra, ok := body["extra_fields"].(map[string]any)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Prepare a studio job before submitting"}})
		return
	}
	job, ok := extra["studio_job_id"].(string)
	if !ok || !studioResource.MatchString("/jobs/"+job) {
		c.Status(http.StatusBadRequest)
		return
	}
	if strings.TrimRight(common.GetContextKeyString(c, constant.ContextKeyChannelBaseUrl), "/") != service.StudioBridgeOrigin() {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"message": "Selected image channel is not the configured studio worker"}})
		return
	}
	path := "/studio/" + strconv.Itoa(c.GetInt("id")) + "/jobs/" + job
	if err := service.StudioBridgeJSON(c.Request.Context(), path+"/authorize", body); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"message": "Studio job was already submitted, expired, or does not match this account/request"}})
		return
	}
	c.Set("media_studio_relay", true) // Never automatically regenerate a paid job.
	// The public studio suffix is routing-only, not part of the provider API.
	c.Request.URL.Path = strings.TrimSuffix(c.Request.URL.Path, "/studio")
	PlaygroundImage(c)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := service.StudioBridgeJSON(ctx, path+"/settle", gin.H{
		"success":    c.Writer.Status() == http.StatusOK,
		"request_id": c.GetString(common.RequestIdKey),
	}); err != nil {
		common.SysError("media studio release pending for job " + job + "; reconcile with request " + c.GetString(common.RequestIdKey))
	}
}
