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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/studioqueue"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestStudioImageSubmitCancelsAdmittedJobWhenAuditInsertFails(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.StudioImageAudit{}, &model.StudioImageLease{}))
	t.Setenv("IMAGE_STUDIO_ENABLED", "true")
	t.Setenv("IMAGE_STUDIO_MODELS", "studio-audit-model")
	t.Setenv("IMAGE_STUDIO_CHANNEL_IDS", "7001")
	t.Setenv("IMAGE_STUDIO_ALLOWED_GROUPS", "image-studio")
	require.NoError(t, db.Create(&model.User{Id: 7007, Username: "audit-owner", Status: common.UserStatusEnabled, Group: "image-studio"}).Error)
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"state":"ready","gpu_count":1,"control_protocol":"qwen-execution-v1"}`)
	}))
	defer worker.Close()
	base := worker.URL
	require.NoError(t, db.Create(&model.Channel{Id: 7001, Type: constant.ChannelTypeImageEdit, Status: common.ChannelStatusEnabled, Name: "worker", Key: strings.Repeat("w", 32), BaseURL: &base, Models: "studio-audit-model", Group: "image-studio"}).Error)
	require.NoError(t, db.Create(&model.Channel{Id: 7002, Type: constant.ChannelTypeStudioReview, Status: common.ChannelStatusEnabled, Name: "reviewer", Key: strings.Repeat("r", 32), BaseURL: &base}).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "image-studio", Model: "studio-audit-model", ChannelId: 7001, Enabled: true}).Error)
	// Initialize the singleton once, then use an isolated queue with no dispatcher.
	// No GPU work can race the audit-write failure or survive this fixture.
	originalQueue := imageJobs()
	q := studioqueue.New(nil)
	studioQueue = q
	t.Cleanup(func() { studioQueue = originalQueue })
	const callback = "test:studio-audit-create-failure"
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "studio_image_audits" {
			tx.AddError(errors.New("forced audit insert failure"))
		}
	}))
	t.Cleanup(func() { db.Callback().Create().Remove(callback) })
	body := []byte(`{"mode":"create","user_prompt":"original","request":{"model":"studio-audit-model","prompt":"prepared","size":"256x256","n":1,"quality":"standard","num_inference_steps":1,"true_cfg_scale":0,"seed":0}}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/media-studio/image-jobs", bytes.NewReader(body))
	c.Request.Header.Set("Idempotency-Key", "audit-failure-request")
	c.Request.Header.Set("X-Trial-Epoch", q.Epoch())
	c.Set("id", 7007)
	SubmitStudioImageJob(c)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "studio_audit_unavailable", response.Error.Code)
	status, exists := q.Lookup(7007, "audit-failure-request")
	require.True(t, exists, "the failure occurs after admission, so the idempotency lookup remains available")
	assert.Equal(t, "cancelled", status.State)
	assert.True(t, status.CancelRequested)
}
