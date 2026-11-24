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

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTokenAdminTestDB(t *testing.T) {
	t.Helper()
	previousDB := DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	t.Cleanup(func() { DB = previousDB })
	require.NoError(t, DB.AutoMigrate(&User{}, &Token{}))
}

func tokenIds(tokens []Token) []int {
	ids := make([]int, 0, len(tokens))
	for _, token := range tokens {
		ids = append(ids, token.Id)
	}
	return ids
}

func TestAdminPersonalTokensQueryRespectsRoleHierarchyAndScope(t *testing.T) {
	setupTokenAdminTestDB(t)

	require.NoError(t, DB.Create(&[]User{
		{Id: 1, Username: "root", Role: common.RoleRootUser, AffCode: "root"},
		{Id: 2, Username: "admin", Role: common.RoleAdminUser, AffCode: "admin"},
		{Id: 5, Username: "admin2", Role: common.RoleAdminUser, AffCode: "admin2"},
		{Id: 3, Username: "ops", Role: common.RoleOpsUser, AffCode: "ops"},
		{Id: 4, Username: "common", Role: common.RoleCommonUser, AffCode: "common"},
	}).Error)
	require.NoError(t, DB.Create(&[]Token{
		{Id: 11, UserId: 1, Name: "root-key", Key: "root-key", ScopeType: TokenScopePersonal},
		{Id: 12, UserId: 2, Name: "admin-key", Key: "admin-key", ScopeType: TokenScopePersonal},
		{Id: 13, UserId: 3, Name: "ops-key", Key: "ops-key", ScopeType: TokenScopePersonal},
		{Id: 14, UserId: 4, Name: "common-key", Key: "common-key", ScopeType: TokenScopePersonal},
		{Id: 15, UserId: 4, Name: "legacy-key", Key: "legacy-key", ScopeType: ""},
		{Id: 16, UserId: 4, Name: "org-key", Key: "org-key", ScopeType: TokenScopeOrganization, OrganizationId: 9},
		{Id: 17, UserId: 5, Name: "admin2-key", Key: "admin2-key", ScopeType: TokenScopePersonal},
	}).Error)

	var adminVisible []Token
	require.NoError(t, AdminPersonalTokensQuery(2, common.RoleAdminUser).Order("id").Find(&adminVisible).Error)
	require.Equal(t, []int{12, 13, 14, 15}, tokenIds(adminVisible))

	var rootVisible []Token
	require.NoError(t, AdminPersonalTokensQuery(1, common.RoleRootUser).Order("id").Find(&rootVisible).Error)
	require.Equal(t, []int{11, 12, 13, 14, 15, 17}, tokenIds(rootVisible))
}
