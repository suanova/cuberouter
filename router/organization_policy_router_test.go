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
package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestOrganizationDetailRouteSelectsReadOnlyForDisabledOrganization(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	user := createOrganizationPolicyRouterUser(t, "router-disabled-owner", common.RoleCommonUser)
	organization, err := service.CreateOrganization(user.Id, service.CreateOrganizationRequest{Name: "Router Disabled Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("status", model.OrganizationStatusDisabled).Error)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	req := httptest.NewRequest(http.MethodGet, "/api/organizations/"+strconv.Itoa(organization.Id), nil)
	req.Header.Set("Authorization", user.GetAccessToken())
	req.Header.Set("New-Api-User", strconv.Itoa(user.Id))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.Contains(t, recorder.Body.String(), `"read_only":true`)
	require.NotContains(t, recorder.Body.String(), "organization_context_mismatch")
}

func TestOrganizationTokenRouteUsesReadAccessForDisabledOrganization(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	user := createOrganizationPolicyRouterUser(t, "router-disabled-token-owner", common.RoleCommonUser)
	organization, err := service.CreateOrganization(user.Id, service.CreateOrganizationRequest{Name: "Router Disabled Token Org"})
	require.NoError(t, err)
	token := model.Token{UserId: user.Id, Key: "999999999999999999999999999999999999999999999999", Name: "disabled-token", Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, ResponsibleUserId: user.Id, Visibility: model.TokenVisibilityPrivate}
	require.NoError(t, model.DB.Create(&token).Error)
	require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("status", model.OrganizationStatusDisabled).Error)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	req := httptest.NewRequest(http.MethodGet, "/api/organizations/"+strconv.Itoa(organization.Id)+"/tokens", nil)
	req.Header.Set("Authorization", user.GetAccessToken())
	req.Header.Set("New-Api-User", strconv.Itoa(user.Id))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.NotContains(t, recorder.Body.String(), "organization_context_mismatch")
}

func TestOrganizationMemberSelfExitRouteUsesAccountContextAuth(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	owner := createOrganizationPolicyRouterUser(t, "router-exit-owner", common.RoleCommonUser)
	member := createOrganizationPolicyRouterUser(t, "router-exit-member", common.RoleCommonUser)
	organization, err := service.CreateOrganization(owner.Id, service.CreateOrganizationRequest{Name: "Router Exit Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	req := httptest.NewRequest(http.MethodDelete, "/api/organizations/"+strconv.Itoa(organization.Id)+"/members/me", nil)
	req.Header.Set("Authorization", member.GetAccessToken())
	req.Header.Set("New-Api-User", strconv.Itoa(member.Id))
	req.Header.Set("X-Account-Context-Type", "organization")
	req.Header.Set("X-Account-Context-Id", strconv.Itoa(organization.Id))
	req.Header.Set("Idempotency-Key", "router-member-exit")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
}

func TestOrganizationAccountContextInviteRevokeRouteUsesWorkspaceMode(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	owner := createOrganizationPolicyRouterUser(t, "router-invite-revoke-owner", common.RoleCommonUser)
	nonMember := createOrganizationPolicyRouterUser(t, "router-invite-revoke-non-member", common.RoleCommonUser)
	organization, err := service.CreateOrganization(owner.Id, service.CreateOrganizationRequest{Name: "Router Invite Revoke Org"})
	require.NoError(t, err)
	invite := model.OrganizationInvite{OrganizationId: organization.Id, Type: model.OrganizationInviteTypeEmail, TargetEmail: "router-revoke@example.com", Role: model.OrganizationRoleMember, Token: "router-revoke-token", Status: model.OrganizationInviteStatusPending, InviterUserId: owner.Id}
	require.NoError(t, model.DB.Create(&invite).Error)

	trustedMode := ""
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	router.Use(func(c *gin.Context) {
		c.Next()
		trustedMode = common.GetContextKeyString(c, constant.ContextKeyOrganizationAccessMode)
	})
	SetApiRouter(router)

	req := httptest.NewRequest(http.MethodDelete, "/api/organizations/"+strconv.Itoa(organization.Id)+"/invitations/"+strconv.Itoa(invite.Id)+"?reason=cleanup", nil)
	req.Header.Set("Authorization", owner.GetAccessToken())
	req.Header.Set("New-Api-User", strconv.Itoa(owner.Id))
	req.Header.Set("X-Account-Context-Type", "organization")
	req.Header.Set("X-Account-Context-Id", strconv.Itoa(organization.Id))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, service.OrganizationAccessModeWorkspace, trustedMode)
	require.NoError(t, model.DB.First(&invite, invite.Id).Error)
	require.Equal(t, model.OrganizationInviteStatusRevoked, invite.Status)

	deniedReq := httptest.NewRequest(http.MethodDelete, "/api/organizations/"+strconv.Itoa(organization.Id)+"/invitations/"+strconv.Itoa(invite.Id), nil)
	deniedReq.Header.Set("Authorization", nonMember.GetAccessToken())
	deniedReq.Header.Set("New-Api-User", strconv.Itoa(nonMember.Id))
	deniedReq.Header.Set("X-Account-Context-Type", "organization")
	deniedReq.Header.Set("X-Account-Context-Id", strconv.Itoa(organization.Id))
	deniedRecorder := httptest.NewRecorder()
	router.ServeHTTP(deniedRecorder, deniedReq)
	require.Equal(t, http.StatusForbidden, deniedRecorder.Code)
}

func TestOrganizationAccountContextTokenDeleteRouteUsesWorkspaceResourcePolicy(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	owner := createOrganizationPolicyRouterUser(t, "router-token-delete-owner", common.RoleCommonUser)
	member := createOrganizationPolicyRouterUser(t, "router-token-delete-member", common.RoleCommonUser)
	nonMember := createOrganizationPolicyRouterUser(t, "router-token-delete-non-member", common.RoleCommonUser)
	organization, err := service.CreateOrganization(owner.Id, service.CreateOrganizationRequest{Name: "Router Token Delete Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)
	token := model.Token{UserId: member.Id, Key: common.GetUUID(), Name: "router-member-private", Status: common.TokenStatusEnabled, ScopeType: model.TokenScopeOrganization, ScopeId: organization.Id, OrganizationId: organization.Id, ResponsibleUserId: member.Id, Visibility: model.TokenVisibilityPrivate}
	require.NoError(t, model.DB.Create(&token).Error)

	trustedMode := ""
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	router.Use(func(c *gin.Context) {
		c.Next()
		trustedMode = common.GetContextKeyString(c, constant.ContextKeyOrganizationAccessMode)
	})
	SetApiRouter(router)

	req := httptest.NewRequest(http.MethodDelete, "/api/organizations/"+strconv.Itoa(organization.Id)+"/tokens/"+strconv.Itoa(token.Id), nil)
	req.Header.Set("Authorization", member.GetAccessToken())
	req.Header.Set("New-Api-User", strconv.Itoa(member.Id))
	req.Header.Set("X-Account-Context-Type", "organization")
	req.Header.Set("X-Account-Context-Id", strconv.Itoa(organization.Id))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, service.OrganizationAccessModeWorkspace, trustedMode)
	var tokenCount int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", token.Id).Count(&tokenCount).Error)
	require.Zero(t, tokenCount)

	deniedReq := httptest.NewRequest(http.MethodDelete, "/api/organizations/"+strconv.Itoa(organization.Id)+"/tokens/"+strconv.Itoa(token.Id), nil)
	deniedReq.Header.Set("Authorization", nonMember.GetAccessToken())
	deniedReq.Header.Set("New-Api-User", strconv.Itoa(nonMember.Id))
	deniedReq.Header.Set("X-Account-Context-Type", "organization")
	deniedReq.Header.Set("X-Account-Context-Id", strconv.Itoa(organization.Id))
	deniedRecorder := httptest.NewRecorder()
	router.ServeHTTP(deniedRecorder, deniedReq)
	require.Equal(t, http.StatusForbidden, deniedRecorder.Code)
}

func TestAdminOrganizationWriteReturnsGoneWhenDissolvedAfterPolicy(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	platformAdmin := createOrganizationPolicyRouterUser(t, "router-post-policy-dissolved-admin", common.RoleAdminUser)
	owner := createOrganizationPolicyRouterUser(t, "router-post-policy-dissolved-owner", common.RoleCommonUser)
	target := createOrganizationPolicyRouterUser(t, "router-post-policy-dissolved-target", common.RoleCommonUser)
	organization, err := service.CreateOrganization(owner.Id, service.CreateOrganizationRequest{Name: "Router Post Policy Dissolved Org"})
	require.NoError(t, err)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	group := router.Group("/api/admin/organizations")
	group.Use(middleware.AdminAuth())
	group.POST("/:id/members", middleware.OrganizationAdminAuth(service.OrganizationCapabilityManageMembers), func(c *gin.Context) {
		require.NoError(t, model.DB.Model(&model.Organization{}).Where("id = ?", organization.Id).Update("status", model.OrganizationStatusDissolved).Error)
		controller.AddOrganizationMember(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/admin/organizations/"+strconv.Itoa(organization.Id)+"/members", strings.NewReader(`{"user_id":`+strconv.Itoa(target.Id)+`,"role":"member"}`))
	req.Header.Set("Authorization", platformAdmin.GetAccessToken())
	req.Header.Set("New-Api-User", strconv.Itoa(platformAdmin.Id))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusGone, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"code":"`+string(types.ErrorCodeOrganizationDissolved)+`"`)
}

func TestOrganizationBillingSummaryRouteRequiresFullMemberVisibility(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	owner := createOrganizationPolicyRouterUser(t, "router-billing-owner", common.RoleCommonUser)
	member := createOrganizationPolicyRouterUser(t, "router-billing-member", common.RoleCommonUser)
	organization, err := service.CreateOrganization(owner.Id, service.CreateOrganizationRequest{Name: "Router Billing Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	req := httptest.NewRequest(http.MethodGet, "/api/organizations/"+strconv.Itoa(organization.Id)+"/billing/summary", nil)
	req.Header.Set("Authorization", member.GetAccessToken())
	req.Header.Set("New-Api-User", strconv.Itoa(member.Id))
	req.Header.Set("X-Account-Context-Type", "organization")
	req.Header.Set("X-Account-Context-Id", strconv.Itoa(organization.Id))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "organization access denied")
}

func TestAdminOrganizationDetailRouteIgnoresAccountContextHeader(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	platformAdmin := createOrganizationPolicyRouterUser(t, "router-admin-org-detail", common.RoleAdminUser)
	creator := createOrganizationPolicyRouterUser(t, "router-admin-org-creator", common.RoleCommonUser)
	organization, err := service.CreateOrganization(creator.Id, service.CreateOrganizationRequest{Name: "Router Admin Detail Org"})
	require.NoError(t, err)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/organizations/"+strconv.Itoa(organization.Id), nil)
	req.Header.Set("Authorization", platformAdmin.GetAccessToken())
	req.Header.Set("New-Api-User", strconv.Itoa(platformAdmin.Id))
	req.Header.Set("X-Account-Context-Type", "organization")
	req.Header.Set("X-Account-Context-Id", strconv.Itoa(organization.Id+1000))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.NotContains(t, recorder.Body.String(), "organization_context_mismatch")
}

func TestOrganizationDetailRoutesPropagateTrustedAccessMode(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	platformAdmin := createOrganizationPolicyRouterUser(t, "router-detail-mode-admin", common.RoleAdminUser)
	owner := createOrganizationPolicyRouterUser(t, "router-detail-mode-owner", common.RoleCommonUser)
	organization, err := service.CreateOrganization(owner.Id, service.CreateOrganizationRequest{Name: "Router Detail Mode Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: platformAdmin.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	ordinaryReq := httptest.NewRequest(http.MethodGet, "/api/organizations/"+strconv.Itoa(organization.Id), nil)
	ordinaryReq.Header.Set("Authorization", platformAdmin.GetAccessToken())
	ordinaryReq.Header.Set("New-Api-User", strconv.Itoa(platformAdmin.Id))
	ordinaryReq.Header.Set("X-Account-Context-Type", "organization")
	ordinaryReq.Header.Set("X-Account-Context-Id", strconv.Itoa(organization.Id))
	ordinaryRecorder := httptest.NewRecorder()
	router.ServeHTTP(ordinaryRecorder, ordinaryReq)

	require.Equal(t, http.StatusOK, ordinaryRecorder.Code)
	require.Contains(t, ordinaryRecorder.Body.String(), `"role":"member"`)
	require.Contains(t, ordinaryRecorder.Body.String(), `"access_mode":"workspace"`)
	require.NotContains(t, ordinaryRecorder.Body.String(), `"can_manage_members":true`)

	adminReq := httptest.NewRequest(http.MethodGet, "/api/admin/organizations/"+strconv.Itoa(organization.Id), nil)
	adminReq.Header.Set("Authorization", platformAdmin.GetAccessToken())
	adminReq.Header.Set("New-Api-User", strconv.Itoa(platformAdmin.Id))
	adminRecorder := httptest.NewRecorder()
	router.ServeHTTP(adminRecorder, adminReq)

	require.Equal(t, http.StatusOK, adminRecorder.Code)
	require.Contains(t, adminRecorder.Body.String(), `"role":"platform_admin"`)
	require.Contains(t, adminRecorder.Body.String(), `"access_mode":"admin"`)
	require.Contains(t, adminRecorder.Body.String(), `"can_manage_members":true`)
}

func TestOrganizationOrdinaryRoutesRejectPlatformNonMemberAndMemberOwnerOperation(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	platformAdmin := createOrganizationPolicyRouterUser(t, "router-ordinary-nonmember-admin", common.RoleAdminUser)
	owner := createOrganizationPolicyRouterUser(t, "router-ordinary-owner", common.RoleCommonUser)
	member := createOrganizationPolicyRouterUser(t, "router-ordinary-member", common.RoleCommonUser)
	organization, err := service.CreateOrganization(owner.Id, service.CreateOrganizationRequest{Name: "Router Ordinary Boundary Org"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: organization.Id, UserId: member.Id, Role: model.OrganizationRoleMember, Status: model.OrganizationMemberStatusActive}).Error)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	nonMemberReq := httptest.NewRequest(http.MethodGet, "/api/organizations/"+strconv.Itoa(organization.Id), nil)
	nonMemberReq.Header.Set("Authorization", platformAdmin.GetAccessToken())
	nonMemberReq.Header.Set("New-Api-User", strconv.Itoa(platformAdmin.Id))
	nonMemberReq.Header.Set("X-Account-Context-Type", "organization")
	nonMemberReq.Header.Set("X-Account-Context-Id", strconv.Itoa(organization.Id))
	nonMemberRecorder := httptest.NewRecorder()
	router.ServeHTTP(nonMemberRecorder, nonMemberReq)
	require.Equal(t, http.StatusForbidden, nonMemberRecorder.Code)

	dissolveReq := httptest.NewRequest(http.MethodDelete, "/api/organizations/"+strconv.Itoa(organization.Id), strings.NewReader(`{"confirm_name":"`+organization.Name+`","reason":"close"}`))
	dissolveReq.Header.Set("Authorization", member.GetAccessToken())
	dissolveReq.Header.Set("New-Api-User", strconv.Itoa(member.Id))
	dissolveReq.Header.Set("Content-Type", "application/json")
	dissolveReq.Header.Set("Idempotency-Key", "router-member-dissolve-denied")
	dissolveRecorder := httptest.NewRecorder()
	router.ServeHTTP(dissolveRecorder, dissolveReq)
	require.Equal(t, http.StatusForbidden, dissolveRecorder.Code)
}

func TestAdminOrganizationStatusRouteLetsServiceSelectEnableCapability(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	platformAdmin := createOrganizationPolicyRouterUser(t, "router-admin-status-enable", common.RoleAdminUser)
	creator := createOrganizationPolicyRouterUser(t, "router-admin-status-creator", common.RoleCommonUser)
	organization, err := service.CreateOrganization(creator.Id, service.CreateOrganizationRequest{Name: "Router Admin Status Org"})
	require.NoError(t, err)
	require.NoError(t, service.DisableOrganizationByPlatform(platformAdmin.Id, organization.Id, service.OrganizationAccessModeAdmin, organization.Slug, "disable first"))

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/organizations/"+strconv.Itoa(organization.Id)+"/status", strings.NewReader(`{"status":"active","reason":"resume"}`))
	req.Header.Set("Authorization", platformAdmin.GetAccessToken())
	req.Header.Set("New-Api-User", strconv.Itoa(platformAdmin.Id))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), string(types.ErrorCodeOrganizationConfirmationMismatch))

	req = httptest.NewRequest(http.MethodPatch, "/api/admin/organizations/"+strconv.Itoa(organization.Id)+"/status", strings.NewReader(`{"status":"active","confirm_name":"`+organization.Slug+`","reason":"resume"}`))
	req.Header.Set("Authorization", platformAdmin.GetAccessToken())
	req.Header.Set("New-Api-User", strconv.Itoa(platformAdmin.Id))
	req.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, model.OrganizationStatusActive, stored.Status)
}

func TestPlatformRootOrganizationStatusRouteEnablesSelfDisabledOrganization(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	platformRoot := createOrganizationPolicyRouterUser(t, "router-root-enable-self-disabled", common.RoleRootUser)
	creator := createOrganizationPolicyRouterUser(t, "router-self-disabled-creator", common.RoleCommonUser)
	organization, err := service.CreateOrganization(creator.Id, service.CreateOrganizationRequest{Name: "Router Self Disabled Org"})
	require.NoError(t, err)
	require.NoError(t, service.DisableOrganization(creator.Id, organization.Id, service.OrganizationAccessModeManagement, organization.Slug, "self maintenance"))

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/organizations/"+strconv.Itoa(organization.Id)+"/status", strings.NewReader(`{"status":"active","confirm_name":"`+organization.Slug+`","reason":"platform resume"}`))
	req.Header.Set("Authorization", platformRoot.GetAccessToken())
	req.Header.Set("New-Api-User", strconv.Itoa(platformRoot.Id))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, model.OrganizationStatusActive, stored.Status)
}

func TestOrganizationStatusRouteRequiresSlugConfirmation(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	owner := createOrganizationPolicyRouterUser(t, "router-status-confirmation-owner", common.RoleCommonUser)
	organization, err := service.CreateOrganization(owner.Id, service.CreateOrganizationRequest{Name: "Router Status Confirmation Org"})
	require.NoError(t, err)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	req := httptest.NewRequest(http.MethodPatch, "/api/organizations/"+strconv.Itoa(organization.Id)+"/status", strings.NewReader(`{"status":"disabled","reason":"maintenance"}`))
	req.Header.Set("Authorization", owner.GetAccessToken())
	req.Header.Set("New-Api-User", strconv.Itoa(owner.Id))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), string(types.ErrorCodeOrganizationConfirmationMismatch))

	req = httptest.NewRequest(http.MethodPatch, "/api/organizations/"+strconv.Itoa(organization.Id)+"/status", strings.NewReader(`{"status":"disabled","confirm_name":"`+organization.Slug+`","reason":"maintenance"}`))
	req.Header.Set("Authorization", owner.GetAccessToken())
	req.Header.Set("New-Api-User", strconv.Itoa(owner.Id))
	req.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, model.OrganizationStatusDisabled, stored.Status)
}

func TestAdminOrganizationOwnerAndDissolveRoutesRejectPlatformAdmin(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	platformAdmin := createOrganizationPolicyRouterUser(t, "router-admin-root-only-admin", common.RoleAdminUser)
	creator := createOrganizationPolicyRouterUser(t, "router-admin-root-only-creator", common.RoleCommonUser)
	newOwner := createOrganizationPolicyRouterUser(t, "router-admin-root-only-target", common.RoleCommonUser)
	organization, err := service.CreateOrganization(creator.Id, service.CreateOrganizationRequest{Name: "Router Admin Root Only Org"})
	require.NoError(t, err)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	transferReq := httptest.NewRequest(http.MethodPut, "/api/admin/organizations/"+strconv.Itoa(organization.Id)+"/owner", strings.NewReader(`{"owner_user_id":`+strconv.Itoa(newOwner.Id)+`,"reason":"handoff"}`))
	transferReq.Header.Set("Authorization", platformAdmin.GetAccessToken())
	transferReq.Header.Set("New-Api-User", strconv.Itoa(platformAdmin.Id))
	transferReq.Header.Set("Content-Type", "application/json")
	transferReq.Header.Set("Idempotency-Key", "router-platform-transfer-denied")
	transferRecorder := httptest.NewRecorder()
	router.ServeHTTP(transferRecorder, transferReq)

	require.Equal(t, http.StatusForbidden, transferRecorder.Code)
	require.Contains(t, transferRecorder.Body.String(), `"code":"organization_access_denied"`)

	dissolveReq := httptest.NewRequest(http.MethodDelete, "/api/admin/organizations/"+strconv.Itoa(organization.Id), strings.NewReader(`{"confirm_name":"`+organization.Name+`","reason":"close"}`))
	dissolveReq.Header.Set("Authorization", platformAdmin.GetAccessToken())
	dissolveReq.Header.Set("New-Api-User", strconv.Itoa(platformAdmin.Id))
	dissolveReq.Header.Set("Content-Type", "application/json")
	dissolveReq.Header.Set("Idempotency-Key", "router-platform-dissolve-denied")
	dissolveRecorder := httptest.NewRecorder()
	router.ServeHTTP(dissolveRecorder, dissolveReq)

	require.Equal(t, http.StatusForbidden, dissolveRecorder.Code)
	require.Contains(t, dissolveRecorder.Body.String(), `"code":"organization_access_denied"`)
	var stored model.Organization
	require.NoError(t, model.DB.First(&stored, organization.Id).Error)
	require.Equal(t, creator.Id, stored.OwnerUserId)
	require.Equal(t, model.OrganizationStatusActive, stored.Status)
}

func TestAdminOrganizationRoutesDoNotExposeTokenCreation(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	platformAdmin := createOrganizationPolicyRouterUser(t, "router-admin-no-token-create", common.RoleAdminUser)
	creator := createOrganizationPolicyRouterUser(t, "router-admin-no-token-create-owner", common.RoleCommonUser)
	organization, err := service.CreateOrganization(creator.Id, service.CreateOrganizationRequest{Name: "Router No Token Create Org"})
	require.NoError(t, err)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)
	for _, path := range []string{
		"/api/admin/organizations/" + strconv.Itoa(organization.Id) + "/tokens",
		"/api/admin/organizations/" + strconv.Itoa(organization.Id) + "/token-batches",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("Authorization", platformAdmin.GetAccessToken())
		req.Header.Set("New-Api-User", strconv.Itoa(platformAdmin.Id))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusNotFound, recorder.Code, path)
	}
}

func TestAdminOrganizationDetailRouteAllowsDissolvedReadOnly(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	platformAdmin := createOrganizationPolicyRouterUser(t, "router-admin-dissolved-read-admin", common.RoleAdminUser)
	creator := createOrganizationPolicyRouterUser(t, "router-admin-dissolved-read-creator", common.RoleCommonUser)
	root := createOrganizationPolicyRouterUser(t, "router-admin-dissolved-read-root", common.RoleRootUser)
	organization, err := service.CreateOrganization(creator.Id, service.CreateOrganizationRequest{Name: "Router Admin Dissolved Read Org"})
	require.NoError(t, err)
	require.NoError(t, service.DissolveOrganization(root.Id, organization.Id, service.OrganizationAccessModeAdmin, service.DissolveOrganizationRequest{ConfirmName: organization.Name, Reason: "closed", IdempotencyKey: "router-dissolved-read"}))

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/organizations/"+strconv.Itoa(organization.Id), nil)
	req.Header.Set("Authorization", platformAdmin.GetAccessToken())
	req.Header.Set("New-Api-User", strconv.Itoa(platformAdmin.Id))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.Contains(t, recorder.Body.String(), `"read_only":true`)
	require.Contains(t, recorder.Body.String(), `"role":"platform_admin"`)
}

func TestAdminOrganizationNestedWriteRouteRejectsDissolvedOrganization(t *testing.T) {
	setupOrganizationPolicyRouterTestDB(t)
	platformAdmin := createOrganizationPolicyRouterUser(t, "router-admin-dissolved-write-admin", common.RoleAdminUser)
	creator := createOrganizationPolicyRouterUser(t, "router-admin-dissolved-write-creator", common.RoleCommonUser)
	target := createOrganizationPolicyRouterUser(t, "router-admin-dissolved-write-target", common.RoleCommonUser)
	root := createOrganizationPolicyRouterUser(t, "router-admin-dissolved-write-root", common.RoleRootUser)
	organization, err := service.CreateOrganization(creator.Id, service.CreateOrganizationRequest{Name: "Router Admin Dissolved Write Org"})
	require.NoError(t, err)
	require.NoError(t, service.DissolveOrganization(root.Id, organization.Id, service.OrganizationAccessModeAdmin, service.DissolveOrganizationRequest{ConfirmName: organization.Name, Reason: "closed", IdempotencyKey: "router-dissolve-closed"}))

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("router-test-secret"))))
	SetApiRouter(router)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/organizations/"+strconv.Itoa(organization.Id)+"/members", strings.NewReader(`{"user_id":`+strconv.Itoa(target.Id)+`,"role":"member"}`))
	req.Header.Set("Authorization", platformAdmin.GetAccessToken())
	req.Header.Set("New-Api-User", strconv.Itoa(platformAdmin.Id))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusGone, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"code":"`+string(types.ErrorCodeOrganizationDissolved)+`"`)
	var memberCount int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", organization.Id, target.Id).Count(&memberCount).Error)
	require.EqualValues(t, 0, memberCount)
}

// setupOrganizationPolicyRouterTestDB 建一个测试专属的进程内 SQLite，并把组织相关表建出来。
//
// 走 model.InitDB() 而不是自己 gorm.Open：InitDB 会执行 initCol()，后者填的是
// `key` / `group` 这两个保留字的引用格式；跳过它的话令牌查询会生成列名为空的坏 SQL，
// 这些路由用例会直接 500。三个包级全局（model.DB / LOG_DB / RedisEnabled）
// 必须原样还原，router 包里还有别的用例共用同一个测试二进制。
func setupOrganizationPolicyRouterTestDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	originalDB, originalLogDB := model.DB, model.LOG_DB
	originalMainType, originalLogType := common.MainDatabaseType(), common.LogDatabaseType()
	originalRedisEnabled := common.RedisEnabled
	originalSQLitePath := common.SQLitePath
	originalMasterNode := common.IsMasterNode
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")

	common.IsMasterNode = false
	common.RedisEnabled = false
	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=busy_timeout(30000)", strings.ReplaceAll(t.Name(), "/", "_"))
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, os.Setenv("SQL_DSN", "local"))
	require.NoError(t, model.InitDB())
	model.LOG_DB = model.DB
	require.NoError(t, model.DB.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.OrganizationInvite{},
		&model.OrganizationDisableRecord{},
		&model.OrganizationTokenSystemBlocker{},
		&model.OrganizationIdempotencyRecord{},
		&model.OrganizationBillingSession{},
		&model.OrganizationAuditLog{},
		&model.OrganizationQuotaAdjustment{},
		&model.Task{},
		&model.Midjourney{},
	))

	t.Cleanup(func() {
		if model.DB != nil {
			if sqlDB, err := model.DB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		model.DB, model.LOG_DB = originalDB, originalLogDB
		common.SetDatabaseTypes(originalMainType, originalLogType)
		common.RedisEnabled = originalRedisEnabled
		common.SQLitePath = originalSQLitePath
		common.IsMasterNode = originalMasterNode
		if hadSQLDSN {
			_ = os.Setenv("SQL_DSN", originalSQLDSN)
		} else {
			_ = os.Unsetenv("SQL_DSN")
		}
	})
}

func createOrganizationPolicyRouterUser(t *testing.T, username string, role int) model.User {
	t.Helper()
	accessToken := username + "-access-token"
	user := model.User{Username: username, Password: "password", DisplayName: username, Role: role, Status: common.UserStatusEnabled, AffCode: username}
	user.SetAccessToken(accessToken)
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}
