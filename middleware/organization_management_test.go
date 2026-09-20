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
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"

	"github.com/gin-gonic/gin"
)

func TestOrganizationManagementAuthWritesPolicyContext(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	user := createOrganizationPolicyMiddlewareUser(t, "org-management-owner", common.RoleCommonUser)
	org := createOrganizationPolicyMiddlewareOrganization(t, user, model.OrganizationStatusActive, model.OrganizationRoleOwner)

	recorder := runOrganizationPolicyMiddlewareRequest(t, user, http.MethodPatch, "/api/organizations/"+strconv.Itoa(org.Id)+"/management", OrganizationManagementAuth())

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"organization_role":"owner"`)
	require.Contains(t, recorder.Body.String(), `"organization_access_mode":"management"`)
	require.Contains(t, recorder.Body.String(), service.OrganizationCapabilityManageMembers)
}

func TestOrganizationManagementAuthIgnoresAccountContextHeader(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	user := createOrganizationPolicyMiddlewareUser(t, "org-management-header", common.RoleCommonUser)
	org := createOrganizationPolicyMiddlewareOrganization(t, user, model.OrganizationStatusActive, model.OrganizationRoleOwner)

	recorder := runOrganizationPolicyMiddlewareRequestWithHeaders(t, user, http.MethodPatch, "/api/organizations/"+strconv.Itoa(org.Id)+"/management", OrganizationManagementAuth(), map[string]string{
		"X-Account-Context-Type": "personal",
		"X-Account-Context-Id":   strconv.Itoa(user.Id),
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"organization_role":"owner"`)
}

func TestOrganizationManagementAuthRequiresSpecificCapability(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	user := createOrganizationPolicyMiddlewareUser(t, "org-management-capability", common.RoleCommonUser)
	org := createOrganizationPolicyMiddlewareOrganization(t, user, model.OrganizationStatusDisabled, model.OrganizationRoleOwner)
	require.NoError(t, model.DB.Create(&model.OrganizationDisableRecord{
		OrganizationId:   org.Id,
		Source:           model.OrganizationDisableSourcePlatform,
		Status:           model.OrganizationRecordStatusActive,
		DisabledByUserId: user.Id,
	}).Error)

	recorder := runOrganizationPolicyMiddlewareRequest(t, user, http.MethodPatch, "/api/organizations/"+strconv.Itoa(org.Id)+"/management", OrganizationManagementAuth(service.OrganizationCapabilityEnableOrganization))

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), string(types.ErrorCodeOrganizationAccessDenied))
}

func TestOrganizationManagementAuthRejectsPlatformAdminNonMember(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	owner := createOrganizationPolicyMiddlewareUser(t, "org-management-platform-owner", common.RoleCommonUser)
	platformAdmin := createOrganizationPolicyMiddlewareUser(t, "org-management-platform-admin", common.RoleAdminUser)
	org := createOrganizationPolicyMiddlewareOrganization(t, owner, model.OrganizationStatusActive, model.OrganizationRoleOwner)

	recorder := runOrganizationPolicyMiddlewareRequest(t, platformAdmin, http.MethodPatch, "/api/organizations/"+strconv.Itoa(org.Id)+"/management", OrganizationManagementAuth())

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), string(types.ErrorCodeOrganizationAccessDenied))
}

func TestOrganizationReadAccessAuthUsesOrganizationMemberRoleForPlatformAdmin(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	owner := createOrganizationPolicyMiddlewareUser(t, "org-read-platform-member-owner", common.RoleCommonUser)
	platformAdmin := createOrganizationPolicyMiddlewareUser(t, "org-read-platform-member-admin", common.RoleAdminUser)
	org := createOrganizationPolicyMiddlewareOrganization(t, owner, model.OrganizationStatusActive, model.OrganizationRoleOwner)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{
		OrganizationId: org.Id,
		UserId:         platformAdmin.Id,
		Role:           model.OrganizationRoleMember,
		Status:         model.OrganizationMemberStatusActive,
	}).Error)

	recorder := runOrganizationPolicyMiddlewareRequestWithHeaders(
		t,
		platformAdmin,
		http.MethodGet,
		"/api/organizations/"+strconv.Itoa(org.Id)+"/management",
		OrganizationReadAccessAuth(),
		map[string]string{
			"X-Account-Context-Type": model.AccountContextTypeOrganization,
			"X-Account-Context-Id":   strconv.Itoa(org.Id),
		},
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"organization_role":"member"`)
	require.Contains(t, recorder.Body.String(), `"organization_access_mode":"workspace"`)
	require.NotContains(t, recorder.Body.String(), `"organization_role":"platform_admin"`)
}

func TestOrganizationReadOnlyAuthAllowsDisabledOwnerViewOnly(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	user := createOrganizationPolicyMiddlewareUser(t, "org-readonly-owner", common.RoleCommonUser)
	org := createOrganizationPolicyMiddlewareOrganization(t, user, model.OrganizationStatusDisabled, model.OrganizationRoleOwner)

	recorder := runOrganizationPolicyMiddlewareRequest(t, user, http.MethodGet, "/api/organizations/"+strconv.Itoa(org.Id)+"/readonly", OrganizationReadOnlyAuth())

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"organization_access_mode":"read_only"`)
	require.Contains(t, recorder.Body.String(), service.OrganizationCapabilityViewReadOnlyOrganization)
	require.NotContains(t, recorder.Body.String(), service.OrganizationCapabilityUpdateOrganization)
}

func TestOrganizationReadOnlyAuthRejectsDissolvedOwnerAndPlatformAdmin(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	owner := createOrganizationPolicyMiddlewareUser(t, "org-dissolved-owner", common.RoleCommonUser)
	platformAdmin := createOrganizationPolicyMiddlewareUser(t, "org-dissolved-platform", common.RoleAdminUser)
	org := createOrganizationPolicyMiddlewareOrganization(t, owner, model.OrganizationStatusDissolved, model.OrganizationRoleOwner)

	ownerRecorder := runOrganizationPolicyMiddlewareRequest(t, owner, http.MethodGet, "/api/organizations/"+strconv.Itoa(org.Id)+"/readonly", OrganizationReadOnlyAuth())
	platformRecorder := runOrganizationPolicyMiddlewareRequest(t, platformAdmin, http.MethodGet, "/api/organizations/"+strconv.Itoa(org.Id)+"/readonly", OrganizationReadOnlyAuth())

	require.Equal(t, http.StatusGone, ownerRecorder.Code)
	require.Contains(t, ownerRecorder.Body.String(), string(types.ErrorCodeOrganizationDissolved))
	require.Equal(t, http.StatusGone, platformRecorder.Code)
	require.Contains(t, platformRecorder.Body.String(), string(types.ErrorCodeOrganizationDissolved))
}

func TestOrganizationReadOnlyAuthRejectsDisabledPlatformAdminNonMember(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	owner := createOrganizationPolicyMiddlewareUser(t, "org-readonly-disabled-owner", common.RoleCommonUser)
	platformAdmin := createOrganizationPolicyMiddlewareUser(t, "org-readonly-disabled-platform", common.RoleAdminUser)
	org := createOrganizationPolicyMiddlewareOrganization(t, owner, model.OrganizationStatusDisabled, model.OrganizationRoleOwner)

	recorder := runOrganizationPolicyMiddlewareRequest(t, platformAdmin, http.MethodGet, "/api/organizations/"+strconv.Itoa(org.Id)+"/readonly", OrganizationReadOnlyAuth())

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), string(types.ErrorCodeOrganizationDisabled))
}

func TestOrganizationAdminAuthRequiresPlatformAndPreservesPlatformRoleForOwner(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	ordinaryOwner := createOrganizationPolicyMiddlewareUser(t, "org-admin-ordinary-owner", common.RoleCommonUser)
	ordinaryOrg := createOrganizationPolicyMiddlewareOrganization(t, ordinaryOwner, model.OrganizationStatusActive, model.OrganizationRoleOwner)
	ordinaryRecorder := runOrganizationPolicyMiddlewareRequest(t, ordinaryOwner, http.MethodGet, "/api/organizations/"+strconv.Itoa(ordinaryOrg.Id)+"/admin", OrganizationAdminAuth())

	platformOwner := createOrganizationPolicyMiddlewareUser(t, "org-admin-platform-owner", common.RoleAdminUser)
	platformOrg := createOrganizationPolicyMiddlewareOrganization(t, platformOwner, model.OrganizationStatusActive, model.OrganizationRoleOwner)
	platformRecorder := runOrganizationPolicyMiddlewareRequest(t, platformOwner, http.MethodGet, "/api/organizations/"+strconv.Itoa(platformOrg.Id)+"/admin", OrganizationAdminAuth())

	require.Equal(t, http.StatusForbidden, ordinaryRecorder.Code)
	require.Contains(t, ordinaryRecorder.Body.String(), string(types.ErrorCodeOrganizationAccessDenied))
	require.Equal(t, http.StatusOK, platformRecorder.Code)
	require.Contains(t, platformRecorder.Body.String(), `"organization_role":"platform_admin"`)
	require.Contains(t, platformRecorder.Body.String(), `"organization_access_mode":"admin"`)
}

func TestOrganizationReadAccessAuthReturnsGoneForDissolvedOwner(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	owner := createOrganizationPolicyMiddlewareUser(t, "org-read-dissolved-owner", common.RoleCommonUser)
	org := createOrganizationPolicyMiddlewareOrganization(t, owner, model.OrganizationStatusDissolved, model.OrganizationRoleOwner)

	recorder := runOrganizationPolicyMiddlewareRequest(
		t,
		owner,
		http.MethodGet,
		"/api/organizations/"+strconv.Itoa(org.Id)+"/management",
		OrganizationReadAccessAuth(service.OrganizationCapabilityViewOrganization),
	)

	require.Equal(t, http.StatusGone, recorder.Code)
	require.Contains(t, recorder.Body.String(), string(types.ErrorCodeOrganizationDissolved))
}

func TestOrganizationReadOnlyAuthReturnsGoneWhenDissolvedPlatformDecisionLacksWriteCapability(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	owner := createOrganizationPolicyMiddlewareUser(t, "org-readonly-dissolved-cap-owner", common.RoleCommonUser)
	platformAdmin := createOrganizationPolicyMiddlewareUser(t, "org-readonly-dissolved-cap-admin", common.RoleAdminUser)
	org := createOrganizationPolicyMiddlewareOrganization(t, owner, model.OrganizationStatusDissolved, model.OrganizationRoleOwner)

	recorder := runOrganizationPolicyMiddlewareRequest(
		t,
		platformAdmin,
		http.MethodGet,
		"/api/organizations/"+strconv.Itoa(org.Id)+"/readonly",
		OrganizationReadOnlyAuth(service.OrganizationCapabilityUpdateOrganization),
	)

	require.Equal(t, http.StatusGone, recorder.Code)
	require.Contains(t, recorder.Body.String(), string(types.ErrorCodeOrganizationDissolved))
}

func TestOrganizationReadOnlyAuthReturnsDisabledForPlatformAdminNonMemberEvenWithWriteCapability(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	owner := createOrganizationPolicyMiddlewareUser(t, "org-readonly-disabled-cap-owner", common.RoleCommonUser)
	platformAdmin := createOrganizationPolicyMiddlewareUser(t, "org-readonly-disabled-cap-admin", common.RoleAdminUser)
	org := createOrganizationPolicyMiddlewareOrganization(t, owner, model.OrganizationStatusDisabled, model.OrganizationRoleOwner)

	recorder := runOrganizationPolicyMiddlewareRequest(
		t,
		platformAdmin,
		http.MethodGet,
		"/api/organizations/"+strconv.Itoa(org.Id)+"/readonly",
		OrganizationReadOnlyAuth(service.OrganizationCapabilityUpdateOrganization),
	)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), string(types.ErrorCodeOrganizationDisabled))
}

func TestOrganizationReadOnlyAuthRejectsWriteMethods(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	user := createOrganizationPolicyMiddlewareUser(t, "org-readonly-write", common.RoleCommonUser)
	org := createOrganizationPolicyMiddlewareOrganization(t, user, model.OrganizationStatusDisabled, model.OrganizationRoleOwner)

	recorder := runOrganizationPolicyMiddlewareRequest(t, user, http.MethodPost, "/api/organizations/"+strconv.Itoa(org.Id)+"/readonly", OrganizationReadOnlyAuth())

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), string(types.ErrorCodeOrganizationAccessDenied))
}

func createOrganizationPolicyMiddlewareUser(t *testing.T, username string, role int) model.User {
	t.Helper()
	user := model.User{Username: username, Password: "password", DisplayName: username, Role: role, Status: common.UserStatusEnabled, AffCode: username}
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

func createOrganizationPolicyMiddlewareOrganization(t *testing.T, owner model.User, status string, role string) model.Organization {
	t.Helper()
	org := model.Organization{Name: owner.Username + " Org", Slug: owner.Username + "-org-" + common.GetRandomString(6), Status: status, CreatedBy: owner.Id, OwnerUserId: owner.Id, Quota: 2000}
	require.NoError(t, model.DB.Create(&org).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: org.Id, UserId: owner.Id, Role: role, Status: model.OrganizationMemberStatusActive}).Error)
	return org
}

func runOrganizationPolicyMiddlewareRequest(t *testing.T, user model.User, method string, path string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	return runOrganizationPolicyMiddlewareRequestWithHeaders(t, user, method, path, handler, nil)
}

func runOrganizationPolicyMiddlewareRequestWithHeaders(t *testing.T, user model.User, method string, path string, handler gin.HandlerFunc, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", user.Id)
		c.Next()
	})
	router.Handle(method, "/api/organizations/:id/management", handler, organizationPolicyProbeHandler)
	router.Handle(method, "/api/organizations/:id/readonly", handler, organizationPolicyProbeHandler)
	router.Handle(method, "/api/organizations/:id/admin", handler, organizationPolicyProbeHandler)

	req := httptest.NewRequest(method, path, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func organizationPolicyProbeHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"organization_role":         common.GetContextKeyString(c, constant.ContextKeyOrganizationRole),
		"organization_access_mode":  common.GetContextKeyString(c, constant.ContextKeyOrganizationAccessMode),
		"organization_capabilities": common.GetContextKeyStringSlice(c, constant.ContextKeyOrganizationCapabilities),
	})
}
