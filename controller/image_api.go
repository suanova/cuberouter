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
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// Only the authenticated Go ingress constructs this identity. No API key is
// passed to Python. The callback rechecks the current token and account in DB.
type studioAPIIdentity struct {
	TokenID   int    `json:"token_id"`
	Group     string `json:"group"`
	ClientIP  string `json:"client_ip"`
	RequestID string `json:"request_id"`
}

func prepareStudioAPIIdentity(c *gin.Context, user *model.User, identity *studioAPIIdentity) error {
	denied := fmt.Errorf("image_api_access_denied")
	if identity == nil || !imageStudioRequestKey.MatchString(identity.RequestID) || !model.ImageStudioAuditEnabled() {
		return denied
	}
	token, err := model.GetTokenById(identity.TokenID)
	if err != nil || token.UserId != user.Id || token.Status != common.TokenStatusEnabled ||
		(token.ExpiredTime != -1 && token.ExpiredTime < time.Now().Unix()) ||
		(!token.UnlimitedQuota && token.RemainQuota <= 0) {
		return denied
	}
	scope, err := service.ValidateTokenScopeForRelay(token)
	if err != nil {
		return denied
	}
	accountGroup := user.Group
	if scope.ScopeType == model.AccountContextTypeOrganization {
		accountGroup = scope.AccountGroup
	}
	group := accountGroup
	if token.Group != "" {
		if _, ok := service.GetAccountUsableGroups(accountGroup)[token.Group]; !ok {
			return denied
		}
		if token.Group != "auto" && !ratio_setting.ContainsGroupRatio(token.Group) {
			return denied
		}
		group = token.Group
	}
	// A changed group requires resubmission; it cannot silently change policy.
	if group != identity.Group || !model.ImageStudioGroupAllowed(accountGroup) {
		return denied
	}
	if ips := token.GetIpLimits(); len(ips) > 0 && !common.IsIpInCIDRList(net.ParseIP(identity.ClientIP), ips) {
		return denied
	}
	if token.ModelLimitsEnabled && !token.GetModelLimitsMap()[ratio_setting.FormatMatchingModelName(os.Getenv("IMAGE_STUDIO_CHANNEL_MODEL"))] {
		return denied
	}
	policy, err := model.ImageAPIModeration(accountGroup)
	if err != nil {
		return err
	}
	if err = middleware.SetupContextForTokenWithScopeContext(c, token, scope); err != nil {
		return err
	}
	common.SetContextKey(c, constant.ContextKeyUserGroup, accountGroup)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
	c.Set("image_api_moderation", policy)
	return nil
}

func imageAPIError(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"message": code, "type": "image_request_error", "code": code}})
}

// Parse the supported Qwen contract without silently dropping model options.
// Multipart is kept in RAM. Unknown options are rejected for Qwen only.
func imageAPIParameters(c *gin.Context) (studioParameters, bool, error) {
	p := studioParameters{Model: "2.1", Mode: "create", Width: 1312, Height: 1312, Steps: 40, CFG: 1, Seed: 42, Images: []string{}}
	configured := strings.TrimSpace(os.Getenv("IMAGE_STUDIO_CHANNEL_MODEL"))
	if configured == "" {
		return p, false, nil
	}
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, imageStudioBodyLimit+1))
	if err != nil || len(data) > imageStudioBodyLimit {
		return p, true, fmt.Errorf("body_limit")
	}
	c.Set(common.KeyBodyStorage, common.NewMemoryBodyStorage(data))
	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	fields := map[string]any{}
	kind, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil {
		return p, true, fmt.Errorf("invalid_content_type")
	}
	if kind == "application/json" {
		if common.Unmarshal(data, &fields) != nil {
			return p, true, fmt.Errorf("invalid_json")
		}
	} else if kind == "multipart/form-data" {
		form, err := multipart.NewReader(bytes.NewReader(data), params["boundary"]).ReadForm(imageStudioBodyLimit)
		if err != nil {
			return p, true, fmt.Errorf("invalid_images")
		}
		defer form.RemoveAll()
		if len(form.Value["model"]) == 1 && form.Value["model"][0] != configured {
			return p, false, nil
		}
		for k, v := range form.Value {
			if len(v) != 1 {
				return p, true, fmt.Errorf("duplicate_field")
			}
			fields[k] = v[0]
		}
		for k, files := range form.File {
			if k != "image" && k != "image[]" {
				return p, true, fmt.Errorf("unsupported_image_field")
			}
			if len(p.Images)+len(files) > 10 {
				return p, true, fmt.Errorf("too_many_images")
			}
			for _, file := range files {
				f, err := file.Open()
				if err != nil {
					return p, true, fmt.Errorf("invalid_image")
				}
				b, err := io.ReadAll(io.LimitReader(f, 10<<20+1))
				f.Close()
				if err != nil || len(b) > 10<<20 {
					return p, true, fmt.Errorf("invalid_image")
				}
				p.Images = append(p.Images, base64.StdEncoding.EncodeToString(b))
			}
		}
		if v, ok := fields["extra_fields"].(string); ok {
			var extra map[string]any
			if common.Unmarshal([]byte(v), &extra) != nil {
				return p, true, fmt.Errorf("invalid_extra_fields")
			}
			fields["extra_fields"] = extra
		}
		if v, ok := fields["n"].(string); ok {
			n, err := strconv.Atoi(v)
			if err != nil {
				return p, true, fmt.Errorf("invalid_n")
			}
			fields["n"] = float64(n)
		}
	} else {
		return p, true, fmt.Errorf("unsupported_content_type")
	}
	if fields["model"] != configured {
		return p, false, nil
	}
	for k := range fields {
		switch k {
		case "model", "prompt", "n", "size", "response_format", "quality", "extra_fields":
		default:
			return p, true, fmt.Errorf("unsupported_parameter_%s", k)
		}
	}
	p.Prompt, _ = fields["prompt"].(string)
	p.UserPrompt = p.Prompt
	if v, ok := fields["n"]; ok && v != float64(1) {
		return p, true, fmt.Errorf("only_n_1_supported")
	}
	if v, ok := fields["response_format"]; ok && v != "b64_json" {
		return p, true, fmt.Errorf("inline_image_result_required")
	}
	if v, ok := fields["quality"]; ok && v != "standard" {
		return p, true, fmt.Errorf("unsupported_quality")
	}
	if v, ok := fields["size"]; ok {
		s, ok := v.(string)
		if !ok {
			return p, true, fmt.Errorf("invalid_size")
		}
		parts := strings.Split(s, "x")
		if len(parts) != 2 {
			return p, true, fmt.Errorf("invalid_size")
		}
		p.Width, err = strconv.Atoi(parts[0])
		if err != nil {
			return p, true, fmt.Errorf("invalid_size")
		}
		p.Height, err = strconv.Atoi(parts[1])
		if err != nil {
			return p, true, fmt.Errorf("invalid_size")
		}
	}
	if v, ok := fields["extra_fields"]; ok {
		extra, ok := v.(map[string]any)
		if !ok {
			return p, true, fmt.Errorf("invalid_extra_fields")
		}
		for k, v := range extra {
			n, ok := v.(float64)
			if !ok {
				return p, true, fmt.Errorf("invalid_extra_fields")
			}
			switch k {
			case "steps":
				p.Steps = int(n)
				if float64(p.Steps) != n {
					return p, true, fmt.Errorf("invalid_steps")
				}
			case "seed":
				p.Seed = int64(n)
				if float64(p.Seed) != n {
					return p, true, fmt.Errorf("invalid_seed")
				}
			case "cfg":
				p.CFG = n
			default:
				return p, true, fmt.Errorf("unsupported_extra_field")
			}
		}
	}
	if c.Request.URL.Path != "/v1/images/generations" {
		p.Mode = "edit"
	}
	_, _, _, err = studioRelayBody(p, configured)
	return p, true, err
}

func imageAPIDispatch(ctx context.Context, owner int, method, path string, payload []byte, epoch, key string) ([]byte, int, error) {
	u, err := url.Parse(os.Getenv("IMAGE_STUDIO_DISPATCHER_URL"))
	token := os.Getenv("IMAGE_STUDIO_DISPATCHER_TOKEN")
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") || len(token) < 32 {
		return nil, 503, fmt.Errorf("dispatcher_unavailable")
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawPath = ""
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, 503, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Image-Owner", fmt.Sprintf("cuberouter-%d", owner))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Trial-Epoch", epoch)
	req.Header.Set("Idempotency-Key", key)
	resp, err := imageStudioHTTP.Do(req)
	if err != nil {
		return nil, 502, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 24<<20+1))
	kind, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || len(data) > 24<<20 || (kind != "application/json" && kind != "image/png") {
		return nil, 502, fmt.Errorf("invalid_dispatcher_response")
	}
	return data, resp.StatusCode, nil
}

// Runs before Distribute so waiting requests do not occupy physical GPUs.
// Unrelated providers and all LLM routes retain their existing handlers.
func QueueImageAPI(c *gin.Context) {
	p, handled, err := imageAPIParameters(c)
	if !handled {
		c.Next()
		return
	}
	if err != nil {
		imageAPIError(c, 400, err.Error())
		return
	}
	middleware.ImageStudioUserRateLimit()(c)
	if c.IsAborted() {
		imageAPIError(c, c.Writer.Status(), "image_request_rate_limit")
		return
	}
	c.Header("Cache-Control", "no-store")
	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil || user == nil || user.Status != common.UserStatusEnabled {
		imageAPIError(c, 403, "account_unavailable")
		return
	}
	key := c.GetHeader("Idempotency-Key")
	if key == "" {
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			imageAPIError(c, 503, "request_id_unavailable")
			return
		}
		key = fmt.Sprintf("%x", b)
	}
	if !imageStudioRequestKey.MatchString(key) {
		imageAPIError(c, 400, "invalid_idempotency_key")
		return
	}
	c.Header("Idempotency-Key", key)
	p.API = &studioAPIIdentity{TokenID: c.GetInt("token_id"), Group: common.GetContextKeyString(c, constant.ContextKeyUsingGroup), ClientIP: c.ClientIP(), RequestID: key}
	if err := prepareStudioAPIIdentity(c, user, p.API); err != nil {
		imageAPIError(c, 403, "image_api_access_denied")
		return
	}
	if _, pinned, _ := service.GetChannelConstraints(c).ResolvedPin(); pinned {
		imageAPIError(c, 400, "specific_channel_not_supported_in_queue")
		return
	}
	ctx := c.Request.Context()
	data, status, err := imageAPIDispatch(ctx, user.Id, "GET", "/session", nil, "", "")
	var session struct {
		Epoch       string `json:"epoch"`
		APIProtocol string `json:"api_protocol"`
	}
	if err != nil || status != 200 || common.Unmarshal(data, &session) != nil || session.APIProtocol != "image-api-queue-v1" {
		imageAPIError(c, 503, "image_api_queue_upgrade_required")
		return
	}
	payload, _ := common.Marshal(p)
	if len(payload) > imageStudioBodyLimit {
		imageAPIError(c, 413, "body_limit")
		return
	}
	queueKey := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("api:%d:%s", p.API.TokenID, key))))
	jobID := queueKey[:32]
	c.Header("X-Image-Job", jobID)
	c.Header("Location", common.WithBasePath("/v1/images/jobs/"+jobID))
	row, err := studioAuditRow(jobID, user.Id, p, "queued")
	if err != nil {
		imageAPIError(c, 503, "audit_unavailable")
		return
	}
	input := p
	input.API = nil
	canonical, _ := common.Marshal(input)
	row.RequestHash = fmt.Sprintf("%x", sha256.Sum256(canonical))
	row.TokenName = c.GetString("token_name")
	row.ModerationPolicy = c.GetString("image_api_moderation")
	created, err := model.ClaimImageStudioAudit(row)
	if err != nil {
		imageAPIError(c, 503, "audit_unavailable")
		return
	}
	if !created {
		previous, err := model.FindImageStudioAudits(jobID, "", user.Id)
		if err != nil || len(previous) != 1 {
			imageAPIError(c, 409, "image_request_no_longer_available")
			return
		}
		if previous[0].RequestHash != row.RequestHash || previous[0].TokenID != p.API.TokenID {
			imageAPIError(c, 409, "idempotency_conflict")
			return
		}
		// A durable record survives dispatcher restarts. Never replay a prior
		// attempt merely because its in-memory result or metadata disappeared.
		c.Abort()
		imageAPIResult(c, jobID, true)
		return
	}
	data, status, err = imageAPIDispatch(ctx, user.Id, "POST", "/api-jobs", payload, session.Epoch, queueKey)
	if err != nil {
		imageAPIError(c, 502, "queue_submission_uncertain_reuse_idempotency_key")
		return
	}
	if status != 200 && status != 202 {
		_ = model.UpdateImageStudioAudit(jobID, user.Id, map[string]any{"state": "admission_rejected"})
		var failure struct {
			Error string `json:"error"`
		}
		_ = common.Unmarshal(data, &failure)
		code := "image_queue_unavailable"
		switch failure.Error {
		case "queue_full", "one_unfinished_job_per_session", "idempotency_conflict", "execution_recovery_pending":
			code = failure.Error
		}
		if status == 429 {
			c.Header("Retry-After", "3")
		}
		imageAPIError(c, status, code)
		return
	}
	var job struct {
		ID string `json:"id"`
	}
	if common.Unmarshal(data, &job) != nil || job.ID != jobID {
		imageAPIError(c, 502, "invalid_queue_response")
		return
	}
	c.Header("X-Image-Job", job.ID)
	c.Header("Location", common.WithBasePath("/v1/images/jobs/"+job.ID))

	c.Abort()
	if c.GetHeader("Prefer") == "respond-async" {
		c.JSON(202, gin.H{"id": job.ID, "status_url": common.WithBasePath("/v1/images/jobs/" + job.ID)})
		return
	}
	// Synchronous compatibility, with a bounded wait and an explicit recovery
	// URL if the inference outlasts the HTTP request. Never resubmit generation.
	timer := time.NewTimer(110 * time.Second)
	defer timer.Stop()
	for {
		if imageAPIResult(c, job.ID, false) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			c.Header("Retry-After", "2")
			c.JSON(202, gin.H{"id": job.ID, "status_url": common.WithBasePath("/v1/images/jobs/" + job.ID)})
			return
		case <-time.After(time.Second):
		}
	}
}

// Returns only the authenticated token's own output, never admin audit fields.
func ImageAPIJob(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	middleware.ImageStudioUserRateLimit()(c)
	if c.IsAborted() {
		imageAPIError(c, c.Writer.Status(), "image_request_rate_limit")
		return
	}
	job := c.Param("job")
	if !studioJobID.MatchString(job) {
		imageAPIError(c, 404, "unknown_job")
		return
	}
	rows, err := model.FindImageStudioAudits(job, "", c.GetInt("id"))
	if err != nil {
		imageAPIError(c, 503, "audit_unavailable")
		return
	}
	if len(rows) != 1 || rows[0].Source != "api" || rows[0].TokenID != c.GetInt("token_id") {
		imageAPIError(c, 404, "unknown_job")
		return
	}
	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil || user == nil || user.Status != common.UserStatusEnabled || prepareStudioAPIIdentity(c, user, &studioAPIIdentity{TokenID: rows[0].TokenID, Group: rows[0].UsingGroup, ClientIP: c.ClientIP(), RequestID: rows[0].RequestID}) != nil {
		imageAPIError(c, 403, "image_api_access_denied")
		return
	}
	imageAPIResult(c, job, true)
}

func imageAPIResult(c *gin.Context, job string, showPending bool) bool {
	owner := c.GetInt("id")
	user, authErr := model.GetUserById(owner, false)
	if authErr != nil || user == nil || user.Status != common.UserStatusEnabled {
		imageAPIError(c, 403, "account_unavailable")
		return true
	}
	token, scopeErr := model.GetTokenById(c.GetInt("token_id"))
	if scopeErr != nil || token.UserId != user.Id {
		imageAPIError(c, 403, "image_api_access_denied")
		return true
	}
	scope, scopeErr := service.ValidateTokenScopeForRelay(token)
	if scopeErr != nil {
		imageAPIError(c, 403, "image_api_access_denied")
		return true
	}
	accountGroup := user.Group
	if scope.ScopeType == model.AccountContextTypeOrganization {
		accountGroup = scope.AccountGroup
	}
	policy, policyErr := model.ImageAPIModeration(accountGroup)
	if policyErr != nil {
		imageAPIError(c, 503, "image_api_policy_unavailable")
		return true
	}
	c.Set("image_api_moderation", policy)
	data, status, err := imageAPIDispatch(c.Request.Context(), owner, "GET", "/jobs/"+job, nil, "", "")
	if err != nil || status != 200 {
		imageAPIError(c, 502, "job_status_unavailable")
		return true
	}
	var result struct {
		State string `json:"state"`
		Error string `json:"error"`
	}
	if common.Unmarshal(data, &result) != nil {
		imageAPIError(c, 502, "invalid_job_status")
		return true
	}
	if result.State == "queued" || result.State == "running" {
		if showPending {
			c.Header("Retry-After", "2")
			c.JSON(202, gin.H{"id": job, "state": result.State})
		}
		return showPending
	}
	if result.State != "completed" {
		rows, _ := model.FindImageStudioAudits(job, "", owner)
		if len(rows) == 1 && rows[0].OutputHash == "" {
			_ = model.UpdateImageStudioAudit(job, owner, map[string]any{"state": result.State})
		}
		code, status := "image_job_failed", 502
		switch result.Error {
		case "output_blocked":
			code, status = result.Error, 422
		case "output_check_unavailable":
			code, status = result.Error, 503
		case "execution_uncertain_no_retry":
			code = result.Error
		}
		if result.State == "expired" || result.State == "released" {
			code, status = "image_result_expired", 410
		}
		imageAPIError(c, status, code)
		return true
	}
	// The policy may have tightened after inference but before result retrieval.
	// Never release an externally reviewed result under a now-required policy.
	rows, err := model.FindImageStudioAudits(job, "", owner)
	if err != nil || len(rows) != 1 {
		imageAPIError(c, 503, "audit_unavailable")
		return true
	}
	if rows[0].TokenID != c.GetInt("token_id") || prepareStudioAPIIdentity(c, user, &studioAPIIdentity{TokenID: rows[0].TokenID, Group: rows[0].UsingGroup, ClientIP: c.ClientIP(), RequestID: rows[0].RequestID}) != nil {
		imageAPIError(c, 403, "image_api_access_denied")
		return true
	}
	if c.GetString("image_api_moderation") == "required" && rows[0].ModerationState != "passed" {
		imageAPIError(c, 503, "output_check_unavailable")
		return true
	}
	data, status, err = imageAPIDispatch(c.Request.Context(), owner, "GET", "/jobs/"+job+"/result", nil, "", "")
	if err != nil || status != 200 || !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) ||
		fmt.Sprintf("%x", sha256.Sum256(data)) != rows[0].OutputHash {
		imageAPIError(c, 502, "image_result_unavailable")
		return true
	}
	c.Header("X-Image-Job", job)
	c.JSON(200, gin.H{"created": rows[0].CreatedAt, "data": []gin.H{{"b64_json": base64.StdEncoding.EncodeToString(data)}}})
	return true
}

// Session callers must use the queued Studio route for the configured Qwen model.
// This prevents a stale frontend from bypassing output review via Playground.
func RequireQueuedStudioImage(c *gin.Context) {
	configured := os.Getenv("IMAGE_STUDIO_CHANNEL_MODEL")
	if configured == "" {
		c.Next()
		return
	}
	var body struct {
		Model string `json:"model"`
	}
	if err := common.UnmarshalBodyReusable(c, &body); err != nil {
		imageAPIError(c, 400, "invalid_parameters")
		return
	}
	if body.Model == configured {
		imageAPIError(c, 409, "use_image_studio_queue")
		return
	}
	c.Next()
}
