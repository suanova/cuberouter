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
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const (
	agentOrgGroupModel = "zz-dsh-agent-org-model"
	agentOrgGroup      = "zz-dsh-org-group"
)

// setDshOrganizationOptionsForControllerTest 写入白名单与绑定映射，并在用例结束时
// 还原原来的 OptionMap 值（同包其他用例可能已经配置过这两项）。
func setDshOrganizationOptionsForControllerTest(t *testing.T, organizationId, channelId int) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	previousAllowed, hadAllowed := common.OptionMap[setting.DshAllowedOrganizationIdsKey]
	previousChannels, hadChannels := common.OptionMap[setting.DshOrganizationChannelsKey]
	common.OptionMap[setting.DshAllowedOrganizationIdsKey] = fmt.Sprintf("[%d]", organizationId)
	common.OptionMap[setting.DshOrganizationChannelsKey] = fmt.Sprintf(`{"%d":%d}`, organizationId, channelId)
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if hadAllowed {
			common.OptionMap[setting.DshAllowedOrganizationIdsKey] = previousAllowed
		} else {
			delete(common.OptionMap, setting.DshAllowedOrganizationIdsKey)
		}
		if hadChannels {
			common.OptionMap[setting.DshOrganizationChannelsKey] = previousChannels
		} else {
			delete(common.OptionMap, setting.DshOrganizationChannelsKey)
		}
	})
}

// seedDshAgentOrganization 建一条 type 62 + task_plugin_key=dsh 的渠道，把它放进
// 一个独立分组，并让该分组可见 agentOrgGroupModel；成员的组织就挂在这个分组上。
func seedDshAgentOrganization(t *testing.T, userId int, modelAccess string) model.Organization {
	t.Helper()
	channelSettings := `{"task_plugin_key":"dsh"}`
	baseURL := "http://dsh-org-1:3080"
	channel := model.Channel{
		Type:    constant.ChannelTypeTaskPlugin,
		Key:     "dsh-agent-model-test",
		Status:  common.ChannelStatusEnabled,
		Name:    "dsh-agent-model-test",
		BaseURL: &baseURL,
		Group:   agentOrgGroup,
		Models:  agentOrgGroupModel,
		Setting: &channelSettings,
	}
	require.NoError(t, model.DB.Create(&channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group: agentOrgGroup, Model: agentOrgGroupModel, ChannelId: channel.Id, Enabled: true,
	}).Error)

	slug := fmt.Sprintf("dsh-agent-models-%d", userId)
	organization := model.Organization{
		Name:      slug,
		Slug:      slug,
		Group:     agentOrgGroup,
		Status:    model.OrganizationStatusActive,
		CreatedBy: userId,
	}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organization.Id,
		UserId:         userId,
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusActive,
		ModelAccess:    modelAccess,
	}).Error)

	setDshOrganizationOptionsForControllerTest(t, organization.Id, channel.Id)
	return organization
}

func performGetUserModelsAgentRequest(t *testing.T, userId int, query string) []string {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	target := "/api/user/models"
	if query != "" {
		target += "?" + query
	}
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	c.Set("id", userId)
	GetUserModels(c)
	return decodeUserModelsResponse(t, recorder)
}

// TestGetUserModelsAgentIncludesOrganizationGroups 保护 DSH 智能体模型下拉：
// 渠道挂在组织分组下，成员的个人可用分组不含它；只有带 endpoint=agent&plugin=dsh
// 时补上组织分组，模型才会出现。成员白名单仍然收窄结果。
func TestGetUserModelsAgentIncludesOrganizationGroups(t *testing.T) {
	db := setupUserModelAccessTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	previousMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previousMemoryCache })

	t.Run("the agent query surfaces the organization group model", func(t *testing.T) {
		user := createMemberModelAccessTestUser(t, db, "agent-org-group-user", common.RoleCommonUser)
		seedDshAgentOrganization(t, user.Id, "")

		models := performGetUserModelsAgentRequest(t, user.Id, "endpoint=agent&plugin=dsh")
		require.Contains(t, models, agentOrgGroupModel)
	})

	// 没有 DSH 参数时行为不变：个人分组列表里没有组织分组，模型不可见。
	t.Run("other callers do not see the organization group", func(t *testing.T) {
		user := createMemberModelAccessTestUser(t, db, "agent-org-group-plain-user", common.RoleCommonUser)
		seedDshAgentOrganization(t, user.Id, "")

		models := performGetUserModelsAgentRequest(t, user.Id, "")
		require.NotContains(t, models, agentOrgGroupModel)
	})

	t.Run("the member allow list still narrows the result", func(t *testing.T) {
		user := createMemberModelAccessTestUser(t, db, "agent-org-group-restricted-user", common.RoleCommonUser)
		seedDshAgentOrganization(t, user.Id, `["zz-some-other-model"]`)

		models := performGetUserModelsAgentRequest(t, user.Id, "endpoint=agent&plugin=dsh")
		require.Empty(t, models, "组织分组只补可见性，不能绕过成员白名单")
	})
}
