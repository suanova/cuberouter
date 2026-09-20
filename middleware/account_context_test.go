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
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"

	"github.com/gin-gonic/gin"
)

func setupAccountContextMiddlewareTestDB(t *testing.T) {
	t.Helper()
	setupOrganizationMiddlewareTestDB(t,
		&model.User{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.UserAccountContext{},
		&model.OrganizationDisableRecord{},
	)
}

func createAccountContextMiddlewareUser(t *testing.T, username string) model.User {
	t.Helper()
	user := model.User{Username: username, Password: "password", DisplayName: username, Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: username}
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

func createAccountContextMiddlewareOrganization(t *testing.T, user model.User, status string, memberStatus string) model.Organization {
	t.Helper()
	org := model.Organization{Name: user.Username + " Org " + common.GetRandomString(6), Slug: user.Username + "-org-" + common.GetRandomString(6), Status: status, CreatedBy: user.Id, Quota: 2000}
	require.NoError(t, model.DB.Create(&org).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationMember{OrganizationId: org.Id, UserId: user.Id, Role: model.OrganizationRoleMember, Status: memberStatus}).Error)
	return org
}

func runAccountContextMiddlewareRequest(t *testing.T, user model.User, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", user.Id)
		c.Next()
	})
	router.GET("/api/probe", AccountContext(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"account_context_type": common.GetContextKeyString(c, constant.ContextKeyAccountContextType),
			"account_context_id":   common.GetContextKeyInt(c, constant.ContextKeyAccountContextId),
			"scope_type":           common.GetContextKeyString(c, constant.ContextKeyScopeType),
			"scope_id":             common.GetContextKeyInt(c, constant.ContextKeyScopeId),
			"organization_id":      common.GetContextKeyInt(c, constant.ContextKeyOrganizationId),
		})
	})
	router.GET("/api/organizations/:id/probe", AccountContext(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"organization_id": common.GetContextKeyInt(c, constant.ContextKeyOrganizationId)})
	})

	req := httptest.NewRequest(http.MethodGet, path, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestAccountContextMiddlewareMissingHeaderDefaultsToPersonal(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	user := createAccountContextMiddlewareUser(t, "ctx-personal")

	recorder := runAccountContextMiddlewareRequest(t, user, "/api/probe", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"account_context_type":"personal"`)
	require.Contains(t, recorder.Body.String(), `"account_context_id":`+strconv.Itoa(user.Id))
	require.Contains(t, recorder.Body.String(), `"organization_id":0`)
}

func TestAccountContextMiddlewarePersonalHeaderRequiresCurrentUserId(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	user := createAccountContextMiddlewareUser(t, "ctx-personal-mismatch")

	recorder := runAccountContextMiddlewareRequest(t, user, "/api/probe", map[string]string{
		"X-Account-Context-Type": "personal",
		"X-Account-Context-Id":   strconv.Itoa(user.Id + 1),
	})

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), string(types.ErrorCodeOrganizationAccessDenied))
}

func TestAccountContextMiddlewareOrganizationHeaderRequiresActiveMembership(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	user := createAccountContextMiddlewareUser(t, "ctx-org")
	org := createAccountContextMiddlewareOrganization(t, user, model.OrganizationStatusActive, model.OrganizationMemberStatusActive)

	recorder := runAccountContextMiddlewareRequest(t, user, "/api/probe", map[string]string{
		"X-Account-Context-Type": "organization",
		"X-Account-Context-Id":   strconv.Itoa(org.Id),
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"account_context_type":"organization"`)
	require.Contains(t, recorder.Body.String(), `"organization_id":`+strconv.Itoa(org.Id))
}

func TestAccountContextMiddlewareOrganizationHeaderRejectsInactiveMembership(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	user := createAccountContextMiddlewareUser(t, "ctx-org-disabled")
	org := createAccountContextMiddlewareOrganization(t, user, model.OrganizationStatusActive, model.OrganizationMemberStatusDisabled)

	recorder := runAccountContextMiddlewareRequest(t, user, "/api/probe", map[string]string{
		"X-Account-Context-Type": "organization",
		"X-Account-Context-Id":   strconv.Itoa(org.Id),
	})

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestAccountContextMiddlewareOrganizationHeaderRejectsDisabledOrganization(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	user := createAccountContextMiddlewareUser(t, "ctx-org-disabled-status")
	org := createAccountContextMiddlewareOrganization(t, user, model.OrganizationStatusDisabled, model.OrganizationMemberStatusActive)

	recorder := runAccountContextMiddlewareRequest(t, user, "/api/probe", map[string]string{
		"X-Account-Context-Type": "organization",
		"X-Account-Context-Id":   strconv.Itoa(org.Id),
	})

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), string(types.ErrorCodeOrganizationDisabled))
}

func TestAccountContextMiddlewareOrganizationHeaderReturnsGoneForDissolvedOrganization(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	user := createAccountContextMiddlewareUser(t, "ctx-org-dissolved-status")
	org := createAccountContextMiddlewareOrganization(t, user, model.OrganizationStatusDissolved, model.OrganizationMemberStatusActive)

	recorder := runAccountContextMiddlewareRequest(t, user, "/api/probe", map[string]string{
		"X-Account-Context-Type": "organization",
		"X-Account-Context-Id":   strconv.Itoa(org.Id),
	})

	require.Equal(t, http.StatusGone, recorder.Code)
	require.Contains(t, recorder.Body.String(), string(types.ErrorCodeOrganizationDissolved))
}

func TestAccountContextMiddlewareOrganizationPathMismatchReturnsStableCode(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	user := createAccountContextMiddlewareUser(t, "ctx-org-mismatch")
	org := createAccountContextMiddlewareOrganization(t, user, model.OrganizationStatusActive, model.OrganizationMemberStatusActive)
	other := createAccountContextMiddlewareOrganization(t, user, model.OrganizationStatusActive, model.OrganizationMemberStatusActive)

	recorder := runAccountContextMiddlewareRequest(t, user, "/api/organizations/"+strconv.Itoa(other.Id)+"/probe", map[string]string{
		"X-Account-Context-Type": "organization",
		"X-Account-Context-Id":   strconv.Itoa(org.Id),
	})

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "organization_context_mismatch")
}

func TestAccountContextMiddlewareOrganizationPathRequiresOrganizationHeader(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	user := createAccountContextMiddlewareUser(t, "ctx-org-requires-header")
	org := createAccountContextMiddlewareOrganization(t, user, model.OrganizationStatusActive, model.OrganizationMemberStatusActive)

	recorder := runAccountContextMiddlewareRequest(t, user, "/api/organizations/"+strconv.Itoa(org.Id)+"/probe", nil)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "organization_context_mismatch")
}

func TestAccountContextMiddlewareOrganizationPathRejectsPersonalHeader(t *testing.T) {
	setupAccountContextMiddlewareTestDB(t)
	user := createAccountContextMiddlewareUser(t, "ctx-org-rejects-personal")
	org := createAccountContextMiddlewareOrganization(t, user, model.OrganizationStatusActive, model.OrganizationMemberStatusActive)

	recorder := runAccountContextMiddlewareRequest(t, user, "/api/organizations/"+strconv.Itoa(org.Id)+"/probe", map[string]string{
		"X-Account-Context-Type": "personal",
		"X-Account-Context-Id":   strconv.Itoa(user.Id),
	})

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "organization_context_mismatch")
}
