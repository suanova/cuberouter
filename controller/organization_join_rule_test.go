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
package controller

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registerWithVerifiedEmail 走完整的注册处理器：注册验证码、带上它发请求。
func registerWithVerifiedEmail(t *testing.T, username, email, code string) *httptest.ResponseRecorder {
	t.Helper()
	require.NoError(t, common.RegisterVerificationCodeWithKey(email, code, common.EmailVerificationPurpose))
	engine := gin.New()
	engine.POST("/api/user/register", Register)
	body, err := common.Marshal(map[string]any{
		"username":          username,
		"password":          "Secret-password1",
		"email":             email,
		"verification_code": code,
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/user/register", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	return recorder
}

func setupRegisterJoinTestEnv(t *testing.T) {
	t.Helper()
	oldRegister, oldPwdReg := common.RegisterEnabled, common.PasswordRegisterEnabled
	oldEmailVerify := common.EmailVerificationEnabled
	oldQuota, oldGenToken := common.QuotaForNewUser, constant.GenerateDefaultToken
	common.RegisterEnabled, common.PasswordRegisterEnabled = true, true
	common.EmailVerificationEnabled = true
	common.QuotaForNewUser, constant.GenerateDefaultToken = 0, false
	t.Cleanup(func() {
		common.RegisterEnabled, common.PasswordRegisterEnabled = oldRegister, oldPwdReg
		common.EmailVerificationEnabled = oldEmailVerify
		common.QuotaForNewUser, constant.GenerateDefaultToken = oldQuota, oldGenToken
	})
}

// 只在邮箱验证分支内、且邮箱已被验证码证明过，才允许自动加入组织。
func TestRegisterAutoJoinsWhitelistedDomain(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	setupRegisterJoinTestEnv(t)

	owner := model.User{Username: "auto-join-owner", AffCode: "auto-join-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&owner).Error)
	organization := model.Organization{Name: "Auto Join Org", Slug: "auto-join-org", Status: model.OrganizationStatusActive, OwnerUserId: owner.Id, CreatedBy: owner.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationJoinRule{
		OrganizationId:    organization.Id,
		MatchType:         model.OrganizationJoinRuleMatchTypeDomain,
		Pattern:           "*.enterprise.com",
		PatternNormalized: "*.enterprise.com",
		CreatedBy:         owner.Id,
	}).Error)

	recorder := registerWithVerifiedEmail(t, "auto-join-user", "newcomer@mail.enterprise.com", "111111")
	require.Equal(t, 200, recorder.Code, recorder.Body.String())
	// 响应不泄露组织：未认证调用者不该能通过注册接口探测组织是否存在。
	assert.NotContains(t, recorder.Body.String(), organization.Name)

	var registered model.User
	require.NoError(t, model.DB.Where("username = ?", "auto-join-user").First(&registered).Error)
	var member model.OrganizationMember
	require.NoError(t, model.DB.Where("organization_id = ? AND user_id = ?", organization.Id, registered.Id).First(&member).Error)
	assert.Equal(t, model.OrganizationRoleMember, member.Role)
	assert.Equal(t, model.OrganizationMemberStatusActive, member.Status)
}

// 非白名单邮箱照常注册，但不进任何组织。
func TestRegisterLeavesUnmatchedEmailPersonal(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	setupRegisterJoinTestEnv(t)

	recorder := registerWithVerifiedEmail(t, "personal-user", "outsider@example.com", "222222")
	require.Equal(t, 200, recorder.Code, recorder.Body.String())

	var registered model.User
	require.NoError(t, model.DB.Where("username = ?", "personal-user").First(&registered).Error)
	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("user_id = ?", registered.Id).Count(&count).Error)
	assert.Zero(t, count)
}

// 命中规则但组织已停用：注册必须成功，只是不加入——一条失效规则不能把整个域的
// 注册全部卡死。
func TestRegisterSucceedsWhenMatchedOrganizationIsDisabled(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	setupRegisterJoinTestEnv(t)

	owner := model.User{Username: "disabled-org-owner", AffCode: "disabled-org-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&owner).Error)
	organization := model.Organization{Name: "Disabled Join Org", Slug: "disabled-join-org", Status: model.OrganizationStatusDisabled, OwnerUserId: owner.Id, CreatedBy: owner.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationJoinRule{
		OrganizationId:    organization.Id,
		MatchType:         model.OrganizationJoinRuleMatchTypeDomain,
		Pattern:           "*.enterprise.com",
		PatternNormalized: "*.enterprise.com",
		CreatedBy:         owner.Id,
	}).Error)

	recorder := registerWithVerifiedEmail(t, "disabled-org-user", "newcomer@enterprise.com", "333333")
	require.Equal(t, 200, recorder.Code, recorder.Body.String())

	var registered model.User
	require.NoError(t, model.DB.Where("username = ?", "disabled-org-user").First(&registered).Error)
	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("user_id = ?", registered.Id).Count(&count).Error)
	assert.Zero(t, count)
}

// 邮箱验证关闭时不存在已验证的邮箱，规则不得生效。
func TestRegisterSkipsAutoJoinWhenEmailVerificationIsOff(t *testing.T) {
	setupControllerOrganizationTestDB(t)
	setupRegisterJoinTestEnv(t)
	common.EmailVerificationEnabled = false

	owner := model.User{Username: "no-verify-owner", AffCode: "no-verify-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&owner).Error)
	organization := model.Organization{Name: "No Verify Org", Slug: "no-verify-org", Status: model.OrganizationStatusActive, OwnerUserId: owner.Id, CreatedBy: owner.Id}
	require.NoError(t, model.DB.Create(&organization).Error)
	require.NoError(t, model.DB.Create(&model.OrganizationJoinRule{
		OrganizationId:    organization.Id,
		MatchType:         model.OrganizationJoinRuleMatchTypeDomain,
		Pattern:           "*.enterprise.com",
		PatternNormalized: "*.enterprise.com",
		CreatedBy:         owner.Id,
	}).Error)

	// 邮箱验证关闭时注册不需要验证码。
	engine := gin.New()
	engine.POST("/api/user/register", Register)
	body, err := common.Marshal(map[string]any{
		"username": "no-verify-user",
		"password": "Secret-password1",
		"email":    "newcomer@enterprise.com",
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/user/register", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	require.Equal(t, 200, recorder.Code, recorder.Body.String())

	var registered model.User
	require.NoError(t, model.DB.Where("username = ?", "no-verify-user").First(&registered).Error)
	assert.Equal(t, "", registered.Email, "关闭邮箱验证时邮箱不入库")
	var count int64
	require.NoError(t, model.DB.Model(&model.OrganizationMember{}).Where("user_id = ?", registered.Id).Count(&count).Error)
	assert.Zero(t, count)
}
