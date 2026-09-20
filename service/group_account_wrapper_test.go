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

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 引入组织后，分组计算从"用户分组"泛化成了"账户分组"，GetUser* 系列退化成
// GetAccount* 的薄包装，个人账户仍然走旧名字。这里钉住两侧必须完全等价：
// 只要有人只改了一边，个人用户的分组继承就会被悄悄改坏。
func TestPersonalGroupHelpersMatchAccountHelpers(t *testing.T) {
	configureRequestAutoGroupsTest(t)

	for _, accountGroup := range []string{"", "default", "vip", "org-only"} {
		assert.Equal(t, GetAccountUsableGroups(accountGroup), GetUserUsableGroups(accountGroup), "usable groups for %q", accountGroup)
		assert.Equal(t, GetAccountAutoGroup(accountGroup), GetUserAutoGroup(accountGroup), "auto groups for %q", accountGroup)

		for _, candidate := range []string{"", "auto", "default", "vip", "svip", "revoked"} {
			assert.Equal(t, IsAccountSelectableGroup(accountGroup, candidate), IsUserSelectableGroup(accountGroup, candidate), "selectable %q in %q", candidate, accountGroup)
			assert.Equal(t, GetAccountGroupRatio(accountGroup, candidate), GetUserGroupRatio(accountGroup, candidate), "ratio for %q in %q", candidate, accountGroup)
		}

		for _, snapshot := range [][]string{{"vip", "default", "svip"}, {"revoked", "vip", "vip"}, nil} {
			assert.Equal(t, FilterAccountTokenAutoGroups(accountGroup, snapshot), FilterUserTokenAutoGroups(accountGroup, snapshot), "filter %v in %q", snapshot, accountGroup)
		}
	}
}

// 个人用户自带的分组即使没写进 UserUsableGroups，也必须可用——这是个人账户
// 分组继承的底线，不能被账户抽象改掉。
func TestPersonalUserOwnGroupStaysUsable(t *testing.T) {
	configureRequestAutoGroupsTest(t)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))

	usable := GetUserUsableGroups("vip")

	assert.Contains(t, usable, "vip")
	assert.True(t, GroupInUserUsableGroups("vip", "vip"))
	assert.True(t, IsUserSelectableGroup("vip", "vip"))
}

// 没配倍率的分组不可选，也不能因为泛化成了"账户分组"就放行。
func TestPersonalTokenAutoGroupsRequireConfiguredRatio(t *testing.T) {
	configureRequestAutoGroupsTest(t)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))

	assert.False(t, IsUserSelectableGroup("default", "vip"))
	assert.Equal(t, []string{"default"}, FilterUserTokenAutoGroups("default", []string{"vip", "default"}))
}
