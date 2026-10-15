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
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDynamicImageChannelsFollowLiveManagementChanges(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	previous, slots := model.DB, imageSlots
	model.DB, imageSlots = db, newImageChannelSlots()
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { model.DB, imageSlots = previous, slots; sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" {
			w.WriteHeader(401)
			return
		}
		_, _ = w.Write([]byte(`{"state":"ready","gpu_count":1}`))
	}))
	defer endpoint.Close()
	add := func(id int, group, name string) {
		require.NoError(t, db.Create(&model.Channel{Id: id, Status: common.ChannelStatusEnabled, BaseURL: &endpoint.URL, Key: "key", Models: name, Group: group}).Error)
		require.NoError(t, db.Create(&model.Ability{ChannelId: id, Group: group, Model: name, Enabled: true}).Error)
	}
	add(1, "trial", "qwen-image-2.1")
	add(2, "trial", "qwen-image-2.1")
	add(3, "other-group", "qwen-image-2.1")
	add(4, "trial", "chat-model")
	t.Setenv("IMAGE_STUDIO_CHANNEL_MODEL", "qwen-image-2.1")
	t.Setenv("IMAGE_STUDIO_CHANNEL_IDS", "auto")
	ctx := func() *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/v1/images/generations", nil)
		common.SetContextKey(c, constant.ContextKeyImageStudio, true)
		return c
	}
	selectChannel := func(expected int) func() {
		ch, g, release, handled, err := reserveConfiguredImageChannel(ctx(), "qwen-image-2.1", "trial", 0)
		require.NoError(t, err)
		require.True(t, handled)
		require.Equal(t, "trial", g)
		require.Equal(t, expected, ch.Id)
		t.Cleanup(release)
		return release
	}
	first := selectChannel(1)
	second := selectChannel(2)
	_, _, _, handled, err := reserveConfiguredImageChannel(ctx(), "qwen-image-2.1", "trial", 0)
	require.True(t, handled)
	require.ErrorIs(t, err, ErrImageChannelBusy)
	count, err := ImageChannelPoolCapacity()
	require.NoError(t, err)
	require.Equal(t, 3, count)
	// Add/replacement becomes eligible without a restart or an env change.
	add(8, "trial", "qwen-image-2.1")
	eighth := selectChannel(8)
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 1).Update("status", common.ChannelStatusManuallyDisabled).Error)
	require.True(t, imageSlots.busy[1], "disable must not release running inference")
	first()
	// Deleting a running channel also drains instead of releasing its lease.
	require.NoError(t, db.Delete(&model.Channel{}, 2).Error)
	require.True(t, imageSlots.busy[2])
	second()
	add(9, "trial", "qwen-image-2.1")
	ninth := selectChannel(9)
	ninth()
	eighth()
	// A group edit takes effect without refreshing the channel cache.
	require.NoError(t, db.Model(&model.Ability{}).Where("channel_id = ?", 9).Update("group", "other-group").Error)
	again := selectChannel(8)
	again()
	_, _, _, handled, err = reserveConfiguredImageChannel(ctx(), "qwen-image-2.1", "trial", 9)
	require.True(t, handled)
	require.Error(t, err)
	// Base URL and key updates are read fresh too.
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 8).Update("key", "wrong").Error)
	_, _, _, _, err = reserveConfiguredImageChannel(ctx(), "qwen-image-2.1", "trial", 0)
	require.ErrorIs(t, err, ErrImageChannelBusy)
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 8).Update("key", "key").Error)
	final := selectChannel(8)
	final()
	_, _, _, handled, err = reserveConfiguredImageChannel(ctx(), "chat-model", "trial", 0)
	require.False(t, handled)
	require.NoError(t, err, "ordinary token/chat routing is unchanged")
	require.NoError(t, db.Model(&model.Channel{}).Where("status = ?", common.ChannelStatusEnabled).Update("status", common.ChannelStatusManuallyDisabled).Error)
	_, _, _, _, err = reserveConfiguredImageChannel(ctx(), "qwen-image-2.1", "trial", 0)
	require.ErrorIs(t, err, ErrImageChannelBusy, "last-channel removal must preserve queued work during refresh lag")
}
