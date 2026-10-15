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
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestStudioPrivateCapacityReflectsChannelChanges(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	old := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = old })
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	t.Setenv("IMAGE_STUDIO_RELAY_TOKEN", strings.Repeat("r", 40))
	t.Setenv("IMAGE_STUDIO_CHANNEL_MODEL", "qwen-image-2.1")
	t.Setenv("IMAGE_STUDIO_CHANNEL_IDS", "auto")
	call := func(auth bool) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/internal/image-studio/capacity", nil)
		if auth {
			c.Request.Header.Set("Authorization", "Bearer "+strings.Repeat("r", 40))
		}
		ImageStudioChannelCapacity(c)
		return w
	}
	require.Equal(t, 401, call(false).Code)
	require.Contains(t, call(true).Body.String(), `"capacity":0`)
	require.NoError(t, db.Create(&model.Channel{Id: 1, Status: common.ChannelStatusEnabled, Key: "private-key"}).Error)
	require.NoError(t, db.Create(&model.Ability{ChannelId: 1, Group: "trial", Model: "qwen-image-2.1", Enabled: true}).Error)
	response := call(true)
	require.Equal(t, 200, response.Code)
	require.Contains(t, response.Body.String(), `"capacity":1`)
	require.NotContains(t, response.Body.String(), "private-key")
	require.NoError(t, db.Model(&model.Channel{}).Where("id = 1").Update("status", 2).Error)
	require.Contains(t, call(true).Body.String(), `"capacity":0`)
}

func TestStudioBusyReplyRetriesOnlyBeforeInferenceAndLostReplyFences(t *testing.T) {
	registry := setupStudioRecoveryTest(t)
	t.Setenv("IMAGE_STUDIO_AUDIT_ENABLED", "false")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	oldDB, oldRedis := model.DB, common.RedisEnabled
	model.DB, common.RedisEnabled = db, false
	t.Cleanup(func() { model.DB, common.RedisEnabled = oldDB, oldRedis })
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	require.NoError(t, db.AutoMigrate(&model.User{}))
	require.NoError(t, db.Create(&model.User{Id: 71, Username: "trial", Status: common.UserStatusEnabled, Group: "trial"}).Error)
	ready := false
	generated := 0
	router := gin.New()
	router.POST("/execute", PrepareImageStudioRelay, func(c *gin.Context) {
		if !ready {
			c.Set("image_studio_pool_busy", true)
			c.Abort()
			return
		}
		generated++
		c.JSON(200, gin.H{"ok": true})
	})
	call := func(job string) *httptest.ResponseRecorder {
		b, _ := common.Marshal(studioTestParameters())
		req := httptest.NewRequest("POST", "/execute", bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+strings.Repeat("r", 40))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Image-Owner", "cuberouter-71")
		req.Header.Set("X-Image-Job", job)
		req.Header.Set("X-Image-Execution-Deadline", strconv.FormatInt(time.Now().Add(25*time.Minute).Unix(), 10))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	job := strings.Repeat("a", 32)
	for i := 0; i < 2; i++ {
		w := call(job)
		require.Equal(t, 429, w.Code)
		require.Contains(t, w.Body.String(), "image-studio-admission-v1")
		require.Zero(t, generated)
	}
	ready = true
	require.Equal(t, 200, call(job).Code)
	require.Equal(t, 1, generated)
	require.Equal(t, 409, call(job).Code)
	require.Equal(t, 1, generated)
	ready = false
	job = strings.Repeat("b", 32)
	require.Equal(t, 429, call(job).Code)
	state, err := registry.reconcile(job, "cuberouter-71")
	require.NoError(t, err)
	require.Equal(t, "fenced", state)
	ready = true
	require.Equal(t, 409, call(job).Code)
	require.Equal(t, 1, generated)
}
