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
package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNormalizeTokenScopeDefaultsLegacyTokenToPersonal(t *testing.T) {
	token := &Token{UserId: 42}

	NormalizeTokenScope(token)

	require.Equal(t, TokenScopePersonal, token.ScopeType)
	require.Equal(t, 42, token.ScopeId)
	require.Equal(t, 42, token.CreatorUserId)
	require.Equal(t, 42, token.ResponsibleUserId)
	require.Equal(t, TokenVisibilityPrivate, token.Visibility)
}

func TestNormalizeTokenScopeKeepsOrganizationScopeValues(t *testing.T) {
	token := &Token{UserId: 42, ScopeType: TokenScopeOrganization, OrganizationId: 7, CreatorUserId: 2, ResponsibleUserId: 3}

	NormalizeTokenScope(token)

	require.Equal(t, TokenScopeOrganization, token.ScopeType)
	require.Equal(t, 7, token.ScopeId)
	require.Equal(t, 2, token.CreatorUserId)
	require.Equal(t, 3, token.ResponsibleUserId)
	require.Equal(t, TokenVisibilityPrivate, token.Visibility)
}

// TestPersonalTokenQueriesFilterOutOrganizationTokens 锁定个人令牌视图的边界。
//
// 组织密钥的 user_id 是责任人，所以「按 user_id 查」会把它当成个人密钥列出来。
// 用户退出组织后仍然看得到（甚至能删掉）别人的 key，这是必须堵住的口子，
// 因此列表、计数、搜索三条读路径和删除路径都要按个人作用域过滤。
// 同时确认老数据（scope_type 为空）仍算个人，否则升级后历史密钥会集体消失。
func TestPersonalTokenQueriesFilterOutOrganizationTokens(t *testing.T) {
	setupModelTestDB(t)
	user := User{Username: "token-user", Password: "password", Status: common.UserStatusEnabled, AffCode: "token-user"}
	require.NoError(t, DB.Create(&user).Error)
	legacy := Token{UserId: user.Id, Key: "111111111111111111111111111111111111111111111111", Name: "legacy", Status: common.TokenStatusEnabled}
	personal := Token{UserId: user.Id, Key: "222222222222222222222222222222222222222222222222", Name: "personal", Status: common.TokenStatusEnabled, ScopeType: TokenScopePersonal}
	organization := Token{UserId: user.Id, Key: "333333333333333333333333333333333333333333333333", Name: "organization", Status: common.TokenStatusEnabled, ScopeType: TokenScopeOrganization, OrganizationId: 99, ScopeId: 99}
	require.NoError(t, DB.Create(&legacy).Error)
	require.NoError(t, DB.Create(&personal).Error)
	require.NoError(t, DB.Create(&organization).Error)

	tokens, err := GetAllUserTokens(user.Id, 0, 20)
	require.NoError(t, err)
	require.Len(t, tokens, 2)
	for _, token := range tokens {
		require.Equal(t, TokenScopePersonal, token.ScopeType)
		require.NotEqual(t, "organization", token.Name)
	}

	total, err := CountUserTokens(user.Id)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)

	found, total, err := SearchUserTokens(user.Id, "personal", "", 0, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, found, 1)
	require.Equal(t, "personal", found[0].Name)
}

// TestPersonalTokenDeletionCannotTouchOrganizationTokens 覆盖写路径。
//
// 只读侧过滤还不够：GetTokenByIds 和 DeleteTokenById 都只按 (id, user_id) 定位，
// 组织密钥恰好同时满足这两个条件，于是个人接口可以直接读到甚至删除组织密钥。
func TestPersonalTokenDeletionCannotTouchOrganizationTokens(t *testing.T) {
	setupModelTestDB(t)
	user := User{Username: "token-delete-user", Password: "password", Status: common.UserStatusEnabled, AffCode: "token-delete-user"}
	require.NoError(t, DB.Create(&user).Error)
	organization := Token{UserId: user.Id, Key: "444444444444444444444444444444444444444444444444", Name: "organization", Status: common.TokenStatusEnabled, ScopeType: TokenScopeOrganization, OrganizationId: 99, ScopeId: 99}
	require.NoError(t, DB.Create(&organization).Error)

	_, err := GetTokenByIds(organization.Id, user.Id)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	require.Error(t, DeleteTokenById(organization.Id, user.Id))

	var stored Token
	require.NoError(t, DB.First(&stored, organization.Id).Error)
	require.False(t, stored.DeletedAt.Valid)

	deleted, err := BatchDeleteTokens([]int{organization.Id}, user.Id)
	require.NoError(t, err)
	require.Zero(t, deleted)
	require.NoError(t, DB.First(&stored, organization.Id).Error)
	require.False(t, stored.DeletedAt.Valid)
}
