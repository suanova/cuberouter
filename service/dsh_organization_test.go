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
package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createDshTestChannel(t *testing.T, name string, status int, channelType int, pluginKey string) model.Channel {
	t.Helper()
	settings := `{"task_plugin_key":"` + pluginKey + `"}`
	baseURL := "http://" + name + ":3080"
	channel := model.Channel{
		Type:    channelType,
		Key:     "dsh-test-key",
		Status:  status,
		Name:    name,
		BaseURL: &baseURL,
		Group:   "default",
		Models:  "dsh-model",
		Setting: &settings,
	}
	require.NoError(t, model.DB.Create(&channel).Error)
	return channel
}

func addDshTestMember(t *testing.T, organizationId, userId int, status string) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: organizationId,
		UserId:         userId,
		Role:           model.OrganizationRoleMember,
		Status:         status,
	}).Error)
}

func TestResolveDshOrganizationForUser(t *testing.T) {
	setupServiceTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}))
	originalMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCache })

	owner := createServiceTestUser(t, "dsh-resolve-owner", common.RoleCommonUser)
	member := createServiceTestUser(t, "dsh-resolve-member", common.RoleCommonUser)
	outsider := createServiceTestUser(t, "dsh-resolve-outsider", common.RoleCommonUser)

	firstOrganization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "DSH First"})
	require.NoError(t, err)
	secondOrganization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "DSH Second"})
	require.NoError(t, err)
	unboundOrganization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "DSH Unbound"})
	require.NoError(t, err)
	require.NotEqual(t, firstOrganization.Id, secondOrganization.Id)

	addDshTestMember(t, firstOrganization.Id, member.Id, model.OrganizationMemberStatusActive)
	addDshTestMember(t, secondOrganization.Id, member.Id, model.OrganizationMemberStatusActive)
	addDshTestMember(t, unboundOrganization.Id, member.Id, model.OrganizationMemberStatusActive)

	firstChannel := createDshTestChannel(t, "dsh-org-first", common.ChannelStatusEnabled, constant.ChannelTypeTaskPlugin, "dsh")
	secondChannel := createDshTestChannel(t, "dsh-org-second", common.ChannelStatusEnabled, constant.ChannelTypeTaskPlugin, "dsh")
	disabledChannel := createDshTestChannel(t, "dsh-org-disabled", common.ChannelStatusManuallyDisabled, constant.ChannelTypeTaskPlugin, "dsh")
	wrongPluginChannel := createDshTestChannel(t, "dsh-org-plugin", common.ChannelStatusEnabled, constant.ChannelTypeTaskPlugin, "other")
	wrongTypeChannel := createDshTestChannel(t, "dsh-org-type", common.ChannelStatusEnabled, constant.ChannelTypeOpenAI, "dsh")

	t.Run("no enabled organization", func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, nil)
		setDshOrganizationBindingsForTest(t, nil)
		_, _, err := ResolveDshOrganizationForUser(member.Id, 0)
		require.ErrorIs(t, err, ErrDshOrganizationMissing)
	})

	t.Run("allowlisted but unbound organizations are not selectable", func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, []int{unboundOrganization.Id})
		setDshOrganizationBindingsForTest(t, nil)
		_, _, err := ResolveDshOrganizationForUser(member.Id, 0)
		require.ErrorIs(t, err, ErrDshOrganizationMissing)
	})

	t.Run("a user outside every enabled organization is rejected", func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, []int{firstOrganization.Id})
		setDshOrganizationBindingsForTest(t, map[int]int{firstOrganization.Id: firstChannel.Id})
		_, _, err := ResolveDshOrganizationForUser(outsider.Id, 0)
		require.ErrorIs(t, err, ErrDshOrganizationMissing)
	})

	t.Run("without a preference the lowest organization id wins", func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, []int{secondOrganization.Id, firstOrganization.Id})
		setDshOrganizationBindingsForTest(t, map[int]int{
			firstOrganization.Id:  firstChannel.Id,
			secondOrganization.Id: secondChannel.Id,
		})
		organization, channelId, err := ResolveDshOrganizationForUser(member.Id, 0)
		require.NoError(t, err)
		require.Equal(t, firstOrganization.Id, organization.Id)
		require.Equal(t, firstChannel.Id, channelId)
	})

	t.Run("an enabled preferred organization is honored", func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, []int{secondOrganization.Id, firstOrganization.Id})
		setDshOrganizationBindingsForTest(t, map[int]int{
			firstOrganization.Id:  firstChannel.Id,
			secondOrganization.Id: secondChannel.Id,
		})
		organization, channelId, err := ResolveDshOrganizationForUser(member.Id, secondOrganization.Id)
		require.NoError(t, err)
		require.Equal(t, secondOrganization.Id, organization.Id)
		require.Equal(t, secondChannel.Id, channelId)
	})

	// 用户把页面切到了别的组织上下文：该组织尚未开通 DSH 时回退到自动选择，
	// 而不是把页面变成 403 死页。
	t.Run("a member preference that is not enabled falls back", func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, []int{firstOrganization.Id})
		setDshOrganizationBindingsForTest(t, map[int]int{firstOrganization.Id: firstChannel.Id})
		organization, _, err := ResolveDshOrganizationForUser(member.Id, unboundOrganization.Id)
		require.NoError(t, err)
		require.Equal(t, firstOrganization.Id, organization.Id)
	})

	t.Run("a preference for an organization the user has left is denied", func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, []int{firstOrganization.Id})
		setDshOrganizationBindingsForTest(t, map[int]int{firstOrganization.Id: firstChannel.Id})
		_, _, err := ResolveDshOrganizationForUser(outsider.Id, firstOrganization.Id)
		require.ErrorIs(t, err, ErrDshOrganizationDenied)
	})

	for _, testCase := range []struct {
		name      string
		channelId int
	}{
		{name: "stopped channel", channelId: disabledChannel.Id},
		{name: "wrong plugin key", channelId: wrongPluginChannel.Id},
		{name: "wrong channel type", channelId: wrongTypeChannel.Id},
		{name: "missing channel", channelId: 9999},
	} {
		t.Run(testCase.name+" makes the organization unusable", func(t *testing.T) {
			setDshAllowedOrganizationsForTest(t, []int{firstOrganization.Id})
			setDshOrganizationBindingsForTest(t, map[int]int{firstOrganization.Id: testCase.channelId})
			_, _, err := ResolveDshOrganizationForUser(member.Id, 0)
			require.ErrorIs(t, err, ErrDshChannelUnavailable)
		})
	}
}

func TestResolveDshOrganizationForToken(t *testing.T) {
	setupServiceTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}))
	originalMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCache })

	owner := createServiceTestUser(t, "dsh-token-owner", common.RoleCommonUser)
	member := createServiceTestUser(t, "dsh-token-member", common.RoleCommonUser)
	exitedMember := createServiceTestUser(t, "dsh-token-exited", common.RoleCommonUser)

	organization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "DSH Token"})
	require.NoError(t, err)
	addDshTestMember(t, organization.Id, member.Id, model.OrganizationMemberStatusActive)
	addDshTestMember(t, organization.Id, exitedMember.Id, model.OrganizationMemberStatusExited)
	channel := createDshTestChannel(t, "dsh-token-org", common.ChannelStatusEnabled, constant.ChannelTypeTaskPlugin, "dsh")

	resetBindings := func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, []int{organization.Id})
		setDshOrganizationBindingsForTest(t, map[int]int{organization.Id: channel.Id})
	}

	// 个人令牌没有组织作用域：严格拒绝，管理员/root 也不例外。
	t.Run("personal tokens are rejected", func(t *testing.T) {
		resetBindings(t)
		_, _, err := ResolveDshOrganizationForToken(member.Id, 0)
		require.ErrorIs(t, err, ErrDshOrganizationRequired)
	})

	t.Run("a non member is rejected", func(t *testing.T) {
		resetBindings(t)
		_, _, err := ResolveDshOrganizationForToken(member.Id+10000, organization.Id)
		require.ErrorIs(t, err, ErrDshOrganizationDenied)
	})

	t.Run("an exited member is rejected", func(t *testing.T) {
		resetBindings(t)
		_, _, err := ResolveDshOrganizationForToken(exitedMember.Id, organization.Id)
		require.ErrorIs(t, err, ErrDshOrganizationDenied)
	})

	t.Run("an organization that is not enabled is rejected", func(t *testing.T) {
		setDshAllowedOrganizationsForTest(t, nil)
		setDshOrganizationBindingsForTest(t, nil)
		_, _, err := ResolveDshOrganizationForToken(member.Id, organization.Id)
		require.ErrorIs(t, err, ErrDshOrganizationDenied)
	})

	t.Run("an organization token resolves the bound channel", func(t *testing.T) {
		resetBindings(t)
		resolved, channelId, err := ResolveDshOrganizationForToken(member.Id, organization.Id)
		require.NoError(t, err)
		require.Equal(t, organization.Id, resolved.Id)
		require.Equal(t, channel.Id, channelId)
	})
}

func TestDshOrganizationGroupsForUser(t *testing.T) {
	setupServiceTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}))
	originalMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCache })

	owner := createServiceTestUser(t, "dsh-groups-owner", common.RoleCommonUser)
	member := createServiceTestUser(t, "dsh-groups-member", common.RoleCommonUser)

	firstOrganization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "DSH Groups One"})
	require.NoError(t, err)
	secondOrganization, err := CreateOrganization(owner.Id, CreateOrganizationRequest{Name: "DSH Groups Two"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).
		Where("id = ?", secondOrganization.Id).
		Update("group", "vip").Error)
	addDshTestMember(t, firstOrganization.Id, member.Id, model.OrganizationMemberStatusActive)
	addDshTestMember(t, secondOrganization.Id, member.Id, model.OrganizationMemberStatusActive)

	firstChannel := createDshTestChannel(t, "dsh-groups-one", common.ChannelStatusEnabled, constant.ChannelTypeTaskPlugin, "dsh")
	secondChannel := createDshTestChannel(t, "dsh-groups-two", common.ChannelStatusEnabled, constant.ChannelTypeTaskPlugin, "dsh")
	disabledChannel := createDshTestChannel(t, "dsh-groups-off", common.ChannelStatusManuallyDisabled, constant.ChannelTypeTaskPlugin, "dsh")

	setDshAllowedOrganizationsForTest(t, []int{firstOrganization.Id, secondOrganization.Id})

	t.Run("returns one normalized group per usable organization", func(t *testing.T) {
		setDshOrganizationBindingsForTest(t, map[int]int{
			firstOrganization.Id:  firstChannel.Id,
			secondOrganization.Id: secondChannel.Id,
		})
		groups := DshOrganizationGroupsForUser(member.Id)
		require.ElementsMatch(t, []string{"default", "vip"}, groups)
	})

	// 渠道不可用的组织不该出现在模型下拉的来源里：那里的模型必然提交失败。
	t.Run("skips organizations whose channel is unusable", func(t *testing.T) {
		setDshOrganizationBindingsForTest(t, map[int]int{
			firstOrganization.Id:  disabledChannel.Id,
			secondOrganization.Id: secondChannel.Id,
		})
		groups := DshOrganizationGroupsForUser(member.Id)
		require.Equal(t, []string{"vip"}, groups)
	})

	t.Run("deduplicates organizations that share a group", func(t *testing.T) {
		require.NoError(t, model.DB.Model(&model.Organization{}).
			Where("id = ?", secondOrganization.Id).
			Update("group", "default").Error)
		setDshOrganizationBindingsForTest(t, map[int]int{
			firstOrganization.Id:  firstChannel.Id,
			secondOrganization.Id: secondChannel.Id,
		})
		groups := DshOrganizationGroupsForUser(member.Id)
		assert.Equal(t, []string{"default"}, groups)
	})

	t.Run("returns nothing for a user without organizations", func(t *testing.T) {
		require.Empty(t, DshOrganizationGroupsForUser(member.Id+10000))
	})
}
