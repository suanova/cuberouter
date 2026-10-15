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
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const studioModerationModel = model.ImageModerationModel

// Only these bounded, validated fields enter the 30-day audit. Never retain the
// response body from an external classifier: it might contain image data.
type studioModerationReport struct {
	State             string             `json:"state"`
	ChannelID         int                `json:"channel_id,omitempty"`
	Source            string             `json:"source,omitempty"`
	AttemptedChannels []int              `json:"attempted_channel_ids,omitempty"`
	Model             string             `json:"model,omitempty"`
	Revision          string             `json:"revision,omitempty"`
	Scores            map[string]float64 `json:"scores,omitempty"`
	Threshold         float64            `json:"threshold,omitempty"`
	Error             string             `json:"error,omitempty"`
}

var studioModelRevision = regexp.MustCompile(`^[0-9a-f]{40}$`)

var studioModerationClient = func() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{Transport: transport, Timeout: 45 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}()

func checkStudioOutput(ctx context.Context, image []byte) studioModerationReport {
	failed := studioModerationReport{State: "error", Error: "output_check_unavailable"}
	required, channels, err := model.ImageModerationChannels(ctx)
	if err != nil {
		return failed
	}
	if required {
		return checkStudioChannelOutput(ctx, image, channels)
	}
	// Any nonempty value other than explicit false opts in, including a typo:
	// configuration errors must not silently turn off an intended safety gate.
	enabled := os.Getenv("IMAGE_STUDIO_MODERATION_ENABLED")
	if enabled == "" || enabled == "false" {
		return studioModerationReport{State: "disabled"}
	}
	endpoint, err := url.Parse(os.Getenv("IMAGE_STUDIO_MODERATION_URL"))
	if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return failed
	}
	loopback := endpoint.Hostname() == "localhost"
	if ip := net.ParseIP(endpoint.Hostname()); ip != nil {
		loopback = ip.IsLoopback()
	}
	if endpoint.Scheme != "https" && !(endpoint.Scheme == "http" && loopback) {
		return failed
	}
	token := os.Getenv("IMAGE_STUDIO_MODERATION_TOKEN")
	if len(token) < 32 || strings.ContainsAny(token, "\r\n") || len(image) > 12<<20 {
		return failed
	}
	report, _, _ := requestStudioModeration(ctx, studioModerationClient, endpoint.String(), token, image)
	report.Source = "legacy_environment"
	return report
}

// Retry availability failures only. A malformed or negative verdict is terminal.
func requestStudioModeration(ctx context.Context, client *http.Client, endpoint, token string, image []byte) (studioModerationReport, bool, bool) {
	failed := studioModerationReport{State: "error", Error: "output_check_unavailable"}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(image))
	if err != nil {
		return failed, false, false
	}
	request.Header.Set("Content-Type", "image/png")
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		return failed, true, true
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return failed, true, false
	}
	kind, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if kind != "application/json" {
		return failed, false, false
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 16385))
	if err != nil || len(body) > 16384 {
		return failed, false, false
	}
	return decodeStudioModeration(image, body), false, false
}

func decodeStudioModeration(image, body []byte) studioModerationReport {
	failed := studioModerationReport{State: "error", Error: "output_check_unavailable"}
	var result struct {
		Protocol        string              `json:"protocol"`
		Model           string              `json:"model"`
		Revision        string              `json:"revision"`
		ImageHash       string              `json:"image_sha256"`
		Decision        string              `json:"decision"`
		ReleaseAllowed  *bool               `json:"release_allowed"`
		WeightsVerified bool                `json:"weights_verified"`
		Threshold       *float64            `json:"threshold"`
		Scores          map[string]*float64 `json:"scores"`
	}
	if common.Unmarshal(body, &result) != nil || result.Protocol != "image-moderation-v1" ||
		result.Model != studioModerationModel || !studioModelRevision.MatchString(result.Revision) ||
		!result.WeightsVerified || result.ImageHash != fmt.Sprintf("%x", sha256.Sum256(image)) ||
		result.Threshold == nil || *result.Threshold != 0.5 || result.ReleaseAllowed == nil || len(result.Scores) != 3 {
		return failed
	}
	report := studioModerationReport{State: "passed", Model: result.Model, Revision: result.Revision,
		Threshold: *result.Threshold, Scores: make(map[string]float64)}
	for _, policy := range []string{"sexual", "violence", "dangerous"} {
		score := result.Scores[policy]
		if score == nil || math.IsNaN(*score) || math.IsInf(*score, 0) || *score < 0 || *score > 1 {
			return failed
		}
		report.Scores[policy] = *score
		if *score >= report.Threshold {
			report.State = "blocked"
		}
	}
	allowed := report.State == "passed"
	if *result.ReleaseAllowed != allowed || (allowed && result.Decision != "allow") || (!allowed && result.Decision != "block") {
		return failed
	}
	return report
}

// This is the only exit for a successful Studio relay. Hold the provider PNG
// until both moderation and its audit have completed; blocked/error bytes never
// reach the dispatcher, browser, thumbnail generator or IndexedDB.
func deliverStudioOutput(c *gin.Context, buffer *studioRelayWriter) {
	if buffer.overflow {
		c.AbortWithStatusJSON(502, gin.H{"error": "image_result_limit"})
		return
	}
	values := map[string]any{"state": "failed"}
	report := studioModerationReport{State: "not_checked"}
	validImage := false
	if buffer.status == http.StatusOK {
		var result struct {
			Data []struct {
				Image string `json:"b64_json"`
			} `json:"data"`
		}
		if common.Unmarshal(buffer.body.Bytes(), &result) == nil && len(result.Data) == 1 {
			if image, err := base64.StdEncoding.Strict().DecodeString(result.Data[0].Image); err == nil && bytes.HasPrefix(image, []byte("\x89PNG\r\n\x1a\n")) {
				validImage = true
				values["output_hash"] = fmt.Sprintf("%x", sha256.Sum256(image))
				if c.GetString("image_api_moderation") == "external" {
					report = studioModerationReport{State: "external", Source: "integrator"}
				} else {
					report = checkStudioOutput(c.Request.Context(), image)
					if c.GetString("image_api_moderation") == "required" && report.State == "disabled" {
						report = studioModerationReport{State: "error", Error: "output_check_unavailable"}
					}
				}
				if report.State == "passed" || report.State == "disabled" || report.State == "external" {
					values["state"] = "completed"
				}
			}
		}
		if !validImage {
			report = studioModerationReport{State: "error", Error: "output_check_unavailable"}
		}
	}
	values["moderation_state"] = report.State
	encoded, _ := common.Marshal(report)
	values["moderation_result"] = string(encoded)
	if model.ImageStudioAuditEnabled() {
		if model.UpdateImageStudioAudit(c.GetString(common.RequestIdKey), c.GetInt("id"), values) != nil {
			c.AbortWithStatusJSON(503, gin.H{"error": "audit_unavailable"})
			return
		}
	}
	if buffer.status == http.StatusOK && (report.State == "blocked" || report.State == "error") {
		code := "output_check_unavailable"
		if report.State == "blocked" {
			code = "output_blocked"
		}
		// A terminal, post-inference decision. The dispatcher must not replay
		// generation. Identity binds this message to its private callback.
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
			"error": code, "protocol": "image-studio-output-v1",
			"job_id": c.GetString(common.RequestIdKey), "owner": fmt.Sprintf("cuberouter-%d", c.GetInt("id")),
		})
		return
	}
	c.Data(buffer.status, "application/json", buffer.body.Bytes())
}
