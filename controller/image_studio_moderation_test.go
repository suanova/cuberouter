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
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var reviewPNG = []byte("\x89PNG\r\n\x1a\nfixture-only-not-a-real-image")

func reviewResponse(image []byte) map[string]any {
	return map[string]any{"protocol": "image-moderation-v1", "model": studioModerationModel,
		"revision": strings.Repeat("a", 40), "image_sha256": fmt.Sprintf("%x", sha256.Sum256(image)),
		"weights_verified": true, "decision": "allow", "release_allowed": true,
		"threshold": 0.5, "scores": map[string]float64{"sexual": 0.01, "violence": 0.02, "dangerous": 0.03}}
}

func setupReview(t *testing.T, change func(map[string]any), status int) {
	t.Helper()
	setupReviewDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer "+strings.Repeat("s", 40), r.Header.Get("Authorization"))
		image, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, reviewPNG, image)
		data := reviewResponse(image)
		if change != nil {
			change(data)
		}
		body, err := common.Marshal(data)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	t.Setenv("IMAGE_STUDIO_MODERATION_ENABLED", "true")
	t.Setenv("IMAGE_STUDIO_MODERATION_URL", server.URL)
	t.Setenv("IMAGE_STUDIO_MODERATION_TOKEN", strings.Repeat("s", 40))
}

func TestStudioOutputDecisionIsBoundToExactImageAndAllPolicies(t *testing.T) {
	tests := []struct {
		name, want string
		change     func(map[string]any)
		status     int
	}{
		{"safe", "passed", nil, 200},
		{"sexual threshold", "blocked", func(d map[string]any) {
			d["scores"].(map[string]float64)["sexual"] = 0.5
			d["decision"] = "block"
			d["release_allowed"] = false
		}, 200},
		{"violence", "blocked", func(d map[string]any) {
			d["scores"].(map[string]float64)["violence"] = 0.9
			d["decision"] = "block"
			d["release_allowed"] = false
		}, 200},
		{"dangerous", "blocked", func(d map[string]any) {
			d["scores"].(map[string]float64)["dangerous"] = 0.9
			d["decision"] = "block"
			d["release_allowed"] = false
		}, 200},
		{"wrong hash", "error", func(d map[string]any) { d["image_sha256"] = "wrong" }, 200},
		{"missing policy", "error", func(d map[string]any) { delete(d["scores"].(map[string]float64), "violence") }, 200},
		{"null policy", "error", func(d map[string]any) { d["scores"] = map[string]any{"sexual": nil, "violence": 0.0, "dangerous": 0.0} }, 200},
		{"invalid score", "error", func(d map[string]any) { d["scores"].(map[string]float64)["sexual"] = -0.1 }, 200},
		{"contradiction", "error", func(d map[string]any) { d["scores"].(map[string]float64)["sexual"] = 0.9 }, 200},
		{"unverified weights", "error", func(d map[string]any) { d["weights_verified"] = false }, 200},
		{"wrong model", "error", func(d map[string]any) { d["model"] = "other" }, 200},
		{"no revision", "error", func(d map[string]any) { delete(d, "revision") }, 200},
		{"no allow flag", "error", func(d map[string]any) { delete(d, "release_allowed") }, 200},
		{"different threshold", "error", func(d map[string]any) { d["threshold"] = 0.9 }, 200},
		{"oversized", "error", func(d map[string]any) { d["extra"] = strings.Repeat("x", 16385) }, 200},
		{"unavailable", "error", nil, 503}, {"unauthenticated", "error", nil, 401},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setupReview(t, tc.change, tc.status)
			report := checkStudioOutput(context.Background(), reviewPNG)
			require.Equal(t, tc.want, report.State)
		})
	}
}

func TestStudioReviewNeverFollowsRedirectAndFailsClosedOnConfigurationOrCancellation(t *testing.T) {
	setupReview(t, nil, 200)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Equal(t, "error", checkStudioOutput(ctx, reviewPNG).State)
	for _, endpoint := range []string{"http://example.com/check", "https://user:password@example.com", ":invalid", ""} {
		t.Setenv("IMAGE_STUDIO_MODERATION_URL", endpoint)
		require.Equal(t, "error", checkStudioOutput(context.Background(), reviewPNG).State)
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "https://example.com", 307) }))
	defer redirect.Close()
	t.Setenv("IMAGE_STUDIO_MODERATION_URL", redirect.URL)
	require.Equal(t, "error", checkStudioOutput(context.Background(), reviewPNG).State)
	t.Setenv("IMAGE_STUDIO_MODERATION_ENABLED", "tru")
	require.Equal(t, "error", checkStudioOutput(context.Background(), reviewPNG).State)
	t.Setenv("IMAGE_STUDIO_MODERATION_ENABLED", "false")
	require.Equal(t, "disabled", checkStudioOutput(context.Background(), reviewPNG).State)
}

func TestStudioDeliveryWithholdsRejectedBytesAndRecordsAudit(t *testing.T) {
	for _, state := range []string{"passed", "blocked", "error"} {
		t.Run(state, func(t *testing.T) {
			setupReview(t, func(d map[string]any) {
				if state == "blocked" {
					d["scores"].(map[string]float64)["sexual"] = 0.99
					d["decision"] = "block"
					d["release_allowed"] = false
				}
				if state == "error" {
					d["weights_verified"] = false
				}
			}, 200)
			t.Setenv("IMAGE_STUDIO_AUDIT_ENABLED", "true")
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			oldDB := model.DB
			model.DB = db
			t.Cleanup(func() { model.DB = oldDB })
			sqlDB, _ := db.DB()
			sqlDB.SetMaxOpenConns(1)
			defer sqlDB.Close()
			require.NoError(t, db.AutoMigrate(&model.ImageStudioAudit{}, &model.ImageModerationPolicy{}, &model.ImageModerationLease{}))
			require.NoError(t, model.CreateImageStudioAudit(&model.ImageStudioAudit{JobID: "job-123", UserID: 71, UserPrompt: "test prompt"}))
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest("POST", "/", nil)
			c.Set("id", 71)
			c.Set(common.RequestIdKey, "job-123")
			encoded := base64.StdEncoding.EncodeToString(reviewPNG)
			body, _ := common.Marshal(map[string]any{"data": []map[string]string{{"b64_json": encoded}}})
			buffer := &studioRelayWriter{ResponseWriter: c.Writer, status: 200}
			buffer.body.Write(body)
			deliverStudioOutput(c, buffer)
			if state == "passed" {
				require.Equal(t, 200, recorder.Code)
				require.Equal(t, body, recorder.Body.Bytes())
			} else {
				require.Equal(t, 422, recorder.Code)
				require.NotContains(t, recorder.Body.String(), encoded)
				require.NotContains(t, recorder.Body.String(), "b64_json")
				require.Contains(t, recorder.Body.String(), "image-studio-output-v1")
				require.Contains(t, recorder.Body.String(), "cuberouter-71")
			}
			audits, err := model.FindImageStudioAudits("job-123", "", 71)
			require.NoError(t, err)
			require.Len(t, audits, 1)
			require.Equal(t, state, audits[0].ModerationState)
			require.Equal(t, fmt.Sprintf("%x", sha256.Sum256(reviewPNG)), audits[0].OutputHash)
			require.NotContains(t, audits[0].ModerationResult, encoded)
			// A missing audit row must also prevent release, not count as a
			// successful UPDATE with zero affected records.
			recorder = httptest.NewRecorder()
			c, _ = gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest("POST", "/", nil)
			c.Set("id", 72)
			c.Set(common.RequestIdKey, "missing-job")
			deliverStudioOutput(c, buffer)
			require.Equal(t, 503, recorder.Code)
			require.NotContains(t, recorder.Body.String(), encoded)
			// Even an allowed image is not released if the configured audit write fails.
			require.NoError(t, db.Migrator().DropTable(&model.ImageStudioAudit{}))
			recorder = httptest.NewRecorder()
			c, _ = gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest("POST", "/", nil)
			deliverStudioOutput(c, buffer)
			require.Equal(t, 503, recorder.Code)
			require.NotContains(t, recorder.Body.String(), encoded)
		})
	}
}

// Opt-in check against the real GPU service. Environment secrets never enter
// test output; the fixture must be an existing benign PNG, never generated here.
func TestStudioRealOutputClassifier(t *testing.T) {
	path := os.Getenv("STUDIO_REVIEW_TEST_PNG")
	if path == "" {
		t.Skip("GPU integration is opt-in")
	}
	setupReviewDB(t)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(data, []byte("\x89PNG")))
	report := checkStudioOutput(context.Background(), data)
	require.Equal(t, "passed", report.State)
	require.Equal(t, studioModerationModel, report.Model)
}
