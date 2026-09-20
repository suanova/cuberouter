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

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"

	"github.com/gin-gonic/gin"
)

func setupTokenAuthMiddlewareTestDB(t *testing.T) {
	t.Helper()
	setupOrganizationMiddlewareTestDB(t,
		&model.User{},
		&model.Token{},
		&model.Log{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationTokenSystemBlocker{},
		&model.OrganizationAuditLog{},
	)
}

func TestTokenAuthOrganizationTokenWritesBillingContext(t *testing.T) {
	setupTokenAuthMiddlewareTestDB(t)
	creator := createTokenAuthMiddlewareUser(t, "token-auth-creator")
	responsible := createTokenAuthMiddlewareUser(t, "token-auth-responsible")
	organization := model.Organization{Name: "Token Auth Org", Slug: "token-auth-org", Status: model.OrganizationStatusActive, Quota: 2000, UsedQuota: 125, CreatedBy: creator.Id, OwnerUserId: creator.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: responsible.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{
		UserId:            responsible.Id,
		Key:               "tokenauthorgcontext00000000000000000000000001",
		Name:              "org-relay-key",
		Status:            common.TokenStatusEnabled,
		ExpiredTime:       -1,
		RemainQuota:       10,
		ScopeType:         model.TokenScopeOrganization,
		ScopeId:           organization.Id,
		OrganizationId:    organization.Id,
		CreatorUserId:     creator.Id,
		ResponsibleUserId: responsible.Id,
		Visibility:        model.TokenVisibilityPrivate,
	}
	require.NoError(t, model.DB.Create(&token).Error)

	recorder := runTokenAuthMiddlewareProbe(t, token.Key)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	require.Contains(t, body, `"id":`+strconv.Itoa(responsible.Id))
	require.Contains(t, body, `"scope_type":"organization"`)
	require.Contains(t, body, `"scope_id":`+strconv.Itoa(organization.Id))
	require.Contains(t, body, `"billing_account_type":"organization"`)
	require.Contains(t, body, `"billing_account_id":`+strconv.Itoa(organization.Id))
	require.Contains(t, body, `"organization_id":`+strconv.Itoa(organization.Id))
	require.Contains(t, body, `"actor_user_id":`+strconv.Itoa(responsible.Id))
	require.Contains(t, body, `"creator_user_id":`+strconv.Itoa(creator.Id))
	require.Contains(t, body, `"responsible_user_id":`+strconv.Itoa(responsible.Id))
	require.Contains(t, body, `"organization_quota":1875`)
}

func TestTokenAuthOrganizationTokenRejectsExhaustedQuota(t *testing.T) {
	setupTokenAuthMiddlewareTestDB(t)
	user := createTokenAuthMiddlewareUser(t, "token-auth-exhausted")
	organization := model.Organization{Name: "Token Exhausted Org", Slug: "token-exhausted-org", Status: model.OrganizationStatusActive, Quota: 2000, CreatedBy: user.Id, OwnerUserId: user.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: user.Id, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{
		UserId:            user.Id,
		Key:               "tokenauthexhausted00000000000000000000000001",
		Name:              "org-exhausted-key",
		Status:            common.TokenStatusEnabled,
		ExpiredTime:       -1,
		RemainQuota:       0,
		ScopeType:         model.TokenScopeOrganization,
		ScopeId:           organization.Id,
		OrganizationId:    organization.Id,
		CreatorUserId:     user.Id,
		ResponsibleUserId: user.Id,
		Visibility:        model.TokenVisibilityPrivate,
	}
	require.NoError(t, model.DB.Create(&token).Error)

	recorder := runTokenAuthMiddlewareProbe(t, token.Key)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), translated(i18n.MsgTokenInvalid))
}

func TestTokenAuthRejectsDisabledUserToken(t *testing.T) {
	setupTokenAuthMiddlewareTestDB(t)
	user := createTokenAuthMiddlewareUser(t, "token-auth-disabled-user")
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("status", common.UserStatusDisabled).Error)
	token := model.Token{
		UserId:         user.Id,
		Key:            "tokenauthdisableduser0000000000000000000001",
		Name:           "disabled-user-key",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		RemainQuota:    10,
		UnlimitedQuota: true,
	}
	require.NoError(t, model.DB.Create(&token).Error)

	recorder := runTokenAuthMiddlewareProbe(t, token.Key)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), translated(i18n.MsgAuthUserBanned))
}

func TestTokenAuthRejectsOrganizationTokenWhenResponsibleUserStatusChangedWithoutStateMachine(t *testing.T) {
	setupTokenAuthMiddlewareTestDB(t)
	admin := createTokenAuthMiddlewareUser(t, "token-auth-org-admin")
	responsible := createTokenAuthMiddlewareUser(t, "token-auth-org-disabled")
	organization := model.Organization{Name: "Token Disabled Responsible Org", Slug: "token-disabled-responsible-org", Status: model.OrganizationStatusActive, Quota: 2000, CreatedBy: admin.Id, OwnerUserId: admin.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: responsible.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", responsible.Id).Update("status", common.UserStatusDisabled).Error)
	token := model.Token{
		UserId:            admin.Id,
		Key:               "tokenauthdisabledresponsible000000000000001",
		Name:              "disabled-responsible-key",
		Status:            common.TokenStatusEnabled,
		ExpiredTime:       -1,
		RemainQuota:       10,
		UnlimitedQuota:    true,
		ScopeType:         model.TokenScopeOrganization,
		ScopeId:           organization.Id,
		OrganizationId:    organization.Id,
		CreatorUserId:     admin.Id,
		ResponsibleUserId: responsible.Id,
		Visibility:        model.TokenVisibilityPrivate,
	}
	require.NoError(t, model.DB.Create(&token).Error)

	recorder := runTokenAuthMiddlewareProbe(t, token.Key)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "organization responsible user disabled")
}

func TestTokenAuthRejectsDisabledOrganizationTokenWithoutAudit(t *testing.T) {
	setupTokenAuthMiddlewareTestDB(t)
	admin := createTokenAuthMiddlewareUser(t, "token-auth-org-audit-admin")
	organization := model.Organization{Name: "Token Disabled Audit Org", Slug: "token-disabled-audit-org", Status: model.OrganizationStatusActive, Quota: 2000, CreatedBy: admin.Id, OwnerUserId: admin.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: admin.Id, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{
		UserId:            admin.Id,
		Key:               "tokenauthdisabledorgaudit00000000000000001",
		Name:              "disabled-org-audit-key",
		Status:            common.TokenStatusDisabled,
		ExpiredTime:       -1,
		RemainQuota:       10,
		UnlimitedQuota:    true,
		ScopeType:         model.TokenScopeOrganization,
		ScopeId:           organization.Id,
		OrganizationId:    organization.Id,
		CreatorUserId:     admin.Id,
		ResponsibleUserId: admin.Id,
		Visibility:        model.TokenVisibilityPrivate,
	}
	require.NoError(t, model.DB.Create(&token).Error)

	recorder := runTokenAuthMiddlewareProbe(t, token.Key)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), translated(i18n.MsgTokenInvalid))
}

func TestTokenAuthDoesNotAuditIdentifiableOrganizationTokenFailures(t *testing.T) {
	setupTokenAuthMiddlewareTestDB(t)
	user := createTokenAuthMiddlewareUser(t, "token-auth-identifiable")
	organization := model.Organization{Name: "Token Identifiable Org", Slug: "token-identifiable-org", Status: model.OrganizationStatusActive, Quota: 2000, CreatedBy: user.Id, OwnerUserId: user.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: user.Id, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{UserId: user.Id, Key: "tokenauthsoftdeleted0000000000000000000001", Name: "soft-deleted-org-key", Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, CreatorUserId: user.Id, ResponsibleUserId: user.Id, Visibility: model.TokenVisibilityPrivate}
	require.NoError(t, model.DB.Create(&token).Error)
	require.NoError(t, model.DB.Delete(&token).Error)

	recorder := runTokenAuthMiddlewareProbe(t, token.Key)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), translated(i18n.MsgTokenInvalid))

	randomRecorder := runTokenAuthMiddlewareProbe(t, "random-invalid-key")
	require.Equal(t, http.StatusUnauthorized, randomRecorder.Code)
}

func TestTokenAuthDoesNotAuditOrganizationLifecycleFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, organization model.Organization, member model.OrganizationMember, token model.Token)
		want   string
	}{
		{
			name: "disabled organization",
			mutate: func(t *testing.T, organization model.Organization, _ model.OrganizationMember, _ model.Token) {
				require.NoError(t, model.DB.Model(&organization).Update("status", model.OrganizationStatusDisabled).Error)
			},
			want: "organization disabled",
		},
		{
			name: "disabled member",
			mutate: func(t *testing.T, _ model.Organization, member model.OrganizationMember, _ model.Token) {
				require.NoError(t, model.DB.Model(&member).Update("status", model.OrganizationMemberStatusDisabled).Error)
			},
			want: "organization member disabled",
		},
		{
			name: "system blocker",
			mutate: func(t *testing.T, organization model.Organization, _ model.OrganizationMember, token model.Token) {
				require.NoError(t, model.DB.Create(&model.OrganizationTokenSystemBlocker{TokenId: token.Id, OrganizationId: organization.Id, Reason: "organization_platform_disabled", Status: model.OrganizationTokenBlockerStatusActive}).Error)
			},
			want: "organization token is blocked",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupTokenAuthMiddlewareTestDB(t)
			user := createTokenAuthMiddlewareUser(t, "token-auth-lifecycle")
			organization := model.Organization{Name: "Token Lifecycle Org", Slug: "token-lifecycle-org", Status: model.OrganizationStatusActive, Quota: 2000, CreatedBy: user.Id, OwnerUserId: user.Id}
			require.NoError(t, model.DB.Create(&organization).Error)
			member := model.OrganizationMember{OrganizationId: organization.Id, UserId: user.Id, Role: model.OrganizationRoleOwner, Status: model.OrganizationMemberStatusActive}
			require.NoError(t, model.DB.Create(&member).Error)
			token := model.Token{UserId: user.Id, Key: "tokenauthlifecycle0000000000000000000000001", Name: "lifecycle-org-key", Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, CreatorUserId: user.Id, ResponsibleUserId: user.Id, Visibility: model.TokenVisibilityPrivate}
			require.NoError(t, model.DB.Create(&token).Error)
			tc.mutate(t, organization, member, token)

			recorder := runTokenAuthMiddlewareProbe(t, token.Key)
			require.Equal(t, http.StatusForbidden, recorder.Code)
			require.Contains(t, recorder.Body.String(), tc.want)
		})
	}
}

func createTokenAuthMiddlewareUser(t *testing.T, username string) model.User {
	t.Helper()
	user := model.User{Username: username, Password: "password", DisplayName: username, Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 1000, Group: "default", AffCode: username}
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

func runTokenAuthMiddlewareProbe(t *testing.T, key string) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.GET("/v1/chat/completions", TokenAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"id":                   c.GetInt("id"),
			"scope_type":           common.GetContextKeyString(c, constant.ContextKeyScopeType),
			"scope_id":             common.GetContextKeyInt(c, constant.ContextKeyScopeId),
			"billing_account_type": common.GetContextKeyString(c, constant.ContextKeyBillingAccountType),
			"billing_account_id":   common.GetContextKeyInt(c, constant.ContextKeyBillingAccountId),
			"organization_id":      common.GetContextKeyInt(c, constant.ContextKeyOrganizationId),
			"actor_user_id":        common.GetContextKeyInt(c, constant.ContextKeyActorUserId),
			"creator_user_id":      common.GetContextKeyInt(c, constant.ContextKeyCreatorUserId),
			"responsible_user_id":  common.GetContextKeyInt(c, constant.ContextKeyResponsibleUserId),
			"organization_quota":   common.GetContextKeyInt(c, constant.ContextKeyOrganizationQuota),
		})
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	request.RemoteAddr = "203.0.113.9:4321"
	request.Header.Set("User-Agent", "searouter-token-auth-test")
	request.Header.Set("Authorization", "Bearer sk-"+key)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}
