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

func seedSelfOrganizationTokens(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.Create(&[]model.User{
		{Id: 101, Username: "alice", DisplayName: "Alice", AffCode: "alice", Status: common.UserStatusEnabled, Role: common.RoleCommonUser},
		{Id: 102, Username: "bob", DisplayName: "Bob", AffCode: "bob", Status: common.UserStatusEnabled, Role: common.RoleCommonUser},
	}).Error)
	require.NoError(t, model.DB.Create(&[]model.Organization{
		{Id: 9, Name: "Acme", Slug: "acme", Status: model.OrganizationStatusActive, OwnerUserId: 102},
		{Id: 10, Name: "Beta", Slug: "beta", Status: model.OrganizationStatusActive, OwnerUserId: 102},
		{Id: 11, Name: "Old", Slug: "old", Status: model.OrganizationStatusDissolved, OwnerUserId: 101},
		{Id: 12, Name: "Foreign", Slug: "foreign", Status: model.OrganizationStatusActive, OwnerUserId: 102},
	}).Error)
	require.NoError(t, model.DB.Create(&[]model.OrganizationMember{
		{OrganizationId: 9, UserId: 101, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive},
		{OrganizationId: 9, UserId: 102, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive},
		{OrganizationId: 10, UserId: 101, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive},
		{OrganizationId: 10, UserId: 102, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive},
		{OrganizationId: 11, UserId: 101, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive},
		{OrganizationId: 12, UserId: 102, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive},
	}).Error)
	require.NoError(t, model.DB.Create(&[]model.Token{
		{Id: 1, UserId: 101, Name: "personal", Key: "personal", ScopeType: model.TokenScopePersonal, Status: common.TokenStatusEnabled},
		{Id: 2, UserId: 101, Name: "acme-public", Key: "acme-public", ScopeType: model.TokenScopeOrganization, OrganizationId: 9, Visibility: model.TokenVisibilityPublic, Status: common.TokenStatusEnabled},
		{Id: 3, UserId: 101, Name: "acme-mine", Key: "acme-mine", ScopeType: model.TokenScopeOrganization, OrganizationId: 9, Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: 101, CreatorUserId: 101, Status: common.TokenStatusEnabled},
		{Id: 4, UserId: 101, Name: "acme-bob", Key: "acme-bob", ScopeType: model.TokenScopeOrganization, OrganizationId: 9, Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: 102, CreatorUserId: 101, Status: common.TokenStatusEnabled},
		{Id: 5, UserId: 102, Name: "acme-other", Key: "acme-other", ScopeType: model.TokenScopeOrganization, OrganizationId: 9, Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: 102, CreatorUserId: 102, Status: common.TokenStatusEnabled},
		{Id: 6, UserId: 101, Name: "beta-mine", Key: "beta-mine", ScopeType: model.TokenScopeOrganization, OrganizationId: 10, Visibility: model.TokenVisibilityPrivate, ResponsibleUserId: 101, CreatorUserId: 101, Status: common.TokenStatusEnabled},
		{Id: 7, UserId: 101, Name: "old-public", Key: "old-public", ScopeType: model.TokenScopeOrganization, OrganizationId: 11, Visibility: model.TokenVisibilityPublic, Status: common.TokenStatusEnabled},
		{Id: 8, UserId: 102, Name: "foreign-public", Key: "foreign-public", ScopeType: model.TokenScopeOrganization, OrganizationId: 12, Visibility: model.TokenVisibilityPublic, Status: common.TokenStatusEnabled},
		{Id: 13, UserId: 101, Name: "acme-deleted", Key: "acme-deleted", ScopeType: model.TokenScopeOrganization, OrganizationId: 9, Visibility: model.TokenVisibilityPublic, Status: common.TokenStatusEnabled},
	}).Error)
	// The soft-deleted key must not reappear in the member's list.
	require.NoError(t, model.DB.Delete(&model.Token{Id: 13}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id: 9, UserId: 101, Name: "acme-disabled", Key: "acme-disabled",
		ScopeType: model.TokenScopeOrganization, OrganizationId: 9,
		Visibility: model.TokenVisibilityPublic, Status: common.TokenStatusDisabled,
	}).Error)
}

func TestListSelfOrganizationTokensOnlyReturnsUsableKeys(t *testing.T) {
	setupServiceTestDB(t)
	seedSelfOrganizationTokens(t)

	tokens, total, err := ListSelfOrganizationTokens(101, SelfOrganizationTokenListRequest{
		Limit: 20,
	})
	require.NoError(t, err)
	require.EqualValues(t, 5, total)
	require.Len(t, tokens, 5)

	byName := map[string]*model.Token{}
	for _, token := range tokens {
		byName[token.Name] = token
	}
	// Public, own private, and keys this member created are usable; another
	// member's private key, a dissolved organization, and a foreign
	// organization are not.
	assert.Contains(t, byName, "acme-public")
	assert.Contains(t, byName, "acme-mine")
	assert.Contains(t, byName, "acme-bob")
	assert.Contains(t, byName, "beta-mine")
	assert.Contains(t, byName, "acme-disabled")
	assert.NotContains(t, byName, "acme-other")
	assert.NotContains(t, byName, "old-public")
	assert.NotContains(t, byName, "foreign-public")
	assert.NotContains(t, byName, "personal")
	assert.NotContains(t, byName, "acme-deleted")

	assert.Equal(t, "Acme", byName["acme-public"].OrganizationName)
	assert.Equal(t, "Beta", byName["beta-mine"].OrganizationName)
	assert.NotEmpty(t, byName["acme-public"].KeyPreview)
	assert.Contains(t, byName["acme-public"].KeyPreview, "*")
	assert.Equal(t, "Alice", byName["acme-mine"].ResponsibleDisplayName)
}

func TestListSelfOrganizationTokensOwnerSeesEveryOrganizationKey(t *testing.T) {
	setupServiceTestDB(t)
	seedSelfOrganizationTokens(t)

	tokens, total, err := ListSelfOrganizationTokens(102, SelfOrganizationTokenListRequest{
		Limit: 20,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 7, total)
	require.Len(t, tokens, 7)

	names := make(map[string]bool, len(tokens))
	for _, token := range tokens {
		names[token.Name] = true
	}
	// The owner sees another member's private key and the public key of the
	// foreign organization they belong to.
	assert.True(t, names["acme-other"])
	assert.True(t, names["foreign-public"])
	assert.True(t, names["acme-public"])
	assert.False(t, names["personal"])
}

func TestListSelfOrganizationTokensFilters(t *testing.T) {
	setupServiceTestDB(t)
	seedSelfOrganizationTokens(t)

	tokens, total, err := ListSelfOrganizationTokens(101, SelfOrganizationTokenListRequest{
		Limit:          20,
		OrganizationID: 9,
		Visibility:     model.TokenVisibilityPrivate,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, tokens, 2)
	for _, token := range tokens {
		assert.Equal(t, 9, token.OrganizationId)
		assert.Equal(t, model.TokenVisibilityPrivate, token.Visibility)
	}

	tokens, total, err = ListSelfOrganizationTokens(101, SelfOrganizationTokenListRequest{
		Limit:   20,
		Keyword: "beta",
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, tokens, 1)
	assert.Equal(t, "beta-mine", tokens[0].Name)

	tokens, total, err = ListSelfOrganizationTokens(101, SelfOrganizationTokenListRequest{
		Limit: 20,
		Token: "sk-beta-mine",
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, tokens, 1)

	tokens, total, err = ListSelfOrganizationTokens(101, SelfOrganizationTokenListRequest{
		Limit: 1,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 5, total)
	require.Len(t, tokens, 1)
}
