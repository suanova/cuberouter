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
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

type studioParameters struct {
	API        *studioAPIIdentity `json:"_api,omitempty"`
	Model      string             `json:"model"`
	Mode       string             `json:"mode"`
	Prompt     string             `json:"prompt"`
	UserPrompt string             `json:"user_prompt,omitempty"`
	Width      int                `json:"width"`
	Height     int                `json:"height"`
	Steps      int                `json:"steps"`
	Seed       int64              `json:"seed"`
	CFG        float64            `json:"cfg"`
	Images     []string           `json:"images"`
}

var studioJobID = regexp.MustCompile(`^[0-9a-f]{32}$`)

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
	deadlineUnix, err := strconv.ParseInt(c.GetHeader("X-Image-Execution-Deadline"), 10, 64)
	executionDeadline := time.Unix(deadlineUnix, 0)
	now := time.Now()
	if err != nil || executionDeadline.After(now.Add(26*time.Minute)) {
		fail(400, "invalid_execution_deadline")
		return
	}
	if !now.Before(executionDeadline) {
		fail(408, "execution_deadline_expired")
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
	apiExecution := c.Request.URL.Path == "/internal/image-studio/execute-api"
	if apiExecution != (p.API != nil) {
		fail(400, "invalid_execution_source")
		return
	}
	if apiExecution {
		defer func() {
			if c.Writer.Status() < 400 || c.GetBool("image_studio_pool_busy") {
				return
			}
			rows, err := model.FindImageStudioAudits(job, "", id)
			if err == nil && len(rows) == 1 && rows[0].OutputHash == "" && (rows[0].State == "queued" || rows[0].State == "running") {
				_ = model.UpdateImageStudioAudit(job, id, map[string]any{"state": "failed"})
			}
		}()
	}
	body, contentType, path, err := studioRelayBody(p, modelName)
	if err != nil {
		fail(400, "invalid_parameters")
		return
	}
	currentUser, err := model.GetUserById(id, false)
	if err != nil || currentUser == nil || currentUser.Status != common.UserStatusEnabled {
		fail(403, "account_unavailable")
		return
	}
	user := currentUser.ToBaseUser()
	// Membership may have been revoked while this job waited in the queue.
	if !apiExecution && !model.ImageStudioGroupAllowed(user.Group) {
		fail(403, "image_studio_access_denied")
		return
	}
	user.WriteContext(c)
	if apiExecution {
		if err := prepareStudioAPIIdentity(c, currentUser, p.API); err != nil {
			fail(403, "image_api_access_denied")
			return
		}
	}
	if err := studioExecutions.claim(job, owner, executionDeadline); err != nil {
		if err == errStudioExecutionCapacity {
			fail(503, "execution_registry_full")
		} else if err == errStudioExecutionDeadline {
			fail(408, "execution_deadline_expired")
		} else {
			fail(409, "execution_already_attempted")
		}
		return
	}
	// This outer defer runs after Distribute's physical-channel release. It
	// remains active even when the dispatcher has lost the response connection.
	defer func() {
		// A busy reply permits another attempt of this unstarted job. This
		// handler must never settle that later attempt after writing its reply.
		if !c.GetBool("image_studio_pool_busy") {
			studioExecutions.settle(job, owner)
		}
	}()
	c.Set(common.RequestIdKey, job)
	if err := studioAudit(job, id, p, "running"); err != nil {
		fail(503, "audit_unavailable")
		return
	}
	if apiExecution {
		if model.UpdateImageStudioAudit(job, id, map[string]any{"token_name": c.GetString("token_name"), "moderation_policy": c.GetString("image_api_moderation")}) != nil {
			fail(503, "audit_unavailable")
			return
		}
	}
	if !apiExecution {
		if err := middleware.SetupContextForToken(c, &model.Token{UserId: id, Name: "image-studio", Group: user.Group, UnlimitedQuota: true}); err != nil {
			fail(403, "account_billing_unavailable")
			return
		}
		common.SetContextKey(c, constant.ContextKeyUsingGroup, user.Group)
	}
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
	if c.GetBool("image_studio_pool_busy") {
		studioExecutions.deferUnstarted(job, owner)
		if model.ImageStudioAuditEnabled() {
			_ = model.UpdateImageStudioAudit(job, id, map[string]any{"state": "queued"})
		}
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"protocol": "image-studio-admission-v1", "state": "not_started",
			"owner": owner, "job_id": job, "error": "image_channel_busy",
		})
		return
	}
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
	deliverStudioOutput(c, buffer)
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
