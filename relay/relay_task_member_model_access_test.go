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
package relay

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	appI18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupMemberModelAccessRelayTestDB 打开测试专属 sqlite 并接管全局句柄（同
// model / middleware 各自测试的既有模式）。Redis 显式关闭：common.RedisEnabled
// 包级默认 true，而测试环境没有 Redis 客户端。
func setupMemberModelAccessRelayTestDB(t *testing.T) {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RedisEnabled = previousRedisEnabled
	})
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = database, database
	require.NoError(t, database.AutoMigrate(
		&model.User{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.Task{},
		&model.Channel{},
	))
	t.Cleanup(func() {
		if sqlDB, err := database.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
}

func createMemberModelAccessRelayUser(t *testing.T, username string) model.User {
	t.Helper()
	user := model.User{
		Username:    username,
		Password:    "password",
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     "aff-" + username,
	}
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

// createMemberModelAccessRelayMembership 挂一条 active 成员行（连同它所属的组织）：
// 成员行只在组织 active 时才构成限制，缺组织行会让受限用例失效。
func createMemberModelAccessRelayMembership(t *testing.T, organizationId, userId int, modelAccess string) {
	t.Helper()
	var organizationCount int64
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organizationId).Count(&organizationCount).Error)
	if organizationCount == 0 {
		name := fmt.Sprintf("relay-member-model-access-org-%d", organizationId)
		require.NoError(t, model.DB.Create(&model.Organization{
			Id:        organizationId,
			Name:      name,
			Slug:      name,
			Status:    model.OrganizationStatusActive,
			CreatedBy: userId,
		}).Error)
	}
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organizationId,
		UserId:         userId,
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusActive,
		ModelAccess:    modelAccess,
	}).Error)
}

func insertMemberModelAccessRelayChannel(t *testing.T) *model.Channel {
	t.Helper()
	channel := &model.Channel{
		Name:   "relay-member-model-access",
		Key:    "sk-relay-member-model-access",
		Status: common.ChannelStatusEnabled,
		Group:  "default",
	}
	require.NoError(t, model.DB.Create(channel).Error)
	return channel
}

// insertMemberModelAccessRelayTask 写入一条属于 userId 的源任务：remix 判定的
// 模型名就取自它的 Properties.OriginModelName。
func insertMemberModelAccessRelayTask(t *testing.T, taskId string, userId, channelId int, originModel string) {
	t.Helper()
	task := &model.Task{
		TaskID:     taskId,
		UserId:     userId,
		ChannelId:  channelId,
		Action:     "text_to_video",
		Status:     model.TaskStatusSuccess,
		Properties: model.Properties{OriginModelName: originModel},
	}
	model.NormalizeTaskBillingScope(task)
	require.NoError(t, model.DB.Create(task).Error)
}

// memberModelAccessRemixContext 构造 ResolveOriginTask 需要的 remix 上下文与
// RelayInfo：路径与 video_id 决定 OriginTaskID，作用域键与 UserId 成对相等，
// asyncTaskScopeFromRelayInfo 才认这个 info；ChannelMeta 需非 nil（ChannelId
// 由它提升，线上由 InitChannelMeta 建立）。
func memberModelAccessRemixContext(t *testing.T, userId, actorUserId int, videoId string) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/"+videoId+"/remix", nil)
	c.Params = gin.Params{gin.Param{Key: "video_id", Value: videoId}}
	info := &relaycommon.RelayInfo{
		UserId:             userId,
		ActorUserId:        actorUserId,
		ScopeType:          model.AccountContextTypePersonal,
		ScopeId:            userId,
		BillingAccountType: model.AccountContextTypePersonal,
		BillingAccountId:   userId,
		ChannelMeta:        &relaycommon.ChannelMeta{},
		TaskRelayInfo:      &relaycommon.TaskRelayInfo{},
	}
	return c, info
}

// renderedMemberModelAccessMessage 按生产路径（i18n.T）渲染消息，避免硬编码文案。
func renderedMemberModelAccessMessage(key string, args map[string]any) string {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return appI18n.T(ctx, key, args)
}

// TestResolveOriginTaskMemberModelAccess 覆盖 remix 这条「提交类但模型为空」的
// 路径：Distribute 的空模型跳过覆盖不到它，这里在取得源任务模型后单点补判，
// 与令牌限制在 remix 上（空模型 miss）的既有行为对称。
func TestResolveOriginTaskMemberModelAccess(t *testing.T) {
	require.NoError(t, appI18n.Init())
	setupMemberModelAccessRelayTestDB(t)
	channel := insertMemberModelAccessRelayChannel(t)

	restricted := createMemberModelAccessRelayUser(t, "relay-member-access-restricted")
	createMemberModelAccessRelayMembership(t, 1, restricted.Id, `["allowed-model"]`)
	unrestricted := createMemberModelAccessRelayUser(t, "relay-member-access-unrestricted")
	insertMemberModelAccessRelayTask(t, "video-allowed", restricted.Id, channel.Id, "allowed-model")
	insertMemberModelAccessRelayTask(t, "video-denied", restricted.Id, channel.Id, "denied-model")
	insertMemberModelAccessRelayTask(t, "video-open", unrestricted.Id, channel.Id, "denied-model")

	tests := []struct {
		name          string
		userId        int
		actorUserId   int
		videoId       string
		wantForbidden bool
		wantModel     string // 源任务模型：拒绝时进文案，放行时校验回填结果
	}{
		{
			name:   "a source model outside the whitelist is rejected",
			userId: restricted.Id, actorUserId: restricted.Id, videoId: "video-denied",
			wantForbidden: true, wantModel: "denied-model",
		},
		{
			name:   "a source model inside the whitelist continues to the later path",
			userId: restricted.Id, actorUserId: restricted.Id, videoId: "video-allowed",
			wantModel: "allowed-model",
		},
		{
			name:   "an actor without a membership is unrestricted",
			userId: unrestricted.Id, actorUserId: unrestricted.Id, videoId: "video-open",
			wantModel: "denied-model",
		},
		{
			name:   "the actor user id falls back to the user id",
			userId: restricted.Id, actorUserId: 0, videoId: "video-denied",
			wantForbidden: true, wantModel: "denied-model",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, info := memberModelAccessRemixContext(t, tc.userId, tc.actorUserId, tc.videoId)

			taskErr := ResolveOriginTask(c, info)

			if !tc.wantForbidden {
				require.Nil(t, taskErr)
				// 走到后续路径：源任务模型已推导、渠道与 key 已锁定
				assert.Equal(t, tc.videoId, info.OriginTaskID)
				assert.Equal(t, tc.wantModel, info.OriginModelName)
				require.NotNil(t, info.LockedChannel)
				locked, ok := info.LockedChannel.(*model.Channel)
				require.True(t, ok)
				assert.Equal(t, channel.Id, locked.Id)
				assert.Equal(t, channel.Id, info.ChannelId)
				return
			}
			require.NotNil(t, taskErr)
			assert.Equal(t, http.StatusForbidden, taskErr.StatusCode)
			message := renderedMemberModelAccessMessage(appI18n.MsgDistributorMemberModelForbidden, map[string]any{"Model": tc.wantModel})
			require.Contains(t, message, tc.wantModel, "the rendered message must name the denied model")
			assert.Contains(t, taskErr.Message, message)
		})
	}
}
