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

// 注意（i18n 的全局副作用）：本文件的测试调用 i18n.Init()——它是 sync.Once，
// 首次调用即把 common.TranslateMessage 永久替换为 i18n.T，属进程级副作用。
// 因此 organization_test_support_test.go 里 translated() 的注释前提（「middleware
// 包内没有任何测试调用 i18n.Init()」）自本文件起不再成立；translated() 对桩与真实
// 实现两种形态都成立，其断言能力不受影响。
import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	appI18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupMemberModelAccessTestDB 建立成员模型白名单用例的 sqlite 环境（用户 / 成员行 /
// 渠道 / 能力 / 任务表），并显式走渠道数据库选择路径：本包其它用例可能改过该全局开关。
func setupMemberModelAccessTestDB(t *testing.T) {
	t.Helper()
	setupOrganizationMiddlewareTestDB(t,
		&model.User{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.Channel{},
		&model.Ability{},
		&model.Task{},
	)
	previousMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previousMemoryCache })
}

func createMemberModelAccessTestUser(t *testing.T, username string) model.User {
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

// createMemberModelAccessTestMembership 为用户挂一条 active 成员行；
// modelAccess 是 organization_members.model_access 的原始 JSON，空串表示不限制。
// 成员行只在组织 active 时才构成限制，所以这里同时保证组织行存在（缺失就补一条 active 的），
// 否则受限用例会因组织缺失而失效，测不到它宣称的限制。
func createMemberModelAccessTestMembership(t *testing.T, organizationId, userId int, modelAccess string) {
	t.Helper()
	var organizationCount int64
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organizationId).Count(&organizationCount).Error)
	if organizationCount == 0 {
		name := fmt.Sprintf("member-model-access-org-%d", organizationId)
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

// insertMemberModelAccessTestChannel 插入一个覆盖给定模型的可用渠道（Insert 会同时
// 写入 abilities），让"放行"用例能够走完整条选渠流程。
func insertMemberModelAccessTestChannel(t *testing.T, models ...string) *model.Channel {
	t.Helper()
	channel := &model.Channel{
		Name:   "member-model-access",
		Key:    "sk-member-model-access",
		Status: common.ChannelStatusEnabled,
		Type:   constant.ChannelTypeOpenAI,
		Models: strings.Join(models, ","),
		Group:  "default",
	}
	require.NoError(t, channel.Insert())
	return channel
}

// insertMemberModelAccessTestTask 写入一条属于 userId 的已完成任务记录，
// 回填模型名用的就是它的 Properties.OriginModelName。
func insertMemberModelAccessTestTask(t *testing.T, taskId string, userId int, originModel string) {
	t.Helper()
	task := &model.Task{
		TaskID:     taskId,
		UserId:     userId,
		Platform:   constant.TaskPlatformSuno,
		Action:     "text_to_video",
		Status:     model.TaskStatusSuccess,
		Properties: model.Properties{OriginModelName: originModel},
	}
	// 作用域列由 Normalize 写入：AsyncTaskScopeFromContext 的查询按 scope_id 精确匹配。
	model.NormalizeTaskBillingScope(task)
	require.NoError(t, model.DB.Create(task).Error)
}

// memberModelAccessContext 构造驱动 Distribute 的最小 gin 上下文：带 model 的
// chat completions 请求 + 使用分组；actorUserId<=0 时不设 actor 键（走回退分支）。
func memberModelAccessContext(t *testing.T, userId, actorUserId int, modelName string) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"`+modelName+`"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUserId, userId)
	if actorUserId > 0 {
		common.SetContextKey(c, constant.ContextKeyActorUserId, actorUserId)
	}
	return recorder, c
}

// memberModelAccessTaskStatusContext 构造任务状态端点的上下文：GET
// /v1/video/generations/:task_id，含 AsyncTaskScopeFromContext 需要的作用域键。
func memberModelAccessTaskStatusContext(t *testing.T, userId, actorUserId int, taskId string) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/video/generations/"+taskId, nil)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	authenticateAsTestUser(c, userId)
	if actorUserId > 0 {
		common.SetContextKey(c, constant.ContextKeyActorUserId, actorUserId)
	}
	c.Params = gin.Params{gin.Param{Key: "task_id", Value: taskId}}
	return recorder, c
}

// renderedMemberModelAccessMessage 按生产路径（i18n.T）渲染消息，避免把文案硬编码进断言。
func renderedMemberModelAccessMessage(key string, args map[string]any) string {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return appI18n.T(ctx, key, args)
}

// TestDistributeMemberModelAccessEnforcement 覆盖成员白名单在 distributor 的收窄语义：
// nil（无成员行）直通，受限用户只放行有效集内的模型（含 gpt-4-gizmo-* 归一），
// 空交集拒绝全部，actor 用户号缺失时回退到令牌用户号。
func TestDistributeMemberModelAccessEnforcement(t *testing.T) {
	require.NoError(t, appI18n.Init())
	setupMemberModelAccessTestDB(t)
	channel := insertMemberModelAccessTestChannel(t, "allowed-model", "unrestricted-model", "gpt-4-gizmo-abc")

	unrestricted := createMemberModelAccessTestUser(t, "member-model-access-unrestricted")
	restricted := createMemberModelAccessTestUser(t, "member-model-access-restricted")
	createMemberModelAccessTestMembership(t, 1, restricted.Id, `["allowed-model"]`)
	normalized := createMemberModelAccessTestUser(t, "member-model-access-normalized")
	createMemberModelAccessTestMembership(t, 1, normalized.Id, `["gpt-4-gizmo-*"]`)
	// 两个组织互不相交的白名单 → 有效集为空集（拒绝全部），与 nil 严格区分。
	disjoint := createMemberModelAccessTestUser(t, "member-model-access-disjoint")
	createMemberModelAccessTestMembership(t, 1, disjoint.Id, `["allowed-model"]`)
	createMemberModelAccessTestMembership(t, 2, disjoint.Id, `["other-model"]`)

	tests := []struct {
		name           string
		userId         int
		actorUserId    int
		model          string
		wantMessageKey string // 空串 = 期望放行
	}{
		{
			name:   "a user without a membership stays unrestricted",
			userId: unrestricted.Id, actorUserId: unrestricted.Id, model: "unrestricted-model",
		},
		{
			name:   "a whitelisted model passes",
			userId: restricted.Id, actorUserId: restricted.Id, model: "allowed-model",
		},
		{
			name:   "a model outside the whitelist is rejected",
			userId: restricted.Id, actorUserId: restricted.Id, model: "denied-model",
			wantMessageKey: appI18n.MsgDistributorMemberModelForbidden,
		},
		{
			name:   "a gizmo entry admits the concrete names it normalizes to",
			userId: normalized.Id, actorUserId: normalized.Id, model: "gpt-4-gizmo-abc",
		},
		{
			name:   "an empty intersection rejects every model",
			userId: disjoint.Id, actorUserId: disjoint.Id, model: "allowed-model",
			wantMessageKey: appI18n.MsgDistributorMemberModelForbidden,
		},
		{
			name:   "the actor user id falls back to the token user id",
			userId: restricted.Id, actorUserId: 0, model: "denied-model",
			wantMessageKey: appI18n.MsgDistributorMemberModelForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder, c := memberModelAccessContext(t, tc.userId, tc.actorUserId, tc.model)

			Distribute()(c)

			if tc.wantMessageKey == "" {
				require.False(t, c.IsAborted(), recorder.Body.String())
				assert.Equal(t, channel.Id, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
				return
			}
			require.True(t, c.IsAborted())
			assert.Equal(t, http.StatusForbidden, recorder.Code)
			message := renderedMemberModelAccessMessage(tc.wantMessageKey, map[string]any{"Model": tc.model})
			require.Contains(t, message, tc.model, "the rendered message must name the denied model")
			assert.Contains(t, recorder.Body.String(), message)
		})
	}
}

// TestDistributeMemberModelAccessIntersectsTokenModelLimit 覆盖成员白名单与令牌模型
// 限制的交集矩阵：任一侧拒绝即拒绝，只有两侧都放行才放行；令牌既有的
// 「enabled 且空 map = 拒绝全部」语义不被白名单意外放宽。
func TestDistributeMemberModelAccessIntersectsTokenModelLimit(t *testing.T) {
	require.NoError(t, appI18n.Init())
	setupMemberModelAccessTestDB(t)
	channel := insertMemberModelAccessTestChannel(t, "allowed-model")

	restricted := createMemberModelAccessTestUser(t, "member-model-access-intersection")
	createMemberModelAccessTestMembership(t, 1, restricted.Id, `["allowed-model"]`)

	tests := []struct {
		name           string
		model          string
		tokenLimit     map[string]bool
		wantMessageKey string // 空串 = 期望放行
	}{
		{
			name:           "the member allow list passes but the token limit rejects",
			model:          "allowed-model",
			tokenLimit:     map[string]bool{"token-only-model": true},
			wantMessageKey: appI18n.MsgDistributorTokenModelForbidden,
		},
		{
			name:           "the token limit passes but the member allow list rejects",
			model:          "denied-model",
			tokenLimit:     map[string]bool{"denied-model": true},
			wantMessageKey: appI18n.MsgDistributorMemberModelForbidden,
		},
		{
			name:       "both sides allow the model",
			model:      "allowed-model",
			tokenLimit: map[string]bool{"allowed-model": true},
		},
		{
			name:           "a token limit that is enabled but empty still rejects",
			model:          "allowed-model",
			tokenLimit:     map[string]bool{},
			wantMessageKey: appI18n.MsgDistributorTokenModelForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder, c := memberModelAccessContext(t, restricted.Id, restricted.Id, tc.model)
			common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
			common.SetContextKey(c, constant.ContextKeyTokenModelLimit, tc.tokenLimit)

			Distribute()(c)

			if tc.wantMessageKey == "" {
				require.False(t, c.IsAborted(), recorder.Body.String())
				assert.Equal(t, channel.Id, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
				return
			}
			require.True(t, c.IsAborted())
			assert.Equal(t, http.StatusForbidden, recorder.Code)
			message := renderedMemberModelAccessMessage(tc.wantMessageKey, map[string]any{"Model": tc.model})
			require.Contains(t, message, tc.model, "the rendered message must name the denied model")
			assert.Contains(t, recorder.Body.String(), message)
		})
	}
}

// TestDistributeMemberModelAccessTaskStatusBackfill 覆盖任务状态端点（GET
// /v1/video/generations/:task_id）：受限成员轮询任务时模型名取自任务记录，
// 记录里的模型照样强判（白名单内 → 放行，白名单外 → 403 且文案点名该模型）；
// 无任务记录时空模型被成员检查跳过（空模型跳过语义见
// TestDistributeMemberModelAccessSkipsEmptyModelRequests）；不受限成员保持现状。
func TestDistributeMemberModelAccessTaskStatusBackfill(t *testing.T) {
	require.NoError(t, appI18n.Init())
	setupMemberModelAccessTestDB(t)

	restricted := createMemberModelAccessTestUser(t, "member-model-access-task-restricted")
	createMemberModelAccessTestMembership(t, 1, restricted.Id, `["allowed-model"]`)
	unrestricted := createMemberModelAccessTestUser(t, "member-model-access-task-unrestricted")
	insertMemberModelAccessTestTask(t, "task-allowed", restricted.Id, "allowed-model")
	insertMemberModelAccessTestTask(t, "task-denied", restricted.Id, "denied-model")

	tests := []struct {
		name           string
		userId         int
		actorUserId    int
		taskId         string
		wantMessageKey string // 空串 = 期望放行
		wantModel      string // 拒绝时消息里点名的模型
	}{
		{
			name:   "the origin model of an allowed task is backfilled and passes",
			userId: restricted.Id, actorUserId: restricted.Id, taskId: "task-allowed",
		},
		{
			name:   "a backfilled origin model outside the whitelist is still rejected",
			userId: restricted.Id, actorUserId: restricted.Id, taskId: "task-denied",
			wantMessageKey: appI18n.MsgDistributorMemberModelForbidden,
			wantModel:      "denied-model",
		},
		{
			name:   "an unknown task leaves the model empty and the member check skips it",
			userId: restricted.Id, actorUserId: restricted.Id, taskId: "task-missing",
		},
		{
			name:   "an unrestricted actor keeps the previous behavior",
			userId: unrestricted.Id, actorUserId: unrestricted.Id, taskId: "task-missing",
		},
		{
			// 有判别力：去掉 actor 回退后 allow=nil，既不回填也不判定，此用例会因
			// 「预期 403 实得放行」失败。
			name:   "the actor user id falls back to the user id for backfill and enforcement",
			userId: restricted.Id, actorUserId: 0, taskId: "task-denied",
			wantMessageKey: appI18n.MsgDistributorMemberModelForbidden,
			wantModel:      "denied-model",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder, c := memberModelAccessTaskStatusContext(t, tc.userId, tc.actorUserId, tc.taskId)

			Distribute()(c)

			if tc.wantMessageKey == "" {
				require.False(t, c.IsAborted(), recorder.Body.String())
				assert.Equal(t, http.StatusOK, recorder.Code)
				return
			}
			require.True(t, c.IsAborted())
			assert.Equal(t, http.StatusForbidden, recorder.Code)
			message := renderedMemberModelAccessMessage(tc.wantMessageKey, map[string]any{"Model": tc.wantModel})
			require.Contains(t, message, tc.wantModel, "the rendered message must name the denied model")
			assert.Contains(t, recorder.Body.String(), message)
		})
	}
}

// memberModelAccessMJFetchContext 构造 MJ 任务轮询上下文：GET /mj/task/:id/fetch。
// 该路径没有模型源（MJ 记录不存模型名），modelRequest.Model 保持为空。
func memberModelAccessMJFetchContext(t *testing.T, userId, actorUserId int) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/mj/task/mj-1/fetch", nil)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUserId, userId)
	if actorUserId > 0 {
		common.SetContextKey(c, constant.ContextKeyActorUserId, actorUserId)
	}
	return recorder, c
}

// TestDistributeMemberModelAccessSkipsEmptyModelRequests 覆盖空模型的跳过语义：
// MJ 任务轮询这类 fetch/状态请求没有模型源，成员检查不作判定（继续走到后续路径）；
// 请求一旦带模型（提交路径）仍照常强制——提交路径紧随其后的
// MsgDistributorModelNameRequired 400 兜底，跳过不构成放行捷径。
func TestDistributeMemberModelAccessSkipsEmptyModelRequests(t *testing.T) {
	require.NoError(t, appI18n.Init())
	setupMemberModelAccessTestDB(t)
	insertMemberModelAccessTestChannel(t, "allowed-model")

	restricted := createMemberModelAccessTestUser(t, "member-model-access-empty-model")
	createMemberModelAccessTestMembership(t, 1, restricted.Id, `["allowed-model"]`)

	t.Run("an MJ task fetch without a model is not blocked by the member check", func(t *testing.T) {
		recorder, c := memberModelAccessMJFetchContext(t, restricted.Id, restricted.Id)

		Distribute()(c)

		require.False(t, c.IsAborted(), recorder.Body.String())
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, relayconstant.RelayModeMidjourneyTaskFetch, c.GetInt("relay_mode"))
	})

	t.Run("a submit request that carries a model is still enforced", func(t *testing.T) {
		recorder, c := memberModelAccessContext(t, restricted.Id, restricted.Id, "denied-model")

		Distribute()(c)

		require.True(t, c.IsAborted())
		assert.Equal(t, http.StatusForbidden, recorder.Code)
		assert.Contains(t, recorder.Body.String(), renderedMemberModelAccessMessage(appI18n.MsgDistributorMemberModelForbidden, map[string]any{"Model": "denied-model"}))
	})
}
