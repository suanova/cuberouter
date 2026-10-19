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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaykittypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPlaygroundControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.Task{}))
	model.DB = database
	t.Cleanup(func() { model.DB = previousDB })
	return database
}

func playgroundControllerTestContext(method, path string) *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestPreparePlaygroundContextSkipsTempTokenForOrganizationScope(t *testing.T) {
	c := playgroundControllerTestContext(http.MethodPost, "/pg/chat/completions")
	c.Set("id", 42)
	common.SetContextKey(c, constant.ContextKeyBillingAccountType, model.AccountContextTypeOrganization)
	c.Set("token_name", "playground-org-default")

	relayInfo, newAPIError := preparePlaygroundContext(c, relaykittypes.RelayFormatOpenAI)

	require.Nil(t, newAPIError)
	require.Nil(t, relayInfo)
	// 个人临时 token 未覆盖组织作用域
	assert.Equal(t, "playground-org-default", c.GetString("token_name"))
	assert.Equal(t, model.AccountContextTypeOrganization, common.GetContextKeyString(c, constant.ContextKeyBillingAccountType))
}

func TestPreparePlaygroundContextRejectsAccessToken(t *testing.T) {
	c := playgroundControllerTestContext(http.MethodPost, "/pg/chat/completions")
	c.Set("id", 42)
	c.Set("use_access_token", true)

	_, newAPIError := preparePlaygroundContext(c, relaykittypes.RelayFormatOpenAI)

	require.NotNil(t, newAPIError)
	// preparePlaygroundContext 返回 relaykit types 的 *NewAPIError，错误码与之同类型比较
	assert.Equal(t, relaykittypes.ErrorCodeAccessDenied, newAPIError.GetErrorCode())
}

func TestPreparePlaygroundVideoFetchScopeRestoresOwnedTaskScope(t *testing.T) {
	setupPlaygroundControllerTestDB(t)

	tests := []struct {
		name               string
		taskUserId         int
		taskScopeType      string
		taskScopeId        int
		taskOrganizationId int
		wantBillingType    string
		wantBillingId      int
		wantOrganizationId int
	}{
		{
			name:            "organization fallback task",
			taskUserId:      7,
			taskScopeType:   model.AccountContextTypeOrganization,
			taskScopeId:     33,
			wantBillingType: model.AccountContextTypeOrganization,
			wantBillingId:   33,
			// 任务行未写 organization_id 时由 NormalizeTaskBillingScope 回落 ScopeId；
			// AsyncTaskScope.Apply 的组织分支按 organization_id 过滤，必须一并还原
			wantOrganizationId: 33,
		},
		{
			name:               "personal task",
			taskUserId:         7,
			taskScopeType:      model.AccountContextTypePersonal,
			taskScopeId:        7,
			wantBillingType:    model.AccountContextTypePersonal,
			wantBillingId:      7,
			wantOrganizationId: 0,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			task := &model.Task{
				TaskID:             "pg-fetch-" + strings.ReplaceAll(test.name, " ", "-"),
				Platform:           "test",
				UserId:             test.taskUserId,
				Status:             model.TaskStatusSubmitted,
				ScopeType:          test.taskScopeType,
				ScopeId:            test.taskScopeId,
				BillingAccountType: test.taskScopeType,
				BillingAccountId:   test.taskScopeId,
				OrganizationId:     test.taskOrganizationId,
				ActorUserId:        test.taskUserId,
				CreatorUserId:      test.taskUserId,
				ResponsibleUserId:  test.taskUserId,
			}
			require.NoError(t, model.DB.Create(task).Error)

			c := playgroundControllerTestContext(http.MethodGet, "/pg/video/generations/"+task.TaskID)
			c.Params = gin.Params{{Key: "task_id", Value: task.TaskID}}
			c.Set("id", test.taskUserId)

			preparePlaygroundVideoFetchScope(c)

			assert.Equal(t, test.wantBillingType, common.GetContextKeyString(c, constant.ContextKeyBillingAccountType))
			assert.Equal(t, test.wantBillingId, common.GetContextKeyInt(c, constant.ContextKeyBillingAccountId))
			assert.Equal(t, test.taskScopeType, common.GetContextKeyString(c, constant.ContextKeyScopeType))
			assert.Equal(t, test.taskScopeId, common.GetContextKeyInt(c, constant.ContextKeyScopeId))
			assert.Equal(t, test.wantOrganizationId, common.GetContextKeyInt(c, constant.ContextKeyOrganizationId))
			assert.Equal(t, 7, common.GetContextKeyInt(c, constant.ContextKeyActorUserId))
		})
	}
}

func TestPreparePlaygroundVideoFetchScopeIgnoresOtherUsersTask(t *testing.T) {
	setupPlaygroundControllerTestDB(t)
	task := &model.Task{
		TaskID:             "pg-fetch-foreign",
		Platform:           "test",
		UserId:             8,
		Status:             model.TaskStatusSubmitted,
		ScopeType:          model.AccountContextTypeOrganization,
		ScopeId:            33,
		BillingAccountType: model.AccountContextTypeOrganization,
		BillingAccountId:   33,
		OrganizationId:     33,
	}
	require.NoError(t, model.DB.Create(task).Error)

	c := playgroundControllerTestContext(http.MethodGet, "/pg/video/generations/"+task.TaskID)
	c.Params = gin.Params{{Key: "task_id", Value: task.TaskID}}
	c.Set("id", 7)

	preparePlaygroundVideoFetchScope(c)

	assert.Empty(t, common.GetContextKeyString(c, constant.ContextKeyBillingAccountType))
	assert.Equal(t, 0, common.GetContextKeyInt(c, constant.ContextKeyBillingAccountId))
}
