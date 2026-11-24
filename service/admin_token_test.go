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
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedAdminTokenService(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.Create(&[]model.User{
		{Id: 201, Username: "root", DisplayName: "Root", Role: common.RoleRootUser, Status: common.UserStatusEnabled, AffCode: "root"},
		{Id: 202, Username: "admin", DisplayName: "Admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, AffCode: "admin"},
		{Id: 203, Username: "ops", DisplayName: "Ops", Role: common.RoleOpsUser, Status: common.UserStatusEnabled, AffCode: "ops"},
		{Id: 204, Username: "alice", DisplayName: "Alice Chen", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "alice"},
		{Id: 205, Username: "bob", DisplayName: "Bob", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "bob"},
	}).Error)
	require.NoError(t, model.DB.Create(&[]model.Token{
		{Id: 301, UserId: 201, Name: "root-key", Key: "root-key", ScopeType: model.TokenScopePersonal, Status: common.TokenStatusEnabled},
		{Id: 302, UserId: 202, Name: "admin-key", Key: "admin-key", ScopeType: model.TokenScopePersonal, Status: common.TokenStatusEnabled},
		{Id: 303, UserId: 203, Name: "ops-key", Key: "ops-key", ScopeType: model.TokenScopePersonal, Status: common.TokenStatusEnabled},
		{Id: 304, UserId: 204, Name: "alice-key", Key: "alice-secret-key", ScopeType: model.TokenScopePersonal, Status: common.TokenStatusEnabled},
		{Id: 305, UserId: 205, Name: "bob-key", Key: "bob-key", ScopeType: model.TokenScopePersonal, Status: common.TokenStatusEnabled},
		{Id: 306, UserId: 204, Name: "alice-expired", Key: "alice-expired", ScopeType: model.TokenScopePersonal, Status: common.TokenStatusExpired},
		{Id: 307, UserId: 204, Name: "org-key", Key: "org-key", ScopeType: model.TokenScopeOrganization, OrganizationId: 9, Status: common.TokenStatusEnabled},
	}).Error)
}

func adminTokenIds(tokens []*model.Token) []int {
	ids := make([]int, 0, len(tokens))
	for _, token := range tokens {
		ids = append(ids, token.Id)
	}
	return ids
}

func findAdminTokenById(tokens []*model.Token, id int) *model.Token {
	for _, token := range tokens {
		if token.Id == id {
			return token
		}
	}
	return nil
}

func TestListAdminPersonalTokensFiltersByRoleAndHydratesOwner(t *testing.T) {
	setupServiceTestDB(t)
	seedAdminTokenService(t)

	tokens, total, err := ListAdminPersonalTokens(
		AdminTokenOperator{UserId: 202, Role: common.RoleAdminUser},
		AdminPersonalTokenListRequest{Limit: 20},
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Equal(t, []int{306, 305, 304, 303, 302}, adminTokenIds(tokens))

	aliceKey := findAdminTokenById(tokens, 304)
	require.NotNil(t, aliceKey)
	assert.Equal(t, "alice", aliceKey.OwnerUsername)
	assert.Equal(t, "Alice Chen", aliceKey.OwnerDisplayName)
	assert.NotEqual(t, "alice-secret-key", aliceKey.Key)

	rootTokens, rootTotal, err := ListAdminPersonalTokens(
		AdminTokenOperator{UserId: 201, Role: common.RoleRootUser},
		AdminPersonalTokenListRequest{Limit: 20},
	)
	require.NoError(t, err)
	require.Equal(t, int64(6), rootTotal)
	require.Equal(t, []int{306, 305, 304, 303, 302, 301}, adminTokenIds(rootTokens))
}

func TestListAdminPersonalTokensFilters(t *testing.T) {
	setupServiceTestDB(t)
	seedAdminTokenService(t)

	adminOp := AdminTokenOperator{UserId: 202, Role: common.RoleAdminUser}

	tokens, total, err := ListAdminPersonalTokens(adminOp, AdminPersonalTokenListRequest{Limit: 20, Username: "chen"})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Equal(t, []int{306, 304}, adminTokenIds(tokens))

	tokens, total, err = ListAdminPersonalTokens(adminOp, AdminPersonalTokenListRequest{Limit: 20, Username: "bob"})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, []int{305}, adminTokenIds(tokens))

	tokens, total, err = ListAdminPersonalTokens(adminOp, AdminPersonalTokenListRequest{Limit: 20, Keyword: "ali"})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Equal(t, []int{306, 304}, adminTokenIds(tokens))

	tokens, total, err = ListAdminPersonalTokens(adminOp, AdminPersonalTokenListRequest{Limit: 20, Token: "sk-alice"})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Equal(t, []int{306, 304}, adminTokenIds(tokens))

	tokens, total, err = ListAdminPersonalTokens(adminOp, AdminPersonalTokenListRequest{Limit: 20, Status: common.TokenStatusExpired})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, []int{306}, adminTokenIds(tokens))

	tokens, total, err = ListAdminPersonalTokens(adminOp, AdminPersonalTokenListRequest{Limit: 2, Offset: 2})
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Equal(t, []int{304, 303}, adminTokenIds(tokens))
}

func TestGetAdminManagedPersonalTokenEnforcesRoleHierarchyAndScope(t *testing.T) {
	setupServiceTestDB(t)
	seedAdminTokenService(t)

	adminOp := AdminTokenOperator{UserId: 202, Role: common.RoleAdminUser}

	_, owner, err := GetAdminManagedPersonalToken(adminOp, 304)
	require.NoError(t, err)
	require.Equal(t, "alice", owner.Username)

	_, _, err = GetAdminManagedPersonalToken(adminOp, 303)
	require.NoError(t, err)

	_, _, err = GetAdminManagedPersonalToken(adminOp, 302)
	require.NoError(t, err)

	_, _, err = GetAdminManagedPersonalToken(adminOp, 301)
	require.ErrorIs(t, err, ErrAdminTokenForbidden)

	_, _, err = GetAdminManagedPersonalToken(adminOp, 307)
	require.ErrorIs(t, err, ErrAdminTokenNotFound)

	_, _, err = GetAdminManagedPersonalToken(adminOp, 999)
	require.ErrorIs(t, err, ErrAdminTokenNotFound)

	_, _, err = GetAdminManagedPersonalToken(AdminTokenOperator{UserId: 201, Role: common.RoleRootUser}, 301)
	require.NoError(t, err)
}
