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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 成员模型白名单可见性用例的三个模型：白名单放行前两个，第三个只在分组内可见。
const (
	memberAccessAllowedModelOne = "zz-member-access-allowed-a"
	memberAccessAllowedModelTwo = "zz-member-access-allowed-b"
	memberAccessGroupOnlyModel  = "zz-member-access-group-only"
)

// setupUserModelAccessTestDB 建好 GetUserModels 链路要用的最小表：用户 / 组织 /
// 成员行（白名单来源）/ 能力（分组可见模型）。表给多了反而掩盖依赖，SQL 日志静音
// 只是不让 fixture 内部查询混进测试输出。
//
// 先走 initModelListColumnNames：分组能力查询依赖 InitDB 初始化过的方言列名
// （commonGroupCol），不跑一次它查询会静默返回空集。
func setupUserModelAccessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	initModelListColumnNames(t)

	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Organization{}, &model.OrganizationMember{}, &model.Ability{},
	))

	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// createMemberModelAccessTestUser 建一个 default 分组的用户；role 决定它是否平台
// 管理员（管理员不受成员白名单限制）。
func createMemberModelAccessTestUser(t *testing.T, db *gorm.DB, username string, role int) model.User {
	t.Helper()
	user := model.User{
		Username: username, Password: "password", AffCode: username,
		Role: role, Status: common.UserStatusEnabled, Group: "default",
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

// attachMemberModelAccess 给用户挂一条 active 成员行（连同它所属的组织），
// modelAccess 是 organization_members.model_access 列的原始 JSON，如 `["a","b"]`；
// 空串表示不限制。slug 由调用方保证唯一——同一个用户挂多条成员行时要能各挂各的。
func attachMemberModelAccess(t *testing.T, db *gorm.DB, userId int, slug string, modelAccess string) {
	t.Helper()
	organization := model.Organization{
		Name:      slug,
		Slug:      slug,
		Group:     "default",
		Status:    model.OrganizationStatusActive,
		CreatedBy: userId,
	}
	require.NoError(t, db.Create(&organization).Error)
	require.NoError(t, db.Create(&model.OrganizationMember{
		OrganizationId: organization.Id,
		UserId:         userId,
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusActive,
		ModelAccess:    modelAccess,
	}).Error)
}

// seedMemberModelAccessGroupAbilities 让给定模型在 default 分组可见。
func seedMemberModelAccessGroupAbilities(t *testing.T, db *gorm.DB, models ...string) {
	t.Helper()
	abilities := make([]model.Ability, 0, len(models))
	for _, modelName := range models {
		abilities = append(abilities, model.Ability{
			Group: "default", Model: modelName, ChannelId: 1, Enabled: true,
		})
	}
	require.NoError(t, db.Create(&abilities).Error)
}

// performGetUserModelsRequest 直调 handler：补齐路由层提供的 :id 参数与鉴权中间件
// 写入的操作者身份。targetId 为空表示本人视图（无 :id）。
func performGetUserModelsRequest(t *testing.T, targetId string, viewerId int) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/user/models", nil)
	if targetId != "" {
		c.Params = gin.Params{{Key: "id", Value: targetId}}
	}
	c.Set("id", viewerId)
	GetUserModels(c)
	return recorder
}

// TestGetUserModelsFiltersByMemberModelAccess 可见模型按被查用户的白名单收窄：
// 分组内可见 a/b/c、白名单 [a,b] → 只回 a/b；白名单为空集 → 一个都不回。
func TestGetUserModelsFiltersByMemberModelAccess(t *testing.T) {
	db := setupUserModelAccessTestDB(t)
	seedMemberModelAccessGroupAbilities(t, db,
		memberAccessAllowedModelOne, memberAccessAllowedModelTwo, memberAccessGroupOnlyModel)
	allowList := fmt.Sprintf(`["%s","%s"]`, memberAccessAllowedModelOne, memberAccessAllowedModelTwo)

	t.Run("querying a restricted user by :id", func(t *testing.T) {
		operator := createMemberModelAccessTestUser(t, db, "member-access-operator", common.RoleAdminUser)
		target := createMemberModelAccessTestUser(t, db, "member-access-target", common.RoleCommonUser)
		attachMemberModelAccess(t, db, target.Id, "member-access-target", allowList)

		models := decodeUserModelsResponse(t,
			performGetUserModelsRequest(t, strconv.Itoa(target.Id), operator.Id))

		require.ElementsMatch(t, []string{memberAccessAllowedModelOne, memberAccessAllowedModelTwo}, models)
	})

	t.Run("the restricted user querying themselves", func(t *testing.T) {
		user := createMemberModelAccessTestUser(t, db, "member-access-self", common.RoleCommonUser)
		attachMemberModelAccess(t, db, user.Id, "member-access-self", allowList)

		models := decodeUserModelsResponse(t, performGetUserModelsRequest(t, "", user.Id))

		require.ElementsMatch(t, []string{memberAccessAllowedModelOne, memberAccessAllowedModelTwo}, models)
	})

	t.Run("an empty intersection hides every model", func(t *testing.T) {
		// 空白名单只能由多条受限成员行的交集产生：库里存的 `[]` 表示不限制（解除配置）。
		user := createMemberModelAccessTestUser(t, db, "member-access-none", common.RoleCommonUser)
		attachMemberModelAccess(t, db, user.Id, "member-access-none-a", fmt.Sprintf(`["%s"]`, memberAccessAllowedModelOne))
		attachMemberModelAccess(t, db, user.Id, "member-access-none-b", fmt.Sprintf(`["%s"]`, memberAccessAllowedModelTwo))

		models := decodeUserModelsResponse(t, performGetUserModelsRequest(t, "", user.Id))

		require.Empty(t, models)
	})
}

// TestGetUserModelsMemberModelAccessNilLeavesResponseUnchanged 守住 nil 直通：没有
// 任何成员白名单的用户，响应必须与过滤前逐字节一致——模型列表仍由
// GetGroupsEnabledModels 原样下发（形状与内容都不变）。
func TestGetUserModelsMemberModelAccessNilLeavesResponseUnchanged(t *testing.T) {
	db := setupUserModelAccessTestDB(t)
	seedMemberModelAccessGroupAbilities(t, db,
		memberAccessAllowedModelOne, memberAccessAllowedModelTwo, memberAccessGroupOnlyModel)
	user := createMemberModelAccessTestUser(t, db, "member-access-unrestricted", common.RoleCommonUser)

	// 基线响应用 gin 自己的渲染器生成：断言的是「响应形状 + 过滤前的模型列表」，
	// 不把 JSON 编码器差异掺进这条回归。
	baseline := service.GetGroupsEnabledModels([]string{"default"})
	require.Len(t, baseline, 3, "fixture 必须真的提供了分组可见模型，否则这条回归退化为空对空")
	baselineRecorder := httptest.NewRecorder()
	baselineCtx, _ := gin.CreateTestContext(baselineRecorder)
	baselineCtx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    baseline,
	})

	recorder := performGetUserModelsRequest(t, "", user.Id)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, baselineRecorder.Body.String(), recorder.Body.String())
}

// TestGetUserModelsMemberModelAccessExemptsAdminTarget 平台管理员即便挂着受限成员行
// 也不受白名单影响：可见模型与无限制用户完全一致（含顺序）。
func TestGetUserModelsMemberModelAccessExemptsAdminTarget(t *testing.T) {
	db := setupUserModelAccessTestDB(t)
	seedMemberModelAccessGroupAbilities(t, db,
		memberAccessAllowedModelOne, memberAccessAllowedModelTwo, memberAccessGroupOnlyModel)
	operator := createMemberModelAccessTestUser(t, db, "member-access-admin-operator", common.RoleAdminUser)
	target := createMemberModelAccessTestUser(t, db, "member-access-admin-target", common.RoleAdminUser)
	attachMemberModelAccess(t, db, target.Id, "member-access-admin-target", fmt.Sprintf(`["%s"]`, memberAccessAllowedModelOne))

	models := decodeUserModelsResponse(t,
		performGetUserModelsRequest(t, strconv.Itoa(target.Id), operator.Id))

	require.Len(t, models, 3, "管理员目标必须仍看到全部三个分组可见模型")
	require.Equal(t, service.GetGroupsEnabledModels([]string{"default"}), models)
}
