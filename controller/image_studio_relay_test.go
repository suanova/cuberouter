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
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestReservedImageChannelDoesNotReplayAnAmbiguousFailure(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := types.NewOpenAIError(errors.New("upstream connection lost"), types.ErrorCode("channel:connection_lost"), http.StatusBadGateway)
	require.True(t, shouldRetry(c, err, 2))
	// API-token image requests use the pool too, without the Studio context flag.
	c.Set("image_channel_reserved", true)
	require.False(t, shouldRetry(c, err, 2))
}

func studioTestParameters() studioParameters {
	return studioParameters{Model: "2.1", Mode: "create", Prompt: "藍線位置加花 🌸\n保留其餘內容", Width: 512, Height: 512, Steps: 4, Seed: 123, CFG: 1}
}

func TestStudioChannelPayloadPreservesPromptAndTenImages(t *testing.T) {
	p := studioTestParameters()
	body, kind, path, err := studioRelayBody(p, "qwen-image-2.1")
	require.NoError(t, err)
	require.Equal(t, "/v1/images/generations", path)
	require.Equal(t, "application/json", kind)
	var request map[string]any
	require.NoError(t, common.Unmarshal(body, &request))
	require.Equal(t, p.Prompt, request["prompt"])
	require.Equal(t, "qwen-image-2.1", request["model"])
	require.Equal(t, float64(123), request["extra_fields"].(map[string]any)["seed"])
	p.Mode = "edit"
	for i := 0; i < 10; i++ {
		p.Images = append(p.Images, base64.StdEncoding.EncodeToString(append([]byte("\x89PNG\r\n\x1a\n"), byte(i))))
	}
	body, kind, path, err = studioRelayBody(p, "qwen-image-2.1")
	require.NoError(t, err)
	require.Equal(t, "/v1/images/edits", path)
	r := httptest.NewRequest("POST", path, bytes.NewReader(body))
	r.Header.Set("Content-Type", kind)
	require.NoError(t, r.ParseMultipartForm(imageStudioBodyLimit))
	defer r.MultipartForm.RemoveAll()
	require.Equal(t, p.Prompt, r.FormValue("prompt"))
	files := r.MultipartForm.File["image[]"]
	require.Len(t, files, 10)
	for i, f := range files {
		opened, err := f.Open()
		require.NoError(t, err)
		actual, err := io.ReadAll(opened)
		opened.Close()
		require.NoError(t, err)
		require.Equal(t, p.Images[i], base64.StdEncoding.EncodeToString(actual))
	}
	p.Images = append(p.Images, p.Images[0])
	_, _, _, err = studioRelayBody(p, "qwen-image-2.1")
	require.Error(t, err)
}

func TestStudioPrivateRelayIdentityMemoryAndReplay(t *testing.T) {
	t.Setenv("IMAGE_STUDIO_ALLOWED_GROUPS", "trial")
	t.Setenv("IMAGE_STUDIO_RELAY_TOKEN", strings.Repeat("r", 40))
	t.Setenv("IMAGE_STUDIO_CHANNEL_MODEL", "qwen-image-2.1")
	t.Setenv("IMAGE_STUDIO_AUDIT_ENABLED", "true")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	oldDB, oldRedis := model.DB, common.RedisEnabled
	model.DB, common.RedisEnabled = db, false
	t.Cleanup(func() { model.DB, common.RedisEnabled = oldDB, oldRedis })
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.ImageStudioAudit{}))
	require.NoError(t, db.Create(&model.User{Id: 71, Username: "studio-test", Status: common.UserStatusEnabled, Group: "trial", Quota: 100000}).Error)
	router := gin.New()
	calls := 0
	router.POST("/internal/image-studio/execute", PrepareImageStudioRelay, func(c *gin.Context) {
		calls++
		require.Equal(t, 71, c.GetInt("id"))
		require.Equal(t, "trial", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		require.True(t, common.GetContextKeyBool(c, constant.ContextKeyImageStudio))
		require.Empty(t, c.GetHeader("Authorization"))
		require.Empty(t, c.GetHeader("Cookie"))
		storage, err := common.GetBodyStorage(c)
		require.NoError(t, err)
		require.False(t, storage.IsDisk())
		require.False(t, shouldRetry(c, nil, 5))
		c.Status(200)
	})
	request := func(auth, owner, job string) int {
		body, _ := common.Marshal(studioTestParameters())
		r := httptest.NewRequest("POST", "/internal/image-studio/execute", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", auth)
		r.Header.Set("X-Image-Owner", owner)
		r.Header.Set("X-Image-Job", job)
		r.Header.Set("Cookie", "browser-session=must-not-forward")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w.Code
	}
	key := strings.Repeat("a", 32)
	t.Cleanup(func() { studioExecutions.Lock(); delete(studioExecutions.seen, key); studioExecutions.Unlock() })
	auth := "Bearer " + strings.Repeat("r", 40)
	require.Equal(t, http.StatusUnauthorized, request("", "cuberouter-71", key))
	require.Equal(t, http.StatusForbidden, request(auth, "cuberouter-999", key))
	require.Equal(t, http.StatusOK, request(auth, "cuberouter-71", key))
	require.Equal(t, http.StatusConflict, request(auth, "cuberouter-71", key))
	require.Equal(t, 1, calls)
	audits, err := model.FindImageStudioAudits(key, "", 71)
	require.NoError(t, err)
	require.Len(t, audits, 1)
	require.Equal(t, "studio-test", audits[0].Username)
	require.Equal(t, studioTestParameters().Prompt, audits[0].ModelPrompt)
	require.Equal(t, "[]", audits[0].InputHashes)
	require.Equal(t, int64(model.ImageStudioAuditRetention.Seconds()), audits[0].ExpiresAt-audits[0].CreatedAt)
	// A previously accepted queued job cannot run after membership is revoked.
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 71).Update("group", "default").Error)
	require.Equal(t, http.StatusForbidden, request(auth, "cuberouter-71", strings.Repeat("c", 32)))
	require.Equal(t, 1, calls)
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 71).Update("group", "trial").Error)
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 71).Update("status", common.UserStatusDisabled).Error)
	require.Equal(t, http.StatusForbidden, request(auth, "cuberouter-71", strings.Repeat("b", 32)))
}
