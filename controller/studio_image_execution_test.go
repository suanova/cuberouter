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
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/studioqueue"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudioImageNativeRelayReviewAndBilling(t *testing.T) {
	setupBillingAliasOptionDB(t)
	sqlDB, err := model.DB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitLogDB())
	require.NoError(t, model.DB.AutoMigrate(&model.Ability{}, &model.StudioImageAudit{}, &model.StudioImageLease{}))
	t.Setenv("IMAGE_STUDIO_ENABLED", "true")
	t.Setenv("IMAGE_STUDIO_MODELS", "studio-native")
	t.Setenv("IMAGE_STUDIO_CHANNEL_IDS", "1")
	t.Setenv("IMAGE_STUDIO_ALLOWED_GROUPS", "default")
	oldPrice, oldBatch, oldLog := ratio_setting.ModelPrice2JSONString(), common.BatchUpdateEnabled, common.LogConsumeEnabled
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = true
	t.Cleanup(func() {
		model.WaitForQuotaCacheWorkers()
		common.BatchUpdateEnabled = oldBatch
		common.LogConsumeEnabled = oldLog
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(oldPrice))
	})
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"studio-native":0.001}`))
	require.NoError(t, model.DB.Create(&model.User{Id: 1, Username: "studio", Role: common.RoleRootUser, Status: common.UserStatusEnabled, Group: "default", Quota: 1000000, Setting: `{"billing_preference":"wallet_only"}`}).Error)
	service.InitHttpClient()
	var encoded bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.NRGBA{R: 200, A: 255})
	require.NoError(t, png.Encode(&encoded, img))
	var policy atomic.Value
	policy.Store("allow")
	var generations atomic.Int32
	var cancelOnGeneration atomic.Pointer[chan struct{}]
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/health":
			_, _ = io.WriteString(w, `{"state":"ready","gpu_count":1,"control_protocol":"qwen-execution-v1"}`)
		case "/v1/images/generations":
			generations.Add(1)
			if cancel := cancelOnGeneration.Load(); cancel != nil {
				close(*cancel)
			}
			assert.Len(t, r.Header.Get("X-Image-Execution-ID"), 32)
			data, err := common.Marshal(map[string]any{"created": 1, "data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(encoded.Bytes())}}})
			assert.NoError(t, err)
			_, _ = w.Write(data)
		case "/v1/moderate":
			imageBytes, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.Equal(t, encoded.Bytes(), imageBytes)
			decision := policy.Load().(string)
			if decision == "cancel" {
				decision = "allow"
			}
			if decision == "down" {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			allow := decision == "allow"
			score := 0.1
			if !allow {
				score = 0.7
			}
			data, err := common.Marshal(service.StudioReview{Protocol: "image-moderation-v1", Model: "google/shieldgemma-2-4b-it", Revision: strings.Repeat("a", 40), ImageSHA256: fmt.Sprintf("%x", sha256.Sum256(imageBytes)), Decision: decision, ReleaseAllowed: &allow, WeightsVerified: true, Threshold: func() *float64 { n := 0.5; return &n }(), Scores: map[string]*float64{"sexual": &score, "violence": &score, "dangerous": &score}})
			assert.NoError(t, err)
			_, _ = w.Write(data)
		case "/internal/cancel":
			var input map[string]string
			assert.NoError(t, common.DecodeJson(r.Body, &input))
			data, err := common.Marshal(map[string]string{"protocol": "qwen-execution-v1", "execution_id": input["execution_id"], "state": "cancelling"})
			assert.NoError(t, err)
			_, _ = w.Write(data)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	base, review := upstream.URL, upstream.URL+"/v1/moderate"
	require.NoError(t, model.DB.Create(&model.Channel{Id: 1, Name: "worker", Type: constant.ChannelTypeImageEdit, Status: common.ChannelStatusEnabled, Key: strings.Repeat("g", 32), BaseURL: &base, Group: "default", Models: "studio-native"}).Error)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 2, Name: "reviewer", Type: constant.ChannelTypeStudioReview, Status: common.ChannelStatusEnabled, Key: strings.Repeat("m", 32), BaseURL: &review, Group: "default", Models: "google/shieldgemma-2-4b-it"}).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: "default", Model: "studio-native", ChannelId: 1, Enabled: true}).Error)
	for i, decision := range []string{"allow", "block", "cancel", "down"} {
		t.Run(decision, func(t *testing.T) {
			policy.Store(decision)
			request := []byte(`{"mode":"create","user_prompt":"user prompt","request":{"model":"studio-native","prompt":"model prompt","size":"256x256","n":1,"quality":"standard","num_inference_steps":1,"true_cfg_scale":0,"seed":0}}`)
			job := fmt.Sprintf("%032x", i+1)
			reservation := reserveStudioImage(studioqueue.Request{Owner: 1, Key: job, Model: "studio-native", Mode: "create", Body: request}, job)
			require.NotNil(t, reservation)
			var before model.User
			require.NoError(t, model.DB.First(&before, 1).Error)
			cancel := make(chan struct{})
			cancelOnGeneration.Store(nil)
			if decision == "cancel" {
				cancelOnGeneration.Store(&cancel)
			}
			result := reservation.Run(context.Background(), cancel)
			reservation.Release()
			model.WaitForQuotaCacheWorkers()
			var after model.User
			require.NoError(t, model.DB.First(&after, 1).Error)
			if decision == "cancel" {
				assert.Empty(t, result.PNG, "an in-flight cancellation discards the reviewed result")
				assert.Empty(t, result.Error)
				assert.Equal(t, before.Quota-500, after.Quota, "completed inference still settles its native charge")
			} else if decision == "allow" {
				assert.Empty(t, result.Error)
				assert.Equal(t, encoded.Bytes(), result.PNG)
				assert.Less(t, after.Quota, before.Quota, "approved output settles native image billing")
			} else {
				code := "output_blocked"
				if decision == "down" {
					code = "output_check_unavailable"
				}
				assert.Equal(t, code, result.Error)
				assert.Empty(t, result.PNG)
				assert.Equal(t, before.Quota, after.Quota, "rejected output refunds native pre-consume")
			}
			var audit model.StudioImageAudit
			require.NoError(t, model.DB.First(&audit, "job_id = ?", job).Error)
			assert.Equal(t, "user prompt", audit.UserPrompt)
			assert.Equal(t, "model prompt", audit.ModelPrompt)
			assert.Equal(t, fmt.Sprintf("%x", sha256.Sum256(encoded.Bytes())), audit.OutputHash)
			if decision == "cancel" {
				assert.Equal(t, "cancelled", audit.State)
			}
		})
	}
	t.Run("cancelled before provider dispatch", func(t *testing.T) {
		job := fmt.Sprintf("%032x", 5)
		request := []byte(`{"mode":"create","user_prompt":"cancelled","request":{"model":"studio-native","prompt":"cancelled","size":"256x256","n":1,"quality":"standard","num_inference_steps":1,"true_cfg_scale":0,"seed":0}}`)
		reservation := reserveStudioImage(studioqueue.Request{Owner: 1, Key: job, Model: "studio-native", Mode: "create", Body: request}, job)
		require.NotNil(t, reservation)
		var before, after model.User
		require.NoError(t, model.DB.First(&before, 1).Error)
		cancel := make(chan struct{})
		close(cancel)
		result := reservation.Run(context.Background(), cancel)
		reservation.Release()
		model.WaitForQuotaCacheWorkers()
		require.NoError(t, model.DB.First(&after, 1).Error)
		assert.Empty(t, result.PNG)
		assert.Empty(t, result.Error)
		assert.Equal(t, before.Quota, after.Quota)
		var audit model.StudioImageAudit
		require.NoError(t, model.DB.First(&audit, "job_id = ?", job).Error)
		assert.Equal(t, "cancelled", audit.State)
	})
	assert.EqualValues(t, 4, generations.Load(), "pre-dispatch cancellation never invokes inference")
}
