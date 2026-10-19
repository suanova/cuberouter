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
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupModelListMemberAccessTestDB 是 /v1/models 用例的 fixture：模型列表的基础表
// （用户/渠道/能力/模型/供应商）外面再补白名单来源的两张表（组织/成员行）。
func setupModelListMemberAccessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Organization{}, &model.OrganizationMember{}))
	return db
}

// performListModelsRequest 直调 handler：补齐路由层提供的分组上下文。actorUserId
// > 0 时写入 actor_user_id（组织 Key 场景，按责任成员判定）；tokenModelLimit 非 nil
// 时打开令牌「可用模型限制」闸门。
func performListModelsRequest(t *testing.T, userId int, actorUserId int, tokenModelLimit map[string]bool) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	ctx.Set("id", userId)
	if actorUserId > 0 {
		common.SetContextKey(ctx, constant.ContextKeyActorUserId, actorUserId)
	}
	if tokenModelLimit != nil {
		common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
		common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, tokenModelLimit)
	}
	ListModels(ctx, constant.ChannelTypeOpenAI)
	return recorder
}

// listModelIDs 按响应顺序取出模型名。
func listModelIDs(t *testing.T, recorder *httptest.ResponseRecorder) []string {
	t.Helper()
	payload := decodeListModelsPayload(t, recorder)
	ids := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		ids = append(ids, item.Id)
	}
	return ids
}

// TestListModelsFiltersByMemberModelAccess /v1/models 与其它可见面同口径：受限成员
// 只看到白名单内的模型；组织 Key 场景按 actor（责任成员）判定，而不是令牌属主。
func TestListModelsFiltersByMemberModelAccess(t *testing.T) {
	withSelfUseModeEnabled(t)
	db := setupModelListMemberAccessTestDB(t)
	seedMemberModelAccessGroupAbilities(t, db,
		memberAccessAllowedModelOne, memberAccessAllowedModelTwo, memberAccessGroupOnlyModel)
	allowList := fmt.Sprintf(`["%s","%s"]`, memberAccessAllowedModelOne, memberAccessAllowedModelTwo)

	t.Run("a restricted member lists only the allow-listed models", func(t *testing.T) {
		user := createMemberModelAccessTestUser(t, db, "list-models-member-access", common.RoleCommonUser)
		attachMemberModelAccess(t, db, user.Id, "list-models-member-access", allowList)

		ids := listModelIDs(t, performListModelsRequest(t, user.Id, 0, nil))

		require.ElementsMatch(t, []string{memberAccessAllowedModelOne, memberAccessAllowedModelTwo}, ids)
	})

	t.Run("the responsible member wins over the token owner", func(t *testing.T) {
		// 组织 Key：令牌属主不受限，但责任成员受限——判定必须落在 actor 上。
		owner := createMemberModelAccessTestUser(t, db, "list-models-token-owner", common.RoleCommonUser)
		member := createMemberModelAccessTestUser(t, db, "list-models-responsible", common.RoleCommonUser)
		attachMemberModelAccess(t, db, member.Id, "list-models-responsible",
			fmt.Sprintf(`["%s"]`, memberAccessAllowedModelOne))

		ids := listModelIDs(t, performListModelsRequest(t, owner.Id, member.Id, nil))

		require.Equal(t, []string{memberAccessAllowedModelOne}, ids)
	})
}

// TestListModelsMemberModelAccessNilLeavesListingUnchanged 守住 nil 直通：不受限用户
// （无成员行、平台管理员）的响应必须与按过滤前配方重建的响应逐字节一致。
func TestListModelsMemberModelAccessNilLeavesListingUnchanged(t *testing.T) {
	withSelfUseModeEnabled(t)
	db := setupModelListMemberAccessTestDB(t)
	seedMemberModelAccessGroupAbilities(t, db,
		memberAccessAllowedModelOne, memberAccessAllowedModelTwo, memberAccessGroupOnlyModel)

	// 基线 = 过滤前 handler 的下发内容：同一份分组可见模型、同一套 owner 解析与
	// 模型构造，只是没有白名单这一道。
	baseline := service.GetGroupsEnabledModels([]string{"default"})
	require.Len(t, baseline, 3, "fixture 必须真的提供了分组可见模型，否则这条回归退化为空对空")
	ownerByModel := getPreferredModelOwners(baseline, []string{"default"})
	expectedModels := make([]dto.OpenAIModels, 0, len(baseline))
	for _, modelName := range baseline {
		expectedModels = append(expectedModels, buildOpenAIModel(modelName, ownerByModel))
	}
	baselineRecorder := httptest.NewRecorder()
	baselineCtx, _ := gin.CreateTestContext(baselineRecorder)
	baselineCtx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    expectedModels,
		"object":  "list",
	})

	t.Run("a user without a membership", func(t *testing.T) {
		user := createMemberModelAccessTestUser(t, db, "list-models-unrestricted", common.RoleCommonUser)

		recorder := performListModelsRequest(t, user.Id, 0, nil)

		require.Equal(t, baselineRecorder.Body.String(), recorder.Body.String())
	})

	t.Run("a platform administrator with a restricted membership", func(t *testing.T) {
		admin := createMemberModelAccessTestUser(t, db, "list-models-admin", common.RoleAdminUser)
		attachMemberModelAccess(t, db, admin.Id, "list-models-admin",
			fmt.Sprintf(`["%s"]`, memberAccessAllowedModelOne))

		recorder := performListModelsRequest(t, admin.Id, 0, nil)

		require.Equal(t, baselineRecorder.Body.String(), recorder.Body.String())
	})
}

// TestListModelsIntersectsTokenModelLimitAndMemberAccess 两道闸门叠加：令牌限制先
// 收窄，成员白名单再收窄，结果是两者的交集（谁都不单独放行）。
func TestListModelsIntersectsTokenModelLimitAndMemberAccess(t *testing.T) {
	withSelfUseModeEnabled(t)
	db := setupModelListMemberAccessTestDB(t)
	seedMemberModelAccessGroupAbilities(t, db,
		memberAccessAllowedModelOne, memberAccessAllowedModelTwo, memberAccessGroupOnlyModel)
	user := createMemberModelAccessTestUser(t, db, "list-models-intersection", common.RoleCommonUser)
	// 白名单 [a, c]：a 两道都过；b 只过令牌限制；c 只过白名单 → 只有 a 留下。
	attachMemberModelAccess(t, db, user.Id, "list-models-intersection",
		fmt.Sprintf(`["%s","%s"]`, memberAccessAllowedModelOne, memberAccessGroupOnlyModel))

	ids := listModelIDs(t, performListModelsRequest(t, user.Id, 0, map[string]bool{
		memberAccessAllowedModelOne: true,
		memberAccessAllowedModelTwo: true,
	}))

	require.Equal(t, []string{memberAccessAllowedModelOne}, ids)
}
