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
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

type studioParameters struct {
	Model      string   `json:"model"`
	Mode       string   `json:"mode"`
	Prompt     string   `json:"prompt"`
	UserPrompt string   `json:"user_prompt,omitempty"`
	Width      int      `json:"width"`
	Height     int      `json:"height"`
	Steps      int      `json:"steps"`
	Seed       int64    `json:"seed"`
	CFG        float64  `json:"cfg"`
	Images     []string `json:"images"`
}

var studioJobID = regexp.MustCompile(`^[0-9a-f]{32}$`)
var studioExecutions = struct {
	sync.Mutex
	seen map[string]time.Time
}{seen: make(map[string]time.Time)}

// claimStudioExecution retains only identifiers, never inputs or outputs. The
// dispatcher never retries callbacks; this also rejects duplicate delivery.
func claimStudioExecution(job string) bool {
	studioExecutions.Lock()
	defer studioExecutions.Unlock()
	now := time.Now()
	for key, expiry := range studioExecutions.seen {
		if now.After(expiry) {
			delete(studioExecutions.seen, key)
		}
	}
	if _, exists := studioExecutions.seen[job]; exists || len(studioExecutions.seen) >= 4096 {
		return false
	}
	studioExecutions.seen[job] = now.Add(time.Hour)
	return true
}

func studioRelayBody(p studioParameters, modelName string) ([]byte, string, string, error) {
	if p.Model != "2.1" || (p.Mode != "create" && p.Mode != "edit") ||
		len(strings.TrimSpace(p.Prompt)) == 0 || len([]rune(p.Prompt)) > 16000 ||
		len([]rune(p.UserPrompt)) > 16000 ||
		p.Width < 256 || p.Width > 1664 || p.Height < 256 || p.Height > 1664 ||
		p.Width%32 != 0 || p.Height%32 != 0 || p.Width*p.Height > 2097152 ||
		p.Steps < 1 || p.Steps > 60 || p.Seed < 0 || p.Seed > 1<<53-1 ||
		math.IsNaN(p.CFG) || math.IsInf(p.CFG, 0) || p.CFG < 0 || p.CFG > 10 ||
		len(p.Images) > 10 || (p.Mode == "create" && len(p.Images) != 0) ||
		(p.Mode == "edit" && len(p.Images) == 0) {
		return nil, "", "", fmt.Errorf("invalid_parameters")
	}
	extra := map[string]any{"steps": p.Steps, "seed": p.Seed, "cfg": p.CFG}
	if p.Mode == "create" {
		body, err := common.Marshal(map[string]any{
			"model": modelName, "prompt": p.Prompt, "n": 1,
			"size":            fmt.Sprintf("%dx%d", p.Width, p.Height),
			"response_format": "b64_json", "extra_fields": extra,
		})
		return body, "application/json", "/v1/images/generations", err
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	extraJSON, _ := common.Marshal(extra)
	for key, value := range map[string]string{
		"model": modelName, "prompt": p.Prompt, "n": "1",
		"size":            fmt.Sprintf("%dx%d", p.Width, p.Height),
		"response_format": "b64_json", "extra_fields": string(extraJSON),
	} {
		if err := w.WriteField(key, value); err != nil {
			return nil, "", "", err
		}
	}
	for i, encoded := range p.Images {
		data, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil || len(data) == 0 || len(data) > 10<<20 {
			return nil, "", "", fmt.Errorf("invalid_image")
		}
		ext := ""
		switch http.DetectContentType(data) {
		case "image/png":
			ext = "png"
		case "image/jpeg":
			ext = "jpg"
		case "image/webp":
			ext = "webp"
		default:
			return nil, "", "", fmt.Errorf("invalid_image_type")
		}
		part, err := w.CreateFormFile("image[]", fmt.Sprintf("reference-%02d.%s", i+1, ext))
		if err != nil {
			return nil, "", "", err
		}
		if _, err = part.Write(data); err != nil {
			return nil, "", "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", "", err
	}
	return buf.Bytes(), w.FormDataContentType(), "/v1/images/edits", nil
}

// PrepareImageStudioRelay is private dispatcher authentication, not a browser
// endpoint. No caller-supplied channel, group, model ID or API token is trusted.
func PrepareImageStudioRelay(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	fail := func(status int, code string) { c.AbortWithStatusJSON(status, gin.H{"error": code}) }
	secret := os.Getenv("IMAGE_STUDIO_RELAY_TOKEN")
	modelName := strings.TrimSpace(os.Getenv("IMAGE_STUDIO_CHANNEL_MODEL"))
	if len(secret) < 32 || modelName == "" {
		fail(503, "image_studio_channel_disabled")
		return
	}
	if subtle.ConstantTimeCompare([]byte(c.GetHeader("Authorization")), []byte("Bearer "+secret)) != 1 {
		fail(401, "dispatcher_auth_required")
		return
	}
	owner := c.GetHeader("X-Image-Owner")
	id, err := strconv.Atoi(strings.TrimPrefix(owner, "cuberouter-"))
	job := c.GetHeader("X-Image-Job")
	if err != nil || id < 1 || owner != fmt.Sprintf("cuberouter-%d", id) || !studioJobID.MatchString(job) {
		fail(400, "invalid_execution_identity")
		return
	}
	if c.ContentType() != "application/json" || c.Request.URL.RawQuery != "" {
		fail(400, "invalid_request")
		return
	}
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, imageStudioBodyLimit+1))
	if err != nil || len(data) > imageStudioBodyLimit {
		fail(413, "body_limit")
		return
	}
	var p studioParameters
	if common.Unmarshal(data, &p) != nil {
		fail(400, "invalid_parameters")
		return
	}
	body, contentType, path, err := studioRelayBody(p, modelName)
	if err != nil {
		fail(400, "invalid_parameters")
		return
	}
	user, err := model.GetUserCache(id)
	if err != nil || user == nil || user.Status != common.UserStatusEnabled {
		fail(403, "account_unavailable")
		return
	}
	// Membership may have been revoked while this job waited in the queue.
	if !model.ImageStudioGroupAllowed(user.Group) {
		fail(403, "image_studio_access_denied")
		return
	}
	if !claimStudioExecution(job) {
		fail(409, "execution_already_attempted")
		return
	}
	c.Set(common.RequestIdKey, job)
	if err := studioAudit(job, id, p, "running"); err != nil {
		fail(503, "audit_unavailable")
		return
	}
	user.WriteContext(c)
	if err := middleware.SetupContextForToken(c, &model.Token{UserId: id, Name: "image-studio", Group: user.Group, UnlimitedQuota: true}); err != nil {
		fail(403, "account_billing_unavailable")
		return
	}
	common.SetContextKey(c, constant.ContextKeyUsingGroup, user.Group)
	common.SetContextKey(c, constant.ContextKeyImageStudio, true)
	// Rewrite only after private authentication. Downstream uses the exact same
	// channel selection, model mapping, adaptor and account billing as /v1.
	c.Request.Header = make(http.Header)
	c.Request.Header.Set("Content-Type", contentType)
	c.Request.URL.Path = path
	c.Request.URL.RawPath = ""
	c.Request.ContentLength = int64(len(body))
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Set(common.KeyBodyStorage, common.NewMemoryBodyStorage(body))
	defer common.CleanupBodyStorage(c)
	if p.Mode == "edit" {
		// Body is bounded at 16 MiB, so multipart image files stay in RAM.
		if err := c.Request.ParseMultipartForm(imageStudioBodyLimit); err != nil {
			fail(400, "invalid_images")
			return
		}
		defer c.Request.MultipartForm.RemoveAll()
	}
	c.Next()
	if model.ImageStudioAuditEnabled() && c.Writer.Status() >= 400 {
		_ = model.UpdateImageStudioAudit(job, id, map[string]any{"state": "failed"})
	}
}

func ImageStudioChannelRelay(c *gin.Context) {
	channel := common.GetContextKeyInt(c, constant.ContextKeyChannelId)
	c.Header("X-Image-Studio-Channel", strconv.Itoa(channel))
	job, owner := c.GetString(common.RequestIdKey), c.GetInt("id")
	if model.ImageStudioAuditEnabled() {
		if model.UpdateImageStudioAudit(job, owner, map[string]any{"state": "running", "channel_id": channel, "model": os.Getenv("IMAGE_STUDIO_CHANNEL_MODEL")}) != nil {
			c.AbortWithStatusJSON(503, gin.H{"error": "audit_unavailable"})
			return
		}
	}
	// Keep the result in bounded RAM until relay settlement/audit completes.
	// An upstream response must not appear successful before those steps finish.
	original := c.Writer
	buffer := &studioRelayWriter{ResponseWriter: original, status: 200}
	c.Writer = buffer
	defer func() { c.Writer = original; original.Header().Del("Content-Length") }()
	Relay(c, types.RelayFormatOpenAIImage)
	c.Writer = original
	original.Header().Del("Content-Length")
	if model.ImageStudioAuditEnabled() {
		values := map[string]any{"state": "failed"}
		if buffer.status == 200 && !buffer.overflow {
			var result struct {
				Data []struct {
					Image string `json:"b64_json"`
				} `json:"data"`
			}
			if common.Unmarshal(buffer.body.Bytes(), &result) == nil && len(result.Data) == 1 {
				if image, err := base64.StdEncoding.Strict().DecodeString(result.Data[0].Image); err == nil && bytes.HasPrefix(image, []byte("\x89PNG\r\n\x1a\n")) {
					values["state"], values["output_hash"] = "completed", fmt.Sprintf("%x", sha256.Sum256(image))
				}
			}
		}
		if model.UpdateImageStudioAudit(job, owner, values) != nil {
			c.AbortWithStatusJSON(503, gin.H{"error": "audit_unavailable"})
			return
		}
	}
	if buffer.overflow {
		c.AbortWithStatusJSON(502, gin.H{"error": "image_result_limit"})
		return
	}
	c.Data(buffer.status, "application/json", buffer.body.Bytes())
}

type studioRelayWriter struct {
	gin.ResponseWriter
	body              bytes.Buffer
	status            int
	written, overflow bool
}

func (w *studioRelayWriter) WriteHeader(code int) {
	if !w.written {
		w.status = code
	}
}
func (w *studioRelayWriter) WriteHeaderNow() { w.written = true }
func (w *studioRelayWriter) Status() int     { return w.status }
func (w *studioRelayWriter) Written() bool   { return w.written }
func (w *studioRelayWriter) Size() int {
	if !w.written {
		return -1
	}
	return w.body.Len()
}
func (w *studioRelayWriter) Flush()                            { w.written = true }
func (w *studioRelayWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }
func (w *studioRelayWriter) Write(p []byte) (int, error) {
	w.written = true
	if w.body.Len()+len(p) > 24<<20 {
		w.overflow = true
		return 0, fmt.Errorf("image_result_limit")
	}
	return w.body.Write(p)
}
